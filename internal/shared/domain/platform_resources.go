package domain

// 플랫폼이 설치하는 공유 인프라의 Helm 릴리스 이름.
//
// 여기 있는 이유는 두 모듈이 같은 이름을 봐야 하기 때문이다 — stack 은 이
// 이름으로 차트를 설치하고 접속 정보를 안내하고, admin 의 시크릿 회전은 같은
// 이름으로 워크로드를 재시작한다. 모듈끼리 서로의 internal 을 참조할 수 없으므로
// 이름의 단일 출처를 shared 에 둔다.
//
// Helm 은 릴리스명을 그대로 Service 이름에 쓰므로, 이 값이 곧 클러스터 안의
// 접속 주소이기도 하다. 한쪽만 바꾸면 다른 쪽이 조용히 어긋난다.
const (
	// PostgresReleaseName 은 플랫폼이 설치하는 PostgreSQL 릴리스다.
	PostgresReleaseName = "nullus-postgresql"
	// MinIOReleaseName 은 플랫폼이 설치하는 MinIO 릴리스다.
	MinIOReleaseName = "nullus-minio"
)
