# Nullus Platform — Air-Gap 이미지 목록 관리

## 개요

`images.txt`는 Nullus Platform을 **인터넷이 차단된(air-gap) 환경**에 배포하기 위해 사전에 수집해야 하는 컨테이너 이미지의 전체 목록입니다. 이 파일을 기반으로 `airgap/scripts/` 하위의 스크립트들이 이미지 번들을 생성하고 대상 클러스터에 적재합니다.

---

## images.txt 구성

파일은 네 섹션으로 구분되며, 전부 `airgap/scripts/00-generate-images.sh` 가 생성합니다(손으로 고치면 재생성 때 사라집니다).

| 섹션 | 내용 |
|------|------|
| **Nullus 앱 + Chart 의존성** | `nullus-api`, `nullus-web` 과 Nullus 차트 의존성 이미지 — `helm template` 렌더 결과 |
| **카탈로그** | 스택 설치가 쓰는 OSS 차트(GitLab·Harbor·Argo CD·SonarQube 등)를 설치 values 로 렌더한 이미지 |
| **인프라** | `kindest/node`(kind 클러스터 노드), `registry:2`(로컬 미러 레지스트리), 차트 밖에서 지정하는 OpenBao·Jenkins 이미지 |
| **런타임 매니페스트** | Go 코드가 만드는 Job·파드와 **생성된 파이프라인의 CI 잡**(GitLab build·deploy, Jenkins 에이전트, SAST 스캐너)이 쓰는 이미지. 단일 출처는 `internal/shared/domain/runtime_images.go` 의 `RuntimeImages()` |

파이프라인 잡 이미지는 이름을 바꾸지 않습니다 — 폐쇄망 노드의 containerd 미러가 내부 레지스트리에서 내줍니다. 목록에 없는 잡 이미지는 첫 파이프라인 실행에서 `ImagePullBackOff` 로 멈춥니다(`internal/cicd/adapter/scaffold/airgap_images_test.go` 가 막습니다).

빈 줄과 `#`으로 시작하는 줄은 스크립트에서 자동으로 무시됩니다.

---

## Chart 의존성 업데이트 후 이미지 목록 재생성 방법

Bitnami postgresql 차트 버전을 올리거나 다른 차트를 추가할 경우 아래 절차로 이미지 태그를 갱신합니다.

```bash
# 1. Helm 의존성 업데이트 (인터넷 접속 가능한 환경에서 실행)
cd deploy/helm/nullus
helm dep update

# 2. postgresql 차트 values에서 이미지 태그 확인
tar xOf charts/postgresql-<version>.tgz postgresql/values.yaml \
  | grep -A2 'repository: bitnami/'

# 3. 출력 결과를 바탕으로 airgap/images/images.txt 의
#    "Chart 의존성" 섹션 태그를 수동으로 갱신

# 4. Chart.lock의 digest와 실제 차트 파일 sha256 일치 여부 확인
sha256sum charts/postgresql-<version>.tgz
```

> **주의**: `postgres-exporter` 이미지는 `postgresql.metrics.enabled: true`일 때만 실제로 사용됩니다. 기본값은 disabled이므로 번들에서 제외해도 되지만, 나중을 위해 목록에 포함해 둡니다.

---

## Pull → Save → Load 워크플로우

### 단계 1: 이미지 Pull (인터넷 접속 가능 환경)

```bash
cd airgap
bash scripts/01-pull-images.sh
```

- `images.txt`의 모든 이미지를 로컬 Docker daemon으로 pull합니다.
- 일부 이미지 pull 실패 시 나머지를 계속 진행하고, 완료 후 실패 목록을 출력합니다.
- `PODMAN=1` 환경변수를 설정하면 `podman pull`을 사용합니다.
- `DRY_RUN=1`로 실제 pull 없이 실행될 명령만 확인할 수 있습니다.

### 단계 2: Bundle 생성 (인터넷 접속 가능 환경)

```bash
bash scripts/02-save-bundle.sh
```

- 모든 이미지를 `airgap/bundle/images.tar.gz`로 저장합니다.
- `bundle/images.tar.gz.sha256`: SHA-256 체크섬 파일 (무결성 검증용)
- `bundle/MANIFEST.txt`: `image@digest` 형식의 이미지 목록

생성된 `bundle/` 디렉토리를 USB, S3, 내부망 파일서버 등으로 air-gap 환경에 전달합니다.

### 단계 3: 이미지 Load (air-gap 환경)

```bash
bash scripts/03-load-bundle.sh
```

- SHA-256 체크섬 검증 후 `docker load`로 이미지를 복원합니다.
- 검증 실패 시 즉시 중단합니다.

### 전체 흐름 요약

```
[인터넷 환경]                        [air-gap 환경]
  01-pull-images.sh                     03-load-bundle.sh
       ↓                                      ↑
  02-save-bundle.sh  →  bundle/ 전달  →  bundle/
```

---

## 운영자 확인 사항

- `kindest/node:v1.30.0` 버전은 실제 배포 대상 Kubernetes 버전과 일치해야 합니다. 변경 시 `images.txt`에서 직접 수정하세요.
- `registry:2`는 air-gap 환경 내부에서 이미지를 서빙하기 위한 Docker Distribution v2 레지스트리입니다.
- Bitnami 이미지는 `docker.io/bitnami/` prefix로 명시되어 있습니다. 내부 레지스트리 미러로 재태깅 시 prefix를 교체하세요.
