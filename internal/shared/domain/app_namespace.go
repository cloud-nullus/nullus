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

	// AppImagePullSecretName 은 앱 네임스페이스의 레지스트리 pull 자격증명이다. 스캐폴딩된
	// Deployment 가 imagePullSecrets 로 이 이름을 참조한다.
	AppImagePullSecretName = "nullus-registry" // #nosec G101 -- Secret 리소스 이름
)
