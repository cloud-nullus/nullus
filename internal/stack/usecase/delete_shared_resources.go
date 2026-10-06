package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// 클러스터 공용 자원 회수.
//
// cert-manager·내부 CA·metrics-server·Gateway API(+Envoy Gateway) CRD 는 클러스터에 하나만 두고 스택이
// 함께 쓴다. 예전에는 회수하는 코드가 없어 스택을 지워도 전부 남았다 — cert-manager 는 스택 네임스페이스
// 밖(cert-manager)에, metrics-server 는 kube-system 에 깔리는데 삭제는 그 자리를 보지 않았고, Gateway
// CRD 는 설치기가 만든 GatewayClass envoy 를 "누가 쓰는 중" 으로 보아 영영 지우지 않았다. 남은 것은
// 나중에 사용자가 같은 것을 설치할 때 Helm ownership 충돌로 막고, 내부 CA 개인키(10년 유효)는 그대로
// 클러스터에 남는다.
//
// 클러스터의 마지막 스택을 지울 때 회수한다. 원칙은 하나다 — Nullus 가 설치한 것은 남기지 않고,
// Nullus 가 설치하지 않은 것은 건드리지 않는다.
//   - 누가 설치했는지: 설치는 클러스터에 있던 cert-manager·metrics-server 를 재사용하므로 릴리스 values
//     의 설치 표시(없으면 옛 설치 지문)로 가린다(domain.IsNullusInstalledRelease). Gateway CRD 는 설치
//     표시 라벨로 가린다.
//   - 쓰는 곳이 있는지: 클러스터 어디든 그 종류의 리소스가 남아 있으면 사용자가 쓰는 것이다 — CRD 를
//     지우면 클러스터 전체의 그 리소스가 함께 사라진다. 스택 네임스페이스도 예외가 아니다. 스택이 자기
//     네임스페이스를 회수했으면 그것이 다 사라지기를 기다린 뒤 보고, 회수하지 않는 자리(default·플랫폼·
//     사용자가 고른 이름)면 거기 남은 것도 사용자 것으로 본다.
//   - 확인하지 못하면 남긴다. 남길 때는 무엇이 쓰는지(왜 남기는지) 삭제 로그에 적는다.

// deleteStepShared 는 클러스터 공용 자원을 회수하는 구간이다.
const deleteStepShared = "deleting_shared"

const (
	defaultSharedWaitTimeout  = 5 * time.Minute
	defaultSharedPollInterval = 5 * time.Second
)

// internalCAIssuers 는 내부 CA 의 발급자다. bootstrap 은 평범한 selfSigned 발급자라 사용자가 가져다
// 쓰기 쉽다 — 둘 다 쓰는 곳으로 센다.
var internalCAIssuers = map[string]bool{
	domain.InternalCAIssuerName:          true,
	domain.InternalCABootstrapIssuerName: true,
}

// certManagerDerivedOwnerKinds 는 cert-manager 가 다른 리소스를 위해 만든 리소스의 주인이다. 주인이 이
// 종류면 쓰는 곳으로 세지 않는다 — 주인이 따로 세어지거나(사용자 Certificate) 방금 지운 것이다(내부 CA).
// 주인 없이 만든 CertificateRequest(istio-csr)나 파드가 주인인 것(csi-driver)은 센다.
var certManagerDerivedOwnerKinds = map[string]bool{
	"Certificate":        true,
	"CertificateRequest": true,
	"Order":              true,
}

// cert-manager 의 리더 선출 Lease 는 차트 기본값대로 kube-system 에 생기고 릴리스가 소유하지 않는다.
var certManagerLeaderElectionLeases = []string{"cert-manager-controller", "cert-manager-cainjector-leader-election"}

const certManagerLeaderElectionNamespace = "kube-system"

// releaseNamespaceKeep 는 릴리스를 지운 네임스페이스에 남아도 그 네임스페이스를 지워도 되는 것이다.
// 쿠버네티스가 네임스페이스마다 만드는 것과 cert-manager 가 릴리스 밖에서 스스로 만드는 것이다.
var releaseNamespaceKeep = map[string]bool{
	"configmap/kube-root-ca.crt":     true,
	"serviceaccount/default":         true,
	"secret/cert-manager-webhook-ca": true,
}

// releaseNamespaceTransientKinds 는 지운 워크로드가 빠지는 동안 잠깐 남는 것이다. 사라지기를 기다린다.
var releaseNamespaceTransientKinds = []string{
	"pod/",
	"replicaset.apps/",
	"controllerrevision.apps/",
	"endpoints/",
	"endpointslice.discovery.k8s.io/",
}

// namespaceInventoryExcluded 는 네임스페이스에 무엇이 남았는지 셀 때 보지 않는 종류다.
var namespaceInventoryExcluded = map[string]bool{
	"events":               true,
	"events.events.k8s.io": true,
	"pods.metrics.k8s.io":  true,
}

// envoyGatewayControllerSelector 는 Envoy Gateway 컨트롤러 Deployment 의 라벨이다. 스택의 것은 스택
// 정리가 지웠으므로 남아 있으면 다른 곳의 Envoy Gateway 다 — 그것은 GatewayClass envoy 와 Envoy CRD 를
// 쓸 수 있다.
const envoyGatewayControllerSelector = "control-plane=envoy-gateway"

// legacyGatewayCRDWindow 는 표시를 달기 전의 설치에서 Gateway CRD 를 Nullus 것으로 볼 시간 폭이다. 설치기는
// CRD 를 깐 직후(kind-nullus-platform 실측 26~31초 뒤) GatewayClass envoy 를 만들었다. 그보다 한참 먼저
// 있던 CRD 는 고객이 깐 것이다.
const legacyGatewayCRDWindow = 15 * time.Minute

const kubectlNone = "<none>"

