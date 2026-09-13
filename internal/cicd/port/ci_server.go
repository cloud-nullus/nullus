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
	// Number 는 화면에 보이는 실행 번호다(Jenkins 빌드 번호, GitLab iid, GitHub run_number).
	Number int
	// ID 는 CI API 가 실행을 가리키는 식별자다(GitLab pipeline id, GitHub run id).
	// 번호와 다를 수 있고, 산출물 조회는 이것으로 한다.
	ID string
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

// CIBuildTrigger 는 CI 서버의 job 을 지금 실행시킨다.
//
// webhook 은 커밋이 있을 때만 돈다. 사용자가 화면에서 "배포 실행" 을 누르는
// 것은 커밋 없이 지금 실행하겠다는 뜻이라, 트리거가 따로 필요하다.
type CIBuildTrigger interface {
	TriggerBuild(ctx context.Context, jobName, branch string) error
}

// CIBuildReader 는 CI 서버에서 빌드 이력을 읽는다.
//
// 플랫폼이 직접 배포하는 경로와 달리, GitOps 경로의 실행 기록은 CI 서버가
// 갖고 있다. 이것을 들이지 않으면 빌드가 성공해도 화면의 실행 통계가
// 영원히 0 으로 남는다.
type CIBuildReader interface {
	ListBuilds(ctx context.Context, jobName, branch string, limit int) ([]CIBuild, error)
}

// 이미지 스캔 리포트 산출물의 이름이다. 스캐폴딩이 파이프라인에 이 이름으로 남기고
// 실행 기록 동기화가 같은 이름으로 읽는다 — 한쪽만 바꾸면 건수가 조용히 빈다.
const (
	// ImageScanReportArtifact 는 산출물을 이름으로 묶는 CI(GitHub upload-artifact)의 묶음 이름이다.
	ImageScanReportArtifact = "trivy-report"
	// ImageScanReportFile 은 리포트 파일 경로다.
	ImageScanReportFile = "trivy-report.json"
)

// MaxCIArtifactBytes 는 산출물 파일 하나를 읽을 때의 상한이다.
//
// 산출물은 사용자 파이프라인이 만든다. 크기를 믿고 통째로 메모리에 올리지 않는다.
// Trivy JSON 리포트는 큰 이미지도 수 MB 수준이다.
const MaxCIArtifactBytes int64 = 32 << 20

// CIArtifactRef 는 읽을 산출물의 위치다.
//
// CI 마다 산출물을 묶는 단위가 다르다 — Jenkins 는 빌드, GitLab 은 잡, GitHub 은
// 실행 안의 이름 붙은 묶음. 어댑터가 필요한 것을 골라 쓰도록 실행과 단계를 통째로 넘긴다.
type CIArtifactRef struct {
	JobName string
	Branch  string
	Build   CIBuild
	Stage   CIStage
	// Name 은 산출물 묶음 이름, Path 는 그 안의 파일 경로다.
	Name string
	Path string
}

// CIArtifactReader 는 실행이 남긴 산출물 파일을 읽는다.
type CIArtifactReader interface {
	// ReadArtifact 는 파일 내용을 돌려준다. 없으면 found=false 이고 오류가 아니다 —
	// 리포트가 없는 실행(스캐너에 닿지 못함, 보존 기간 만료)은 정상 경로다.
	ReadArtifact(ctx context.Context, ref CIArtifactRef) (data []byte, found bool, err error)
}
