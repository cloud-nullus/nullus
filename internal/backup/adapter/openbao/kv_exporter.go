// Package openbao 는 OpenBao KV 논리 export/import 어댑터다.
//
// 설계: docs/11_기능설계/Nullus_백업복구_설계.md §3.2 (nullus-plan#75)
//
// raft snapshot API 를 쓰지 않는(못 쓰는) 이유: 스택의 OpenBao 는 단일
// replica 라 raft 대신 file 스토리지를 쓴다(internal/stack/adapter/helm/
// openbao-values.go:78). `bao operator raft snapshot` 은 raft 전용이다.
//
// 그래서 KV 경로를 재귀 순회해 값을 내보낸다. 봉인이 필요 없어 무중단이고,
// 포맷이 안정적이라 버전 이식도 가능하다. 대신 KV v2 의 버전 이력은
// 최신본만 남는다 — 그 한계는 설계에 기록돼 있다.
package openbao

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/cloud-nullus/draft/internal/backup/port"
	"github.com/cloud-nullus/draft/internal/shared/secrets"
)

// exportFormat 은 산출물 스키마다. 버전을 박아 두어 나중에 포맷이 바뀌어도
// 옛 백업본을 읽을 수 있게 한다.
type exportFormat struct {
	Version int                       `json:"version"`
	Mount   string                    `json:"mount"`
	Paths   map[string]map[string]any `json:"paths"`
}

const formatVersion = 1

// Resolver 는 스택 ID 로 그 스택의 OpenBao Store 를 만든다.
//
// OpenBao 는 스택마다 배포되므로 전역 인스턴스 하나를 가정할 수 없다
// (internal/shared/secrets/store.go:23). secrets.Router 가 이 모양을 만족한다.
type Resolver interface {
	ForStack(ctx context.Context, provider, stackID string) (secrets.Store, error)
}

type KVExporter struct {
	resolver Resolver
	mount    string
}

func NewKVExporter(resolver Resolver) *KVExporter {
	return &KVExporter{resolver: resolver, mount: "kv/nullus"}
}

func (e *KVExporter) store(ctx context.Context, stackID string) (secrets.KVBrowser, error) {
	if e.resolver == nil {
		return nil, fmt.Errorf("시크릿 저장소 resolver 가 없습니다")
	}
	st, err := e.resolver.ForStack(ctx, "openbao", stackID)
	if err != nil {
		return nil, fmt.Errorf("스택 %s 의 금고에 연결할 수 없습니다: %w", stackID, err)
	}
	browser, ok := st.(secrets.KVBrowser)
	if !ok {
		return nil, fmt.Errorf("이 금고 구현은 전체 순회를 지원하지 않습니다 (%T)", st)
	}
	return browser, nil
}

func (e *KVExporter) Export(ctx context.Context, stackID string, out io.Writer) (port.KVExportResult, error) {
	st, err := e.store(ctx, stackID)
	if err != nil {
		return port.KVExportResult{}, err
	}

	paths, err := listRecursive(ctx, st, e.mount)
	if err != nil {
		return port.KVExportResult{}, fmt.Errorf("금고 경로 순회: %w", err)
	}
	sort.Strings(paths)

	doc := exportFormat{Version: formatVersion, Mount: e.mount, Paths: map[string]map[string]any{}}
	for _, p := range paths {
		v, err := st.GetSecret(ctx, p)
		if err != nil {
			// 한 경로를 못 읽었다고 전체를 버리지 않는다. 빠진 것은 복구 후
			// 참조 정합성 검사(§6.4)에서 dangling 으로 드러난다.
			continue
		}
		doc.Paths[p] = v
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return port.KVExportResult{}, fmt.Errorf("금고 export 직렬화: %w", err)
	}
	return port.KVExportResult{PathCount: len(doc.Paths)}, nil
}

func (e *KVExporter) Import(ctx context.Context, stackID string, in io.Reader) error {
	st, err := e.store(ctx, stackID)
	if err != nil {
		return err
	}
	var doc exportFormat
	if err := json.NewDecoder(in).Decode(&doc); err != nil {
		return fmt.Errorf("금고 export 해석: %w", err)
	}
	if doc.Version != formatVersion {
		return fmt.Errorf("지원하지 않는 금고 export 형식입니다: version %d", doc.Version)
	}

	keys := make([]string, 0, len(doc.Paths))
	for p := range doc.Paths {
		keys = append(keys, p)
	}
	sort.Strings(keys)

	var failed []string
	for _, p := range keys {
		if err := st.PutSecret(ctx, p, doc.Paths[p]); err != nil {
			failed = append(failed, p)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("금고 경로 %d 건을 복원하지 못했습니다: %s",
			len(failed), strings.Join(failed[:min(len(failed), 5)], ", "))
	}
	return nil
}

// PathExists 는 참조 정합성 검사에 쓴다 (§6.4).
func (e *KVExporter) PathExists(ctx context.Context, stackID, path string) (bool, error) {
	st, err := e.store(ctx, stackID)
	if err != nil {
		return false, err
	}
	v, err := st.GetSecret(ctx, path)
	if err != nil {
		return false, nil // 못 읽으면 없는 것으로 본다 — 검사의 목적이 그것이다
	}
	return len(v) > 0, nil
}

// listRecursive 는 KV 트리를 훑어 잎 경로를 모은다.
func listRecursive(ctx context.Context, st secrets.KVBrowser, prefix string) ([]string, error) {
	var out []string
	entries, err := st.ListKeys(ctx, prefix)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		full := strings.TrimSuffix(prefix, "/") + "/" + strings.TrimSuffix(e, "/")
		if strings.HasSuffix(e, "/") {
			children, err := listRecursive(ctx, st, full)
			if err != nil {
				continue
			}
			out = append(out, children...)
			continue
		}
		out = append(out, full)
	}
	return out, nil
}

var _ port.KVExporter = (*KVExporter)(nil)