// sharedReclaim 은 한 번의 회수다. 남긴 것과 실패한 것을 세어 끝에 한 줄로 알린다.
type sharedReclaim struct {
	uc         *DeleteStack
	kubeconfig []byte
	stack      *domain.Stack
	stackID    string
	crds       map[string]crdRow
	// stopped 는 다른 스택이 보여 회수를 멈췄다는 뜻이다. 이유는 한 번만 적는다.
	stopped   bool
	reclaimed int
	kept      int
	failed    int
}

// reclaimSharedClusterResources 는 클러스터의 마지막 스택을 지울 때 Nullus 가 설치한 공용 자원을 회수한다.
// stackNamespaceDeleted 는 스택 정리가 스택 네임스페이스를 지우기로 했는지다.
//
// 순서가 있다. 스택 네임스페이스가 다 빠진 뒤에 본다 — 그 안의 리소스가 finalizer 를 풀려면 컨트롤러가
// 살아 있어야 한다. 내부 CA 는 cert-manager 가 살아 있을 때 지운다. cert-manager 는 CRD 보다 먼저
// 지운다. GatewayClass 는 Gateway API CRD 보다 먼저 지운다. 단계마다 다른 스택이 생기지 않았는지 다시
// 본다 — 회수는 몇 분 걸리고, 그 사이 새 스택이 이 자원을 재사용할 수 있다.
func (uc *DeleteStack) reclaimSharedClusterResources(ctx context.Context, kubeconfig []byte, stack *domain.Stack, stackID string, stackNamespaceDeleted bool) {
	if len(kubeconfig) == 0 || stack == nil {
		return
	}
	r := &sharedReclaim{uc: uc, kubeconfig: kubeconfig, stack: stack, stackID: stackID}
	uc.emit(ctx, stackID, deleteStepShared, "info", "클러스터 공용 자원을 확인합니다")
	defer r.summarize(ctx)

	if !r.lastStack(ctx) {
		return
	}
	if stackNamespaceDeleted && !r.waitStackNamespaceGone(ctx) {
		return
	}
	if !r.loadCRDs(ctx) {
		return
	}
	for _, step := range []func(context.Context){
		r.reclaimInternalCA,
		r.reclaimCertManager,
		r.reclaimMetricsServer,
		r.reclaimGateway,
	} {
		if !r.lastStack(ctx) {
			return
		}
		step(ctx)
	}
}

// lastStack 은 같은 클러스터에 다른 스택이 없는지 본다. 있거나 확인하지 못하면 이유를 적고 false 다.
// 지우는 스택의 레코드는 정리 전에 이미 지워져 목록에 없다.
func (r *sharedReclaim) lastStack(ctx context.Context) bool {
	if r.stopped {
		return false
	}
	r.stopped = true
	if r.uc.stackRepo == nil {
		r.keep(ctx, "이 클러스터의 다른 스택을 확인할 수 없어 클러스터 공용 자원을 남깁니다")
		return false
	}
	stacks, err := r.uc.stackRepo.ListByCluster(ctx, r.stack.ClusterID)
	if err != nil {
		r.keep(ctx, fmt.Sprintf("이 클러스터의 다른 스택을 확인하지 못해 클러스터 공용 자원을 남깁니다: %v", err))
		return false
	}
	var names []string
	for _, other := range stacks {
		if other != nil && other.ID != r.stack.ID {
			names = append(names, other.Name)
		}
	}
	if len(names) == 0 {
		r.stopped = false
		return true
	}
	r.keep(ctx, fmt.Sprintf("이 클러스터에 다른 스택이 있어 클러스터 공용 자원(cert-manager·내부 CA·metrics-server·Gateway API)을 남깁니다: %s",
		summarizeRefs(names)))
	return false
}

// waitStackNamespaceGone 은 스택 네임스페이스가 다 지워지기를 기다린다. 남은 것이 있는 채로 컨트롤러를
// 지우면 그 finalizer 를 풀 주체가 사라져 네임스페이스와 CRD 가 영구 Terminating 에 갇힌다.
func (r *sharedReclaim) waitStackNamespaceGone(ctx context.Context) bool {
	namespace := strings.TrimSpace(r.stack.Namespace)
	ok, err := r.poll(ctx, func() (bool, error) {
		out, err := r.read(ctx, "get", "namespace", namespace, "--ignore-not-found", "-o", "name")
		return strings.TrimSpace(out) == "", err
	})
	switch {
	case err != nil:
		r.keep(ctx, fmt.Sprintf("스택 네임스페이스 %s 가 지워졌는지 확인하지 못해 클러스터 공용 자원을 남깁니다: %v", namespace, err))
	case !ok:
		r.keep(ctx, fmt.Sprintf("스택 네임스페이스 %s 가 아직 지워지는 중이라 클러스터 공용 자원을 남깁니다 — "+
			"네임스페이스가 사라진 뒤 남은 cert-manager 등은 직접 지워야 합니다", namespace))
	}
	return ok && err == nil
}

// crdRow 는 CRD 한 건과 그 소유 표시다.
type crdRow struct {
	name             string
	by               string // LabelInstalledBy 라벨
	release          string // meta.helm.sh/release-name
	releaseNamespace string // meta.helm.sh/release-namespace
	created          time.Time
}

func (r *sharedReclaim) loadCRDs(ctx context.Context) bool {
	out, err := r.read(ctx, "get", "crd", "--no-headers", "-o",
		`custom-columns=NAME:.metadata.name,BY:.metadata.labels.nullus\.io/installed-by,`+
			`RELEASE:.metadata.annotations.meta\.helm\.sh/release-name,RELEASE_NS:.metadata.annotations.meta\.helm\.sh/release-namespace,`+
			`CREATED:.metadata.creationTimestamp`)
	if err != nil {
		r.keep(ctx, fmt.Sprintf("CRD 목록을 읽지 못해 클러스터 공용 자원을 남깁니다: %v", err))
		return false
	}
	r.crds = map[string]crdRow{}
	for _, fields := range kubectlRows(out, 5) {
		created, _ := time.Parse(time.RFC3339, fields[4])
		r.crds[fields[0]] = crdRow{name: fields[0], by: fields[1], release: fields[2], releaseNamespace: fields[3], created: created}
	}
	return true
}

