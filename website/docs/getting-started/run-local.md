# 로컬에서 띄우기

플랫폼(API + 웹)만 띄워 화면을 보는 최단 경로입니다. 클러스터에 스택을 실제로
설치하는 전 과정은 [E2E 가이드](e2e.md)입니다.

## 가장 짧은 길

```bash
./scripts/runbook_local.sh preflight   # 도구 확인
./scripts/runbook_local.sh up          # 인프라 + 마이그레이션 + API + 프런트엔드
```

끝나면 확인합니다.

```bash
curl http://localhost:8090/health
# {"status":…, "db":"connected", "version":…}
```

`db` 가 `connected` 까지 나와야 제대로 뜬 것입니다.

| 주소 | 무엇 |
|------|------|
| `http://localhost:8090` | API |
| `http://localhost:5173` | 웹 (Vite) |
| `http://localhost:8180` | Keycloak |

프런트엔드는 API 주소를 환경변수로 받지 않습니다. `/api/v1` 상대 경로로 요청하고
Vite 가 `/api` 와 `/ws` 를 `localhost:8090` 으로 넘깁니다.

## 손으로 하는 길

```bash
make dev        # Docker Compose 기동 + 마이그레이션 적용
make run        # bin/api 빌드 후 실행  → http://localhost:8090
make web-dev    # Vite 개발 서버        → http://localhost:5173
```

`make dev` 가 띄우는 컨테이너는 **넷**입니다 — PostgreSQL · 오브젝트 스토리지 ·
Redis · **Keycloak**.

!!! note "온보딩 문서는 Keycloak 을 빼고 셋이라고 적습니다"
    `make dev-up` 은 서비스 이름을 지정하지 않고 전부 올리므로 Keycloak 도 함께
    뜹니다.

!!! note "오브젝트 스토리지 서비스 이름은 `minio` 지만 구현체는 SILO 입니다"
    MinIO 의 AGPL 포크(`pgsty/silo`)를 씁니다. 환경변수도 `MINIO_ROOT_*` 를 그대로
    쓰므로(호환) 이름만 보고 MinIO 라고 오해하기 쉽습니다. `make dev` 가
    출력하는 배너도 아직 "MinIO" 라고 적습니다.

## 자주 쓰는 make 타깃

| 타깃 | 하는 일 |
|------|---------|
| `dev-up` · `dev-down` · `dev-clean` | Compose 기동 / 종료 / 볼륨까지 삭제 |
| `dev-status` · `dev-logs` | 상태 / 로그 따라가기 |
| `dev` | `dev-up` + `migrate-up` |
| `build` | `bin/api` 와 `bin/nullus-bootstrap` 빌드 |
| `run` · `run-dev` | `.env.dev` 를 읽어 API 실행 |
| `test` | `go test ./...` — **태그 없는 테스트만** |
| `test-integration` | `go test -tags integration ./e2e/` (testcontainers 로 Postgres 를 직접 띄움) |
| `test-golden-path` | **kind `nullus-platform` 에 실제로 배포하는 60분 E2E** |
| `test-cover` | `coverage.out` · `coverage.html` |
| `lint` | `golangci-lint run ./...` — **Go 만** |
| `migrate-up` · `migrate-down` · `migrate-status` | 마이그레이션 |
| `db-shell` | `psql` 접속 |
| `web-dev` · `web-build` · `web-test` | 프런트엔드 |
| `clean` | `bin/` · `coverage.*` · `web/dist` 삭제 |

!!! warning "문서가 틀리게 적고 있는 것들"
    - `make build` 는 `dist/` 를 만들지 않습니다. `bin/api` 와
      `bin/nullus-bootstrap` 둘뿐입니다 (`dist/` 는 프런트엔드 산출물 경로)
    - `make lint` 는 eslint 를 돌리지 않습니다. 프런트엔드는
      `cd web && npm run lint`
    - `make test` 는 "통합 테스트 전체" 가 아닙니다. 빌드 태그가 없어
      `integration` · `e2e` 테스트가 **빠집니다**
    - `make test-golden-path` 는 "템플릿 관련 테스트" 가 아니라 kind 클러스터에
      실제로 배포하는 60분 E2E 입니다

