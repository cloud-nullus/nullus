package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func newAccessDomainCreateStack() *CreateStack {
	return NewCreateStack(stackrepo.NewMemoryStackRepository(), stackrepo.NewMemoryTemplateRepository())
}

// access_domain 이 ".io" 로 저장돼 hostname "jenkins..io" 와 인증서 "*..io" 가
// 만들어진 적이 있다. 설치를 시작하기 전에 막는다.
func TestCreateStack_RejectsMalformedAccessDomain(t *testing.T) {
	uc := newAccessDomainCreateStack()

	_, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "domain-stack",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Config:    domain.StackConfig{AccessDomain: ".io"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "access_domain")
}

func TestCreateStack_AcceptsRealAccessDomain(t *testing.T) {
	uc := newAccessDomainCreateStack()

	out, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "domain-stack",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Config:    domain.StackConfig{AccessDomain: "stack.nullus.io"},
	})

	require.NoError(t, err)
	assert.Equal(t, "stack.nullus.io", out.Stack.Config.(domain.StackConfig).AccessDomain)
}