func (r *sharedReclaim) hasCRD(name string) bool {
	_, ok := r.crds[name]
	return ok
}

// reclaimInternalCA 는 설치가 만든 내부 CA(발급자 둘·CA 인증서·개인키 Secret)를 지운다.
//
// 이름이 Nullus 전용이라 표시 없이도 Nullus 것이다. 고객의 cert-manager 를 재사용한 클러스터에서도
// 지운다. 이 발급자로 인증서를 받는 곳이 있으면 남긴다 — 지우면 그 인증서가 갱신되지 않는다.
func (r *sharedReclaim) reclaimInternalCA(ctx context.Context) {
	var caCertNamespaces, users []string
	if r.hasCRD("certificates.cert-manager.io") {
		out, err := r.read(ctx, "get", "certificates.cert-manager.io", "-A", "--no-headers",
			"-o", "custom-columns=NS:.metadata.namespace,NAME:.metadata.name,ISSUER:.spec.issuerRef.name")
		if err != nil {
			r.keep(ctx, fmt.Sprintf("인증서 목록을 읽지 못해 내부 CA 를 남깁니다: %v", err))
			return
		}
		for _, fields := range kubectlRows(out, 3) {
			namespace, name, issuer := fields[0], fields[1], fields[2]
			switch {
			case name == domain.InternalCACertName:
				caCertNamespaces = append(caCertNamespaces, namespace)
			case internalCAIssuers[issuer]:
				users = append(users, namespace+"/"+name)
			}
		}
	}
	if r.hasCRD("certificaterequests.cert-manager.io") {
		out, err := r.read(ctx, "get", "certificaterequests.cert-manager.io", "-A", "--no-headers",
			"-o", "custom-columns=NS:.metadata.namespace,NAME:.metadata.name,ISSUER:.spec.issuerRef.name,OWNER:.metadata.ownerReferences[0].kind")
		if err != nil {
			r.keep(ctx, fmt.Sprintf("인증서 요청 목록을 읽지 못해 내부 CA 를 남깁니다: %v", err))
			return
		}
		// 주인이 cert-manager 리소스가 아닌 요청은 쓰는 곳이다 — 주인 없이 만든 요청(istio-csr)과 파드가
		// 주인인 요청(csi-driver)이 그렇다.
		for _, fields := range kubectlRows(out, 4) {
			if internalCAIssuers[fields[2]] && !certManagerDerivedOwnerKinds[fields[3]] {
				users = append(users, fields[0]+"/certificaterequests/"+fields[1])
			}
		}
	}
	if len(users) > 0 {
		r.keep(ctx, fmt.Sprintf("내부 CA 발급자로 인증서를 받는 곳이 있어 내부 CA 를 남깁니다: %s", summarizeRefs(users)))
		return
	}

	var issuers []string
	if r.hasCRD("clusterissuers.cert-manager.io") {
		out, err := r.read(ctx, "get", "clusterissuers.cert-manager.io", "-o", "name")
		if err != nil {
			r.keep(ctx, fmt.Sprintf("발급자 목록을 읽지 못해 내부 CA 를 남깁니다: %v", err))
			return
		}
		present := map[string]bool{}
		for _, name := range parseResourceNames(out) {
			present[name] = true
		}
		for _, name := range []string{domain.InternalCAIssuerName, domain.InternalCABootstrapIssuerName} {
			if present[name] {
				issuers = append(issuers, name)
			}
		}
	}
	// 개인키는 cert-manager 가 없어도 남아 있을 수 있다(지난 회수가 CRD 만 지웠다).
	secretsOut, err := r.read(ctx, "get", "secret", "-A", "--field-selector", "metadata.name="+domain.InternalCASecretName,
		"--no-headers", "-o", "custom-columns=NS:.metadata.namespace")
	if err != nil {
		r.keep(ctx, fmt.Sprintf("내부 CA 개인키 위치를 읽지 못해 내부 CA 를 남깁니다: %v", err))
		return
	}
	secretNamespaces := uniqueNamespaces(secretsOut)
	if len(issuers) == 0 && len(caCertNamespaces) == 0 && len(secretNamespaces) == 0 {
		return
	}

	r.uc.emit(ctx, r.stackID, deleteStepShared, "info", "내부 CA(발급자·CA 인증서·개인키)를 지웁니다")
	ok := true
	// 발급자부터 지운다. CA 인증서가 먼저 사라지면 발급자가 키를 잃고 오류 상태로 남는다.
	if len(issuers) > 0 {
		args := append([]string{"delete", "clusterissuers.cert-manager.io"}, issuers...)
		ok = r.delete(ctx, "내부 CA 발급자", append(args, "--ignore-not-found")...) && ok
	}
	for _, namespace := range caCertNamespaces {
		ok = r.delete(ctx, "내부 CA 인증서",
			"delete", "certificates.cert-manager.io", domain.InternalCACertName, "-n", namespace, "--ignore-not-found") && ok
	}
	// cert-manager 는 Certificate 를 지워도 Secret 을 남긴다 — 개인키이므로 직접 지운다.
	for _, namespace := range secretNamespaces {
		ok = r.delete(ctx, "내부 CA 개인키",
			"delete", "secret", domain.InternalCASecretName, "-n", namespace, "--ignore-not-found") && ok
	}
	if ok {
		r.reclaimed++
	}
}