## 프런트엔드 명령

```bash
cd web
npm run dev          # Vite 개발 서버
npm run build        # tsc -b && vite build
npm run lint         # eslint .
npx vitest run       # 단위 테스트
npm run e2e          # Playwright (chromium)
```

!!! warning "`npm run test` 는 없습니다"
    `CONTRIBUTING.md` 가 `npm run test -- <파일>` 과 `npm run test -- --watch`
    를 쓰라고 적지만 그 스크립트가 **없습니다.** `npx vitest run` 또는
    `npx vitest` 를 쓰세요.

    같은 문서의 `npm run lint:fix` · `npm run eslint -- --fix` ·
    `npm run prettier -- --write` 도 모두 없는 스크립트입니다. Prettier 는
    의존성에도 들어 있지 않습니다.

그 밖에 있는 스크립트: `preview` · `e2e:visual` · `e2e:visual:update` ·
`e2e:headed` · `e2e:report` · `e2e:stack-critical` · `e2e:shots` ·
`inventory[:check]` · `theme:generate[:check]` · `responsive:audit[:check]`

## 데이터베이스

| 항목 | 값 |
|------|-----|
| 도구 | golang-migrate (이미지에는 v4.20.1, 로컬은 `go install` 로 자동 설치) |
| 위치 | `db/migrations/` — up/down 쌍 96개, 번호 `000001`~`000098` |
| 로컬 적용 | `make migrate-up` |
| 접속 문자열(로컬 기본) | `postgres://nullus:nullus_dev@localhost:5433/nullus?sslmode=disable` |

Kubernetes 에 배포할 때는 **API 가 스스로 돌리지 않습니다.** 차트의 pre-upgrade
훅 Job 이 API 이미지로 `migrate` 를 실행합니다(`migration.enabled` 기본 `true`).

!!! note "Compose 의 Postgres 는 마이그레이션을 실행하지 않습니다"
    `db/migrations` 를 마운트하지만 하위 디렉터리라 초기화 스크립트가 무시합니다.
    적용은 항상 `migrate` 가 합니다.

## 환경변수를 바꿀 때

!!! danger "`.env.example` 의 DB 변수는 읽히지 않습니다"
    `.env.example` 과 `.env.dev` 에 들어 있는
    `NULLUS_DB_HOST` · `NULLUS_DB_PORT` · `NULLUS_DB_NAME` · `NULLUS_DB_USER` ·
    `NULLUS_DB_PASSWORD` · `NULLUS_DB_SSLMODE` 여섯 개는 **코드가 보지
    않습니다.** 설정 키가 `database.*` 라서 이름이 `NULLUS_DATABASE_*` 여야
    합니다.

    `make run` 이 지금 DB 에 붙는 것은 `configs/config.yaml` 의 기본값이 우연히
    같은 값이기 때문입니다. **포트나 비밀번호를 바꾸면 조용히 깨집니다.**

    `NULLUS_MINIO_ENDPOINT` · `_ACCESS_KEY` · `_SECRET_KEY` 세 줄은 설정 구조에
    해당 항목이 아예 없어 완전히 쓰이지 않습니다.

올바른 이름은 [환경 변수](../reference/config.md)에 정리해 두었습니다.
또한 `make run`/`run-dev` 가 읽는 파일은 `.env` 가 아니라 **`.env.dev`** 입니다.

!!! note "백엔드 핫 리로드는 설정돼 있지 않습니다"
    세팅 가이드가 Air 설치를 안내하지만 `.air.toml` 이 저장소에 없고 어떤
    스크립트도 `air` 를 부르지 않습니다.
