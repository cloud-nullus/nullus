# 준비물

아래 버전은 README 가 아니라 **`go.mod` · `package.json` · CI 워크플로 ·
Dockerfile · 차트**에서 직접 확인한 값입니다.

## 필수

| 도구 | 필요한 값 | 근거 |
|------|----------|------|
| **Go** | **1.26.1** 이상 | `go.mod` 가 `go 1.26.1`. CI 는 `go-version-file: go.mod` 로 설치하고, 이미지 빌더는 `golang:1.26.9-alpine` |
| **Node** | **22** | CI 가 `node-version: '22'` 로 고정. 웹 이미지도 `node:22-alpine`. `package.json` 에 `engines` 필드는 없습니다 |
| **Docker / Docker Compose** | 버전 하한을 코드가 강제하지 않음 | 로컬 준비 점검 스크립트가 **존재만** 확인하고 버전은 출력만 합니다 |
| **PostgreSQL** | 아래 표 참고 | — |

!!! warning "README 의 `Go 1.24+` 는 낡았습니다"
    README 와 `CLAUDE.md` 가 아직 `Go 1.24+` 라고 적고 있지만 `go.mod` 는
    1.26.1 을 요구합니다. CI 워크플로에도 그 불일치로 한 번 깨졌던 이력이
    주석으로 남아 있습니다.

## PostgreSQL — 하나의 숫자가 아닙니다

쓰는 자리마다 다릅니다.

| 자리 | 버전 |
|------|------|
| CI 서비스 컨테이너 | **18** |
| 로컬 Docker Compose | **17** |
| Helm 차트의 서브차트 | Bitnami postgresql **16.7.21** (PostgreSQL 17.5 계열) |
| API 이미지에 들어 있는 클라이언트 | `postgresql17-client` |

!!! warning "README 의 `PostgreSQL 18+` 를 그대로 믿지 마세요"
    18 은 **CI 에서만** 씁니다. "18 이상" 을 요구하면 차트 기본 배포가 그
    요구를 못 맞춥니다. 로컬 개발은 17 로 맞추는 게 안전합니다.

## Kubernetes 쪽 (E2E 를 돌릴 때)

| 도구 | 값 |
|------|-----|
| **kind 노드 이미지** | `kindest/node:v1.35.1` |
| API 이미지의 kubectl | v1.34.12 |
| API 이미지의 Helm | v3.22.0 |
| CI 검사용 Helm | v3.16.4 |
| 에어갭 OCI(plain-http) | Helm **3.14 이상** 필요 |

!!! warning "`Kubernetes 1.26+` 라는 표기는 근거가 없습니다"
    README 와 온보딩 문서가 1.26+ 라고 적지만, 실제로 검증하는 대상은
    **1.35.1** 이고 Helm 차트에는 `kubeVersion` 제약이 아예 없습니다. 즉 1.26
    지원을 뒷받침하는 코드 근거가 없습니다.

로컬 kind 클러스터는 **둘**입니다.

| 이름 | 노드 |
|------|------|
| `nullus-platform` | control-plane 1 + worker 2 |
| `nullus-develop` | control-plane 1 + worker 1 |

## 프레임워크 버전 (참고)

Vite `^8.0.0` · React `^19.2.4` · TypeScript `~5.9.3` · Tailwind `^4.2.4` ·
vitest `^4.1.0` · MUI `^9.3.1`

## 준비 점검

```bash
./scripts/runbook_local.sh preflight
```

`go` · `node` · `npm` · `docker` · `lsof` · `curl` 이 있는지 확인하고
(`kind` · `kubectl` · `helm` 은 선택), 각 버전을 출력합니다. 버전 하한을
강제하지는 않습니다.
