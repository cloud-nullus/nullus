package usecase

import (
	"encoding/json"
	"log/slog"

	"github.com/cloud-nullus/draft/internal/backup/domain"
)

// manifestToMap 은 매니페스트를 JSONB 컬럼에 넣을 수 있는 형태로 바꾼다.
//
// 구조체를 그대로 두지 않고 map 으로 옮기는 이유는 저장 계층이 JSONB 를
// 쓰기 때문이다. 직렬화가 실패하면 빈 map 을 돌려준다 — 매니페스트를 못
// 만들었다고 백업 자체를 실패시키면, 실제로는 있는 산출물을 못 쓰게 된다.
func manifestToMap(m domain.Manifest) map[string]any {
	raw, err := json.Marshal(m)
	if err != nil {
		slog.Error("매니페스트 직렬화 실패", "error", err)
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		slog.Error("매니페스트 역직렬화 실패", "error", err)
		return map[string]any{}
	}
	return out
}

// manifestFromMap 은 저장된 매니페스트를 되읽는다.
func manifestFromMap(v map[string]any) (domain.Manifest, error) {
	var m domain.Manifest
	raw, err := json.Marshal(v)
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(raw, &m)
}
