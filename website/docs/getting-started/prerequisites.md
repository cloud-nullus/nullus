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

## PostgreSQL — 하나의 숫자가 아닙니다

쓰는 자리마다 다릅니다.

| 자리 | 버전 |
|------|------|
| CI 서비스 컨테이너 | **18** |
| 로컬 Docker Compose | **17** |
| Helm 차트가 깔아 주는 것 | **17.5.0** (Bitnami postgresql 차트 16.7.21 — 차트 버전이고 PG 버전이 아닙니다) |
| API 이미지에 들어 있는 클라이언트 | `postgresql17-client` |

**로컬 개발은 17 로 맞추세요.** 18 은 CI 에서만 씁니다 — 테스트가 18 에서도
도는지 보는 쪽이고, 차트가 깔아 주는 것은 17.5 입니다.

## Kubernetes 쪽 (E2E 를 돌릴 때)

| 도구 | 값 |
|------|-----|
| **kind 노드 이미지** | `kindest/node:v1.35.1` |
| API 이미지의 kubectl | v1.34.12 |
| API 이미지의 Helm | v3.22.0 |
| CI 검사용 Helm | v3.16.4 |
| 에어갭 OCI(plain-http) | Helm **3.14 이상** 필요 |

**지원 범위의 근거는 클라이언트 라이브러리입니다.** 서버는 `client-go v0.35.2`
로 부르고, client-go 의 공식 호환 범위는 같은 마이너 ±1 — 즉 **1.34~1.36** 입니다.
차트가 쓰는 리소스(`apps/v1` · `batch/v1` · `networking.k8s.io/v1`)는 더 낮은
버전에서도 있지만, 차트에 `kubeVersion` 제약이 없어 더 낮은 서버를 막지는
않습니다. 실제로 돌려 보는 것은 kind **v1.35.1** 하나입니다.

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
