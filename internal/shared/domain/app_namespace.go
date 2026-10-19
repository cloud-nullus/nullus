package domain

// 앱 네임스페이스 — CI/CD 모듈이 파이프라인을 만들 때 앱을 배포할 자리로 만드는 네임스페이스.
//
// CI/CD(stack 밖, internal/cicd)가 만들고 백업(internal/backup)이 담아야 한다. 두 모듈은 서로 import
// 하지 않으므로 이름을 여기 둔다. 앱의 Deployment·Service·HTTPRoute 는 Argo CD 가 Git 에서 되살리지만,
// 이 네임스페이스 자체와 그 안의 이미지 pull Secret 은 Nullus 가 직접 만든 것이라 Git 에 없다 —
// 앱 네임스페이스를 잃으면 Deployment 는 돌아와도 pull 이 실패한다(2026-10-12 리허설 실측).
const (
	// AppManagedByLabel / AppManagedByCICD 는 CI/CD 모듈이 만든 리소스의 표시다.
	AppManagedByLabel = "app.kubernetes.io/managed-by"
	AppManagedByCICD  = "nullus-cicd"

	// AppImagePullSecretName 은 앱별 이름 이전에 만든 파이프라인의 pull 자격증명이다. 네임스페이스에
	// 하나라 같은 네임스페이스의 다음 앱이 덮어썼다. 그 전에 만든 Deployment 가 아직 이 이름을 참조하므로
	// 백업은 계속 담는다. 새 파이프라인은 AppImagePullSecretNameFor 를 쓴다.
	AppImagePullSecretName = "nullus-registry" // #nosec G101 -- Secret 리소스 이름

	// AppImagePullSecretType 은 pull 자격증명 Secret 의 종류다. 백업이 라벨과 함께 이것으로 고른다.
	AppImagePullSecretType = "kubernetes.io/dockerconfigjson" // #nosec G101 -- Secret 종류 이름
)

// AppImagePullSecretNameFor 는 앱 하나의 레지스트리 pull 자격증명 이름이다. 스캐폴딩된 Deployment 가
// imagePullSecrets 로 이 이름을 참조한다.
//
// 앱마다 따로 둔다. GitLab 프로젝트 레지스트리의 토큰은 그 프로젝트 이미지만 읽는다 — 이름이 네임스페이스에
// 하나면 같은 네임스페이스에 만든 다음 앱이 그 토큰을 덮어써 앞 앱의 pull 이 insufficient_scope 로
// 막힌다(2026-10-19 kind 실측).
func AppImagePullSecretNameFor(app string) string {
	return AppImagePullSecretName + "-" + app
}
