package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 앱 네임스페이스는 스택 네임스페이스 밖이다. 앱의 Deployment 는 Argo CD 가 Git 에서 되살리지만,
// Nullus 가 파이프라인을 만들 때 앱 네임스페이스에 둔 이미지 pull Secret(nullus-registry)은 어디에도
// 없다 — 2026-10-12 리허설에서 앱 네임스페이스까지 잃은 뒤 복구하자 Deployment 는 돌아왔는데
// FailedToRetrieveImagePullSecret 로 pull 이 실패했다. 어느 네임스페이스가 이 스택의 앱인지는
// Argo CD Application 의 destination 이 말한다 — 덤프가 이미 담는 것이다.

func appNSPayload(t *testing.T) (nsList, appNS, pullSecret string) {
	t.Helper()
	ns := map[string]any{"apiVersion": "v1", "kind": "List", "items": []any{
		map[string]any{"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
			"metadata": map[string]any{"name": "demo-api", "namespace": "nullus", "resourceVersion": "7"},
			"spec":     map[string]any{"destination": map[string]any{"namespace": "demo-api", "server": "https://kubernetes.default.svc"}}},
		map[string]any{"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
			"metadata": map[string]any{"name": "self", "namespace": "nullus"},
			"spec":     map[string]any{"destination": map[string]any{"namespace": "nullus"}}},
		map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]any{"name": "gitlab", "namespace": "nullus"}},
	}}
	b, _ := json.Marshal(ns)
	// kubectl get namespace <이름> -o json 은 List 가 아니라 객체 하나를 준다.
	n := map[string]any{"apiVersion": "v1", "kind": "Namespace",
		"metadata": map[string]any{"name": "demo-api", "uid": "u1", "resourceVersion": "3",
			"labels": map[string]any{shareddomain.AppManagedByLabel: shareddomain.AppManagedByCICD}},
		"status": map[string]any{"phase": "Active"}}
	nb, _ := json.Marshal(n)
	// 앱마다 따로 둔 pull Secret 과, 그 전에 만든 파이프라인의 네임스페이스 공용 이름이 섞여 있다.
	pull := func(name, uid string) map[string]any {
		return map[string]any{"apiVersion": "v1", "kind": "Secret", "type": "kubernetes.io/dockerconfigjson",
			"metadata": map[string]any{"name": name, "namespace": "demo-api", "uid": uid,
				"labels": map[string]any{shareddomain.AppManagedByLabel: shareddomain.AppManagedByCICD}},
			"data": map[string]any{".dockerconfigjson": "e30="}}
	}
	s := map[string]any{"apiVersion": "v1", "kind": "List", "items": []any{
		pull(shareddomain.AppImagePullSecretName, "u2"),
		pull(shareddomain.AppImagePullSecretNameFor("demo-api"), "u3"),
	}}
	sb, _ := json.Marshal(s)
	return string(b), string(nb), string(sb)
}

// pullSecretListArgs 는 앱 네임스페이스의 pull Secret 을 고르는 조회다 — 이름이 아니라 라벨과 종류로 고른다.
const pullSecretListArgs = "get secret -n demo-api -l " + shareddomain.AppManagedByLabel + "=" + shareddomain.AppManagedByCICD +
	" --field-selector type=kubernetes.io/dockerconfigjson -o json"

func TestDump_앱_네임스페이스와_pull_Secret_을_담는다(t *testing.T) {
	nsList, appNS, pullSecret := appNSPayload(t)
	k := &fakeSharedKubectl{respond: func(args []string, _ string) (string, error) {
		a := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(a, "api-resources --verbs=list --namespaced=true"):
			return "deployments.apps\nsecrets\napplications.argoproj.io\n", nil
		case strings.HasPrefix(a, "api-resources --verbs=list --namespaced=false"):
			return "", nil
		case strings.HasPrefix(a, "get ") && strings.Contains(a, "-n nullus -o json"):
			return nsList, nil
		case strings.HasPrefix(a, "get namespace demo-api -o json"):
			return appNS, nil
		case a == pullSecretListArgs:
			return pullSecret, nil
		}
		return "", nil
	}}
	d := &ResourceDumper{exec: k.run}

	var out bytes.Buffer
	_, err := d.Dump(context.Background(), "nullus", &out)
	require.NoError(t, err)

	var doc struct {
		AppNamespaces struct {
			Items []map[string]any `json:"items"`
		} `json:"app_namespaces"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
	require.Len(t, doc.AppNamespaces.Items, 3, "Namespace 와 pull Secret 두 개(앱별·기존 공용 이름)")
	kinds := []string{}
	for _, it := range doc.AppNamespaces.Items {
		kinds = append(kinds, it["kind"].(string))
		meta := it["metadata"].(map[string]any)
		assert.Nil(t, meta["uid"], "서버 관리 필드는 걷어낸다")
		assert.Nil(t, meta["resourceVersion"])
		assert.Nil(t, it["status"])
	}
	assert.ElementsMatch(t, []string{"Namespace", "Secret", "Secret"}, kinds)
	joined := strings.Join(k.joined(), "\n")
	assert.NotContains(t, joined, "get namespace nullus ", "스택 네임스페이스 자신은 앱 네임스페이스가 아니다")
}

func TestDump_pull_Secret_이_없는_앱_네임스페이스는_네임스페이스만_담는다(t *testing.T) {
	nsList, appNS, _ := appNSPayload(t)
	k := &fakeSharedKubectl{respond: func(args []string, _ string) (string, error) {
		a := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(a, "api-resources --verbs=list --namespaced=true"):
			return "deployments.apps\napplications.argoproj.io\n", nil
		case strings.HasPrefix(a, "get ") && strings.Contains(a, "-n nullus -o json"):
			return nsList, nil
		case strings.HasPrefix(a, "get namespace demo-api -o json"):
			return appNS, nil
		}
		return "", nil // secret 없음 → --ignore-not-found 가 빈 출력
	}}
	d := &ResourceDumper{exec: k.run}
	var out bytes.Buffer
	_, err := d.Dump(context.Background(), "nullus", &out)
	require.NoError(t, err)
	var doc struct {
		AppNamespaces struct {
			Items []map[string]any `json:"items"`
		} `json:"app_namespaces"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
	require.Len(t, doc.AppNamespaces.Items, 1)
	assert.Equal(t, "Namespace", doc.AppNamespaces.Items[0]["kind"])
}

