package port

import (
	"context"
	"time"
)

// CIJobSpec 은 CI 서버에 만들 job 하나의 요청이다.
type CIJobSpec struct {
	// Name 은 job 이름이다. 앱 이름과 같게 두어 화면에서 짝을 찾기 쉽게 한다.
	Name string
	// RepoCloneURL 은 CI 가 스캔할 저장소 주소다.
	RepoCloneURL string
	// RepoOwner / RepoName 은 organization 소스가 요구하는 분해된 형태다.
	RepoOwner string
	RepoName  string
	// ServerURL 은 SCM 서버의 루트 주소다. Gitea 소스는 리포 주소가 아니라
	// 서버 주소를 받아 API 로 브랜치를 훑는다.
	ServerURL string
	// CredentialID 는 CI 서버에 등록된 SCM 자격증명 식별자다.
	// 비어 있으면 익명으로 스캔한다 — private 리포에서는 실패한다.
	CredentialID string
	// PipelinePath 는 파이프라인 정의 파일의 리포 내 경로다.
	PipelinePath string
}

// CIJob 은 만들어진 job 이다.
type CIJob struct {
	Name string
	URL  string
}

// CIJobProvisioner 는 CI 서버에 job 을 만든다.
//
// SCMProvisioner 와 분리한다. GitLab CI·GitHub Actions 는 파이프라인 정의를
// 푸시하면 자동으로 감지하지만, Jenkins 는 job 이 먼저 존재해야 한다 —
// Jenkinsfile 만 커밋해서는 아무 일도 일어나지 않는다. 그 차이를 흡수하는
// 자리이므로 이 포트를 지원하지 않는 플랫폼에서는 nil 이고, 호출부는 nil 이면
// 건너뛴다(기존 GitLab/GitHub 경로 무영향).
type CIJobProvisioner interface {
	// EnsureJob 은 job 을 만들거나 이미 있으면 그대로 둔다(멱등).
	EnsureJob(ctx context.Context, spec CIJobSpec) (*CIJob, error)
	// DeleteJob 은 job 을 지운다. 이미 없으면 성공으로 본다.
	DeleteJob(ctx context.Context, name string) error
}

// SCMWebhookProvisioner 는 저장소에 webhook 을 건다.
//
// Jenkins multibranch job 은 스스로 폴링하지 않는 한 새 커밋을 모른다.
// 폴링은 지연이 크고 리포가 늘수록 부하가 커지므로 push webhook 을 건다.
type SCMWebhookProvisioner interface {
	EnsureWebhook(ctx context.Context, projectID, targetURL, secret string) error
}

// CICredentialResolver 는 스택별 CI 서버 접속 자격증명을 돌려준다.
//
// 기동 시점에 고정할 수 없다 — CI 서버는 스택마다 따로 서고 관리자 비밀번호도
// 스택마다 다르게 생성된다(provisioning_secrets 가 OpenBao 에 넣는다).
// 고정 문자열을 쓰면 비어 있거나 다른 스택의 자격증명으로 붙게 된다.
//
// SCMTokenSpec 을 재사용한다 — 스택·클러스터·조직·환경이라는 조회 축이 SCM
// 토큰과 완전히 같기 때문이다.
type CICredentialResolver interface {
	ResolveCICredential(ctx context.Context, spec SCMTokenSpec) (user, secret string, err error)
}

// CIBuild 는 CI 서버가 실행한 빌드 하나다.
type CIBuild struct {
	Number int
	// Result 는 CI 가 보고한 결과다(SUCCESS/FAILURE/ABORTED). 실행 중이면 빈 값이다.
	Result   string
	Building bool
	// StartedAt 은 빌드 시작 시각, Duration 은 실행 시간이다.
	// 실행 중인 빌드의 Duration 은 0 이다.
	StartedAt time.Time
	Duration  time.Duration
	// Stages 는 실행 안의 단계다. CI 가 단계 정보를 주지 않으면 비어 있다 —
	// 비어 있는 것과 "모두 성공" 은 다르다.
	Stages []CIStage
}

// CIBuildReader 는 CI 서버에서 빌드 이력을 읽는다.
//
// 플랫폼이 직접 배포하는 경로와 달리, GitOps 경로의 실행 기록은 CI 서버가
// 갖고 있다. 이것을 들이지 않으면 빌드가 성공해도 화면의 실행 통계가
// 영원히 0 으로 남는다.
type CIBuildReader interface {
	ListBuilds(ctx context.Context, jobName, branch string, limit int) ([]CIBuild, error)
}
