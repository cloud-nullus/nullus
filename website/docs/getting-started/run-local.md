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

!!! note "헷갈리기 쉬운 네 가지"
    - `make build` 는 **Go 바이너리만** 만듭니다 — `bin/api` · `bin/nullus-bootstrap`.
      프런트엔드 산출물(`web/dist`)은 `make web-build` 입니다. `cmd/nullus`(CLI)는
      어느 타깃도 빌드하지 않습니다
    - `make lint` 는 **Go 만** 봅니다. 프런트엔드는 `cd web && npm run lint`
    - `make test` 는 빌드 태그가 없어 `integration` · `e2e` 테스트가 **빠집니다**.
      그쪽은 `make test-integration`
    - `make test-golden-path` 는 kind 클러스터에 실제로 배포하는 **60분** E2E 입니다

## 프런트엔드 명령

```bash
cd web
npm run dev          # Vite 개발 서버
npm run build        # tsc -b && vite build
npm run lint         # eslint .
npx vitest run       # 단위 테스트
npm run e2e          # Playwright (chromium)
```

!!! note "`test` 스크립트는 없습니다"
    단위 테스트는 `npx vitest run`(watch 는 인수 없이 `npx vitest`)으로 돌립니다.
    `lint:fix` 도 없어서 자동 수정은 `npx eslint . --fix` 입니다. Prettier 는
    의존성이 아니라 `npx prettier --write .` 로 받아 씁니다 — CI 의 포맷 검사는
    자체 prettier 를 씁니다.

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

환경변수 이름은 설정 키를 그대로 따릅니다 — `database.host` 는
`NULLUS_DATABASE_HOST` 입니다. `.env.example` · `.env.dev` · `Makefile` 이 이
이름을 씁니다.

!!! note "옛 `NULLUS_DB_*` 는 먹지 않습니다"
    `NULLUS_DB_HOST` 처럼 줄여 쓴 이름은 어느 설정 키와도 맞지 않아 **조용히
    무시됩니다.** 손에 있는 `.env` 가 그 이름을 쓰고 있으면 포트나 비밀번호를
    바꿔도 반영되지 않습니다 — `NULLUS_DATABASE_*` 로 바꾸세요.
    (`internal/shared/config/database_env_test.go` 가 두 경우를 함께 고정합니다.)

    `NULLUS_MINIO_ENDPOINT` · `_ACCESS_KEY` · `_SECRET_KEY` 세 줄은 설정 구조에
    해당 항목이 아예 없어 완전히 쓰이지 않습니다.

올바른 이름은 [환경 변수](../reference/config.md)에 정리해 두었습니다.
또한 `make run`/`run-dev` 가 읽는 파일은 `.env` 가 아니라 **`.env.dev`** 입니다.

!!! note "백엔드 핫 리로드는 설정돼 있지 않습니다"
    세팅 가이드가 Air 설치를 안내하지만 `.air.toml` 이 저장소에 없고 어떤
    스크립트도 `air` 를 부르지 않습니다.