// 앱 네임스페이스가 이미 살아 있으면(스택 네임스페이스만 잃은 경우) 그 안의 Secret 을 덮지 않는다 —
// 그쪽이 지금 쓰는 자격증명이다. 없을 때만 만든다. 그리고 네임스페이스 리소스보다 **먼저** 둔다:
// Argo CD 가 동기화할 때 Secret 이 있어야 첫 파드부터 pull 이 된다.
func TestApply_앱_네임스페이스와_pull_Secret_을_없을_때만_먼저_만든다(t *testing.T) {
	_, appNS, pullSecret := appNSPayload(t)
	var nsItem map[string]any
	require.NoError(t, json.Unmarshal([]byte(appNS), &nsItem))
	var secrets struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(pullSecret), &secrets))
	secretItem := secrets.Items[0]
	doc := map[string]any{
		"app_namespaces": map[string]any{"apiVersion": "v1", "kind": "List", "items": []any{nsItem, secretItem}},
		"namespaced":     map[string]any{"apiVersion": "v1", "kind": "List", "items": []any{map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]any{"name": "gitlab"}}}},
	}
	payload, _ := json.Marshal(doc)

	k := &fakeSharedKubectl{respond: func(args []string, in string) (string, error) {
		a := strings.Join(args, " ")
		if strings.HasPrefix(a, "create -n demo-api -f -") {
			return "", errors.New(`Error from server (AlreadyExists): secrets "nullus-registry" already exists`)
		}
		return "", nil
	}}
	d := &ResourceDumper{exec: k.run}
	require.NoError(t, d.Apply(context.Background(), "nullus", bytes.NewReader(payload)))

	calls := k.joined()
	var nsCreate, secretCreate, nsApply = -1, -1, -1
	for i, c := range calls {
		switch {
		case strings.HasPrefix(c, "create -f -") && strings.Contains(k.stdins[i], `"kind":"Namespace"`):
			nsCreate = i
		case strings.HasPrefix(c, "create -n demo-api -f -"):
			secretCreate = i
		case strings.HasPrefix(c, "apply -n nullus -f -"):
			nsApply = i
		}
	}
	require.GreaterOrEqual(t, nsCreate, 0, "네임스페이스를 만든다(없을 때만)")
	require.GreaterOrEqual(t, secretCreate, 0, "pull Secret 을 만든다(이미 있으면 그대로 — AlreadyExists 는 오류가 아니다)")
	require.GreaterOrEqual(t, nsApply, 0)
	assert.Less(t, nsCreate, nsApply, "앱 네임스페이스가 스택 리소스보다 먼저")
	assert.Less(t, secretCreate, nsApply)
	assert.NotContains(t, strings.Join(calls, "\n"), "apply -n demo-api", "앱 네임스페이스의 것은 apply 로 덮지 않는다")
}

func TestApply_app_namespaces_가_없는_옛_덤프도_그대로_돈다(t *testing.T) {
	doc := map[string]any{"namespaced": map[string]any{"apiVersion": "v1", "kind": "List", "items": []any{}}}
	payload, _ := json.Marshal(doc)
	k := &fakeSharedKubectl{}
	d := &ResourceDumper{exec: k.run}
	require.NoError(t, d.Apply(context.Background(), "nullus", bytes.NewReader(payload)))
	assert.NotContains(t, strings.Join(k.joined(), "\n"), "create ")
}
