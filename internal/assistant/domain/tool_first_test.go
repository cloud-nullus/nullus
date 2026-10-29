package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequiresToolFirst(t *testing.T) {
	yes := []string{
		"실패한 스택이 있어?", "등록된 클러스터 목록과 각 클러스터 용도를 알려줘", "설치할 수 있는 스택 템플릿은 몇 개야?",
		"왜 설치가 실패했어", "최근 배포 로그 보여줘", "How many clusters are registered?", "list the stack templates",
		"  스택 상태  ",
	}
	for _, q := range yes {
		assert.True(t, RequiresToolFirst(q), q)
	}
	no := []string{"", "   ", "안녕", "고마워", "너는 누구야?", "DevSecOps 가 뭐야?", "thanks"}
	for _, q := range no {
		assert.False(t, RequiresToolFirst(q), q)
	}
}

func TestLastUserQuestion(t *testing.T) {
	assert.Equal(t, "", LastUserQuestion(nil))
	turns := []Turn{
		{Role: RoleUser, Content: "첫 질문"},
		{Role: RoleAssistant, Content: "답"},
		{Role: RoleUser, Content: "두 번째 질문"},
		{Role: RoleAssistant, Content: "답 2"},
	}
	assert.Equal(t, "두 번째 질문", LastUserQuestion(turns))
	assert.Equal(t, "", LastUserQuestion([]Turn{{Role: RoleAssistant, Content: "혼자"}}))
}
