# Nullus Air-Gap Helm 번들

오프라인 kind 클러스터에 Nullus 플랫폼을 배포하기 위한 Helm 아티팩트 디렉터리입니다.

---

## 디렉터리 구조

```
airgap/helm/
├── values-airgap.yaml        # 오프라인 환경 전용 values 재정의
├── Chart.lock                # 20-bundle-charts.sh 실행 후 복사됨
├── charts/                   # 의존성 차트 (.tgz) 복사 위치
│   └── postgresql-16.7.21.tgz
└── nullus-0.1.0-alpha.tgz    # helm package 결과물 (번들 후 생성)
```

---

## values-airgap.yaml 주요 재정의

| 항목 | 기본값 (values.yaml) | Air-Gap 재정의 |
|------|----------------------|----------------|
| `api.image.repository` | `ghcr.io/cloud-nullus/nullus-api` | `localhost:5001/cloud-nullus/draft/nullus-api` |
| `api.image.tag` | `0.1.0-alpha` | `main` |
| `web.image.repository` | `ghcr.io/cloud-nullus/nullus-web` | `localhost:5001/cloud-nullus/draft/nullus-web` |
| `web.image.tag` | `0.1.0-alpha` | `main` |
| `postgresql.image.registry` | `docker.io` | `localhost:5001` |
| `postgresql.image.repository` | `bitnami/postgresql` | `bitnamilegacy/postgresql` (2025-08 Bitnami 정책 변경) |
| `postgresql.volumePermissions.image.registry` | `docker.io` | `localhost:5001` |
| `ingress.enabled` | `false` | `false` (kind 포트맵이 80/443 처리) |
| `api.resources` | `{}` | requests: 100m/128Mi, limits: 500m/512Mi |
| `web.resources` | `{}` | requests: 100m/128Mi, limits: 500m/512Mi |
| `secrets.dbPassword` | `change-me` | `CHANGE-ME` (반드시 교체) |
| `secrets.encryptionKey` | `change-me-32-bytes-minimum` | `CHANGE-ME` (반드시 교체) |

### 이미지 매핑 요약

| 원본 이미지 | localhost:5001 이미지 |
|-------------|----------------------|
| `ghcr.io/cloud-nullus/draft/nullus-api:main` | `localhost:5001/cloud-nullus/draft/nullus-api:main` |
| `ghcr.io/cloud-nullus/draft/nullus-web:main` | `localhost:5001/cloud-nullus/draft/nullus-web:main` |
| `docker.io/bitnamilegacy/postgresql:17.5.0-debian-12-r20` | `localhost:5001/bitnamilegacy/postgresql:17.5.0-debian-12-r20` |
| `docker.io/bitnamilegacy/os-shell:12-debian-12-r49` | `localhost:5001/bitnamilegacy/os-shell:12-debian-12-r49` |

> 태그는 `airgap/images/images.txt` 와 항상 동기화 상태여야 합니다.

---

## 번들 생성 절차 (연결된 머신)

```bash
# 1. 이미지 pull & save (Agent 1 담당)
./airgap/scripts/01-pull-images.sh

# 2. Helm 차트 번들 생성
./airgap/scripts/20-bundle-charts.sh

# 3. airgap/ 전체를 에어갭 환경으로 전달
```

`20-bundle-charts.sh` 는 다음을 수행합니다.

1. `helm dep update deploy/helm/nullus` — 의존성 차트를 `deploy/helm/nullus/charts/` 에 다운로드
2. `*.tgz` 와 `Chart.lock` 을 `airgap/helm/charts/` 로 복사
3. `helm package deploy/helm/nullus -d airgap/helm/` — 전체 차트를 단일 `.tgz` 로 패키징
4. `nullus-<appVersion>.tgz.sha256` 체크섬 파일 생성

---

## chart 의존성 업그레이드 방법

PostgreSQL 차트 버전을 올릴 때는 아래 순서로 진행합니다.

1. `deploy/helm/nullus/Chart.yaml` 의 `postgresql` version 수정
2. 연결된 머신에서 `helm dep update deploy/helm/nullus` 실행
3. 새 postgresql chart 의 `values.yaml` 에서 이미지 태그 확인
4. `airgap/images/images.txt` 의 `bitnamilegacy/postgresql`, `bitnamilegacy/os-shell` 태그 갱신
5. `airgap/helm/values-airgap.yaml` 의 `postgresql.image.tag`, `volumePermissions.image.tag` 갱신
6. `20-bundle-charts.sh` 재실행

---

## 설치 (에어갭 kind 클러스터)

```bash
# secrets 를 직접 지정하는 경우
EXTRA_ARGS="--set secrets.dbPassword=실제패스워드 --set secrets.encryptionKey=32자이상키" \
  ./airgap/scripts/21-install-nullus.sh

# 기본 실행 (secrets 는 values-airgap.yaml 의 CHANGE-ME — 개발/테스트 전용)
./airgap/scripts/21-install-nullus.sh
```

> **운영 환경**: `--set` 대신 Sealed Secrets 또는 External Secrets Operator 를 사용하세요.