// reclaimCertManager 는 Nullus 가 설치한 cert-manager(릴리스·CRD·네임스페이스)를 지운다.
//
// cert-manager 는 한 클러스터에 하나라 CRD 를 함께 쓴다. 릴리스가 하나라도 Nullus 것이 아니면 전부
// 남긴다. 쓰는 곳은 셋을 본다 — cert-manager 리소스, cert-manager 에 기대는 다른 구성요소의 CRD
// (trust-manager 등), cainjector 로 CA 를 주입받는 웹훅·APIService·CRD.
func (r *sharedReclaim) reclaimCertManager(ctx context.Context) {
	release := domain.CertManagerReleaseName
	namespaces, ok := r.nullusReleaseNamespaces(ctx, release, "cert-manager")
	if !ok || len(namespaces) == 0 {
		return
	}
	releaseNamespaces := map[string]bool{}
	for _, namespace := range namespaces {
		releaseNamespaces[namespace] = true
	}
	// 차트가 CRD 를 keep 정책으로 깔아 릴리스를 지워도 남는다. 그 릴리스가 소유한 CRD 만 지운다.
	var owned, foreign []string
	for _, crd := range r.sortedCRDs() {
		if !strings.HasSuffix(crd.name, domain.CertManagerCRDSuffix) {
			continue
		}
		if crd.release == release && releaseNamespaces[crd.releaseNamespace] {
			owned = append(owned, crd.name)
		} else {
			foreign = append(foreign, crd.name)
		}
	}
	if len(foreign) > 0 {
		r.keep(ctx, fmt.Sprintf("cert-manager 에 기대는 다른 구성요소의 CRD 가 있어 cert-manager 를 남깁니다: %s", summarizeRefs(foreign)))
		return
	}
	// 내부 CA 는 앞에서 지웠다. 남았다면(쓰는 곳이 있거나 지우지 못했다) 그 이유를 이미 적었다.
	users := r.crdUsers(ctx, owned, func(resource, _, name, ownerKind string) bool {
		if certManagerDerivedOwnerKinds[ownerKind] {
			return true
		}
		switch resource {
		case "clusterissuers":
			return internalCAIssuers[name]
		case "certificates":
			return name == domain.InternalCACertName
		}
		return false
	})
	if len(users) > 0 {
		r.keep(ctx, fmt.Sprintf("cert-manager 를 쓰는 리소스가 남아 있어 cert-manager 를 남깁니다: %s", summarizeRefs(users)))
		return
	}
	injected, err := r.caInjectionUsers(ctx)
	if err != nil {
		r.keep(ctx, fmt.Sprintf("cert-manager 로 CA 를 주입받는 곳을 확인하지 못해 cert-manager 를 남깁니다: %v", err))
		return
	}
	if len(injected) > 0 {
		r.keep(ctx, fmt.Sprintf("cert-manager 로 CA 를 주입받는 곳이 있어 cert-manager 를 남깁니다: %s", summarizeRefs(injected)))
		return
	}

	if !r.uninstall(ctx, release, namespaces, "cert-manager") {
		return
	}
	if len(owned) > 0 {
		args := append([]string{"delete", "crd"}, owned...)
		if !r.delete(ctx, "cert-manager CRD", append(args, "--ignore-not-found", "--timeout=120s")...) {
			return
		}
	}
	for _, namespace := range namespaces {
		r.deleteReleaseNamespace(ctx, namespace, "cert-manager")
	}
	leaseArgs := append([]string{"delete", "lease"}, certManagerLeaderElectionLeases...)
	r.delete(ctx, "cert-manager 리더 선출 Lease",
		append(leaseArgs, "-n", certManagerLeaderElectionNamespace, "--ignore-not-found")...)
	r.reclaimed++
}

// caInjectionUsers 는 cainjector 로 CA 를 주입받는 웹훅·APIService·CRD 중 cert-manager 자신의 것이 아닌 것이다.
func (r *sharedReclaim) caInjectionUsers(ctx context.Context) ([]string, error) {
	out, err := r.read(ctx, "get", "validatingwebhookconfigurations,mutatingwebhookconfigurations,apiservices,crd", "--no-headers", "-o",
		`custom-columns=KIND:.kind,NAME:.metadata.name,FROM:.metadata.annotations.cert-manager\.io/inject-ca-from,`+
			`FROM_SECRET:.metadata.annotations.cert-manager\.io/inject-ca-from-secret,`+
			`APISERVER_CA:.metadata.annotations.cert-manager\.io/inject-apiserver-ca,RELEASE:.metadata.annotations.meta\.helm\.sh/release-name`)
	if err != nil {
		return nil, err
	}
	var users []string
	for _, fields := range kubectlRows(out, 6) {
		kind, name, release := fields[0], fields[1], fields[5]
		injected := fields[2] != kubectlNone || fields[3] != kubectlNone || fields[4] != kubectlNone
		if injected && release != domain.CertManagerReleaseName {
			users = append(users, strings.ToLower(kind)+"/"+name)
		}
	}
	return users, nil
}

