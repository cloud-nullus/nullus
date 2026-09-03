# Nullus 백업/복구 로컬 테스트 가이드

> 로컬에서 **설치부터 백업·복구까지** 한 번에 확인하는 절차다.
> 설계: [`docs/11_기능설계/Nullus_백업복구_설계.md`](../11_기능설계/Nullus_백업복구_설계.md) · EPIC: [nullus-plan#75](https://github.com/cloud-nullus/nullus-plan/issues/75)

작성일: 2026-09-02 · 개정: 2026-09-03 (실환경 리허설 반영)

**이 문서는 운영 런북이 아니다.** 운영 런북(B3-4)은 별도이며, 여기서는 *개발자가 자기 기계에서* 백업/복구가 실제로 도는지 확인한다.
플랫폼 자체 기동은 [로컬 테스트 가이드](./Nullus_로컬_테스트_가이드.md)가 다루므로 중복하지 않고 참조한다.

---

## 0. 먼저 — 세 갈래 중 하나를 고른다

목적에 따라 드는 시간이 다르다. **위에서부터 시도하는 것을 권한다** — 아래로 갈수록 손이 많이 가고, 위쪽이 실패하면 아래쪽도 실패한다.

| | 무엇을 보나 | 시간 | 필요한 것 |
|---|---|---|---|
| **A. 자동 리허설** | 정지 → 아카이브 → 파괴 → 복원 → 지문 대조 **전 과정** | **~90초** | Docker, kind, `pg_dump` |
| **B. 통합 테스트** | 저장 어댑터(PostgreSQL) · 목적지(MinIO) 각각 | ~20초 | Docker |
| **C. 손으로 전 과정** | API·설정·화면까지 포함한 실제 사용 흐름 | 30분+ | 위 전부 + 로컬 플랫폼 |

C 는 "실제로 이렇게 쓰인다" 를 확인할 때만 하면 된다. **회귀 확인이 목적이면 A 로 충분하다.**

### 0.1 실환경에서 확인된 것 (2026-09-03)

A 는 *메커니즘* 을 본다. 그것만으로는 부족했다 — **A 와 통합 테스트를 모두 통과하고 CI 도 초록인 코드에서 결함 5건이 실제 스택 위에서 나왔다.** 그래서 스택(Gitea·Jenkins·Harbor·Argo CD)을 설치하고 React 파이프라인을 돌린 상태에서 C 를 끝까지 수행했고, 결과를 여기 적어 둔다.

| 측정 | 값 |
|---|---|
| 백업 | `succeeded` · 371,609,845 bytes · 산출물 13건 |
| 산출물 구성 | `platform_db` 1 · `keycloak_db` 1 · `openbao_kv` 1 · `ns_resources` 1 · `volume` 9 |
| **정지 창** | **50.6초** |
| 복구 후 기동 | **41초에 25/26 파드**, 최종 비정상 0 |
| 지문 대조 | Git 커밋·HEAD SHA·커밋 메시지 · Harbor digest·태그 · Jenkins 빌드 이력 **6/6 일치** |

복구 뒤 새 소스 커밋을 넣어 **파이프라인이 전 구간 다시 도는 것**까지 확인했다(체크아웃 → 빌드 → Harbor push → CD 되커밋). 애플리케이션도 Argo CD 가 Git 을 읽어 올바른 태그로 배포했다.

> **왜 이 항목들을 재는가.** 복구가 `succeeded` 를 반환하는 것과 스택이 동작하는 것은 다른 이야기다. 실제로 `RestoreRun.status = succeeded` 인데 Gitea·Harbor·Jenkins 가 `CreateContainerConfigError` 로 멈춰 있던 적이 있다 — 배선(ESO CR)과 권한(ClusterRole)이 백업에서 빠져 있었기 때문이다. **상태값이 아니라 데이터를 봐야 한다.**

---

## 1. 사전 요구사항

[로컬 테스트 가이드 §1](./Nullus_로컬_테스트_가이드.md) 에 더해 아래가 필요하다.

| 도구 | 왜 |
|---|---|
| **`pg_dump` / `pg_restore`** (서버와 같은 major) | 백업/복구가 이 바이너리를 exec 한다. 없으면 `pg_dump: not found` 로 **조용히** 실패한다 |
| **kind** | 볼륨 정지 백업은 실제 클러스터가 있어야 한다 |
| Docker | testcontainers(PostgreSQL·MinIO) |

### 1.1 `pg_dump` 설치 — macOS 에서 걸리는 지점

Homebrew 의 `libpq` 는 **keg-only** 라 PATH 에 안 잡힌다. 설치해도 `which pg_dump` 가 못 찾는다.

```bash
brew install libpq
export PATH="/opt/homebrew/opt/libpq/bin:$PATH"   # Intel Mac: /usr/local/opt/libpq/bin
pg_dump --version                                  # pg_dump (PostgreSQL) 18.x
```

> **major 만 맞으면 된다.** 클라이언트 18.3 으로 서버 18.4 를 덤프해도 문제없다 — PostgreSQL 의 호환 규칙이 major 기준이다. major 가 낮으면(예: 16 클라이언트 / 17 서버) 코드가 먼저 막는다.

### 1.2 Go 가 PATH 에 없는 경우

이 저장소는 `go.mod` 의 toolchain 을 쓴다. 시스템에 Go 가 없으면 모듈 캐시의 것을 직접 가리킨다.

```bash
export PATH="$HOME/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.1.darwin-arm64/bin:$PATH"
go version   # go1.26.1
```

`go.mod` 의 Go 버전이 바뀌면 이 경로도 함께 바뀐다.

---

## 2. A — 자동 리허설 (권장)

설계 §10.2 의 **지문 대조** 방식을 그대로 자동화한 것이다. 백업 전에 심어 둔 증거(파일 sha256 · replica 수 · DB 행)가 복원 후에도 같은 값인지 본다.

```bash
# 1) kind 클러스터 (없으면 만든다)
kind create cluster --name nullus-develop --config scripts/kind-cluster.yaml
kind get kubeconfig --name nullus-develop > /tmp/nullus-kc.yaml

# 2) 실행
export NULLUS_REHEARSAL_KUBECONFIG=/tmp/nullus-kc.yaml
export PATH="/opt/homebrew/opt/libpq/bin:$PATH"
go test -tags rehearsal ./internal/backup/rehearsal/ -v -timeout 25m
```

PostgreSQL 과 MinIO 는 testcontainers 가 알아서 띄우고 지운다. 클러스터에는 `nullus-backup-rehearsal` 네임스페이스만 만들고 끝나면 지운다.

**통과하면 이런 출력이 나온다:**

```
지문: file=fingerprint.bin sha256=bd4e9a3b1201fda9… replicas=2 db_rows=3
[3.469s] 2. 백업 실행
실제 정지 창: 3.097s
산출물 5 건, 총 277923 bytes
[48.1s] 3. 환경 초기화
[6.39s] 4. 복구 실행
--- PASS: TestRecoveryRehearsal
```

> ⚠️ **여기 나오는 시간은 RTO 근거가 아니다.** 데이터가 256KiB 다. 설계 §2.2 의 RTO 4시간은 100Gi 급 전제이고, 실환경 리허설로만 실측할 수 있다. 이 리허설이 검증하는 것은 시간이 아니라 **절차가 성립하는가** 다.

### 2.1 무엇을 검증하고 무엇을 검증하지 않나

| 검증한다 | 검증하지 않는다 |
|---|---|
| 정지 → 파드 종료 대기 → 아카이브 → 재개 | 실제 OSS 데이터(GitLab 커밋 · Harbor digest · Jenkins 빌드) |
| 네임스페이스 **완전 파괴** 후 PVC 재생성 → 볼륨 복원 | OpenBao 금고 (리허설은 KV 를 no-op 으로 둔다) |
| 리소스 덤프/복원, replica 복원 | RTO·정지 창 실측 |
| 플랫폼 DB + Keycloak DB(같은 인스턴스, 별도 database) | 다중 노드 (RWO 볼륨이 노드에 흩어지는 경우) |
| 산출물 봉인/해제, sha256 무결성 | |

---

## 2A. A′ — 인클러스터 리허설 (위상 검증)

플랫폼을 차트로 kind 에 배포하고, **파드 안에서** 백업을 돌려 **클러스터 밖** 오브젝트 스토리지에 실제로 쓰이는지 본다.

```bash
./scripts/backup-rehearsal-incluster.sh
```

스크립트가 알아서 한다: 목적지 MinIO 를 kind 도커 네트워크에 띄우고(**클러스터 밖**), API 이미지를 빌드해 노드로 반입하고, 차트를 배포하고, 백업을 트리거한 뒤, **클러스터 밖에서** 산출물을 조회한다. 끝나면 정리한다(`--keep` 으로 남길 수 있다).

```
[  OK  ] 백업 모듈 준비 확인
[  OK  ] 백업 7b3a77cb-… — 73217 bytes
[  OK  ] egress 확인 — 파드가 클러스터 밖 스토리지에 2 건을 썼다
[  OK  ] 읽기 방향 확인 (verify)
```

**목적지를 클러스터 안에 두지 않는 것이 요점이다.** 안에 두면 리허설은 통과하지만 설계 §4.2 와 어긋난 위상을 "검증됐다" 고 기록하게 된다 — 클러스터가 죽을 때 백업본도 같이 죽는 구성이다.

> `kind load` 는 이 조합(kind 0.24 + Docker 28.x)에서 `failed to detect containerd snapshotter` 로 죽는다. 스크립트가 `ctr images import` 로 우회한다.

---

## 3. B — 통합 테스트

어댑터별로 실제 인프라에 붙여 본다. 리허설이 실패했을 때 **어느 계층인지** 좁히는 데 쓴다.

```bash
export TESTCONTAINERS_RYUK_DISABLED=true   # Docker Desktop 에서 정리 컨테이너가 막힐 때

# 저장 어댑터 — 실제 PostgreSQL 18 (JSONB·TEXT[]·CASCADE·nullable 왕복)
go test -tags integration ./internal/backup/adapter/repository/ -v

# 목적지 — 실제 MinIO (멀티파트·sha256 메타데이터·접두사 삭제 격리)
go test -tags integration ./internal/backup/adapter/store/ -v
```

클러스터가 없어도 도는 단위 테스트는 태그 없이 돈다:

```bash
go test ./internal/backup/...
```

---

## 4. C — 손으로 전 과정

실제 사용 흐름을 보고 싶을 때만 한다.

### 4.1 플랫폼 기동

```bash
./scripts/runbook_local.sh up --kind
```

이 명령이 띄우는 것 중 백업에 쓰이는 것:

| | 주소 | 용도 |
|---|---|---|
| PostgreSQL | `localhost:5433` (nullus / nullus_dev) | 플랫폼 DB — **백업 대상** |
| MinIO | `localhost:9000` (nullus / nullus_dev) | **백업 목적지** |
| API | `localhost:8090` | 백업 API |

> **로컬에서는 MinIO 가 목적지로 쓸 만하다.** 설계 §4.1 이 "MinIO 재사용 불가" 라고 한 것은 *스택 안에 설치된* MinIO 를 말한다 — 그것은 백업 대상과 같은 클러스터에 있고 자신도 백업 대상이다. 여기 MinIO 는 docker-compose 쪽이라 kind 클러스터와 실패 도메인이 다르므로 로컬 검증에는 맞는다. **운영에서는 클러스터 밖 오브젝트 스토리지를 써야 한다.**

### 4.2 버킷 생성

목적지 버킷은 미리 있어야 한다. 없으면 백업이 **정지 창에 들어가기 전에** 실패한다(설계 §9 F8 — 의도된 동작이다).

```bash
docker compose -f docker-compose.dev.yaml exec minio mc mb local/nullus-backup
```

`local` 별칭이 없다는 오류가 나면 먼저 등록한다:

```bash
docker compose -f docker-compose.dev.yaml exec minio \
  mc alias set local http://localhost:9000 nullus nullus_dev
```

### 4.3 Keycloak DB — 로컬에서 걸리는 지점

**로컬 Keycloak 은 H2 를 쓴다**(`docker-compose.dev.yaml` 의 `KC_DB: dev-file`). PostgreSQL 로 된 Keycloak DB 가 아예 없다.

그런데 백업은 `backup.keycloak_database.host` 가 비면 **기동을 거부한다** — 그것을 빠뜨리면 복구해도 아무도 로그인할 수 없기 때문이다(설계 §1.2). 로컬에서는 빈 database 를 하나 만들어 대역으로 쓴다.

```bash
docker compose -f docker-compose.dev.yaml exec postgres \
  psql -U nullus -d nullus -c 'CREATE DATABASE keycloak;'
```

> **이것은 대역일 뿐 실제 Keycloak 상태를 담지 않는다.** 로컬에서 "Keycloak 복원" 은 검증되지 않는다. 그 경로는 §2 자동 리허설(같은 인스턴스의 별도 database 구성)이나 실환경에서 확인한다.

### 4.4 백업 설정

`configs/config.yaml` 의 `backup:` 블록을 고치거나 환경변수로 준다. 환경변수는 `NULLUS_` 접두사에 `.` → `_` 규칙이다.

```bash
export NULLUS_BACKUP_ENABLED=true

# 산출물 암호화 키 — 정확히 32바이트.
# ENCRYPTION_KEY 및 DB 비밀번호와 **다른 값**이어야 한다(설계 §5.2).
# 같으면 키 하나를 잃을 때 둘 다 잃는다. 코드가 같은 값이면 기동을 막는다.
export NULLUS_BACKUP_SEAL_KEY='nullus-backup-seal-key-32bytes!!'

export NULLUS_BACKUP_DESTINATION_ENDPOINT=localhost:9000
export NULLUS_BACKUP_DESTINATION_BUCKET=nullus-backup
export NULLUS_BACKUP_DESTINATION_ACCESS_KEY=nullus
export NULLUS_BACKUP_DESTINATION_SECRET_KEY=nullus_dev
export NULLUS_BACKUP_DESTINATION_USE_SSL=false

export NULLUS_BACKUP_KEYCLOAK_DATABASE_HOST=localhost
export NULLUS_BACKUP_KEYCLOAK_DATABASE_PORT=5433
export NULLUS_BACKUP_KEYCLOAK_DATABASE_NAME=keycloak
export NULLUS_BACKUP_KEYCLOAK_DATABASE_USER=nullus
export NULLUS_BACKUP_KEYCLOAK_DATABASE_PASSWORD=nullus_dev

export PATH="/opt/homebrew/opt/libpq/bin:$PATH"   # pg_dump
./scripts/runbook_local.sh refresh
```

**`NULLUS_` 접두사가 없는 변수도 있다.** 이것들을 빠뜨리면 기동은 되는데 엉뚱한 곳에서 조용히 어긋난다:

```bash
# 클러스터 kubeconfig 를 푸는 키. 32바이트.
export ENCRYPTION_KEY='nullus-dev-key-32bytes-padding!!'
```

`ENCRYPTION_KEY` 가 없으면 kubeconfig 복호화가 실패하고, 설치기가 **executor 없이 시뮬레이션 모드로 떨어진다.** 로그에 이렇게 남는다:

```
WARN step executor is nil; running simulated install step
```

이때 스택은 **`state=completed`, `progress=100` 이 되는데 파드는 한 개도 뜨지 않는다.** 40초 만에 "설치 완료" 가 되고, 그 뒤 파이프라인 생성이 `ENCRYPTION_KEY must be 32 bytes` 로 깨지고서야 원인이 드러난다. 설치 직후 파드 수를 세는 습관이 이 시간을 아낀다:

```bash
kubectl -n <네임스페이스> get pods --no-headers | wc -l   # 0 이면 시뮬레이션이다
```

기동 로그에 아래가 보이면 준비된 것이다:

```
백업 모듈 준비 완료 destination=localhost:9000 bucket=nullus-backup
```

설정이 잘못되면 **기동 자체가 멈춘다.** 이것은 의도다 — 설정이 틀린 채로 뜨면 "백업이 돌고 있다" 는 착각만 남고, 그 착각은 복구를 시도할 때에야 깨진다.

### 4.5 대상 스택 준비

플랫폼 전용 백업만 볼 거라면 이 단계는 건너뛴다(§4.6 에서 `platform_only` 를 쓴다).

볼륨까지 보려면 kind 클러스터를 Nullus 에 등록하고 스택을 설치한다 — [로컬 테스트 가이드 §4.3 · §5](./Nullus_로컬_테스트_가이드.md) 참고. 전체 스택은 무겁고 시간이 오래 걸리므로, **정지 백업 메커니즘만 보려면 §2 자동 리허설이 훨씬 빠르다.**

### 4.6 백업 실행

```bash
# 플랫폼 전용 — 무중단이라 확인 문자열이 필요 없다
curl -sX POST localhost:8090/api/v1/admin/backups \
  -H 'Content-Type: application/json' \
  -H 'X-Org-Id: 00000000-0000-0000-0000-000000000001' \
  -d '{"mode":"platform_only"}' | jq

# 볼륨 포함 — 워크로드를 잠시 멈추므로 네임스페이스를 다시 입력해야 한다
curl -sX POST localhost:8090/api/v1/admin/backups \
  -H 'Content-Type: application/json' \
  -H 'X-Org-Id: 00000000-0000-0000-0000-000000000001' \
  -d '{"mode":"full","stack_id":"<스택 ID>","namespace":"nullus","confirm":"nullus"}' | jq
```

확인 문자열을 빼면 `BACKUP_CONFIRM_REQUIRED` 로 거절된다. **다운타임이 생긴다는 사실을 모르고 누르면 안 되기 때문이다.**

#### 대상을 항목별로 고르기

`mode` 는 preset 이고, 실제로 무엇을 뜰지는 `scope` 가 정한다. 다섯 항목을 개별로 고를 수 있다:

```bash
curl -sX POST localhost:8090/api/v1/admin/backups \
  -H 'Content-Type: application/json' \
  -H 'X-Org-Id: 00000000-0000-0000-0000-000000000001' \
  -d '{"mode":"full","namespace":"nullus",
       "scope":["platform_db","keycloak_db","openbao_kv","ns_resources"]}' | jq
```

| 값 | 무엇이 들어가나 | 빠지면 |
|---|---|---|
| `platform_db` | Nullus 메타데이터 (스택·파이프라인·클러스터) | 복구 후 플랫폼이 스택을 모른다 |
| `keycloak_db` | 계정과 권한 | 복구 후 로그인할 수 없다 |
| `openbao_kv` | 토큰·자격증명 | 도구들이 서로 인증하지 못한다 |
| `ns_resources` | 배선 (Deployment·Secret·CRD·ClusterRole·Argo CD Application) | 복구가 `succeeded` 인데 스택이 뜨지 않는다 |
| `volume` | OSS 내부 데이터 (Git 저장소·빌드 이력·이미지) | **이것만 서비스를 멈춘다** |

**확인 문자열은 `volume` 이 들어갈 때만 요구한다.** `mode` 가 `full` 이어도 볼륨을 빼면 무중단이므로 확인이 필요 없다 — 멈추지도 않는데 확인을 강요하면 그 문구는 의미 없는 절차가 되고, 정작 진짜 멈추는 백업에서도 습관적으로 넘기게 된다.

모르는 이름은 `BACKUP_INVALID_COMPONENT` 로 거절한다. 조용히 버리면 고른 것과 백업된 것이 달라지고, 그 사실은 복구할 때에야 드러난다.

화면으로는 **관리 → 백업**(`/admin/backup`)에서 같은 선택을 할 수 있다. 볼륨을 고르면 경고와 확인 입력이 뜨고, 빼면 사라진다.

응답의 `status` 를 본다:

| status | 뜻 |
|---|---|
| `succeeded` | 전부 성공 |
| `partial` | **일부만 성공.** `error` 필드가 어떤 컴포넌트가 빠졌는지 말한다. 쓸 수는 있지만 완전하지 않다 |
| `failed` | 산출물 없음 |

### 4.7 이력과 무결성 확인

```bash
BID=$(curl -s localhost:8090/api/v1/admin/backups \
  -H 'X-Org-Id: 00000000-0000-0000-0000-000000000001' | jq -r '.items[0].id')

# 상세 — 매니페스트와 산출물 목록
curl -s localhost:8090/api/v1/admin/backups/$BID \
  -H 'X-Org-Id: 00000000-0000-0000-0000-000000000001' | jq

# 무결성만 검증 (복원하지 않는다)
curl -sX POST localhost:8090/api/v1/admin/backups/$BID/verify \
  -H 'X-Org-Id: 00000000-0000-0000-0000-000000000001' | jq
```

`verify` 는 복구 리허설을 상시로 돌릴 수 없는 상황에서 **"이 백업이 열리기는 하는가"** 를 확인하는 최소한의 방어선이다.

목적지에 실제로 올라갔는지도 직접 본다:

```bash
docker compose -f docker-compose.dev.yaml exec minio \
  mc ls --recursive local/nullus-backup
```

매니페스트에 **비밀값이 없는지** 확인해 두면 좋다 — 매니페스트만 암호화하지 않기 때문이다(설계 §4.4).

```bash
curl -s localhost:8090/api/v1/admin/backups/$BID \
  -H 'X-Org-Id: 00000000-0000-0000-0000-000000000001' \
  | jq '.manifest' | grep -iE 'password|secret|token|access_key' && echo "⚠️ 비밀값 발견" || echo "OK"
```

**차트 버전이 기록됐는지** 도 본다. 복구가 다른 버전으로 재설치하면 도구들이 기동하며 스키마 마이그레이션을 돌리고, 그 순간 데이터는 백업 시점의 모습이 아니게 된다 — 되돌릴 수 없다.

```bash
curl -s localhost:8090/api/v1/admin/backups/$BID \
  -H 'X-Org-Id: 00000000-0000-0000-0000-000000000001' \
  | jq -r '.manifest.helm_releases[] | "\(.name)\t\(.version)\trev=\(.revision)"'
```

```
gitea     12.7.0   rev=1
harbor    1.15.0   rev=1
argo-cd   7.7.16   rev=1
openbao   0.28.4   rev=1
...
```

릴리스마다 리비전이 여러 개 쌓이지만 **가장 높은 리비전만** 담긴다 — 백업 시점에 실제로 돌던 버전이라야 의미가 있기 때문이다. 복구는 대상 네임스페이스의 현재 버전과 대조해 어긋나면 로그로 알린다(막지는 않는다: 버전을 올린 뒤 데이터만 회수하는 길이 있다).

### 4.8 파괴 후 복구

```bash
# 스택 네임스페이스를 통째로 지운다 (부분 초기화는 검증이 되지 않는다)
kubectl delete ns nullus

# 복구 — 파괴적이므로 백업 ID 를 다시 입력해야 한다
curl -sX POST localhost:8090/api/v1/admin/restores \
  -H 'Content-Type: application/json' \
  -H 'X-Org-Id: 00000000-0000-0000-0000-000000000001' \
  -d "{\"backup_run_id\":\"$BID\",\"namespace\":\"nullus\",\"mode\":\"full\",\"confirm\":\"$BID\"}" | jq
```

응답에서 두 가지를 본다:

- `schema_check.allowed` — 이 코드로 복원해도 되는 백업인가
- `integrity_report.dangling` — DB 가 가리키는데 금고에 없는 시크릿 경로. **비어 있지 않으면 해당 토큰을 재등록해야 한다.** 이 목록은 경고이지 중단 사유가 아니어서, 보지 않으면 나중에 파이프라인 인증 실패로만 드러난다

> **복구된 볼륨은 설치 preflight 가 막지 않는다.** 원래 이 가드는 이전 설치의 볼륨이 남아 있으면 설치를 세운다 — 새 설치가 옛 데이터베이스를 물려받으면 이번에 만든 비밀번호와 어긋나고, 그 사실이 20분 뒤 Gitea 의 `28P01` 이나 Harbor 의 `401` 로 드러나기 때문이다. 복구는 볼륨과 함께 그 시점의 Secret 과 금고까지 되돌리므로 어긋날 것이 없고, 그것을 구분하려고 복구가 PVC 에 출처를 남긴다:
>
> ```bash
> kubectl -n <네임스페이스> get pvc -o json \
>   | jq -r '.items[] | "\(.metadata.name)\t\(.metadata.annotations["backup.nullus.io/restored-from"] // "-")"'
> ```
>
> 남은 볼륨이 **전부 같은 백업에서 왔을 때만** 통과한다. 하나라도 출처가 없거나 서로 다르면 막는다 — 그때는 어느 시점의 상태인지 말할 수 없고, 말할 수 없으면 막는 편이 싸다.

### 4.9 지문 대조 — 여기까지가 검증이다

**"화면이 뜬다" 는 복원 검증이 아니다.** 백업 전에 심어 둔 값이 그대로인지 본다.

```bash
# ① 클러스터 kubeconfig 복호화가 되는가
#    이것이 안 되면 화면은 정상인데 어떤 클러스터도 조작할 수 없다 (설계 §1.3)
curl -s localhost:8090/api/v1/admin/clusters/<클러스터 ID>/namespaces \
  -H 'X-Org-Id: 00000000-0000-0000-0000-000000000001' | jq

# ② 워크로드가 원래 replica 로 돌아왔는가
kubectl -n nullus get deploy

# ③ 볼륨 내용이 같은가 — 파드 **안에서** 해시한다
kubectl -n nullus exec deploy/<워크로드> -- sha256sum /data/<파일>
```

> ③ 을 파드 안에서 하는 이유가 있다. 파일을 `kubectl exec` 로 꺼내 와서 해시하면 그 경로가 조용히 자를 수 있다 — 리허설을 만들 때 실제로 그렇게 헷갈렸고, **백업을 잘못 고발할 뻔했다.** 큰 데이터를 검증 도구가 나르지 않는 것이 원칙이다.

#### 스택을 설치했다면 — 도구의 데이터를 직접 대조한다

볼륨 파일 해시는 *바이트* 가 같은지만 말한다. 도구가 그 바이트를 읽어 같은 상태로 살아났는지는 API 로 물어봐야 안다. 백업 **전** 과 복구 **후** 에 같은 값을 뽑아 비교한다:

```bash
# Git — 커밋 수·HEAD SHA·커밋 메시지
curl -s -u "gitea_admin:$GP" \
  'http://localhost:13000/api/v1/repos/nullus/<repo>/commits?limit=50' | jq -r '.[].sha'

# Jenkins — 빌드 번호와 결과
curl -s -u "$JU:$JP" \
  'http://localhost:18080/job/<job>/job/main/api/json?tree=builds%5Bnumber,result%5D' | jq

# Harbor — 이미지 digest 와 태그
curl -s -u "admin:$HP" \
  'http://localhost:18081/api/v2.0/projects/nullus/repositories/<repo>/artifacts' | jq -r '.[].digest'
```

**세 값이 서로 맞물려 있어야 의미가 있다.** Harbor 태그는 Git 커밋 SHA 에서 오고, CD 가 그 태그로 배포 매니페스트를 되커밋한다. 하나만 봐서는 "복구된 척" 을 걸러내지 못한다.

> **수집이 실패하면 대조를 하지 마라.** 지문을 뽑는 스크립트가 조용히 빈 값을 내면 `diff` 가 통과해 **거짓 "일치"** 가 된다. 실제로 그렇게 될 뻔했다 — 셸 안에서 `python3 -c '... [\"sha\"] ...'` 의 따옴표 이스케이프가 깨져 세 값이 전부 빈 문자열이었다. 수집 실패는 **오류로 드러나야** 하고, 빈 값이 섞였으면 대조 자체를 거부해야 한다.

#### 복구 후 애플리케이션이 배포되는가

앱의 Deployment 는 **Git 에서 파생된다** — Argo CD 가 매니페스트를 읽어 만든다. 그래서 확인할 것은 이미지 태그다:

```bash
kubectl -n <네임스페이스> get applications.argoproj.io
kubectl -n <네임스페이스> get deploy <앱> -o jsonpath='{.spec.template.spec.containers[0].image}'
```

태그가 `:bootstrap` 이면 **아직 조정되지 않은 것이다.** 그 태그는 스캐폴드 초기값이라 레지스트리에 존재한 적이 없다. Argo CD 의 기본 동기화 주기는 3분이고, 기다리기 싫으면 강제할 수 있다:

```bash
kubectl -n <네임스페이스> annotate applications.argoproj.io <앱> \
  argocd.argoproj.io/refresh=hard --overwrite
```

`Synced / Healthy` 가 되고 태그가 실제 커밋 SHA 로 바뀌면 된 것이다.

---

## 5. 걸리기 쉬운 곳

리허설을 만들며 실제로 겪은 것들이다.

| 증상 | 원인 | 조치 |
|---|---|---|
| 기동이 `backup.seal_key 는 정확히 32바이트여야 합니다` 로 멈춤 | 키 길이 | **바이트** 기준 32자. 한글은 3바이트다 |
| 기동이 `backup.keycloak_database.host 가 비어 있습니다` 로 멈춤 | 로컬엔 Keycloak DB 가 없다 | §4.3 |
| 백업이 `partial`, `error` 에 `pg_dump: not found` | `pg_dump` 가 PATH 에 없다 | §1.1. **API 프로세스의** PATH 여야 한다 — `refresh` 전에 export |
| 백업이 `partial`, `error` 에 `설정되지 않았습니다` | DB 대상 host 가 빈 값 | §4.4 |
| 백업이 정지 창도 안 열고 실패 | 목적지 검사 실패(버킷 없음·자격증명·연결) | §4.2. **이것은 의도된 동작이다** — 정지 창을 쓰고도 산출물을 못 만드는 것이 최악이라 먼저 막는다 |
| `파드가 5m 안에 종료되지 않았습니다` | PDB·finalizer 로 파드가 안 죽는다 | 해당 워크로드를 확인. **쓰기가 멈추지 않은 채로 복사할 수는 없다** |
| 복구가 `the object has been modified` | 옛 버전 산출물(서버 관리 필드 포함) | 새로 백업받는다. 현재 코드는 덤프 시 정리한다 |
| 리허설이 `네임스페이스가 종료되지 않습니다` | 앞선 실행의 네임스페이스가 아직 Terminating | 기다리거나 finalizer 확인 |
| testcontainers 가 정리 컨테이너에서 막힘 | Docker Desktop 권한 | `export TESTCONTAINERS_RYUK_DISABLED=true` |
| 설치가 40초 만에 `completed` 인데 파드가 0개 | `ENCRYPTION_KEY` 누락 → kubeconfig 복호화 실패 → **시뮬레이션 설치** | §4.4. 로그의 `step executor is nil` 이 증거다 |
| 파이프라인 생성이 `ENCRYPTION_KEY must be 32 bytes` | 같은 원인 | 위와 같다. 설치 직후 파드 수를 세면 훨씬 일찍 잡힌다 |
| 스택 생성이 `stack name ... already exists` | 네임스페이스만 지우면 `stacks` 레코드가 남는다 | DB 의 스택 행도 지운다. 외래키는 `pipelines.stack_id` 와 `stack_config_versions.stack_id` 둘뿐이다 |
| 복구 뒤 앱이 `ImagePullBackOff`, 태그가 `:bootstrap` | Argo CD 가 아직 조정하지 않았다 | §4.9. `:bootstrap` 은 레지스트리에 없는 초기값이다 |
| 복구 뒤 도메인이 옛 IP 를 가리킨다 | **복구는 Service 의 ClusterIP 를 보존하지 않는다** | 클러스터 안에서는 DNS 로 해석하므로 무해하다. IP 를 박아 둔 외부 배선(kind 노드 `/etc/hosts` 등)이 있으면 다시 쓴다 |
| kind 노드 `/etc/hosts` 에서 항목이 안 지워진다 | 그 파일은 bind mount 라 `sed -i` 가 새 파일을 만들어 갈아끼우지 못한다 | `grep -v ... > /tmp/h && cat /tmp/h > /etc/hosts` 로 **제자리 덮어쓰기** 한다. 안 그러면 낡은 항목이 누적되고 앞의 것이 이긴다 |
| 네임스페이스를 지운 뒤 port-forward 가 안 돌아온다 | 대상 파드가 사라져 세션이 끊긴다 | 복구 후 다시 건다. 지문 수집이 이것 때문에 실패하면 **대조를 하지 말고** 포워드부터 살린다 |
| Jenkins 빌드가 `SUCCESS` 인데 아무것도 안 만들어졌다 | HEAD 가 CD 의 `[skip ci]` 커밋이라 Build·Deploy 단계가 `when` 조건에서 걸러졌다 | 콘솔 로그에서 `Stage "Build" skipped due to when conditional` 을 확인한다. **성공 표시만으로 파이프라인이 돈다고 말할 수 없다** |

---

## 6. 로컬로는 확인할 수 없는 것

정직하게 적어 둔다. 2026-09-03 실환경 리허설(§0.1)로 아래 항목들이 **해소됐다**:

- ~~실제 OSS 데이터의 정합성~~ → Gitea 저장소·Harbor digest·Jenkins 빌드 이력을 대조해 일치 확인
- ~~OpenBao 금고~~ → KV export/import 왕복 후 ESO 가 Secret 을 다시 실체화하는 것까지 확인
- ~~정지 창 실측~~ → 371MB 기준 **50.6초** (다만 아래 단서를 볼 것)

**남아 있는 것:**

- **RTO 실측** — 데이터량이 지배한다. 371MB 로 잰 50.6초는 **규모의 근거가 아니다.** 수십 GB 에서 다시 재야 한다
- **다중 노드** — RWO 볼륨이 노드에 흩어지면 §3.5 의 "한 Job 이 전부 마운트" 전제가 깨진다. 리허설은 단일 워커였다
- **에어갭** — 이미지 반입과 egress 경로(B4-2)
- **Argo CD 자동 동기화** — 리허설에서는 `refresh=hard` 로 앞당겼다. 기본 주기 3분 내 자동 회복은 **추정이지 관찰이 아니다**
- **Service ClusterIP 미보존의 영향** — 클러스터 안에서는 DNS 로 해석되어 무해했으나, IP 에 고정된 외부 배선이 있는 환경에서는 재설정이 필요하다
- **차트 버전이 다른 대상으로의 복구** — 매니페스트에 `helm_releases` 를 기록하고 대조해 경고까지는 하지만, 실제로 버전이 어긋난 상태로 복구했을 때 무엇이 깨지는지는 재보지 않았다

---

## 참고

- 설계: [`docs/11_기능설계/Nullus_백업복구_설계.md`](../11_기능설계/Nullus_백업복구_설계.md)
- 플랫폼 기동: [`Nullus_로컬_테스트_가이드.md`](./Nullus_로컬_테스트_가이드.md) · [Windows](./Nullus_로컬_테스트_가이드_Windows.md)
- 리허설 코드: `internal/backup/rehearsal/rehearsal_test.go`
- 실환경 리허설 결과(2026-09-03): [nullus#243 코멘트](https://github.com/cloud-nullus/nullus/pull/243#issuecomment-5518788795)
- EPIC: [nullus-plan#75](https://github.com/cloud-nullus/nullus-plan/issues/75)
