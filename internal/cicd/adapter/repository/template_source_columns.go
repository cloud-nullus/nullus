package repository

import (
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// sourceColumnList 는 pipeline_templates·pipelines 가 같은 이름으로 가진 원본 열이다(마이그레이션 000101).
const sourceColumnList = `source_repo_url, source_commit, source_label, source_build_profile`

// sourceColumns 는 원본 열을 읽어 들일 자리다.
type sourceColumns struct {
	repoURL, commit, label, profile string
}

// dest 는 Scan 에 넘길 포인터다. sourceColumnList 와 순서가 같다.
func (c *sourceColumns) dest() []any {
	return []any{&c.repoURL, &c.commit, &c.label, &c.profile}
}

// source 는 읽은 열을 도메인 값으로 옮긴다. 저장소 주소가 비면 원본이 없는 것이다.
func (c *sourceColumns) source() *domain.TemplateSource {
	if strings.TrimSpace(c.repoURL) == "" {
		return nil
	}
	return &domain.TemplateSource{
		RepoURL:      c.repoURL,
		Commit:       c.commit,
		Label:        c.label,
		BuildProfile: domain.BuildProfile(c.profile),
	}
}

// sourceArgs 는 원본을 열 값으로 옮긴다. 원본이 없으면 모두 빈 문자열이다(열이 NOT NULL).
func sourceArgs(s *domain.TemplateSource) []any {
	if s == nil {
		return []any{"", "", "", ""}
	}
	return []any{s.RepoURL, s.Commit, s.Label, string(s.BuildProfile)}
}