// reclaimMetricsServer 는 Nullus 가 설치한 metrics-server 를 지운다.
//
// kube-system 에 깔리므로 스택 정리가 보지 않는다. 릴리스를 지우면 APIService 도 함께 사라진다 — APIService
// 만 남으면 API discovery 가 실패해 클러스터의 네임스페이스 삭제가 교착된다. 사용자 HPA·VPA 가 있으면
// 오토스케일이 멈추므로 남긴다. `kubectl top` 같은 직접 조회는 알아낼 수 없다.
func (r *sharedReclaim) reclaimMetricsServer(ctx context.Context) {
	release := domain.MetricsServerReleaseName
	namespaces, ok := r.nullusReleaseNamespaces(ctx, release, "metrics-server")
	if !ok || len(namespaces) == 0 {
		return
	}
	kinds := []string{"horizontalpodautoscalers"}
	if r.hasCRD("verticalpodautoscalers.autoscaling.k8s.io") {
		kinds = append(kinds, "verticalpodautoscalers.autoscaling.k8s.io")
	}
	var users []string
	for _, kind := range kinds {
		out, err := r.read(ctx, "get", kind, "-A", "--no-headers", "-o", "custom-columns=NS:.metadata.namespace,NAME:.metadata.name")
		if err != nil {
			r.keep(ctx, fmt.Sprintf("%s 목록을 읽지 못해 metrics-server 를 남깁니다: %v", kind, err))
			return
		}
		for _, fields := range kubectlRows(out, 2) {
			users = append(users, fields[0]+"/"+fields[1])
		}
	}
	if len(users) > 0 {
		r.keep(ctx, fmt.Sprintf("오토스케일러가 metrics-server 의 메트릭을 쓰고 있어 metrics-server 를 남깁니다: %s", summarizeRefs(users)))
		return
	}
	if r.uninstall(ctx, release, namespaces, "metrics-server") {
		r.reclaimed++
	}
}

// nullusReleaseNamespaces 는 그 이름의 공용 릴리스가 있는 네임스페이스다. 하나라도 Nullus 것이 아니거나
// 확인하지 못하면 이유를 적고 ok=false 다.
func (r *sharedReclaim) nullusReleaseNamespaces(ctx context.Context, release, title string) ([]string, bool) {
	out, err := r.read(ctx, "get", "secret", "-A", "-l", "owner=helm,name="+release,
		"--no-headers", "-o", "custom-columns=NS:.metadata.namespace")
	if err != nil {
		r.keep(ctx, fmt.Sprintf("%s 설치 위치를 확인하지 못해 남깁니다: %v", title, err))
		return nil, false
	}
	namespaces := uniqueNamespaces(out)
	if len(namespaces) == 0 {
		return nil, true
	}
	if r.uc.releaseManagerFactory == nil {
		r.keep(ctx, fmt.Sprintf("릴리스 values 를 읽을 수 없어 %s 를 남깁니다", title))
		return nil, false
	}
	manager := r.uc.releaseManagerFactory(r.kubeconfig)
	if manager == nil {
		r.keep(ctx, fmt.Sprintf("릴리스 values 를 읽을 수 없어 %s 를 남깁니다", title))
		return nil, false
	}
	for _, namespace := range namespaces {
		values, err := manager.GetValues(ctx, release, namespace)
		if err != nil {
			r.keep(ctx, fmt.Sprintf("%s(네임스페이스 %s)의 values 를 읽지 못해 남깁니다: %v", title, namespace, err))
			return nil, false
		}
		if !domain.IsNullusInstalledRelease(release, values) {
			r.keep(ctx, fmt.Sprintf("%s(네임스페이스 %s)는 Nullus 가 설치하지 않은 %s 라 남깁니다 — 설치가 클러스터에 있던 것을 재사용했습니다",
				title, namespace, title))
			return nil, false
		}
	}
	return namespaces, true
}

// uninstall 은 공용 릴리스를 지운다. 이미 없으면 지운 것으로 본다.
func (r *sharedReclaim) uninstall(ctx context.Context, release string, namespaces []string, title string) bool {
	if !r.lastStack(ctx) {
		return false
	}
	installer := r.installer()
	if installer == nil {
		r.keep(ctx, fmt.Sprintf("Helm 실행기가 없어 %s 를 남깁니다", title))
		return false
	}
	for _, namespace := range namespaces {
		r.uc.emit(ctx, r.stackID, deleteStepShared, "info", fmt.Sprintf("%s(%s)를 지웁니다", title, namespace))
		if err := installer.Uninstall(ctx, release, namespace); err != nil && !isReleaseNotFoundError(err) {
			r.fail(ctx, fmt.Sprintf("%s 릴리스(%s)를 지우지 못했습니다: %v", title, namespace, err))
			return false
		}
	}
	return true
}

// deleteReleaseNamespace 는 공용 릴리스를 지운 자리를 회수한다. 지켜야 할 자리이거나, 릴리스 몫이 아닌
// 것이 남아 있으면 네임스페이스는 남긴다 — 고객이 미리 만든 네임스페이스일 수 있다. 무엇이 남았는지는
// 나열 가능한 모든 종류로 센다. 지운 워크로드의 파드가 빠지는 동안은 기다린다.
func (r *sharedReclaim) deleteReleaseNamespace(ctx context.Context, namespace, title string) {
	if isProtectedNamespace(namespace, r.uc.platformNamespace) {
		return
	}
	// 망가진 aggregated API(KEDA 의 external.metrics 등)가 하나라도 있으면 api-resources 는 나머지 목록을
	// 내놓고도 실패로 끝난다. 그 목록으로 센다 — 그 그룹의 종류는 빠지지만 실제 리소스가 아니다. 목록
	// 자체를 못 받았으면 남긴다.
	typesOut, err := r.read(ctx, "api-resources", "--verbs=list", "--namespaced", "-o", "name")
	if err != nil && (!isPartialDiscoveryError(err) || len(kubectlRefs(typesOut)) == 0) {
		r.keep(ctx, fmt.Sprintf("네임스페이스 %s 에 남은 것을 확인하지 못해 네임스페이스는 남깁니다: %v", namespace, err))
		return
	}
	var kinds []string
	for _, kind := range kubectlRefs(typesOut) {
		if !namespaceInventoryExcluded[kind] {
			kinds = append(kinds, kind)
		}
	}
	var others, transient []string
	ok, err := r.poll(ctx, func() (bool, error) {
		out, err := r.read(ctx, "get", strings.Join(kinds, ","), "-n", namespace, "-o", "name", "--ignore-not-found")
		if err != nil {
			return false, err
		}
		others, transient = nil, nil
		for _, ref := range kubectlRefs(out) {
			switch {
			case releaseNamespaceKeep[ref]:
			case hasAnyPrefix(ref, releaseNamespaceTransientKinds):
				transient = append(transient, ref)
			default:
				others = append(others, ref)
			}
		}
		return len(others) > 0 || len(transient) == 0, nil
	})
	switch {
	case err != nil:
		r.keep(ctx, fmt.Sprintf("네임스페이스 %s 에 남은 것을 확인하지 못해 네임스페이스는 남깁니다: %v", namespace, err))
	case len(others) > 0:
		r.keep(ctx, fmt.Sprintf("네임스페이스 %s 에 %s 몫이 아닌 리소스가 있어 네임스페이스는 남깁니다: %s",
			namespace, title, summarizeRefs(others)))
	case !ok:
		r.keep(ctx, fmt.Sprintf("네임스페이스 %s 의 파드가 아직 빠지지 않아 네임스페이스는 남깁니다: %s", namespace, summarizeRefs(transient)))
	default:
		r.delete(ctx, "네임스페이스 "+namespace, "delete", "namespace", namespace, "--ignore-not-found", "--wait=false")
	}
}

