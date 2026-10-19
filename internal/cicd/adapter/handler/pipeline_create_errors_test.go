package handler

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/cicd/port"
	"github.com/cloud-nullus/draft/internal/cicd/usecase"
)

func TestCreatePipelineErrorStatus(t *testing.T) {
	cases := map[string]struct {
		err    error
		status int
		code   string
	}{
		"stack not found":  {usecase.ErrStackNotFound, http.StatusBadRequest, "STACK_NOT_FOUND"},
		"org mismatch":     {usecase.ErrStackOrgMismatch, http.StatusForbidden, "STACK_ORG_MISMATCH"},
		"cluster mismatch": {usecase.ErrStackClusterMismatch, http.StatusBadRequest, "STACK_CLUSTER_MISMATCH"},
		"source needs a stack": {
			fmt.Errorf("template x: %w", usecase.ErrTemplateSourceRequiresStack),
			http.StatusBadRequest, "TEMPLATE_SOURCE_REQUIRES_STACK",
		},
		// 원본 호스트 쪽 문제다. 요청을 고쳐도 낫지 않으므로 400 으로 돌려주지 않는다.
		"source unreachable": {
			fmt.Errorf("provision: %w", port.ErrSourceUnreachable),
			http.StatusBadGateway, "TEMPLATE_SOURCE_UNAVAILABLE",
		},
		"source not found": {fmt.Errorf("x: %w", port.ErrSourceNotFound), http.StatusBadGateway, "TEMPLATE_SOURCE_UNAVAILABLE"},
		"source too large": {fmt.Errorf("x: %w", port.ErrSourceTooLarge), http.StatusBadGateway, "TEMPLATE_SOURCE_UNAVAILABLE"},
		// 템플릿의 원본 자체가 쓸 수 없는 모양이다(심볼릭 링크 · 지원하지 않는 호스트) — 템플릿을 고쳐야 낫는다.
		"source unsupported": {fmt.Errorf("x: %w", port.ErrSourceUnsupported), http.StatusUnprocessableEntity, "TEMPLATE_SOURCE_UNSUPPORTED"},
		"anything else":      {errors.New("boom"), http.StatusBadRequest, "PIPELINE_CONFIG_INVALID"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			status, code := createPipelineErrorStatus(tc.err)
			assert.Equal(t, tc.status, status)
			assert.Equal(t, tc.code, code)
		})
	}
}
