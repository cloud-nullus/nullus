# Nullus 백업/복구 로컬 테스트 가이드

> 로컬에서 **설치부터 백업·복구까지** 한 번에 확인하는 절차다.
> 설계: [`docs/11_기능설계/Nullus_백업복구_설계.md`](../11_기능설계/Nullus_백업복구_설계.md) · EPIC: [nullus-plan#75](https://github.com/cloud-nullus/nullus-plan/issues/75)

작성일: 2026-09-02

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

---

## 6. 로컬로는 확인할 수 없는 것

정직하게 적어 둔다. **아래는 실환경 리허설의 몫이다**(설계 §10.2.2).

- **실제 OSS 데이터의 정합성** — Gitaly 리포지토리, Harbor 의 레지스트리 blob + DB 이중 정합, Jenkins 실행 중 빌드
- **OpenBao 금고** — 실제 금고와 unseal key 취급
- **RTO / 정지 창 실측** — 데이터량이 지배한다. 로컬 수백 KiB 로 잰 값은 근거가 아니다
- **다중 노드** — RWO 볼륨이 노드에 흩어지면 §3.5 의 "한 Job 이 전부 마운트" 전제가 깨진다
- **에어갭** — 이미지 반입과 egress 경로(B4-2)

---

## 참고

- 설계: [`docs/11_기능설계/Nullus_백업복구_설계.md`](../11_기능설계/Nullus_백업복구_설계.md)
- 플랫폼 기동: [`Nullus_로컬_테스트_가이드.md`](./Nullus_로컬_테스트_가이드.md) · [Windows](./Nullus_로컬_테스트_가이드_Windows.md)
- 리허설 코드: `internal/backup/rehearsal/rehearsal_test.go`
- EPIC: [nullus-plan#75](https://github.com/cloud-nullus/nullus-plan/issues/75)