// gatewayClass 는 GatewayClass 한 건에서 회수 판단에 쓰는 것이다.
type gatewayClass struct {
	Metadata struct {
		Name              string            `json:"name"`
		Labels            map[string]string `json:"labels"`
		Annotations       map[string]string `json:"annotations"`
		CreationTimestamp time.Time         `json:"creationTimestamp"`
	} `json:"metadata"`
	Spec struct {
		ControllerName string `json:"controllerName"`
		ParametersRef  any    `json:"parametersRef"`
	} `json:"spec"`
}

// reclaimGateway 는 설치기가 만든 GatewayClass 와 Nullus 가 설치한 Gateway API·Envoy Gateway CRD 를 지운다.
//
// CRD 는 설치 표시가 있는 것만 지운다. 표시를 달기 전의 설치는 GatewayClass envoy 가 옛 설치기가 apply 한
// 모양 그대로 남아 있는 것으로 알아보고, 그때는 Helm 이 소유하지 않으면서 그 GatewayClass 와 같은 무렵에
// 생긴 Gateway CRD 만 Nullus 것으로 본다. 다른 곳에서 Envoy Gateway 컨트롤러가 돌거나, 다른
// GatewayClass(Istio 등)·Gateway 가 있으면 누군가 Gateway API 를 쓰는 것이라 남긴다.
func (r *sharedReclaim) reclaimGateway(ctx context.Context) {
	var own *gatewayClass
	legacy := false
	if r.hasCRD("gatewayclasses.gateway.networking.k8s.io") {
		out, err := r.read(ctx, "get", "gatewayclasses.gateway.networking.k8s.io", "-o", "json")
		if err != nil {
			r.keep(ctx, fmt.Sprintf("GatewayClass 목록을 읽지 못해 Gateway API 를 남깁니다: %v", err))
			return
		}
		var list struct {
			Items []gatewayClass `json:"items"`
		}
		if err := json.Unmarshal([]byte(out), &list); err != nil {
			r.keep(ctx, fmt.Sprintf("GatewayClass 목록을 읽지 못해 Gateway API 를 남깁니다: %v", err))
			return
		}
		for i := range list.Items {
			class := &list.Items[i]
			switch {
			case class.Metadata.Name != domain.EnvoyGatewayClassName || class.Spec.ControllerName != domain.EnvoyGatewayControllerName:
			case class.Metadata.Labels[domain.LabelInstalledBy] == domain.InstalledByNullus:
				own = class
			case isLegacyEnvoyGatewayClass(class):
				own, legacy = class, true
			}
		}
	}
	var candidates, foreign []string
	for _, crd := range r.sortedCRDs() {
		if !domain.IsGatewayCRD(crd.name) {
			continue
		}
		switch {
		case crd.by == domain.InstalledByNullus:
			candidates = append(candidates, crd.name)
		case legacy && crd.by == kubectlNone && crd.release == kubectlNone && !crd.created.IsZero() &&
			absDuration(own.Metadata.CreationTimestamp.Sub(crd.created)) <= legacyGatewayCRDWindow:
			candidates = append(candidates, crd.name)
		default:
			foreign = append(foreign, crd.name)
		}
	}
	if own == nil && len(candidates) == 0 {
		return
	}
	controllers, err := r.read(ctx, "get", "deployments", "-A", "-l", envoyGatewayControllerSelector, "--no-headers",
		"-o", "custom-columns=NS:.metadata.namespace,NAME:.metadata.name")
	if err != nil {
		r.keep(ctx, fmt.Sprintf("Envoy Gateway 컨트롤러를 확인하지 못해 Gateway API 를 남깁니다: %v", err))
		return
	}
	if rows := kubectlRows(controllers, 2); len(rows) > 0 {
		var refs []string
		for _, fields := range rows {
			refs = append(refs, fields[0]+"/"+fields[1])
		}
		r.keep(ctx, fmt.Sprintf("다른 곳에서 Envoy Gateway 컨트롤러가 돌고 있어 GatewayClass %s 와 Gateway API CRD 를 남깁니다: %s",
			domain.EnvoyGatewayClassName, summarizeRefs(refs)))
		return
	}

	classDeleted := false
	if own != nil {
		var ok bool
		if classDeleted, ok = r.reclaimEnvoyGatewayClass(ctx); !ok {
			return
		}
	}
	if len(candidates) == 0 {
		return
	}
	if len(foreign) > 0 {
		r.uc.emit(ctx, r.stackID, deleteStepShared, "info",
			fmt.Sprintf("Nullus 가 설치하지 않은 Gateway API CRD 는 남깁니다: %s", summarizeRefs(foreign)))
	}
	users := r.crdUsers(ctx, candidates, func(resource, _, name, _ string) bool {
		return classDeleted && resource == "gatewayclasses" && name == domain.EnvoyGatewayClassName
	})
	if len(users) > 0 {
		r.keep(ctx, fmt.Sprintf("Gateway API 를 쓰는 리소스가 남아 있어 Gateway API·Envoy Gateway CRD 를 남깁니다: %s",
			summarizeRefs(users)))
		return
	}
	r.uc.emit(ctx, r.stackID, deleteStepShared, "info",
		fmt.Sprintf("Gateway API·Envoy Gateway CRD %d개를 지웁니다", len(candidates)))
	args := append([]string{"delete", "crd"}, candidates...)
	if r.delete(ctx, "Gateway API·Envoy Gateway CRD", append(args, "--ignore-not-found", "--timeout=120s")...) {
		r.reclaimed++
	}
}

