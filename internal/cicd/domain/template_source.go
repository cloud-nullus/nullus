package domain

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

// BuildProfile 은 가져온 원본을 어떻게 빌드하고 띄울지다.
//
// 원본 Dockerfile 을 그대로 쓸지, 플랫폼이 Dockerfile 을 대신 쓸지, 배포 매니페스트에
// 어떤 헬스 체크와 자원을 둘지를 한 이름으로 묶는다. 이름별 실제 값은 스캐폴딩
// (adapter/scaffold)이 가진다 — 매니페스트 모양은 거기서 정한다.
type BuildProfile string

const (
	// BuildProfileRepository 는 원본 Dockerfile 을 그대로 쓰고 배포 설정은 기본값이다.
	BuildProfileRepository BuildProfile = ""
	// BuildProfileSpringBoot 는 원본 Dockerfile 을 쓰고, 없으면 플랫폼이 쓴다(Maven 으로 실행 jar →
	// JRE). JVM 에 맞춘 자원과 헬스 체크를 둔다. eGov 부트 샘플 v5.0.0 에는 Dockerfile 이 없다.
	BuildProfileSpringBoot BuildProfile = "spring-boot"
	// BuildProfileMavenWAR 는 플랫폼이 Dockerfile 을 쓴다(Maven 빌드 → Tomcat ROOT.war).
	// 미리 빌드한 WAR 를 복사하는 원본 Dockerfile 은 CI 의 `docker build .` 로 돌지 않는다.
	BuildProfileMavenWAR BuildProfile = "maven-war"
)

// IsValid 는 아는 프로필인지 본다.
func (p BuildProfile) IsValid() bool {
	switch p {
	case BuildProfileRepository, BuildProfileSpringBoot, BuildProfileMavenWAR:
		return true
	}
	return false
}

// ReplacesDockerfile 은 원본에 Dockerfile 이 있어도 플랫폼 것으로 바꾸는지다.
func (p BuildProfile) ReplacesDockerfile() bool {
	return p == BuildProfileMavenWAR
}

// GeneratesDockerfile 은 원본에 Dockerfile 이 없을 때 플랫폼이 만들 수 있는지다.
func (p BuildProfile) GeneratesDockerfile() bool {
	return p == BuildProfileMavenWAR || p == BuildProfileSpringBoot
}

// TemplateSource 는 템플릿이 새 저장소로 가져오는 외부 원본이다.
//
// 파이프라인을 만들 때 이 커밋의 파일을 그대로 복사해 앱 저장소의 첫 커밋으로 둔다.
// 원본을 따라가지 않는다 — 가져온 순간부터 그 저장소는 사용자 것이고, 새 버전은
// 템플릿의 Commit 을 올렸을 때 새로 만드는 파이프라인부터 들어온다.
//
// 파이프라인도 같은 값을 들고 있다. 템플릿의 Commit 은 시간이 지나면 올라가므로,
// 그 저장소가 어느 버전에서 시작했는지는 파이프라인 쪽에 따로 남아야 한다.
type TemplateSource struct {
	// RepoURL 은 원본 저장소 주소다(https).
	RepoURL string `json:"repo_url"`
	// Commit 은 가져올 커밋의 전체 SHA 다. 태그·브랜치는 받지 않는다 — 태그도 다시
	// 찍힌다(eGov 웹 샘플의 v4.3.0 은 2025-03 릴리스인데 2026-05 커밋을 가리킨다).
	Commit string `json:"commit"`
	// Label 은 화면에 보일 버전 이름이다(예: v5.0.0). 가져올 대상을 정하지 않는다.
	Label string `json:"label,omitempty"`
	// BuildProfile 은 가져온 원본을 어떻게 빌드하고 띄울지다.
	BuildProfile BuildProfile `json:"build_profile,omitempty"`
}

// 열 길이(pipeline_templates.source_* · pipelines.source_*)와 같다.
const (
	maxSourceRepoURLLength = 500
	maxSourceLabelLength   = 100
	sourceCommitLength     = 40
	shortCommitLength      = 12
)

// Validate 는 원본을 가져올 수 있는 모양인지 본다. 어기면 ErrTemplateInvalid 를 감싼다.
func (s *TemplateSource) Validate() error {
	repoURL := strings.TrimSpace(s.RepoURL)
	if repoURL == "" {
		return fmt.Errorf("%w: source.repo_url is required", ErrTemplateInvalid)
	}
	if utf8.RuneCountInString(repoURL) > maxSourceRepoURLLength {
		return fmt.Errorf("%w: source.repo_url must be at most %d characters", ErrTemplateInvalid, maxSourceRepoURLLength)
	}
	u, err := url.Parse(repoURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || strings.Trim(u.Path, "/") == "" {
		return fmt.Errorf("%w: source.repo_url must be an https repository URL", ErrTemplateInvalid)
	}
	if !isFullCommitSHA(s.Commit) {
		return fmt.Errorf("%w: source.commit must be a full %d-character lowercase commit SHA", ErrTemplateInvalid, sourceCommitLength)
	}
	if utf8.RuneCountInString(s.Label) > maxSourceLabelLength {
		return fmt.Errorf("%w: source.label must be at most %d characters", ErrTemplateInvalid, maxSourceLabelLength)
	}
	if !s.BuildProfile.IsValid() {
		return fmt.Errorf("%w: source.build_profile must be one of %q, %q, %q", ErrTemplateInvalid,
			BuildProfileRepository, BuildProfileSpringBoot, BuildProfileMavenWAR)
	}
	return nil
}

// ShortCommit 은 커밋 메시지와 화면에 쓸 짧은 SHA 다.
func (s *TemplateSource) ShortCommit() string {
	if len(s.Commit) <= shortCommitLength {
		return s.Commit
	}
	return s.Commit[:shortCommitLength]
}

func isFullCommitSHA(v string) bool {
	if len(v) != sourceCommitLength {
		return false
	}
	for _, r := range v {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
