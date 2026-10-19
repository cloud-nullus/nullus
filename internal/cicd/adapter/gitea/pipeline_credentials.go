package gitea

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/shared/externalsecret"
	"github.com/cloud-nullus/draft/internal/shared/secrets"
)

const (
	// ESOSecretStoreName 은 스택 네임스페이스에 있는 SecretStore 이름이다.
	//
	// 스택 모듈이 만든다(helm/external-secrets.go). 모듈 간 직접 import 가
	// 금지돼 값을 각자 들고 있으므로, 갈라지면 ESO 가 Secret 을 만들지 못하고
	// agent 파드는 FailedMount 로 멈춘다.
	ESOSecretStoreName = "nullus-openbao"

	// CISecretPrefix 는 파이프라인 자격증명 Secret 이름의 접두사다.
	//
	// Argo CD 리포 Secret 의 기존 규약(nullus-repo-<app>)과 짝을 맞춘다.
	// 파이프라인 단위라 한 자격증명의 유출이 다른 파이프라인으로 번지지 않고,
	// 파이프라인을 지울 때 함께 지울 수 있다.
	CISecretPrefix = "nullus-ci-"
)

// SecretWriter 는 OpenBao 기록과 회수에 필요한 최소 동작만 노출한다.
type SecretWriter interface {
	PutTokenForStack(ctx context.Context, provider, stackID, path, value string) error
	// DeleteTreeForStack 은 접두사 아래의 값을 모두 지운다. 파이프라인을 지울 때 쓴다.
	DeleteTreeForStack(ctx context.Context, provider, stackID, prefix string) error
}

// CISecretName 은 앱의 파이프라인 자격증명 Secret 이름이다.
//
// 스캐폴딩이 만든 Jenkinsfile 의 envFrom.secretRef 가 같은 이름을 가리킨다
// (scaffold 의 ciSecretName). 갈라지면 agent 파드가 없는 Secret 을 참조해
// 기동하지 못한다.
func CISecretName(app string) string {
	return CISecretPrefix + strings.TrimSpace(app)
}

// CredentialPlane 은 파이프라인 자격증명을 OpenBao → ESO → K8s Secret 으로
// 나른다.
//
// Gitea 에는 GitLab 같은 프로젝트 CI 변수 저장소가 없다. Jenkins Credentials 를
// 1차 저장소로 쓰지 않는 이유는 자격증명 사본이 하나 더 생기고 회전 경로가
// 둘로 갈리기 때문이다 — OpenBao 가 단일 출처라는 원칙이 깨진다.
//
// 매니페스트를 적용하지 않고 렌더링만 한다. 클러스터 접근은 유스케이스가
// 한곳에서 맡는다(Argo CD 리소스와 같은 경로로 적용된다).
type CredentialPlane struct {
	secrets SecretWriter
	// env / orgID / stackID 는 OpenBao 경로 접두사와 스택 범위 접근에 쓴다.
	env       string
	orgID     string
	stackID   string
	namespace string
}

// NewCredentialPlane 은 CredentialPlane 을 만든다.
func NewCredentialPlane(secrets SecretWriter, env, orgID, stackID, namespace string) *CredentialPlane {
	return &CredentialPlane{
		secrets:   secrets,
		env:       strings.TrimSpace(env),
		orgID:     strings.TrimSpace(orgID),
		stackID:   strings.TrimSpace(stackID),
		namespace: strings.TrimSpace(namespace),
	}
}