// legacyEnvoyGatewayClassManifest 는 표시를 달기 전의 설치기가 kubectl apply 로 만든 GatewayClass 다.
// apply 는 그 내용을 last-applied-configuration 에 남긴다.
const legacyEnvoyGatewayClassManifest = `{"apiVersion":"gateway.networking.k8s.io/v1","kind":"GatewayClass",` +
	`"metadata":{"annotations":{},"name":"envoy"},"spec":{"controllerName":"gateway.envoyproxy.io/gatewayclass-controller"}}`

// isLegacyEnvoyGatewayClass 는 표시 없는 GatewayClass envoy 가 옛 설치기가 만든 것인지 본다. 이름과
// 컨트롤러만 같은 고객 것과 가르려고, 옛 설치기가 apply 한 내용과 그대로 같은지까지 본다.
func isLegacyEnvoyGatewayClass(class *gatewayClass) bool {
	if len(class.Metadata.Labels) > 0 || class.Spec.ParametersRef != nil {
		return false
	}
	applied := strings.TrimSpace(class.Metadata.Annotations["kubectl.kubernetes.io/last-applied-configuration"])
	return applied == legacyEnvoyGatewayClassManifest
}

// reclaimEnvoyGatewayClass 는 설치기의 GatewayClass envoy 를 쓰는 Gateway 가 없으면 지운다. deleted 는
// 지웠는지, ok=false 는 확인하지 못해 Gateway API 전체를 남겨야 한다는 뜻이다.
//
// 컨트롤러(Envoy Gateway)는 스택 정리에서 이미 지웠다. 컨트롤러가 달아 둔 finalizer 가 남으면 지울 주체가
// 없어 영원히 Terminating 이므로 걷어 낸다 — 자기 것이고, 쓰는 Gateway 도 다른 컨트롤러도 없음을 확인했다.
func (r *sharedReclaim) reclaimEnvoyGatewayClass(ctx context.Context) (deleted, ok bool) {
	class := domain.EnvoyGatewayClassName
	if r.hasCRD("gateways.gateway.networking.k8s.io") {
		out, err := r.read(ctx, "get", "gateways.gateway.networking.k8s.io", "-A", "--no-headers",
			"-o", "custom-columns=NS:.metadata.namespace,NAME:.metadata.name,CLASS:.spec.gatewayClassName")
		if err != nil {
			r.keep(ctx, fmt.Sprintf("Gateway 목록을 읽지 못해 Gateway API 를 남깁니다: %v", err))
			return false, false
		}
		var users []string
		for _, fields := range kubectlRows(out, 3) {
			if fields[2] == class {
				users = append(users, fields[0]+"/"+fields[1])
			}
		}
		if len(users) > 0 {
			r.keep(ctx, fmt.Sprintf("GatewayClass %s 를 쓰는 Gateway 가 남아 있어 Gateway API 를 남깁니다: %s", class, summarizeRefs(users)))
			return false, false
		}
	}
	r.uc.emit(ctx, r.stackID, deleteStepShared, "info", fmt.Sprintf("GatewayClass %s 를 지웁니다", class))
	if !r.delete(ctx, "GatewayClass "+class,
		"delete", "gatewayclasses.gateway.networking.k8s.io", class, "--ignore-not-found", "--wait=false") {
		return false, false
	}
	remaining, err := r.read(ctx, "get", "gatewayclasses.gateway.networking.k8s.io", class, "--ignore-not-found", "-o", "name")
	if err == nil && len(parseResourceNames(remaining)) > 0 {
		if !r.delete(ctx, "GatewayClass "+class+" 의 finalizer",
			"patch", "gatewayclasses.gateway.networking.k8s.io", class, "--type=merge", "-p", `{"metadata":{"finalizers":null}}`) {
			return false, false
		}
	}
	r.reclaimed++
	return true, true
}

// crdUsers 는 CRD 로 만든 리소스다(클러스터 범위 리소스 포함). skip 이 참이면 세지 않는다. 조회에 실패하면
// 이유를 가리지 않고 쓰는 것으로 본다 — CRD 는 방금 목록에서 봤으므로 실패는 "없음" 이 아니라
// "확인하지 못함" 이다.
func (r *sharedReclaim) crdUsers(ctx context.Context, crds []string, skip func(resource, namespace, name, ownerKind string) bool) []string {
	var users []string
	for _, crd := range crds {
		out, err := r.read(ctx, "get", crd, "-A", "--no-headers",
			"-o", "custom-columns=NS:.metadata.namespace,NAME:.metadata.name,OWNER:.metadata.ownerReferences[0].kind")
		if err != nil {
			users = append(users, crd+"(조회 실패)")
			continue
		}
		resource := strings.SplitN(crd, ".", 2)[0]
		for _, fields := range kubectlRows(out, 3) {
			namespace, name, ownerKind := fields[0], fields[1], fields[2]
			if skip != nil && skip(resource, namespace, name, ownerKind) {
				continue
			}
			if namespace == kubectlNone {
				users = append(users, resource+"/"+name)
			} else {
				users = append(users, namespace+"/"+resource+"/"+name)
			}
		}
	}
	return users
}