// Provision 은 값들을 OpenBao 에 쓰고 그것을 참조하는 ExternalSecret 을 만든다.
//
// 변수를 한 번에 다 받는다 — ExternalSecret 은 항목 전체를 한 문서로 선언하므로
// 변수마다 따로 적용하면 마지막 것만 남는다.
func (p *CredentialPlane) Provision(
	ctx context.Context,
	app string,
	vars []port.PipelineVariable,
) (string, error) {
	if p == nil || p.secrets == nil {
		return "", fmt.Errorf("자격증명 저장소가 배선되지 않았습니다")
	}
	if len(vars) == 0 {
		return "", nil
	}
	appName := strings.TrimSpace(app)
	prefix, err := p.pipelinePathPrefix(appName)
	if err != nil {
		return "", err
	}

	// 순서를 고정한다 — 같은 입력이 같은 매니페스트를 내야 재적용이 의미 없는
	// diff 를 만들지 않는다.
	sorted := append([]port.PipelineVariable(nil), vars...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })

	entries := make([]externalsecret.Entry, 0, len(sorted))
	for _, v := range sorted {
		key := strings.TrimSpace(v.Key)
		if key == "" {
			continue
		}
		path := prefix + strings.ToLower(key)
		if err := p.secrets.PutTokenForStack(ctx, SecretProvider, p.stackID, path, v.Value); err != nil {
			return "", fmt.Errorf("자격증명 기록 실패 (%s): %w", path, err)
		}
		entries = append(entries, externalsecret.Entry{SecretKey: key, RemotePath: path})
	}
	if len(entries) == 0 {
		return "", nil
	}

	manifest, err := externalsecret.Render(externalsecret.Spec{
		Name:            CISecretName(appName),
		Namespace:       p.namespace,
		SecretStoreName: ESOSecretStoreName,
		Entries:         entries,
		// 파이프라인을 지울 때 이 표시로 플랫폼이 만든 것만 지운다.
		Labels: map[string]string{
			shareddomain.AppManagedByLabel: shareddomain.AppManagedByCICD,
			"app.kubernetes.io/name":       appName,
		},
	})
	if err != nil {
		return "", fmt.Errorf("render pipeline credential secret for %q: %w", appName, err)
	}
	return manifest, nil
}

// CredentialObject 는 Provision 이 만든 ExternalSecret 의 자리다.
func (p *CredentialPlane) CredentialObject(app string) (port.PipelineCredentialObject, error) {
	appName := strings.TrimSpace(app)
	prefix, err := p.pipelinePathPrefix(appName)
	if err != nil {
		return port.PipelineCredentialObject{}, err
	}
	return port.PipelineCredentialObject{
		Namespace:       p.namespace,
		Name:            CISecretName(appName),
		RemoteKeyPrefix: prefix,
	}, nil
}

// Revoke 는 Provision 이 금고에 쓴 이 파이프라인 몫의 값을 모두 지운다.
//
// 키 목록을 기억해 두지 않고 경로 아래를 통째로 지운다 — 재프로비저닝으로 변수가 바뀌었으면
// 지금 ExternalSecret 이 가리키지 않는 옛 값도 남아 있다.
func (p *CredentialPlane) Revoke(ctx context.Context, app string) error {
	if p == nil || p.secrets == nil {
		return fmt.Errorf("자격증명 저장소가 배선되지 않았습니다")
	}
	prefix, err := p.pipelinePathPrefix(strings.TrimSpace(app))
	if err != nil {
		return err
	}
	if err := p.secrets.DeleteTreeForStack(ctx, SecretProvider, p.stackID, prefix); err != nil {
		var denied *secrets.StatusError
		if secrets.IsForbidden(err) && errors.As(err, &denied) {
			// 삭제 권한은 스택 부트스트랩 때 정책으로 들어간다. root token 이 폐기된 기존 스택은
			// 정책을 고칠 수 없어 값이 남는다. 키마다 같은 403 이 나오므로 하나만 싣는다.
			return fmt.Errorf("금고 정책이 삭제를 막았습니다 — 파이프라인 자격증명 삭제 권한이 생기기 전에 "+
				"설치한 스택이면 정책에 그 권한이 없습니다. 금고에서 직접 지워야 합니다: %w", denied)
		}
		return err
	}
	return nil
}

// pipelinePathPrefix 는 한 파이프라인의 자격증명이 사는 금고 경로다(끝이 / 다). 경로 규칙은
// 스택 모듈의 금고 정책과 같이 secrets 패키지 한곳에 있다.
func (p *CredentialPlane) pipelinePathPrefix(app string) (string, error) {
	return secrets.PipelineCredentialPrefix(p.env, p.orgID, app)
}