func (r *sharedReclaim) sortedCRDs() []crdRow {
	rows := make([]crdRow, 0, len(r.crds))
	for _, row := range r.crds {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	return rows
}

// poll 은 done 이 참이 되거나 상한에 닿을 때까지 기다린다. 상한에 닿으면 false 다.
func (r *sharedReclaim) poll(ctx context.Context, done func() (bool, error)) (bool, error) {
	timeout, interval := r.uc.sharedWaitTimeout, r.uc.sharedPollInterval
	if timeout <= 0 {
		timeout = defaultSharedWaitTimeout
	}
	if interval <= 0 {
		interval = defaultSharedPollInterval
	}
	deadline := time.Now().Add(timeout)
	for {
		ok, err := done()
		if err != nil || ok {
			return ok, err
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// read 는 출력을 읽는 kubectl 호출이다(표준 출력만).
func (r *sharedReclaim) read(ctx context.Context, args ...string) (string, error) {
	return r.uc.readKubectl(ctx, r.kubeconfig, args...)
}

func (r *sharedReclaim) installer() port.HelmInstaller {
	if r.uc.executorFactoryFunc == nil {
		return nil
	}
	return r.uc.executorFactoryFunc(r.kubeconfig)
}

// delete 는 회수하기로 한 것을 지운다. 실패하면 실패로 적는다. 지우기 직전에 다른 스택이 생기지 않았는지
// 다시 본다 — 쓰는 곳을 확인한 뒤에도 CRD 삭제·파드 대기로 몇 분이 지날 수 있다.
func (r *sharedReclaim) delete(ctx context.Context, title string, args ...string) bool {
	if !r.lastStack(ctx) {
		return false
	}
	if _, err := r.uc.runKubectl(ctx, r.kubeconfig, args...); err != nil {
		r.fail(ctx, fmt.Sprintf("%s 를 지우지 못했습니다: %v", title, err))
		return false
	}
	return true
}

func (r *sharedReclaim) keep(ctx context.Context, message string) {
	r.kept++
	slog.Warn("cluster shared resource kept during stack delete", "stack_id", r.stackID, "reason", message)
	r.uc.emit(ctx, r.stackID, deleteStepShared, "warn", message)
}

func (r *sharedReclaim) fail(ctx context.Context, message string) {
	r.failed++
	slog.Error("cluster shared resource reclaim failed during stack delete", "stack_id", r.stackID, "error", message)
	r.uc.emit(ctx, r.stackID, deleteStepShared, "error", message)
}

func (r *sharedReclaim) summarize(ctx context.Context) {
	switch {
	case r.kept > 0 || r.failed > 0:
		r.uc.emit(ctx, r.stackID, deleteStepShared, "warn", fmt.Sprintf(
			"클러스터 공용 자원 일부가 남았습니다(남김 %d건, 실패 %d건) — 이유는 위에 적었습니다", r.kept, r.failed))
	case r.reclaimed > 0:
		r.uc.emit(ctx, r.stackID, deleteStepShared, "info", "Nullus 가 설치한 클러스터 공용 자원을 모두 회수했습니다")
	default:
		r.uc.emit(ctx, r.stackID, deleteStepShared, "info", "회수할 클러스터 공용 자원이 없습니다")
	}
}

// kubectlRows 는 --no-headers custom-columns 출력에서 칸 수가 맞는 줄만 고른다.
func kubectlRows(out string, columns int) [][]string {
	var rows [][]string
	for _, line := range strings.Split(out, "\n") {
		if fields := strings.Fields(line); len(fields) == columns {
			rows = append(rows, fields)
		}
	}
	return rows
}

// kubectlRefs 는 -o name 출력의 리소스 참조(kind/name)다. 공백이 든 줄은 참조가 아니다.
func kubectlRefs(out string) []string {
	var refs []string
	for _, line := range strings.Split(out, "\n") {
		ref := strings.TrimSpace(line)
		if ref == "" || strings.ContainsAny(ref, " \t") {
			continue
		}
		refs = append(refs, ref)
	}
	return refs
}

// uniqueNamespaces 는 네임스페이스 한 열 출력에서 값을 중복 없이 모은다.
func uniqueNamespaces(out string) []string {
	seen := map[string]bool{}
	var namespaces []string
	for _, ref := range kubectlRefs(out) {
		if ref == kubectlNone || seen[ref] {
			continue
		}
		seen[ref] = true
		namespaces = append(namespaces, ref)
	}
	sort.Strings(namespaces)
	return namespaces
}

// summarizeRefs 는 긴 목록을 앞의 몇 개와 남은 수로 줄인다.
func summarizeRefs(refs []string) string {
	const limit = 5
	sorted := append([]string(nil), refs...)
	sort.Strings(sorted)
	if len(sorted) <= limit {
		return strings.Join(sorted, ", ")
	}
	return fmt.Sprintf("%s 외 %d개", strings.Join(sorted[:limit], ", "), len(sorted)-limit)
}

func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// isPartialDiscoveryError 는 일부 API 그룹만 discovery 에 실패했다는 kubectl 오류인지 본다.
func isPartialDiscoveryError(err error) bool {
	return strings.Contains(err.Error(), "unable to retrieve the complete list of server APIs")
}

// isReleaseNotFoundError 는 지울 릴리스가 이미 없다는 Helm 오류인지 본다. "not found" 만 보면
// `context "kind" not found` 같은 접속 오류까지 "이미 지웠다" 로 읽고 CRD 를 지우러 간다.
func isReleaseNotFoundError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "release: not found")
}
