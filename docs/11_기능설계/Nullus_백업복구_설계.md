# Nullus 백업/복구 설계 (nullus-plan#75)

> EPIC: [cloud-nullus/nullus-plan#75](https://github.com/cloud-nullus/nullus-plan/issues/75) — 백업/복구 설계 및 개발
> 본 문서는 EPIC #75 **Phase 0(B0-1 ~ B0-6)의 산출물**이다. Phase 1 이후의 구현은 별도 이슈로 분리한다.
> 시한 근거: 2026-08-22 데모데이 로드맵 공표 — *"10월까지 남은 기능·버그 패치·백업/복구·보안 보완"*

작성일: 2026-09-01
개정: 2026-09-02 (범위 확대 — 설치된 OSS 내부 데이터 포함) · **2026-09-03 (실환경 리허설 반영 — Phase 1~3 구현 완료, B4-1 완료)**
기준 실측: 마이그레이션 최신 버전 76 · 에어갭 번들 이미지 79개

> **이 문서는 "왜 그렇게 정했는가" 를 다룬다.** 구조는 [`Nullus_백업복구_아키텍처.html`](../20_아키텍처/Nullus_백업복구_아키텍처.html), 사용 절차는 [`Nullus_백업복구_사용_가이드.md`](../50_운영/Nullus_백업복구_사용_가이드.md), 로컬 검증은 [`Nullus_백업복구_로컬_테스트_가이드.md`](../50_운영/Nullus_백업복구_로컬_테스트_가이드.md) 가 다룬다.

---

## 요약 (비관계자용 — 이것만 읽어도 논의 가능)

**무엇을 하기로 했는가.**
Nullus가 관리하는 것을 **전부** 백업하고, **백업한 시점의 상태 그대로 되살린다.** 플랫폼이 기억하는 정보(스택·파이프라인·클러스터)뿐 아니라, **설치된 도구들 안에 쌓인 실제 데이터** — Git 저장소, 컨테이너 이미지, Jenkins 빌드 이력, 저장된 파일 — 까지 포함한다.

**지금 상태.**
백업 기능이 하나도 없다. 저장소 전수 확인 결과 관련 코드 0건이고, 운영 중인 PoC 환경 문서에는 *"노드 재생성 시 데이터 소실. 백업 없음"* 이라고 리스크가 이미 적혀 있다(`deploy/csp/zadara/README.md:286`).

**설계하면서 드러난 것 — 이슈에 적힌 전제 세 개가 실제 코드와 달랐다.**

1. **백업할 데이터베이스는 하나가 아니라 둘이다.** 이슈는 "PostgreSQL 24개 테이블"을 말하지만, 플랫폼은 자기 DB 말고 **로그인 담당(Keycloak)이 쓰는 별도 DB를 하나 더** 띄운다. 하나만 백업하면 복구 후 아무도 로그인할 수 없다.
2. **DB만 백업해도 복구가 안 된다.** 등록된 클러스터의 접속 정보는 **암호화되어** 저장되는데, 그 암호를 푸는 열쇠는 DB 밖에 있다. 열쇠 없이 DB만 되살리면 클러스터 목록은 보이지만 **어느 클러스터에도 접속할 수 없다.**
3. **이슈가 쓰라고 한 "OpenBao 자체 snapshot 기능"은 현재 구성에 존재하지 않는다.** 그 기능은 특정 저장 방식(raft)에서만 제공되는데, Nullus는 단일 노드라는 이유로 **다른 방식(file)을 의도적으로 선택**해 두었다(`openbao-values.go:78`).

**그리고 "상태 그대로 복구"를 요구하면 부딪히는 벽이 하나 있다.**

여러 도구의 데이터를 **같은 한 순간의 모습으로** 얼려서 뜨려면, 보통은 스토리지가 제공하는 "스냅샷" 기능을 쓴다. **현재 환경에는 그 기능이 없다.** PoC가 쓰는 `local-path`는 스냅샷을 지원하지 않는 방식이고, 저장소 전체에 관련 흔적이 0건이다.

그래서 선택지는 사실상 둘뿐이다:

| | 무중단으로 뜨기 | **잠깐 멈추고 뜨기** |
|---|---|---|
| 방법 | 도구를 켜 둔 채, 도구마다 "지금 저장해"라고 시키는 절차를 7종 각각 만든다 | 도구를 전부 잠깐 멈추고, 디스크를 통째로 복사한 뒤 다시 켠다 |
| 상태 일치 | 도구마다 뜬 시각이 조금씩 다르다 | **전부 같은 순간** |
| 만들 것 | 도구 7종의 정지 절차를 각각 설계·검증 | 멈추고 켜는 순서 **하나** |
| 폐쇄망 반입 | 백업 도구 이미지를 새로 들여와야 한다 | **이미 있는 것만으로 된다** |
| 대가 | 없음 | 백업하는 동안 사용 불가 (**추정** 100GB 기준 20분 내외 — 실제 값은 리허설에서 측정한다) |

**"상태 그대로"를 원한다면 오른쪽이 유일한 길이고, 다행히 만들 것도 더 적다.** 그래서 **정지 백업**을 v1으로 채택한다. 야간 창에서 하루 한 번 도는 것을 기본으로 한다.

**백업본은 어디에 두는가.**
**대상 클러스터 밖의 오브젝트 스토리지**에 둔다(조직 내부망, S3 호환). 클러스터가 통째로 사라져도 백업본은 남는다. 여기서 놓치기 쉬운 함정이 하나 있는데 — **그 스토리지에 접속할 열쇠를 스택 안(OpenBao)에 두면 안 된다.** 스택이 죽는 순간 백업본은 멀쩡한데 가져올 수단이 없어진다. 열쇠는 플랫폼 쪽에 따로 둔다.

**가장 위험한 것.**
비밀 금고(OpenBao)를 여는 열쇠가 **금고와 같은 클러스터 안에만 있다.** 코드 주석이 그렇게 설계했다고 명시한다 — *"unseal key가 대상 클러스터를 벗어나지 않는다"*(`openbao-init.go:31`). 평상시엔 옳은 설계지만, **클러스터가 통째로 사라지면 백업본이 있어도 열 수 없다.** 열쇠를 밖으로 꺼내 보관하는 절차가 v1 필수다.

**완료 기준.**
**복구 리허설을 통과하기 전까지 이 EPIC은 완료가 아니다.** 팀 내부에 "백업은 됐는데 복원이 안 됐다"는 경험(2026-07-19 회의, 이기하)이 있고, 그것이 이 EPIC의 존재 이유다.

---

## 0. 현재 상태 실측

| 확인 항목 | 결과 | 근거 |
|---|---|---|
| 백업/복구 코드 | **0건** | 파일명·식별자 전수 확인 |
| 플랫폼 DB 테이블 | **23개 + `schema_migrations` = 24** | `db/migrations/*.up.sql` 의 `CREATE TABLE` 전수 |
| 마이그레이션 최신 버전 | **74** (`golang-migrate`) | `db/migrations/000074_stack_deploy_logs.up.sql` |
| 그중 seed 마이그레이션 | **18개** | `db/migrations/*seed*.up.sql` |
| **CSI VolumeSnapshot 지원** | **0건** — `snapshot.storage.k8s.io` 참조 없음 | 저장소 전수 |
| **PoC StorageClass** | **`local-path`** — 스냅샷 불가 방식 | `deploy/csp/zadara/README.md:60-69` |
| PoC 노드 구성 | **워커 1대**(`replicaCount: 1` 고정), control-plane 2vCPU/4GB 겸 bastion | `deploy/csp/zadara/README.md:287-288` |
| 감사로그 보존 정책 | **없음** — 삭제·아카이빙 코드 0건 | `audit_logs`(`000011`) |
| ROADMAP | `Disaster Recovery — P2` **장기 항목만** | `docs/70_전략/ROADMAP.md:363` |

---

## 1. B0-1 — 보호 대상 인벤토리

판정 기준은 **"백업 시점의 상태로 되돌릴 수 있는가"** 다. 재설치로 *비슷한* 상태에 도달하는 것으로는 요구를 충족하지 못한다.

### 1.1 판정표

| # | 대상 | 재생성 가능성 | v1 범위 | 방식 |
|---|---|---|---|---|
| **A1** | 플랫폼 PostgreSQL — 사용자 생성 데이터 19개 테이블 | 불가 | ✅ | `pg_dump -Fc` |
| **A2** | 플랫폼 PostgreSQL — seed 데이터 4개 테이블 | 가능(마이그레이션) | ✅ (분리 안 함) | 위와 동일 |
| **A3** | **Keycloak PostgreSQL** — realm, 계정, OIDC 클라이언트 | 부분 가능 — 재프로비저닝은 *클라이언트 등록*만 되살린다 | ✅ **백업 확정**(Q2) | `pg_dump -Fc` · §1.2 |
| **B1** | OpenBao KV — `kv/nullus/{env}/{org}/...` | 불가 | ✅ | KV 논리 export · §3.2 |
| **B2** | OpenBao 정책 / Kubernetes Auth | 가능(부트스트랩 Job 멱등) | ❌ 재프로비저닝 | `openbao-bootstrap.go` |
| **B3** | OpenBao unseal key | 불가 | ✅ (백업본과 **분리** 보관) | §5.3 |
| **C1** | `ENCRYPTION_KEY` | 불가 | ✅ (에스크로) | §1.3 |
| **D1** | **클러스터 리소스** — Helm 릴리스, Gateway/HTTPRoute, Secret/ConfigMap, PVC 정의 | 재설치로는 *비슷하게만* | ✅ **포함** | 네임스페이스 리소스 덤프 · §3.4 |
| **E1** | **워크로드 데이터** — GitLab/Gitea 저장소, Harbor/Nexus 이미지·아티팩트, Jenkins 잡·빌드 이력, MinIO 오브젝트, 스택 PostgreSQL, OpenSearch/Loki 인덱스 | **불가** | ✅ **포함** | **정지 백업** · §3.4 |

**v1 정의: "스택 네임스페이스 전체를 한 시점의 모습 그대로 뜨고, 그 모습 그대로 되돌린다."**

### 1.2 A3 — 이슈에 없던 두 번째 데이터베이스

플랫폼은 자기 PostgreSQL 과 별개로 **Keycloak 전용 PostgreSQL** 을 하나 더 띄운다. 즉 `pg_dump` 대상이 **두 개**이며, A1만 복구하면 플랫폼 데이터는 살아나지만 **로그인 경로가 없다.**

#### 왜 둘인가 — 설계 의도가 아니다

Nullus/스택 분리 때문이 아니다. **Bitnami keycloak 서브차트가 자기 PostgreSQL 을 딸고 오는 기본 동작**의 결과다:

> `deploy/helm/nullus/values.yaml:180-183`
> *"서브차트가 자기 PostgreSQL 을 함께 띄운다 — 상위 postgresql 과 별개다. 둘 다 `<릴리즈>-postgresql` 로 이름이 잡혀 **StatefulSet/Service/Secret 등 7종이 그대로 충돌한다. 설치가 "already exists" 로 실패하므로 이름을 분리한다.**"*

인과가 반대다 — **"분리하려고 이름을 나눈" 것이 아니라 "이미 둘이라 설치가 깨지니 이름을 나눈"** 것이다. `nameOverride: keycloak-postgresql` 은 아키텍처 결정이 아니라 충돌 회피 패치다.

Keycloak 에 관한 한 설계 의도는 **정반대(공유)** 다:

> `deploy/helm/nullus/Chart.yaml:12-14`
> *"Keycloak 은 **플랫폼 구성요소**다. 스택마다 IdP 를 띄우면 Nullus 자체 로그인용 Keycloak 과 이중화되므로, **하나를 공유**하고 스택 설치기는 클라이언트만 등록한다."*

실제로 `helm_step_metadata.go` 와 `values.go` 의 `installing_*` case 에 **keycloak 설치 단계가 없다** — 스택 설치기는 Keycloak 을 설치하지 않는다.

**분리 축은 "컨트롤 플레인 vs 스택 네임스페이스"이고, Keycloak DB 둘은 양쪽 다 플랫폼 쪽에 있다** — 분리 축을 가로지르지 않는다. 대조군: OpenBao 는 진짜로 스택마다 배포된다(`internal/shared/secrets/store.go:23` — *"OpenBao 는 스택마다 배포되므로"*).

#### ⚠️ 배포 경로가 둘로 갈라져 있다 — 백업이 위치를 가정하면 안 된다

| 경로 | Keycloak 배치 | 전용 PostgreSQL |
|---|---|---|
| **Helm 차트** (`deploy/helm/nullus`) | `keycloak.enabled=true` **서브차트** — nullus 릴리스와 같은 네임스페이스 | 릴리스 안에 `nullus-keycloak-postgresql` |
| **에어갭 스크립트** (`airgap/scripts/22-install-platform-stack.sh:98-99`) | **독립 릴리스** `keycloak` — **`nullus-auth` 네임스페이스** | 그 네임스페이스에 자체 postgres (`--set postgresql.image.*`) |

**같은 플랫폼인데 Keycloak DB 의 네임스페이스·릴리스명이 배포 경로마다 다르다.** 따라서 백업 구현은 **이름을 하드코딩하지 않고 조회로 찾아야 한다** — `§1.5` 의 PVC 열거와 같은 원칙이다. 이 규칙을 테스트로 고정한다(B1-4).

#### 결정 1 (Q7) — **두 DB 를 한 인스턴스로 통합한다**

분리가 의도된 설계가 아니므로 **지킬 이유가 없다.** 차트 양쪽이 이미 지원한다:

| 필요한 것 | 지원 |
|---|---|
| Keycloak 이 외부 DB 를 보게 | `keycloak.postgresql.enabled=false` + `externalDatabase.{host,port,user,database,password}` (keycloak 24.4.5 `values.yaml:1353`) |
| 플랫폼 PostgreSQL 에 두 번째 database 생성 | `primary.initdb.scripts` (postgresql 16.7.21 `values.yaml:351`) |

**형태는 "같은 인스턴스 · 별도 database · 별도 role" 하나만 채택한다.** 같은 database 에 스키마를 공유하는 형태는 **금지** — Keycloak 은 자체 Liquibase 로 자기 스키마를 관리하므로 `golang-migrate` 와 소유권이 섞이고, CLAUDE.md 의 *"테이블은 모듈별로 소유한다"* 원칙에도 어긋난다(Keycloak 은 모듈이 아니라 다른 제품이다).

**백업 설계에 주는 변화:**

| 항목 | 효과 |
|---|---|
| **버전 이원화 해소** | 지금 Keycloak DB 는 `17.2.0-debian-12-r6`, 플랫폼은 `17.5.0-debian-12-r20`. 통합하면 §3.1 의 `pg_dump` 버전 함정이 절반 사라지고 **에어갭 이미지도 1개 감소** |
| 연결 지점 | 백업 Job 이 붙을 엔드포인트가 하나 |
| 운영 | StatefulSet/PVC/Secret 한 벌 · PoC 단일 워커에서 인스턴스 하나 절약 |
| **정합성** | ⚠️ **자동으로 얻어지지 않는다.** PostgreSQL 은 database 간 일관 스냅샷을 제공하지 않으므로 `pg_dump` 는 **여전히 2회**다. 진짜 이득은 Phase 3 물리 백업/PITR 로 갈 때 한 인스턴스가 한 번에 덮인다는 것 |

**전제와 함정 (통합 작업 시):**

1. **`initdb.scripts` 는 첫 부팅에만 실행된다.** 기존 PVC 가 있으면 `keycloak` database 가 안 생기고 Keycloak 이 조용히 기동 실패한다. 신규 설치는 무비용, 기존 배포는 수동 `CREATE DATABASE` 필요.
2. **권한 격리 필수** — Keycloak role 이 `nullus` database 에 `REVOKE CONNECT` 되어야 한다. 안 하면 Keycloak 침해 시 플랫폼 DB 까지 열린다.
3. **두 배포 경로를 모두 처리해야 한다**(위 표). 에어갭 경로는 `nullus-auth` 네임스페이스에서 플랫폼 네임스페이스의 PostgreSQL 로 **크로스 네임스페이스 연결**이 된다.
4. **`26-migrate-db.sh:80` 이 `app.kubernetes.io/name=postgresql` 셀렉터로 파드를 찾는다.** 지금은 `nameOverride` 덕에 우연히 안 겹치는데, 통합하면 그 override 가 사라진다. 오작동하진 않지만 이 배선이 load-bearing 임을 알고 건드려야 한다.

> **통합은 이 EPIC 밖의 별도 인프라 이슈로 분리한다.** 로그인 경로 전체를 건드리는 차트 변경이라 10월 시한에 백업 구현과 묶으면 둘 다 위험해진다. **다만 Phase 1 착수 전에 끝나면 §3.1·§4.4·§6.1 이 단순해지므로 순서상 앞에 두는 것이 이득이다.** 백업 설계는 통합 여부와 무관하게 성립하도록 작성했다 — 통합 전에도 백업은 필요하다.

#### 결정 2 (Q2) — **Keycloak DB 를 백업한다** (재프로비저닝으로 대체하지 않는다)

Keycloak 이 완전한 SoT 는 아니다:

> `internal/auth/adapter/keycloak/sso_client.go:17` — *"OpenBao 가 Source of Truth 여야 하고, Keycloak 이 유실돼도 복원할 수 있어야 하기 때문이다."*

**그러나 재프로비저닝으로 복원되는 것은 *OSS 별 OIDC 클라이언트 등록*뿐이다.** 사용자 계정, realm 커스터마이징, 그룹·역할 매핑, 로그인 테마 설정은 복원되지 않는다. **"그 상태 그대로"가 요구인 이상 이것들이 살아나야 하므로 dump 한다.**

이 결정으로 §10.2 의 "재프로비저닝만으로 충분한지 판정" 항목은 사라지고, 대신 **복원된 계정으로 로그인되는지**를 검증한다.

### 1.3 C1 — DB 백업만으로는 클러스터에 접속할 수 없다

`clusters.kubeconfig` 는 평문이 아니다.

| 사실 | 근거 |
|---|---|
| `BYTEA` 컬럼에 **AES-256-GCM 암호문** | `000014_cluster_kubeconfig.up.sql:1`, `pkg/crypto/aes_gcm.go` |
| 키는 32바이트 `ENCRYPTION_KEY` **환경변수** | `internal/admin/adapter/handler/cluster_handler.go:44`, `:186` |
| Helm Secret `encryption-key` 에서 주입 | `deploy/helm/nullus/templates/secret.yaml:10` |
| **키는 DB 안에 없다** | — |

`pg_dump` 산출물만 안전하게 두는 백업은 헛것이다. 복구 후 `clusters` 행은 살아나지만 `kubeconfig` 를 열 수 없어 **등록된 어떤 클러스터도 조작할 수 없다.**

**결정:** `ENCRYPTION_KEY` 에스크로(§5)를 **B1-1 보다 먼저** 처리한다. 코드 없이 오늘 가능하고, 그 자체로 현재 리스크를 낮춘다.

### 1.4 E1 — 범위에 넣는다. 그 대가를 먼저 적는다

이전 개정에서는 E1 을 v1 범위 밖으로 두었다. **범위에 넣기로 결정했으므로**, 그 결정이 무엇을 바꾸는지 숨기지 않고 적는다.

| 바뀌는 것 | 이전(E1 제외) | 현재(E1 포함) |
|---|---|---|
| 백업 크기 | 수십 MB (DB dump + KV) | **수십~수백 GB** — §1.5 |
| 백업 방식 | 논리 dump 만 | **볼륨 전체 복사 필요** |
| 무중단 여부 | 무중단 | **정지 창 필요** — §3.4 |
| 백업 목적지 | 전용 PVC 로 충분 | **별도 오브젝트 스토리지 필수** — §4.2 |
| RTO | 1시간 | **4시간** — §2 |
| 복구 성격 | 재설치 + 데이터 주입 | **네임스페이스 통째 복원** — §6 |

**이 확대를 받아들이는 이유:** "재설치로 비슷한 상태"와 "백업 시점 그대로"는 다른 요구다. CI/CD 플랫폼에서 Git 커밋 이력과 빌드 이력이 사라지면, 플랫폼이 살아나도 조직의 작업 산출물은 사라진 것이다. **E1 이 실제 자산이다.**

### 1.5 백업 크기 산정

설치기와 에어갭 values 가 **명시적으로 지정하는** PVC 크기:

| 도구 | PVC | 근거 |
|---|---|---|
| 스택 PostgreSQL | 20Gi | `values.go:130` (`installing_postgresql`) |
| Jenkins | 20Gi | `values.go:334` |
| Nexus | 20Gi | `values.go:266` (`storageSize`) |
| gitlab-postgres | 8Gi | `stack-values/gitlab-postgres.yaml:30` |
| Harbor(registry) | 5Gi | `stack-values/harbor.yaml:89` — `values.go:191` 은 `enabled` 만 켜고 크기를 안 준다 |
| OpenSearch | 4Gi | `stack-values/opensearch.yaml:13` |
| MinIO | 2Gi | `stack-values/minio.yaml:10` |
| OpenBao | `OpenBaoDataStorageSize` | `openbao-values.go:24` |
| **GitLab · Gitea** | ⚠️ **미지정 — 차트 기본값** | `values.go` 의 `installing_gitlab`(398행~) / `installing_gitea`(337행~) 에 `persistence` 항목이 **없다** |

**명시된 것만 합쳐 79Gi**, 여기에 OpenBao 와 GitLab·Gitea 차트 기본값이 더해진다. 보존 3세대면 목적지에 **수백 GB** 를 준비해야 하며, 압축 없이는 성립하지 않는다(§4.3).

> ⚠️ **부수 발견 — 백업 크기를 미리 알 수 없다.** GitLab 과 Gitea 는 설치기가 볼륨 크기를 지정하지 않아 차트 기본값을 따른다. GitLab 차트는 gitaly·redis·object storage 등 **여러 PVC 를 자체 기본값으로** 만든다. 따라서 백업 목적지 용량 산정(§4.2, F8)은 **정적 계산이 아니라 실행 시점 PVC 조회**로 해야 한다. 설계에 그대로 반영한다 — §3.4 5단계는 네임스페이스의 PVC 를 **열거해서** 처리하지, 알려진 목록을 순회하지 않는다.

> Grafana(`persistence.enabled: false`)와 Prometheus(`values.go:568` `enabled: false`)는 볼륨이 없다 — 대시보드는 코드/ConfigMap 에서, 메트릭은 재수집으로 회복된다. **백업 대상이 아니다.**

---

## 2. B0-2 — RPO / RTO 목표

| 계층 | RPO | RTO | 근거 |
|---|---|---|---|
| A1/A3 플랫폼·Keycloak DB | **24시간** | **1시간** | 논리 dump, 크기가 작다 |
| B1 OpenBao KV | **24시간** (§2.1 skew 주의) | 1시간 | — |
| **D1+E1 스택 네임스페이스** | **24시간** (야간 정지 창 1회) | **4시간** | 데이터량이 지배한다 — §2.2 |
| B3/C1 키 자재 | **변경 시 즉시**(이벤트) | **0** (복구 착수 전제) | 주기 백업 대상이 아니라 *변경 시 에스크로* 대상 |

### 2.1 RPO 를 깨는 숨은 요인 — DB 와 금고는 서로를 참조한다

`token_sources` 는 토큰 값을 담지 않는다. **경로만** 담는다.

```
token_sources.path  TEXT NOT NULL     -- 000047_token_rotation_tables.up.sql:6
   ↓ 가리킴
kv/nullus/{env}/{org}/{module}/{provider}/{name}   -- internal/shared/secrets/paths.go
```

그리고 **토큰 회전 스케줄러가 기본 5분 주기로 둘을 함께 갱신한다**(`internal/admin/scheduler/token_rotation.go:46-48`).

백업 시점이 어긋나면 복구된 DB 가 금고에 없는 경로를 가리킨다. 이 불일치는 **에러 없이 지나갔다가 파이프라인 실행 시점에 인증 실패로 드러난다.**

**대응:** ① 하나의 `backup_run` 으로 묶고 순서를 **OpenBao → DB** 로 고정한다(금고를 먼저 뜨면 "DB 는 아는데 금고엔 없다" 방향의 불일치만 남고, 이는 §6.4 검사로 탐지 가능하다). ② 복구 절차에 **참조 정합성 검사를 필수 단계로** 넣는다. ③ 정지 백업 모드에서는 **회전 스케줄러도 함께 멈춘다** — 이때는 skew 가 0 이 된다.

### 2.2 RTO 4시간의 근거

100Gi 급 데이터를 오브젝트 스토리지에서 읽어 local-path 로 되쓰는 시간이 지배한다. **아래는 계산이지 실측이 아니다** — 100MB/s 가정 시 복원만 ~20분이지만, 여기에 ①정합성 검사 ②도구 기동 대기(GitLab 은 기동에만 수 분) ③스모크가 붙는다. **1시간은 E1 을 포함한 상태에서 현실적이지 않다.**

**이 숫자는 B4-1 리허설의 실측으로 반드시 갱신한다.** 리허설이 6시간 걸렸다면 목표를 6시간으로 고치거나 절차를 줄이는 것이지, 4시간이라고 계속 적어두는 것이 아니다.

---

## 3. B0-3 — 도구 선정

**공통 제약: 에어갭에서 동작할 것.** 후보는 `airgap/images/images.txt`(74개) 에 있거나, 반입 가치가 입증된 것만이다.

### 3.1 플랫폼 / Keycloak DB — `pg_dump -Fc` **채택**

| 후보 | 판정 | 이유 |
|---|---|---|
| **`pg_dump -Fc`** | ✅ **채택** | 이미지가 번들에 있다(`bitnamilegacy/postgresql:17.5.0-debian-12-r20`). 단일 파일, 압축 내장, `pg_restore` 로 선택적 복원 |
| 논리 복제 | ❌ 탈락 | **대기 인스턴스가 필요하다.** 단일 노드 에어갭에서 비용이 이득을 넘는다. Active-Passive 는 ROADMAP `DR(P2)` 영역 |
| 물리 백업 / PITR | ⏭ Phase 3 | RPO 를 분 단위로 줄이는 유일한 경로. v1 에는 과하다 |

> ⚠️ **버전 함정.** 번들에 PostgreSQL 이미지가 **3개 버전** 섞여 있다 — `14.8.0`, `17.2.0-debian-12-r6`(Keycloak DB), `17.5.0-debian-12-r20`(플랫폼 DB). `pg_dump` 는 **서버 버전보다 낮으면 실패**한다. 백업 Job 은 서버 버전을 조회해 그 이상의 클라이언트를 선택해야 하고, 이 규칙을 테스트로 고정한다(B1-4).
>
> **DB 통합(§1.2 결정 1)이 완료되면 `17.2.0` 이 사라져 이 함정의 절반이 해소된다.** 그래도 버전 조회 로직은 남긴다 — 외부 DB(`postgresql.enabled=false`) 모드에서는 서버 버전을 여전히 알 수 없다.
>
> 그리고 **Keycloak DB 의 위치를 하드코딩하지 않는다** — 배포 경로에 따라 네임스페이스·릴리스명이 다르다(§1.2).

### 3.2 OpenBao — ⚠️ 이슈의 전제가 성립하지 않는다

이슈 B2-1 은 *"OpenBao 자체 snapshot API"* 를 전제한다. **현재 구성에는 없다.**

> `internal/stack/adapter/helm/openbao-values.go:78-92`
> *"단일 replica 구성이므로 **raft 대신 file 스토리지를 쓰고**…"*
> ```hcl
> storage "file" { path = "/openbao/data" }
> ```

`bao operator raft snapshot save` 는 **raft 전용**이다.

| 안 | 장점 | 단점 | 판정 |
|---|---|---|---|
| **A. KV 논리 export** (경로 재귀 순회 후 `list`+`read`) | **봉인 불필요**. 포맷 안정, 버전 이식 가능, 추가 이미지 없음 | KV 이외 마운트 미포함, **KV v2 버전 이력 유실** | ✅ **v1 채택** |
| B. PVC 파일 복사 | 이력까지 완전 보존 | 일관성 보장하려면 봉인 필요 → 스택 전체가 시크릿을 못 읽는다 | ⚠️ **정지 백업 모드에서는 성립** — §3.4 |
| C. raft 전환 후 snapshot | 공식 경로 확보 | 기존 설치본 마이그레이션. 10월 시한 밖 | ⏭ Phase 2 (Q3) |

**결정:** A안을 기본으로 하되, **정지 백업 모드에서는 B안이 함께 성립한다** — 워크로드가 멈춘 상태에서는 `/openbao/data` 가 정지 상태이므로 파일 복사가 일관적이다. 스택 볼륨을 통째로 뜨는 정지 백업이 OpenBao PVC 도 함께 담으므로, **A안은 "금고만 따로 복원"용 보조 산출물**이 된다.

정책·auth 는 부트스트랩 Job 이 멱등하게 재구성한다(`openbao-bootstrap.go`). **값은 백업하고 구조는 재생성한다.**

### 3.3 Velero — E1 포함으로 **재검토했고, v1 에서는 탈락**

이전 개정은 *"Velero 가 지켜줄 대상이 재설치로 대체 가능"* 이라는 이유로 탈락시켰다. **E1 이 범위에 들어오면 그 근거는 무너진다.** 그래서 다시 평가했다.

| 항목 | 평가 |
|---|---|
| **CSI VolumeSnapshot 경로** | ❌ **사용 불가.** `local-path` 는 CSI 드라이버가 아니라 hostPath 기반 프로비저너라 VolumeSnapshot 을 지원하지 않는다. 저장소에 `snapshot.storage.k8s.io` 참조 **0건** |
| **File System Backup(kopia) 경로** | ⚠️ 동작하지만 **파드가 살아 있어야 한다** — FSB 는 볼륨을 마운트한 파드를 통해 읽는다. 즉 **"정지 후 백업"과 구조적으로 상충**한다 |
| 무중단 정합성 | 도구별 pre/post 훅을 **7종 각각** 설계·검증해야 한다 (GitLab·Jenkins·Harbor·Nexus·Gitea·MinIO·OpenSearch) |
| 에어갭 | velero + plugin 이미지 **신규 반입** 필요 (번들 74개에 0건) |
| 다중 노드 | ✅ node-agent DaemonSet 으로 자연 지원 — **정지 백업 대비 유일한 우위** |

**판정: v1 탈락, Phase 2 재도입 후보.**

이유는 요구사항과 정면으로 맞는다. **사용자가 요구한 것은 "그 상태 그대로"** 이고, Velero FSB 는 파드가 떠 있어야 동작하므로 **도구 간 시점 일치를 구조적으로 보장하지 못한다.** 그것을 보완하려면 도구 7종의 정지 훅을 만들어야 하는데, 그 작업량이면 §3.4 의 정지 백업을 만들고 리허설까지 끝낼 수 있다.

> **Phase 2 재도입 조건:** ① 무중단 백업이 요구사항이 되거나, ② 워커가 2대 이상으로 늘어 RWO 볼륨이 노드에 흩어지거나(§3.4 의 단일 Job 전제가 깨진다), ③ StorageClass 가 CSI 스냅샷 지원 제품으로 교체될 때.

### 3.4 D1+E1 — **정지 백업(cold backup) 채택**

**핵심 제약부터.** 여러 볼륨을 같은 순간의 모습으로 얼리는 수단이 현재 환경에 없다:

```
deploy/csp/zadara/README.md:60   StorageClass — local-path-provisioner
저장소 전체                       snapshot.storage.k8s.io 참조 0건
```

`local-path` 는 노드 로컬 디렉터리를 그대로 내주는 프로비저너다. 스냅샷 API 자체가 없다. **따라서 "그 상태 그대로"를 얻는 방법은 하나뿐이다 — 쓰기를 멈추고 복사한다.**

#### 백업 절차

| # | 단계 | 비고 |
|---|---|---|
| 1 | **리소스 매니페스트 덤프** — 스택 네임스페이스 전 리소스 + Helm 릴리스 Secret(`sh.helm.release.v1.*`) | D1. 워크로드가 살아 있을 때 뜬다 |
| 2 | **회전 스케줄러 정지** | §2.1 skew 를 0 으로 만든다 |
| 3 | **워크로드 정지** — Deployment/StatefulSet replica 를 0 으로. **원래 replica 수를 매니페스트에 기록** | 복원·재개의 기준값 |
| 4 | 파드 종료 대기 (볼륨 언마운트 확인) | 여기까지가 정지 창의 시작 |
| 5 | **PVC 덤프 Job** — 네임스페이스의 PVC 를 **열거해** 전부 마운트하고, `tar` + 압축 → 외부 오브젝트 스토리지로 **스트리밍** | §3.5 · §3.6 |
| 6 | **워크로드 재개** — 3단계에 기록한 replica 수로 복원 | 정지 창 종료 |
| 7 | 플랫폼 DB / Keycloak DB / OpenBao KV export | 무중단 (컨트롤 플레인은 멈추지 않는다) |
| 8 | 매니페스트 확정 + 무결성 해시 | §4.4 |

> **정지 대상은 스택 네임스페이스이지 컨트롤 플레인이 아니다.** Nullus API·UI 는 계속 뜬다 — 백업 진행 상황을 화면에서 볼 수 있어야 하기 때문이다. 멈추는 것은 백업 대상인 도구들이다.

#### 왜 이 방식인가

| | Velero FSB + 도구별 훅 | **정지 백업** |
|---|---|---|
| 시점 일관성 | 도구별로만 | ✅ **전 도구 단일 시점** |
| 구현량 | 도구 7종 훅 각각 | ✅ **정지/재개 순서 하나** |
| 에어갭 반입 | velero 이미지 2종 | ✅ **`busybox:1.37` — 이미 번들에 있다** |
| 도구 추가 시 | 새 도구마다 훅 설계 | ✅ **자동 포함** (PVC 를 순회하므로) |
| 다중 노드 | ✅ 자연 지원 | ⚠️ §3.5 제약 |
| 다운타임 | 없음 | ⚠️ 백업 시간만큼 |
| 성숙도 | 검증된 OSS | ⚠️ 자체 구현 |

**"도구 추가 시 자동 포함"이 장기적으로 가장 큰 이득이다.** 카탈로그에 도구가 늘어날 때마다(`helm_step_metadata.go` 에 이미 15종 이상) 백업 훅을 따로 만들어야 한다면 백업은 반드시 뒤처진다. PVC 를 순회하는 방식은 그 문제가 없다.

### 3.5 정지 백업의 전제와 한계 — 명시한다

| 전제 | 현재 충족 | 깨지면 |
|---|---|---|
| **RWO PVC 들이 같은 노드에 있다** (한 Job 파드가 전부 마운트) | ✅ 워커 1대 (`replicaCount: 1` 고정, README:287) | ⚠️ 워커 2대 이상이면 **노드별 Job 으로 분할** 필요. 이 분기를 설계에 넣되 v1 구현은 단일 노드 경로만 |
| **정지 창을 잡을 수 있다** | PoC/온프렘 전제상 가능 | 24/7 요구 시 → Velero 재도입(§3.3) |
| **목적지가 스택과 다른 실패 도메인** | ✅ **해소됨** — 클러스터 외부 오브젝트 스토리지(§4.2) | — |

> 세 번째 전제는 **클러스터 외부 오브젝트 스토리지로 결정되며 해소됐다**(§4.2). 남은 것은 그 결정이 파생시킨 두 가지다 — **자격증명 순환 의존 회피**(§4.2.1)와 **egress 실측**(§4.2.2).

---

### 3.6 아카이버는 무엇으로 만드는가

정지 백업의 5단계는 "PVC 를 읽어 외부 S3 로 올린다" 하나다. 필요한 도구는 둘 — **`tar`** 와 **S3 업로더**.

| 후보 | 판정 |
|---|---|
| **`mc pipe`** (MinIO 클라이언트) | ✅ **채택.** 선례가 있다 — 설치기가 GitLab 버킷 부트스트랩에 이미 `mc` Job 을 쓴다(`internal/stack/adapter/helm/object-storage-buckets.go:55`). S3 호환 엔드포인트·path-style·사설 CA 를 그대로 다룬다 |
| `mc mirror` (파일 단위 동기화) | ❌ 탈락. 파일 권한·소유자·심볼릭/하드링크 보존이 불완전하다. **"그 상태 그대로"가 요구인 이상 `tar` 가 맞다** — Git 저장소와 레지스트리 blob 은 권한·링크에 민감하다 |
| Go S3 SDK 를 API 파드에 | ⚠️ **채택으로 정정** — 아래 참조 |

파이프라인은 한 줄이다:

```sh
tar -C /vol/<pvc> -cf - . | zstd -T0 | mc pipe backup/<bucket>/<run-id>/volumes/<pvc>.tar.zst
```

> ⚠️ **구현하며 정정 — 컨트롤 플레인이 데이터 경로에 들어간다 (2026-09-02).**
> 초안은 "볼륨을 마운트한 Job 이 목적지로 직접 올린다" 를 권고했다. 구현해 보니 그것이 **§5(키 취급)와 충돌**한다:
> 산출물은 봉인해서 올려야 하고(§5.1), 봉인 키는 컨트롤 플레인에만 있어야 한다(§4.2.1·§5.2). Job 이 직접 올리려면 그 키를 **백업 대상 클러스터 안으로** 넣어야 하는데, 그러면 클러스터가 침해될 때 백업본까지 함께 열린다 — 백업을 두는 이유 자체가 사라진다.
> **결정: 대역폭을 두 번 쓰는 대신 키가 대상 클러스터를 벗어나지 않게 한다.** 헬퍼 파드에서 `tar` 를 exec 해 스트림을 받아(kubectl cp 와 같은 방식) 컨트롤 플레인에서 봉인한 뒤 목적지로 올린다. 중간 파일은 만들지 않는다.
> 무중단·직접 업로드가 필요해지면 S3 SSE-C 로 Job 에 키를 넘기는 경로를 검토한다 — Q11.

> ⚠️ **부수 발견 — `mc` 이미지 태그가 에어갭 번들과 어긋나 있다.**
> 설치기 코드는 `minio/mc:RELEASE.2025-05-21T01-59-54Z` 를 참조하는데(`object-storage-buckets.go:55`), 번들에는 `minio/mc:RELEASE.2018-07-13T00-53-22Z` 와 `quay.io/minio/mc:RELEASE.2024-11-21T17-21-54Z` 만 있다(`airgap/images/images.txt`). **이 불일치는 백업 이전에 이미 존재하는 결함**이며, 에어갭에서 GitLab 버킷 부트스트랩이 ImagePull 로 실패할 수 있다. 백업이 같은 이미지를 쓰므로 **B1-5 착수 전에 정합을 맞춘다** — 별도 이슈로 분리 제안.
> 또한 `tar` 와 `zstd` 가 선택한 `mc` 이미지에 있는지 확인해야 한다. 없으면 `busybox:1.37`(번들 보유, `tar` 포함)과 `mc` 를 한 파드의 **두 컨테이너 + 공유 볼륨**으로 나누거나, `deploy/images/jenkins` 선례처럼 **전용 백업 이미지를 만든다**. B1-5 의 첫 결정 항목이다.

## 4. B0-4 — 저장 위치 · 보존 정책

### 4.1 MinIO 재사용 — ❌ 불가

이슈는 "MinIO 재사용 여부"를 묻는다. **현재의 MinIO 는 백업 목적지로 쓸 수 없다.**

| 사실 | 근거 |
|---|---|
| MinIO 는 플랫폼 구성요소가 아니라 **스택 도구**다 — 릴리스명 `nullus-minio` | `internal/shared/domain/platform_resources.go:21` |
| standalone, `replicas: 1`, `persistence 2Gi` | `airgap/helm/stack-values/minio.yaml` |
| **보호 대상과 같은 클러스터·같은 스토리지 위에 있다** | — |
| **그리고 그 MinIO 자신이 E1 백업 대상이다** | §1.1 E1 |

자기가 백업 대상인 저장소에 자기 백업을 넣을 수는 없다. 게다가 2Gi 는 §1.5 규모를 담지 못한다.

### 4.2 저장 위치 — **클러스터 외부 오브젝트 스토리지 (결정됨)**

**결정: 백업 목적지는 대상 클러스터 밖의 S3 호환 오브젝트 스토리지다.** Q8 은 이것으로 닫힌다.

이 결정이 §3.5 의 마지막 미충족 전제("목적지가 스택과 다른 실패 도메인")를 해소한다. 클러스터·노드·스토리지가 통째로 사라져도 백업본은 남는다.

**코드베이스에 이미 있는 개념이다.** 스택 설치는 외부 오브젝트 스토리지 연결을 `existing-connect` 모드로 지원한다:

```go
// internal/stack/domain/config.go:60
type StorageTarget struct {
    Mode             string  // "existing-connect"
    Endpoint         string
    ResourceName     string  // 버킷
    AccessSecretRef  string  // 자격증명을 담은 Secret 이름
    AuthID           string  // access key
    AuthPasswordKey  string  // secret key 가 담긴 키 이름
    ...
}
```

**백업 목적지도 같은 어휘를 쓴다.** 새 개념을 만들지 않는다 — 사용자가 스택 설치에서 이미 이해한 모델이고, DDD 관점에서 Ubiquitous Language 를 유지하는 것이 맞다.

| 항목 | 값 |
|---|---|
| 프로토콜 | S3 호환 (path-style) — `sharedObjectStorageSecretManifest` 가 이미 `path_style: true` 로 쓴다(`helm-values.go`) |
| 위치 | **조직 내부망의 오브젝트 스토리지.** 에어갭이므로 인터넷 S3 가 아니다 |
| 버킷 | 백업 전용. 스택이 쓰는 버킷과 분리한다 |
| 전송 | 클러스터 → 외부 **egress**. §4.2.2 |

> **부수 이득:** S3 호환으로 고정해 두면 Phase 2 에서 Velero 를 도입할 때 **같은 엔드포인트를 BSL(Backup Storage Location)로 그대로 재사용**할 수 있다. §3.3 의 전환 비용이 그만큼 낮아진다.

#### 4.2.1 ⚠️ 자격증명 순환 의존 — 반드시 피해야 하는 배선

목적지가 외부로 나가면서 **새 질문이 생긴다: 그 스토리지의 access key 를 어디에 보관하는가.**

현재 스택의 오브젝트 스토리지 자격증명은 **OpenBao 가 원천이다:**

> `internal/stack/adapter/helm/helm-values.go:415`
> *"nullus-object-storage 는 ExternalSecret 이 소유한다."* (authentication.provider=openbao 인 경우)

**백업 목적지 자격증명을 같은 방식으로 두면 순환이 생긴다:**

```
스택이 죽는다 → OpenBao 가 죽는다 → 목적지 자격증명을 못 읽는다
             → 백업본이 멀쩡히 있는데 가져올 수 없다
```

이것은 §5.1 원칙("백업본 안에 그것을 여는 키를 넣지 않는다")과 **같은 종류의 실패**다. 백업본 자체는 밖에 있지만, 그것을 여는 열쇠가 안에 있으면 결과는 같다.

**결정:**

| 항목 | 배선 |
|---|---|
| 백업 목적지 자격증명의 원천 | **컨트롤 플레인 네임스페이스의 K8s Secret.** 스택 OpenBao 를 **거치지 않는다** |
| ESO / ExternalSecret | **사용하지 않는다** — ESO 자체가 스택 구성요소다 |
| 에스크로 | `ENCRYPTION_KEY`·unseal key 와 **같은 등급으로 오프라인 보관**(§5.2) |
| 복구 시 | 0단계 전제 확인에 **"목적지 자격증명 확보"** 를 포함(§6.1) |

> 즉 키 자재가 3종에서 **4종**이 된다. §5.2 표를 그에 맞게 확장한다.

#### 4.2.2 네트워크 요구사항 — 검증 필요

| 항목 | 현재 상태 |
|---|---|
| **Egress(클러스터 → 외부 스토리지)** | ⚠️ **미확인.** zadara PoC 문서는 ingress 만 기술한다 — *"외부 → node-10은 22/tcp만"*(`README.md:156`). 나가는 방향은 기록이 없다. **B4-2 에서 실측한다** |
| **TLS / 사설 CA** | ✅ 이미 지원. 차트가 사내 CA 를 신뢰 저장소에 병합한다(`deployment.yaml:38-39`, `caBundle.secretName`) |
| 대역폭 | §1.5 규모를 정지 창 안에 올려야 한다. **정지 창 길이를 결정하는 것은 디스크가 아니라 이 링크다** |

> **대역폭이 정지 창을 지배한다.** 로컬 디스크는 100MB/s 급이지만 내부망 링크가 1Gbps(≈125MB/s)면 비슷하고, 그보다 낮으면 정지 창이 그만큼 늘어난다. **B4-1 2단계에서 실측한 정지 창이 이 설계의 수용 가능성을 판정한다.**

### 4.3 압축·중복제거 — 선택이 아니라 필수

§1.5 기준 한 세대가 수십 GB, 보존 3세대면 수백 GB 다. 그대로는 성립하지 않는다.

| 대책 | v1 |
|---|---|
| `tar` + `zstd`(또는 `gzip`) 스트리밍 압축 | ✅ 5단계에서 파이프로 처리 — 중간 파일을 만들지 않는다(디스크 여유가 없다) |
| 세대 간 증분 | ❌ v1 제외 — 전체 백업만. **증분은 복구 검증을 어렵게 만든다** |
| 중복제거 | ❌ v1 제외 — Phase 2 에서 kopia 도입 시 자연 획득 |

> 증분을 v1 에서 빼는 이유: 이 EPIC 의 완료 기준은 *복구가 실제로 되는 것*이다. 증분 체인은 한 세대가 깨지면 이후 전체가 깨지고, 그 검증은 전체 백업보다 훨씬 복잡하다. **먼저 되게 만들고, 그 다음 작게 만든다.**

### 4.4 산출물 구조

```
backup-<run-id>/
  manifest.json              # 평문
  platform-db.dump.enc       # pg_dump -Fc              (A1/A2)
  keycloak-db.dump.enc       # pg_dump -Fc              (A3) — 통합 후에도 별개 database 이므로 파일도 별개
  openbao-kv.json.enc        # KV 논리 export           (B1)
  namespace-resources.yaml.enc   # 리소스 + Helm 릴리스  (D1)
  volumes/<pvc-name>.tar.zst.enc # PVC 별 아카이브       (E1)
```

`manifest.json` 에 담기는 것 — **열어보기 전에 복구 가능 여부를 판단할 수 있어야 한다:**

| 필드 | 용도 |
|---|---|
| `schema_version` | `schema_migrations` 최신값(현재 74). §6.2 기준 |
| `pg_server_version` / `pg_dump_version` | §3.1 버전 함정 |
| `workloads[]` — 이름·종류·**정지 전 replica 수** | §3.4 3단계 기록. **복원의 필수 입력** |
| `volumes[]` — PVC 이름·크기·StorageClass·`sha256` | 복원 시 PVC 재생성 파라미터 |
| `quiesce_window` — 정지 시작/종료 시각 | 실제 다운타임 기록. RTO/정지창 산정 근거 |
| `components[]` + 각 `sha256` | 무결성 |
| `encryption.key_id`, `algorithm` | **어떤 키로 잠갔는지.** 키 자체는 절대 넣지 않는다 |

**`manifest.json` 은 암호화하지 않는다.** 키를 잃은 상황에서도 "무엇이 들어 있고 어떤 키가 필요한가"는 읽을 수 있어야 한다. 대신 **비밀값을 한 조각도 넣지 않는다** 를 테스트로 고정한다(B1-4).

### 4.5 보존 정책

| 항목 | 값 |
|---|---|
| 일간 / 주간 / 월간 | 7 / 4 / 3 — **단, §1.5 규모에서는 목적지 용량이 실질 상한이다** |
| 총량 상한 | 설정값. 초과 시 **알림 후** 가장 오래된 일간부터 삭제 |
| 삭제 방식 | 파일 제거 후에도 `backup_runs` 이력은 남긴다 — "언제 백업이 끊겼나"의 근거 |

**감사로그가 무한 증가한다.** `audit_logs`·`stack_deploy_logs`·`notification_history`·`token_rotation_events` 에 보존 코드가 없다(실측 0건). 회전 스케줄러가 5분마다 도는 것을 감안하면 꾸준히 자란다. 매니페스트에 컴포넌트별 크기를 기록하고 **직전 대비 증가율이 임계를 넘으면 경고**한다. 로그 테이블 자체의 수명주기는 **이 EPIC 밖**이므로 별도 이슈로 분리한다(Q4).

---

## 5. B0-5 — 시크릿 취급 정책

### 5.1 원칙

> **백업본 안에, 그 백업본을 여는 키를 넣지 않는다.**

위반하면 백업본 하나 유출 = 전부 유출이다.

### 5.2 키 자재 4종

| 키 | 지금 어디에 | 위험 | v1 처리 |
|---|---|---|---|
| **`ENCRYPTION_KEY`** | Helm Secret → 파드 환경변수 | DB 백업본과 함께 잃으면 **클러스터 접근 영구 상실**(§1.3) | **사람이 보관.** 백업본에 **넣지 않는다** |
| **OpenBao unseal key** | 대상 클러스터 K8s Secret — **밖으로 안 나간다** | 클러스터 소실 시 **금고 영구 봉인**(§5.3) | **사람이 보관.** 반출 절차 런북화 |
| **백업 암호화 키** | (신설) | 산출물을 잠근다 | **`ENCRYPTION_KEY` 와 다른 키.** 같으면 하나 잃고 둘 다 잃는다 |
| **백업 목적지 자격증명** (S3 access/secret key) | (신설) — **컨트롤 플레인 K8s Secret** | **스택 OpenBao 에 두면 순환 의존**(§4.2.1) — 스택이 죽으면 백업본을 가져올 수 없다 | **사람이 보관.** ESO 를 거치지 않는다 |

**자동화 / 사람 보관 경계 (B2-2):**

| 자동화한다 | 사람이 보관한다 |
|---|---|
| 산출물 암호화·복호화, 무결성 검증, 참조 정합성 검사, unseal key **반출 명령 제공** | 키의 물리적 보관, 에스크로 승인, 클러스터 소실 시 키 반입 |

> **플랫폼은 키를 다루는 절차를 제공하지만 키를 대신 보관하지 않는다.** 플랫폼이 보관하면 플랫폼 소실 = 키 소실이 되어 백업의 의미가 사라진다.

### 5.3 가장 큰 단일 위험 — unseal key 가 금고와 같은 곳에 있다

```
openbao-init.go:31       "초기화 결과는 파드 내부 emptyDir 로만 전달되므로
                          unseal key 가 대상 클러스터를 벗어나지 않는다."
openbao-values.go:22     OpenBaoUnsealKeysSecret = "openbao-unseal-keys"
openbao-init.go:19-20    KeyShares = 1 / KeyThreshold = 1
```

평상시엔 옳은 설계다. 코드 주석도 1/1 로 둔 이유를 밝힌다 — *"…분할이 런타임 보안을 높이지 않고 복잡도만 늘린다. **오프라인 백업본을 여러 관리자에게 나누려는 조직만 이 값을 올리면 된다.**"*

**마지막 문장이 이 EPIC 이 마주한 상황이다.** 클러스터가 사라지면 Secret 도 사라지고, **백업 시점에 금고가 열려 있어야 export 를 뜰 수 있으므로** 봉인된 채 남은 상황에서는 export 조차 불가능하다.

> 정지 백업이 이 위험을 **부분적으로** 덜어준다 — OpenBao PVC 를 파일째 뜨므로 §3.2 B안이 함께 성립한다. **그러나 그 파일을 복원해도 unseal key 없이는 열 수 없다.** 키 반출은 여전히 필수다.

**v1 필수 조치 (B2-2):**
1. **unseal key 반출 절차 런북화** — 설치 직후 오프라인 매체로.
2. **반출 여부를 플랫폼이 확인·경고** — 스택 상세에 *"이 스택의 unseal key 가 아직 반출되지 않았습니다"* 배너. 강제하지 않되 침묵하지도 않는다.
3. `KeyShares`/`Threshold` 상향을 조직 선택지로 문서화 — 이미 상수라 구조 변경 불필요.

> root token 은 Kubernetes Auth 검증 후 폐기되고, unseal key 만 있으면 `bao operator generate-root` 로 재발급된다(`openbao-bootstrap.go:230-236`). **백업 대상에서 제외한다.**

### 5.4 #68(BYOK) 종속 — 무엇을 지금 정하고 무엇을 미루는가

이슈는 *"B0-5 는 #68 의 Cycle 4 결정(~2026-09-14)을 기다린다"* 고 했다. **전부 기다릴 필요는 없다.**

| 지금 확정 (BYOK 무관) | #68 결정 대기 |
|---|---|
| 백업본에 키를 넣지 않는다 | 백업 암호화 키를 **사용자 제공 키로 감쌀지**(봉투 암호화) |
| `ENCRYPTION_KEY` 에스크로 절차 | 키 관리 주체 — 플랫폼 생성 vs 사용자 반입 |
| unseal key 반출 절차 | 키 회전 시 기존 백업본 재암호화 여부 |
| 알고리즘 — AES-256-GCM(`pkg/crypto` 재사용) | KMS/HSM 연동 |

**암호화를 인터페이스 뒤에 둔다.** v1 은 플랫폼 생성 키를 쓰되 `manifest.json` 에 `encryption.key_id` 를 남겨, 나중에 BYOK 구현체로 교체해도 기존 백업본을 계속 열 수 있게 한다. **9/14 결정을 기다리느라 Phase 1 착수가 막히지 않는다 — 10월 시한을 지키는 데 이것이 핵심이다.**

```go
// internal/backup/port/sealer.go
type Sealer interface {
    KeyID() string
    Seal(ctx context.Context, plaintext io.Reader, out io.Writer) error
    Unseal(ctx context.Context, ciphertext io.Reader, out io.Writer) error
}
// v1: PlatformKeySealer (AES-256-GCM 스트리밍)
// #68 이후: EnvelopeSealer (사용자 KEK 로 DEK 를 감싼다)
```

> ⚠️ E1 규모에서는 **스트리밍 암호화**여야 한다. `pkg/crypto/aes_gcm.go` 의 현재 API 는 `[]byte` 전체를 받아 base64 문자열을 돌려주므로(`Encrypt(key, plaintext []byte) (string, error)`) **수십 GB 에 그대로 쓸 수 없다.** 같은 알고리즘의 스트리밍 변형이 필요하다 — B1-4 의 구현 항목으로 명시한다.

---

## 6. 복구 설계

**복구는 "dump 를 되돌리는 것"이 아니다.** 순서를 지키지 않으면 각 단계가 서로를 깨뜨린다.

### 6.1 복구 순서

| # | 단계 | 실패 시 |
|---|---|---|
| **0** | **전제 확인** — `ENCRYPTION_KEY`, 백업 암호화 키, **목적지 자격증명**(§4.2.1), 목적지 **네트워크 도달성**이 모두 확보되었는가 | **중단.** 키 없이 진행하면 "복구된 것처럼 보이는 망가진 상태"가 된다 |
| **1** | 무결성 검증 — `manifest.json` 의 sha256 대조 | 중단 |
| **2** | **스키마 버전 정합성 검사**(B1-2) — §6.2 | 중단 |
| **3** | **대상 네임스페이스 정리** — 기존 릴리스·PVC 제거 | **파괴적.** 확인 문자열 요구(§7.2) |
| **4** | **PVC 재생성** — 매니페스트의 크기·StorageClass 대로 | 크기 부족 시 중단 |
| **5** | **볼륨 복원 Job** — 아카이브를 PVC 로 풀어쓴다. **워크로드는 아직 0** | 롤백 |
| **6** | **리소스 복원** — 네임스페이스 리소스 + Helm 릴리스 Secret | — |
| **7** | 플랫폼 DB / Keycloak DB 복원 — **database 단위로**(§6.6) | Keycloak 실패는 §6.3 |
| **8** | OpenBao — 금고 기동 → unseal → (필요 시) KV import → 부트스트랩 Job 재실행 | 9단계에서 드러남 |
| **9** | **워크로드 재개** — 매니페스트의 `workloads[].replicas` 로 복원 | — |
| **10** | **참조 정합성 검사** — §6.4 | **경고 + 목록 보고.** 중단하지 않되 조용히 넘기지도 않는다 |
| **11** | 스모크 — 로그인 → 클러스터 목록 → **kubeconfig 복호화** → 스택 상세 → **Git 저장소 clone** → **이미지 pull** → 파이프라인 재실행 | — |

> **5단계와 9단계의 분리가 핵심이다.** 볼륨을 채우기 전에 워크로드가 뜨면 도구들이 **빈 디스크를 보고 초기화**해 버린다 — GitLab 은 새 인스턴스로 재초기화되고, 그 순간 복원이 무의미해진다. **데이터를 먼저, 프로세스를 나중에.**
> 11단계에 **kubeconfig 복호화**를 명시한 이유: §1.3 실패를 잡아내는 유일한 지점이다. "화면이 뜬다"로는 검출되지 않는다. **Git clone·이미지 pull** 은 E1 이 실제로 복원됐는지 확인하는 지점이다.

### 6.2 스키마 버전 정합성 (B1-2)

`golang-migrate` 의 `schema_migrations` 가 기준이다(현재 74).

| 상황 | 동작 |
|---|---|
| 백업 == 현재 | 그대로 복원 |
| 백업 < 현재 | **복원 → 그 다음 `migrate up`.** 순서 역전 시 실패 |
| 백업 > 현재 | **차단** — 구버전 코드가 신버전 스키마를 읽는 것은 정의되지 않은 동작 |
| `dirty` 플래그 | **차단** — 마이그레이션이 중단된 백업본은 신뢰할 수 없다 |

> ⚠️ **Helm 훅과 충돌한다.** 마이그레이션 Job 은 `post-install,pre-upgrade` 훅으로 자동 실행된다(`deploy/helm/nullus/templates/migration-job.yaml:26`). **복구 도중 `helm upgrade` 를 하면 마이그레이션이 예상 밖 시점에 끼어든다.** 런북에 *"복구 중에는 `helm upgrade` 를 하지 않는다"* 를 명시한다.

### 6.3 Keycloak 복원 실패는 치명적이지 않다

`users.password_hash` 경로가 살아 있으면(A1 포함) 관리자는 포털 ID/PW 로 진입할 수 있다. 마이그레이션 주석이 목적을 밝힌다 — *"IdP 가 죽어도 들어갈 수단이 있어야 하므로"*(`000073`). 7단계 Keycloak 실패 시 **중단하지 말고 경고 후 진행**하며, 11단계에 **"ID/PW 로그인 진입 가능"** 을 별도 체크로 넣는다.

### 6.4 참조 정합성 검사

```
for each row in token_sources where deleted_at is null:
    if not OpenBao.exists(row.path) → dangling 목록에 추가
```
결과를 `restore_runs.integrity_report` 에 저장하고 화면에 표시한다. dangling 은 **해당 토큰의 재등록이 필요하다**는 뜻이며, 사용자가 알아야 조치할 수 있다.

### 6.5 부분 복구

전체 복구만 지원하면 실제 사고 대응에 못 쓴다. **v1 은 두 가지 축소 모드를 제공한다:**

| 모드 | 대상 | 용도 |
|---|---|---|
| `platform-only` | A1/A3/B1 (§6.1 의 0~2, 7, 8, 10) | 컨트롤 플레인만 손상. **스택은 건드리지 않는다** |
| `stack-only` | D1/E1 (§6.1 의 3~6, 9) | 스택 네임스페이스만 손상 |

전체 복구는 두 모드의 합이며 §6.1 순서를 그대로 따른다.

---

### 6.6 공유 인스턴스에서의 database 단위 복원

DB 통합(§1.2 결정 1) 이후에는 두 database 가 한 인스턴스에 있다. **복원은 반드시 database 단위여야 하며, 인스턴스를 통째로 되돌리면 안 된다** — 한쪽을 복원하다 다른 쪽을 날린다.

| 규칙 | 이유 |
|---|---|
| `pg_restore --dbname=<대상>` 으로 **대상 database 만** 지정한다 | 인스턴스 전체 복원 금지 |
| database 재생성이 필요하면 **제3의 database(`postgres`)에 접속해서** `DROP`/`CREATE` 한다 | 접속 중인 database 는 drop 할 수 없다 |
| 복원 대상이 아닌 database 의 **연결을 끊지 않는다** | 플랫폼 DB 를 복원하는 동안 Keycloak 이 죽을 이유가 없다 |
| `platform-only` 모드는 `nullus` database 만 건드린다 | §6.5 |

통합 전(현재)에는 인스턴스가 둘이라 이 문제가 없다. **통합이 백업 설계에 새 제약을 만드는 유일한 지점이므로 명시한다.**

## 7. 데이터 모델과 API

### 7.1 테이블

```sql
-- backup_runs : 한 번의 백업 실행
id UUID PK
org_id UUID NOT NULL
stack_id UUID                       -- E1 대상 스택. NULL 이면 플랫폼 전용 백업
trigger VARCHAR(20) NOT NULL        -- manual | schedule
mode VARCHAR(20) NOT NULL           -- full | platform_only | stack_only
scope TEXT[] NOT NULL               -- {platform_db, keycloak_db, openbao_kv, ns_resources, volumes}
status VARCHAR(20) NOT NULL         -- pending|running|succeeded|partial|failed
schema_version INTEGER
quiesce_started_at TIMESTAMPTZ      -- §3.4 정지 창 — 실제 다운타임 근거
quiesce_ended_at TIMESTAMPTZ
manifest JSONB NOT NULL DEFAULT '{}'::jsonb
total_bytes BIGINT
error TEXT
started_at / finished_at / created_at TIMESTAMPTZ

-- backup_artifacts : 산출물 1건
id UUID PK
backup_run_id UUID NOT NULL REFERENCES backup_runs(id) ON DELETE CASCADE
component VARCHAR(40) NOT NULL      -- platform_db | keycloak_db | openbao_kv | ns_resources | volume
resource_name TEXT                  -- component=volume 일 때 PVC 이름
location TEXT NOT NULL              -- 경로. 값 자체는 담지 않는다
size_bytes BIGINT
checksum_sha256 TEXT NOT NULL
encryption_key_id TEXT
created_at TIMESTAMPTZ

-- restore_runs
id UUID PK
backup_run_id UUID REFERENCES backup_runs(id)
mode VARCHAR(20) NOT NULL
status VARCHAR(20) NOT NULL
schema_check JSONB                  -- §6.2
integrity_report JSONB              -- §6.4 dangling 목록
started_at / finished_at TIMESTAMPTZ
```

> **`status = partial` 을 둔 이유:** 세 컴포넌트 중 일부만 성공하는 경우가 실제로 생긴다(예: DB 는 떴는데 볼륨 Job 이 실패). `failed` 로 뭉뚱그리면 *"부분적으로 쓸 수 있는 백업"* 과 *"아무것도 없는 상태"* 가 구분되지 않는다. **복구 시점에 그 차이가 결정적이다.**
> **`quiesce_*` 를 별도 컬럼으로 둔 이유:** 정지 창은 사용자가 감수하는 비용이다. 실제 다운타임이 얼마였는지 이력으로 남지 않으면 정지 창 정책을 조정할 근거가 없다.
> **비밀값은 어느 컬럼에도 담지 않는다** — `location` 은 경로, `manifest` 는 메타데이터뿐. 테스트로 고정한다(B1-4).

### 7.2 API

| Method | Path | 용도 |
|---|---|---|
| `POST` | `/api/v1/backups` | 백업 트리거 (`mode`, `stack_id`). 비동기 |
| `GET` | `/api/v1/backups` | 이력 조회 |
| `GET` | `/api/v1/backups/:id` | 상세 + 매니페스트 |
| `POST` | `/api/v1/backups/:id/verify` | **무결성 검증만** (복원 없이) |
| `POST` | `/api/v1/restores` | 복구 실행 — §6.1 0~2단계 선검사 |
| `GET` | `/api/v1/restores/:id` | 진행/검사 결과 |
| `DELETE` | `/api/v1/backups/:id` | 보존 정책 외 수동 삭제 |

**권한:** 전 경로 조직 관리자 이상, `audit_logs` 기록 필수.
**확인 절차:** `POST /backups`(정지 창 발생)와 `POST /restores`(파괴적)는 **대상 식별자 재입력**을 요구한다. 백업이 다운타임을 만든다는 사실을 사용자가 모르고 누르면 안 된다.

> `verify` 를 둔 이유: **복구 리허설 없이도 "이 백업이 열리기는 하는가"를 상시 확인**할 수 있어야 한다. "백업은 되는데 복원이 안 됐다"에 대한 최소한의 상시 방어선이다.

---

## 8. 코드 배치 — 새 바운디드 컨텍스트 `internal/backup/`

| 근거 | 내용 |
|---|---|
| 독립 Aggregate | `BackupRun` 이 자체 식별자와 수명주기를 갖는다 |
| 테이블 소유 | `backup_runs`/`backup_artifacts`/`restore_runs` — CLAUDE.md 모듈별 소유 원칙 |
| **다른 모듈 테이블을 직접 읽지 않는다** | §6.4 는 `token_sources` 를 읽지만 **admin 모듈의 공개 인터페이스를 통해서만** |
| Ubiquitous Language | BackupRun, RestoreRun, Artifact, QuiesceWindow, RetentionPolicy, Sealer |

```
internal/backup/
  domain/
    backup_run.go        # Aggregate Root, 상태 전이(partial 포함)
    artifact.go
    quiesce.go           # 정지/재개 순서와 replica 복원 규칙 — 순수 함수
    retention.go
    errors.go
  usecase/
    run_backup.go        # §3.4 8단계
    run_restore.go       # §6.1 순서를 여기서 강제한다
    verify_backup.go
    apply_retention.go
  port/
    repository.go
    dumper.go            # DBDumper (pg_dump/pg_restore)
    kv_exporter.go       # OpenBaoKVExporter
    workload_scaler.go   # 정지/재개 (§3.4 3·6단계)
    volume_archiver.go   # PVC 덤프/복원 Job (§3.5)
    resource_dumper.go   # 네임스페이스 리소스 + Helm 릴리스
    sealer.go            # §5.4 BYOK 교체점 — 스트리밍
    artifact_store.go    # S3 호환 외부 스토리지 (§4.2). 자격증명은 컨트롤 플레인 Secret 에서만 온다 (§4.2.1)
  adapter/
    handler/backup_handler.go
    repository/postgres_backup.go
    postgres/pg_dumper.go        # 버전 선택 (§3.1)
    openbao/kv_exporter.go       # §3.2 A안
    kube/workload_scaler.go      # replica 0 ↔ 복원
    kube/volume_archiver.go      # busybox tar Job
    kube/resource_dumper.go
    store/s3_store.go
  scheduler/
    backup_scheduler.go  # B3-1 — token_rotation.go 패턴 재사용
```

**스케줄러는 선례를 따른다.** `internal/admin/scheduler/token_rotation.go` 가 `interval`/`iterTimeout`/`inFlight atomic.Bool`(중복 실행 방지)/`slog` 골격을 확립해 두었다. 재사용하면 B3-1 의 설계 비용이 거의 0 이다. **정지 백업은 중복 실행이 특히 치명적이므로**(정지 창이 겹치면 워크로드가 못 뜬다) `inFlight` 가드가 필수다.

### 8.1 TDD 계획 (B1-4)

| 레이어 | 테스트 |
|---|---|
| `domain/quiesce.go` | **replica 복원 규칙** — 정지 전 3이면 재개 후 3. 정지 중 실패 시 반드시 재개. **0이었던 워크로드를 1로 되살리지 않는다** |
| `domain/retention.go` | 일7/주4/월3 경계, 총량 초과, 빈 이력 |
| `domain/backup_run.go` | 상태 전이 — 특히 `partial` 진입/이탈 |
| `usecase/run_restore.go` | **§6.1 순서 강제** — 키 부재 시 중단, 백업버전>현재 차단, dirty 차단, **볼륨 복원 전 워크로드 기동 금지** |
| `adapter/postgres/pg_dumper.go` | **§3.1 버전 선택** — 서버 17.5 에 14.8 클라이언트를 고르지 않는다 |
| `adapter/kube/volume_archiver.go` | PVC 목록 → Job 스펙. **모든 PVC 가 마운트에 포함**되는지 |
| `port/sealer.go` 구현체 | **스트리밍** — 대용량 입력에서 메모리가 선형 증가하지 않는다(§5.4) |
| `adapter/repository` | 이력 저장/조회 | `testcontainers` |
| **불변식** | 매니페스트·DB 어디에도 **비밀값이 없다** — golden 파일 + 금칙어 스캔 |
| **불변식** | **목적지 자격증명이 스택 OpenBao/ESO 를 경유하지 않는다**(§4.2.1) — 배선 테스트로 고정. 이 실수는 평시에 드러나지 않고 **재해 시에만** 드러난다 |
| **불변식** | **Keycloak DB 의 네임스페이스·릴리스명을 하드코딩하지 않는다**(§1.2) — 배포 경로에 따라 다르다. 조회로 찾는지 테스트로 고정 |
| E2E | 백업 → 검증 → 복구 → 스모크 (Playwright, B3-2 UI 포함) |

> `quiesce.go` 를 순수 함수로 뽑는 이유: **정지 후 재개 실패가 이 설계의 최악 시나리오**다(백업하려다 서비스를 못 살림). 클러스터 없이 전 경로를 테스트할 수 있어야 한다.

---

## 9. 실패 모드

| # | 실패 | 왜 위험한가 | 대응 |
|---|---|---|---|
| **F1** | `ENCRYPTION_KEY` 유실 후 DB 복구 | 화면은 정상, **모든 클러스터 조작 불가** | §6.1 0단계 + 11단계 복호화 스모크 |
| **F2** | unseal key 가 클러스터와 함께 소실 | 금고 영구 봉인 | §5.3 반출 절차 + 미반출 배너 |
| **F3** | **정지 후 재개 실패** | **백업하려다 서비스를 못 살린다** | `defer` 재개 보장 + `quiesce.go` 전 경로 테스트 + 재개 실패 시 즉시 알림 |
| **F4** | **볼륨 복원 전 워크로드 기동** | 도구가 빈 디스크를 보고 **재초기화** → 복원 무의미 | §6.1 5·9단계 분리를 usecase 에서 강제 |
| **F5** | DB/금고 백업 시점 skew | dangling 참조가 **런타임에야** 드러난다 | §2.1 묶음 실행 + §6.4 검사 |
| **F6** | `pg_dump` 버전 < 서버 버전 | **조용히 실패**하거나 부분 dump | §3.1 버전 선택 + 테스트 |
| **F7** | **목적지 자격증명을 스택 OpenBao 에서 조달** | 스택이 죽으면 **백업본이 멀쩡해도 못 가져온다** | §4.2.1 — 컨트롤 플레인 Secret + 에스크로. 배선을 테스트로 고정 |
| **F7b** | **정지 중 외부 스토리지 도달 불가**(링크 단절·인증 만료) | 정지 창을 쓰고 산출물이 없다 | 정지 **전에** 목적지 연결·인증·쓰기 권한을 검사(§9 F8 과 같은 자리) |
| **F8** | 목적지 용량 부족으로 백업 중단 | 정지 창만 쓰고 산출물이 없다 | 사전 용량 검사 → 부족 시 **정지 전에** 실패. 크기는 **실행 시점 PVC 조회**로 구한다(§1.5) |
| **F9** | 복구 중 `helm upgrade` 개입 | 마이그레이션 훅이 예상 밖 시점에 실행 | §6.2 런북 금지 조항 |
| **F10** | **백업 중단이 알려지지 않음** | "백업이 있다고 믿는" 상태가 가장 위험하다 | B3-3 실패 알림 + 마지막 성공 시점 상시 노출 |

> **F3·F7b·F8 이 정지 백업 채택으로 새로 생긴 위험이며, 셋 다 같은 처방을 공유한다 — 정지 창에 들어가기 전에 실패할 수 있는 것은 전부 먼저 검사한다.** 정지 창을 소비하고도 산출물을 못 만드는 것이 최악의 조합이다. §3.4 절차에서 목적지 검사(연결·인증·쓰기·용량)는 **3단계 정지보다 앞**에 온다.
> **F10 이 가장 흔하고 가장 비싸다.** 백업 실패보다 백업 실패를 모르는 것이 더 나쁘다. 알림(#63 의존)이 선택 기능이 아닌 이유다.

---

## 10. 검증 계획 (B4 — 완료 기준)

### 10.1 왜 이것이 완료 기준인가

> 2026-07-19 회의, 이기하: *"OpenStack 기반의 OS 볼륨 백업은 카카오클라우드에서 **백업은 되지만 복원이 잘 안 됐던 경험**이 있어 신뢰하기 어렵다."*

**복구가 검증되지 않은 백업은 백업이 아니다.** 리허설 통과 전까지 이 EPIC 은 완료가 아니며, "구현 완료"로 보고하지 않는다.

### 10.2 B4-1 복구 리허설

| # | 단계 | 기록할 것 |
|---|---|---|
| 1 | **기준 환경 구성** — 스택 1개 전 도구 설치, **GitLab 저장소에 커밋 3회**, **Harbor 에 이미지 push**, **Jenkins 빌드 2회 실행**, **MinIO 오브젝트 업로드**, 클러스터 2개 등록, 토큰 1회 회전, **Keycloak 에 일반 사용자 계정 1개 생성** | 각 도구의 **검증 가능한 지문**(커밋 SHA, 이미지 digest, 빌드 번호, 오브젝트 etag) |
| 2 | 백업 실행 → 산출물이 **외부 오브젝트 스토리지에 적재됨을 확인** | 소요 시간, **실제 정지 창**, 크기, **egress 대역폭**(§4.2.2) |
| 3 | 키 자재 반출 확인 (§5.2) | 체크리스트 |
| 4 | **환경 초기화** — 네임스페이스 삭제 후 재설치 (부분 초기화 금지) | — |
| 4b | **목적지 독립성 확인** — 4단계 이후에도 외부 스토리지의 산출물이 **온전한지** 검증(`verify`) | 이것이 §4.2 결정의 실증이다 |
| 5 | 복구 실행 — §6.1 0~11단계 | **각 단계 소요 시간** → RTO 실측 |
| 6 | **검증 — 지문 대조** | ① `git log` 커밋 SHA 3개 일치 ② 이미지 digest 일치 및 pull 성공 ③ Jenkins 빌드 번호·로그 존재 ④ MinIO 오브젝트 etag 일치 ⑤ DB 행 수 ⑥ 로그인 ⑦ **kubeconfig 복호화** ⑧ dangling 0건 ⑨ 파이프라인 재실행 성공 |
| 7 | **Keycloak 복원 검증** — 백업 전에 만든 **일반 사용자 계정으로 로그인**되는가, realm 커스터마이징(로그인 테마)이 남아 있는가 | Q2 가 "백업한다"로 확정됐으므로, 판정이 아니라 **검증** 항목이다 |
| 8 | **부분 복구 모드 검증**(§6.5) | `platform-only` / `stack-only` 각각 |

**합격선:** 6단계 전 항목 통과 + 5단계 총 시간이 §2 RTO 이내. **초과하면 목표치를 실측값으로 갱신하거나 절차를 줄인다** — 목표를 그대로 두고 넘어가지 않는다.

> 1단계의 **"검증 가능한 지문"이 이 리허설의 핵심**이다. "GitLab 이 뜬다"는 복원 검증이 아니다. **백업 전에 넣은 커밋이 복원 후에도 같은 SHA 로 있는가** — 그것만이 "그 상태 그대로"의 증거다.

### 10.2.1 1차 리허설 결과 (2026-09-02) — **통과**

축소 규모로 실행했다. 실제 OSS 대신 PVC 를 잡는 워크로드 하나를 썼고, 검증한 것은 **메커니즘**이다 — 정지 → 볼륨 아카이브 → 네임스페이스 파괴 → PVC 재생성 → 볼륨 복원 → 워크로드 재개.

| 구성 | 실제 |
|---|---|
| 클러스터 | kind (k8s 1.35.1), **StorageClass `rancher.io/local-path`** — §3.4 가 전제한 제약을 그대로 재현 |
| 플랫폼 DB | PostgreSQL 18 (컨테이너). **Keycloak DB 를 같은 인스턴스의 별도 database 로** 두어 §1.2 결정 1 의 통합 배치도 함께 검증 |
| 목적지 | MinIO (컨테이너), 클러스터 밖 |
| 코드 | `internal/backup/rehearsal/` (`-tags rehearsal`), 2회 연속 통과 |

| 단계 | 소요 |
|---|---|
| 기준 환경 구성 | 6.5s |
| **백업 실행** | **3.5s** (그중 **실제 정지 창 3.1s**) |
| 환경 초기화(네임스페이스 삭제) | 48.1s |
| **복구 실행** | **6.4s** |
| 워크로드 기동 대기 | 2.0s |
| 지문 대조 | 0.2s |

**지문 대조 결과 — 전부 일치:** 볼륨 파일 sha256(파드 안에서 계산) · 파일 크기 262144 bytes · Deployment replica 2 · 플랫폼 DB 행 3.

> ⚠️ **이 숫자는 RTO 근거가 아니다.** 데이터가 256KiB 다. §2.2 의 RTO 4시간은 100Gi 급을 전제하며, **실환경 리허설로 대체해야 한다.** 여기서 검증된 것은 시간이 아니라 **절차가 실제로 성립하는가** 다.

#### 이 리허설이 잡아낸 것 — 구현만으로는 드러나지 않았던 결함 4건

리허설을 마지막에 두지 않기로 한 판단(§12.1)이 여기서 값을 했다. 단위 테스트 전부와 CI 를 통과한 코드에서 아래가 나왔다.

| # | 결함 | 증상 | 조치 |
|---|---|---|---|
| **1** | `pg_dump` 버전 가드가 **minor 까지** 비교 | 클라이언트 18.3 / 서버 18.4 에서 백업이 막힘. **서버가 마이너 패치만 받아도 백업이 멈춘다** — 막으려던 것과 정반대 방향의 사고 | major 기준으로 정정(§3.1). PostgreSQL 의 실제 호환 규칙이 major 다 |
| **2** | 리소스 덤프가 **없는 CRD 에서 전체 실패** | Gateway API 가 없는 클러스터에서 `ns_resources` 가 통째로 실패. `--ignore-not-found` 는 없는 *객체*를 다루지 없는 *리소스 타입*은 다루지 못한다 | `api-resources` 로 존재하는 종류만 추려서 조회 |
| **3** | Keycloak DB **미설정 시 이해 불가한 실패** | `pg_dump` 가 로컬 소켓에 붙으려다 죽음. **기본 설정이 빈 값이라 모든 백업이 `partial` 이 될 수 있었다** | 기동 시점에 차단 + 런타임에 명확한 메시지 |
| **4** | 리소스 덤프에 **서버 관리 필드가 그대로** | 복구 시 `resourceVersion`·`uid`·`status` 때문에 apply 가 "the object has been modified" 로 실패 | 덤프 시 정리(server-managed 필드·자동 생성 객체·`clusterIP` 제거) |
| **5** | **PVC 를 두 경로가 소유** | 볼륨 경로(EnsurePVC)와 리소스 경로(apply)가 같은 PVC 를 다뤄 `spec.volumeName` 충돌로 복구 실패 | PVC 는 **볼륨 경로가 단독 소유**. 리소스 덤프에서 제외 |

> **1·3 은 "조용히 잘못되는" 부류라 특히 비쌌다.** 둘 다 백업이 실패했다는 사실이 늦게 드러나는 형태이고, §9 F10 이 지목한 바로 그 위험이다.

#### 리허설 자신에 대한 교훈

첫 시도에서 복원된 파일이 **잘린 것처럼** 보였다(262144 → 98304 bytes). 원인은 백업이 아니라 **검증 도구**였다 — 지문을 `kubectl exec` 의 stdin 으로 밀어넣었는데 그 경로가 조용히 잘랐다. 리허설이 자기 기준선을 세우지 못하면 **백업을 잘못 고발한다.**

지금은 지문 생성과 해시를 **파드 안에서** 하고, kubectl 로는 짧은 결과만 받는다. 큰 데이터를 검증 도구가 나르지 않는 것이 원칙이다.

### 10.2.1b 인클러스터 리허설 (2026-09-02) — **Q9 해소**

축소 리허설(§10.2.1)은 코드를 **호스트 프로세스로** 돌린다. 실제로는 플랫폼이 **파드 안에서** 돌고 거기서 클러스터 밖 스토리지로 나가야 하는데, **그 egress 경로가 §4.2.2 에서 "미확인" 으로 남아 있었다.** 위상을 실제와 맞춘 별도 리허설로 그것을 닫았다.

`scripts/backup-rehearsal-incluster.sh` · 2회 연속 통과.

| 구성 | 실제 |
|---|---|
| 플랫폼 | **kind 안의 파드** — 차트로 배포(`deploy/helm/nullus`) |
| 플랫폼 DB | 차트의 PostgreSQL 서브차트. Keycloak DB 는 `initdb` 로 **같은 인스턴스의 별도 database** (§1.2 결정 1) |
| 목적지 | **클러스터 밖** — kind 도커 네트워크의 별도 컨테이너. 운영의 "조직 내부망 오브젝트 스토리지" 와 같은 자리 |

**결과:** 백업 `succeeded`, 산출물 2건(71KiB + 871B)이 **클러스터 밖 스토리지에서 조회됨**. `verify` 로 읽기 방향까지 확인.

> 목적지를 클러스터 **안**에 두지 않은 것이 요점이다. 안에 두면 리허설은 통과하지만 §4.2 와 어긋난 위상을 "검증됐다" 고 기록하게 된다 — 클러스터가 죽을 때 백업본도 같이 죽는 구성이다.

#### 이 리허설이 잡아낸 것 — 결함 3건

| # | 결함 | 증상 |
|---|---|---|
| **6** | **차트에 백업 설정이 없다** | `templates/configmap.yaml` 이 명시적 allow-list 라 `backup:` 블록이 렌더되지 않았다. **차트로 배포하면 기능을 켤 방법 자체가 없었다** — 로컬에서만 되는 기능이었던 셈이다 |
| **7** | **비밀값이 환경변수로 안 들어옴** | 봉인 키·목적지 자격증명은 ConfigMap 이 아니라 Secret 에서 와야 한다(§5.2). 그런데 viper 의 `AutomaticEnv` 는 **설정 파일에 있는 키만** 본다 — `NULLUS_BACKUP_SEAL_KEY` 가 조용히 무시됐다. `config.go` 주석이 이미 지목한 함정을 그대로 밟았다 |
| **8** | 스택 없는 백업이 `partial` | 금고는 스택마다 배포되는데, 스택이 없어도 export 를 시도해 실패했다. **스택 설치 전의 플랫폼 백업은 항상 partial 이 됐다** — 없는 것을 못 떴다고 실패로 세면 진짜 실패를 알아볼 수 없다 |

> **6번이 특히 컸다.** 단위 테스트·통합 테스트·축소 리허설을 전부 통과하고 CI 도 초록이었는데, **정작 차트로는 배포할 수 없는 기능**이었다. 코드가 아니라 배선의 결함이라 코드 테스트로는 원리적으로 잡히지 않는다.

차트 변경: `values.yaml` 에 `config.backup` / `secrets.backupSealKey` / `secrets.backupDestinationSecretKey`, `configmap.yaml` 에 `backup:` 블록(비밀값 제외), `secret.yaml`·`deployment.yaml` 에 비밀값 주입.

### 10.2.2 실환경 리허설 (2026-09-03) — **통과. B4-1 완료**

축소 리허설이 검증하지 못한 것들을 실제 스택 위에서 확인했다. **두 가지 스택 구성**으로 각각 설치 → 파이프라인 → 애플리케이션 배포 → 백업 → 네임스페이스 파괴 → 복구 → 지문 대조를 완주했다.

| | Gitea + Jenkins + Harbor | GitLab + Argo CD |
|---|---|---|
| 파드 | 29 | 33 |
| 산출물 | 371 MB · 13건 | 255 MB · 9건 |
| **정지 창** | **50.3초** | **38.4초** |
| 복구 후 기동 | 21초 (25/26) | 41초 (29/31) |
| **지문 대조** | **8/8 일치** | **7/7 일치** |

지문은 도구가 그 바이트를 읽어 같은 상태로 살아났는지를 본다 — Git 커밋 수·HEAD SHA·커밋 메시지, CI 빌드/파이프라인 이력(실패·skipped 포함), 레지스트리 digest·태그, 그리고 **앱의 이미지 태그와 ready replica**. 세 값이 서로 맞물려 있어야 "복구된 척" 을 거른다: 레지스트리 태그는 Git 커밋 SHA 에서 오고, CD 가 그 태그로 배포 매니페스트를 되커밋한다.

**앱이 떠 있는 상태를 백업하고, 떠 있는 상태로 복구했다.** 초기 리허설들은 빌드 직후 1초 만에 백업이 나가 Argo CD 가 동기화하기 전이었고, 그래서 "앱이 안 뜬 상태" 를 백업하고 있었다 — 그 상태로는 이 항목을 검증할 수 없다.

#### 이 리허설이 잡아낸 것 — 구현과 CI 를 통과한 결함 8건

앞선 두 리허설(§10.2.1, §10.2.1b)과 마찬가지로, **단위·통합 테스트와 CI 가 모두 초록인 코드에서** 나왔다.

| # | 결함 | 성격 |
|---|---|---|
| ① | `backup_runs.stack_id` 가 UUID — 스택 대상 백업이 행 생성에서 죽음 | 타입 |
| ② | KV export 가 API server proxy transport 에서만 실패(HTTP LIST 미지원) | 운영 경로 |
| ③ | ESO `ExternalSecret`/`SecretStore` 가 덤프에 없음 | 배선 |
| ④ | 클러스터 범위 CRD 가 덤프에 없음 | 배선 |
| ⑤ | 클러스터 범위 RBAC(ClusterRole/Binding)이 덤프에 없음 | 배선 |
| ⑥ | Argo CD `Application` 이 덤프에 없음 — 복구 후 앱을 배포할 주체가 사라짐 | 배선 |
| ⑦ | Harbor 가 차트 기본값 `not-a-secure-key` 로 DB 자격증명을 암호화 | 보안 |
| ⑧ | GitLab 러너가 인증 토큰을 받아 등록 불가 — CI 전면 불능 | 자격증명 |

**③④⑤⑥ 은 같은 뿌리다.** 복구 모델이 "네임스페이스 리소스를 apply" 인데 설치는 **Helm 릴리스**로 이루어진다 — Helm 이 만드는 클러스터 범위 리소스와 컨트롤러가 재생성하는 리소스가 그 사이로 빠진다. 네 번 같은 이유로 났고, **빠진 것을 하나씩 세는 방식으로는 끝을 알 수 없다.** 근본안은 §11.2 에 남긴다.

**③⑤⑥ 은 증상이 같다**: 복구가 `succeeded` 를 반환하고도 도구들이 `CreateContainerConfigError` 로 멈춘 채 남는다. `RestoreRun.status` 는 "단계가 실행됐다" 는 뜻이지 "스택이 동작한다" 가 아니다 — 자동 복구 경로에 스모크 검증이 없다는 것이 §11.2 의 항목이 된 이유다.

#### 리허설 자신에 대한 교훈

검증 도구가 조용히 실패하면 **거짓 "일치" 를 보고한다.** 지문 수집 스크립트에서 셸 안의 따옴표 이스케이프가 깨져 세 값이 전부 빈 문자열이 된 적이 있고, 그대로 뒀으면 빈 값끼리 비교해 통과했을 것이다. 수집 실패는 **오류로 드러나야** 하고, 값이 하나라도 비면 **대조 자체를 거부해야** 한다.

같은 이유로 "성공" 표시를 믿지 않는다. Jenkins 가 `SUCCESS` 인데 로그를 보니 `Stage "Build" skipped due to when conditional` 이었던 적이 있다 — HEAD 가 CD 의 `[skip ci]` 커밋이라 아무 일도 하지 않은 성공이었다.

### 10.3 B4-2 에어갭 검증

`airgap/` 절차만으로 백업·복구가 도는지 확인한다. 확인 항목:

1. **egress 실측**(§4.2.2) — 폐쇄망 클러스터에서 외부 오브젝트 스토리지로 나가는 경로가 실제로 열리는가. **미확인 항목이므로 여기서 처음 검증된다**
2. **사설 CA** — 엔드포인트가 사내 CA 서명일 때 `caBundle` 배선으로 통하는가
3. **이미지 정합** — `mc`/`tar` 이미지가 번들에 실제로 있는가(§3.6 부수 발견)

별도 이미지 반입이 필요하면 **§3 도구 선정이 틀린 것**이므로 설계로 되돌아간다.

---

## 11. 결정 사항과 미결정 사항

### 11.1 결정됨

| # | 항목 | 결정 |
|---|---|---|
| **Q2** | Keycloak DB 취급 | **백업한다.** 재프로비저닝은 OIDC 클라이언트 등록만 되살리고 **계정·realm 커스터마이징은 못 살린다**(§1.2 결정 2) |
| **Q7** | Keycloak DB 통합 | **통합한다** — 같은 인스턴스·별도 database·별도 role. 분리는 Bitnami 서브차트 기본값의 결과이지 설계 의도가 아니었다(§1.2 결정 1). **이 EPIC 밖 — 별도 인프라 이슈** |
| **Q8** | 백업 목적지 | **클러스터 외부 S3 호환 오브젝트 스토리지**(§4.2). 자격증명은 **컨트롤 플레인 Secret 에서만** 온다(§4.2.1) |
| — | E1 범위 | **포함한다** — 설치된 OSS 내부 데이터까지 백업·복원(§1.4) |
| — | 백업 방식 | **정지 백업**(§3.4). CSI 스냅샷이 없는 환경에서 "그 상태 그대로"를 얻는 유일한 수단 |

### 11.2 미결정

| # | 항목 | 언제 정하나 | 착수를 막나 |
|---|---|---|---|
| **Q1** | 백업 암호화 키를 사용자 제공 키로 감쌀지 | #68 Cycle 4(~2026-09-14) | ❌ — §5.4 인터페이스로 우회 |
| ~~**Q9**~~ | ~~egress 경로가 실제로 열리는가~~ | ✅ **해소**(§10.2.1b). 에어갭 실환경 방화벽은 B4-2 | — |
| ~~**Q10**~~ | ~~아카이버 이미지~~ | ✅ **해소** — `busybox:1.37` 확정, `mc` 태그 불일치도 반영됨 | — |
| **Q11** | 무중단·직접 업로드가 필요해질 때 S3 SSE-C 로 Job 에 키를 넘길지 | Velero 재도입(Q5)과 같은 시점 | ❌ |
| **Q3** | OpenBao 를 raft 로 전환할지(§3.2 C안) | Phase 2, #68 과 같은 사이클 | ❌ |
| **Q5** | 무중단 백업 요구 시점 → Velero 재도입(§3.3) | 다중 노드 전환 또는 24/7 요구 시 | ❌ |
| **Q4** | 로그 테이블 보존 정책(§4.5) | **이 EPIC 밖** — 별도 이슈 | ❌ |

### 11.3 실환경 리허설이 새로 연 것 (2026-09-03)

구현과 검증을 마치고 나서야 보이는 문제들이다. **모두 이 EPIC 밖**이며 별도로 다룬다.

| # | 항목 | 왜 지금 정하지 않나 |
|---|---|---|
| **Q12** | **복구를 Helm 릴리스 재적용으로 바꿀지** | ③④⑤⑥ 결함의 공통 뿌리다(§10.2.2). 리소스를 apply 하는 대신 릴리스를 다시 설치하면 **손으로 관리하는 종류 목록이 사라진다.** 선행 조건 둘은 이미 갖췄다 — 차트 버전 기록(매니페스트 `helm_releases`)과 복구 볼륨을 통과시키는 preflight(`backup.nullus.io/restored-from`). 남은 문제는 차트가 자체 생성하는 비밀값인데, 실측해 보니 복원된 DB 를 못 읽게 만드는 것은 Harbor 의 `secretKey` 하나뿐이고 그것은 이미 금고에서 온다(⑦ 수정). **설계 판단이 필요한 규모라 별도 이슈로 뗀다** |
| **Q13** | **복구 성공 판정에 스모크 검증을 넣을지** | `succeeded` 가 "스택이 동작한다" 를 뜻하지 않는다(§10.2.2). 설계 §6.1 11단계에 스모크가 있으나 **자동 복구 경로에는 없다.** 무엇을 어디까지 확인할지가 정해져야 한다 |
| **Q14** | **Service ClusterIP 미보존의 영향 범위** | 클러스터 안에서는 DNS 로 해석되어 무해했다. IP 를 박아 둔 외부 배선이 있는 환경에서 재설정이 필요한지는 그런 환경에서 확인해야 한다 |
| **Q15** | **RTO 실측** | 371MB 로 잰 50.3초는 **규모의 근거가 아니다.** 수십 GB 에서 다시 재야 §2 의 RTO 4시간이 근거를 갖는다 |

> **기술적 차단 요인은 남아 있지 않다.** Q1·Q9·Q10 은 우회·검증·구현으로 흡수됐고, Q12~Q15 는 이 EPIC 의 완료를 막지 않는다.

---

## 12. EPIC 태스크 매핑

| 태스크 | 상태 | 본 문서 |
|---|---|---|
| **B0-1** 보호 대상 인벤토리 | ✅ | §1 — **E1·D1 포함으로 확대**, A3 백업 확정(Q2), 배포 경로 분기 발견(§1.2) |
| **B0-2** RPO/RTO | ✅ | §2 — RPO 24h / RTO **4h**(E1 반영), B4-1 에서 실측 갱신 |
| **B0-3** 도구 선정 | ✅ | §3 — `pg_dump -Fc` / **KV 논리 export**(snapshot API 부재) / **Velero v1 탈락·Phase 2 후보** / **정지 백업 채택** |
| **B0-4** 저장 위치·보존 | ✅ | §4 — MinIO 재사용 불가, **클러스터 외부 S3 호환 오브젝트 스토리지로 결정**, 자격증명 순환 의존 회피(§4.2.1) |
| **B0-5** 시크릿 취급 | ✅ | §5 — **키 4종**(목적지 자격증명 포함), #68 종속분만 분리, **스트리밍 암호화 필요 명시** |
| **B0-6** 설계 문서 | ✅ | 본 문서 |
| B1-1 ~ B1-4 | ✅ **구현 완료** | §6·§7·§8 — `internal/backup/` |
| **B2-1** OpenBao 백업/복구 | **전제 수정됨** | §3.2 — snapshot API 부재 → KV 논리 export + 정지 시 PVC 동봉 |
| B2-2 unseal 키 취급 | 설계 완료 | §5.2·§5.3 |
| **B2-1** 구현 | ✅ | KV 재귀 export/import (`adapter/openbao`) |
| **B3-1** 스케줄·보존 | ✅ 구현 완료 | §4.5 — 일7/주4/월3, 최근 성공본 보존 |
| **B3-2** UI | ✅ 구현 완료 | 관리 → 백업(`/admin/backup`) — 대상 항목별 선택, 정지 창 고지. [사용 가이드](../50_운영/Nullus_백업복구_사용_가이드.md) |
| **B3-3** 실패 알림 | ⚠️ 구조화 로그까지 | 채널 발송은 #63 에 달렸다 |
| **B3-4** 운영 런북 | ⚠️ 사용 가이드로 일부 대체 | 재해 시나리오별 런북은 남음 |
| **B4-1** 복구 리허설 | ✅ **완료** — 축소(§10.2.1, 결함 5건) · 인클러스터(§10.2.1b, 결함 3건) · **실환경 2개 스택**(§10.2.2, 결함 8건). 지문 8/8 · 7/7 일치 | §10 |
| B4-2 에어갭 검증 | 절차 확정 | §10.3 |
| **B1-5 / B1-6** 볼륨·리소스 | ✅ 구현 완료 | 정지 백업/복원, 리소스 덤프/복원 |
| **분리 제안** | — | **Keycloak DB 통합**(§1.2 결정 1) · **로그 테이블 보존 정책**(Q4) · **Helm 재적용 복구**(Q12) · **복구 스모크 검증**(Q13) — 이 EPIC 밖 |
| **부수 수정** | ✅ | `mc` 이미지 태그 정합(§3.6) · 에어갭 런타임 이미지 5종 · Harbor 암호화 키(⑦) · GitLab 러너 등록 토큰(⑧) |

### 12.1 남은 일 (2026-09-03 기준)

Phase 1~3 구현과 B4-1 이 끝났다. 아래가 남는다.

| 순서 | 작업 | 왜 |
|---|---|---|
| **1** | **키 자재 4종 에스크로 런북**(`ENCRYPTION_KEY` · unseal key · 봉인 키 · 목적지 자격증명) | 코드 없이 오늘 가능하고 그 자체로 현재 리스크를 낮춘다. **키를 잃으면 백업본이 있어도 복구할 수 없다** |
| **2** | **B4-2 에어갭 검증**(§10.3) | 폐쇄망 egress 는 §10.2.1b 로 확인되지 않는다. 방화벽 변경 리드타임이 있으므로 먼저 확인만이라도 |
| **3** | **B3-3 알림 채널**(#63 종속) · **B3-4 재해 런북** | RPO 를 실제로 보장하려면 실패가 사람에게 닿아야 한다 |
| **4** | **RTO 실측**(Q15) — 수십 GB 규모 | §2 의 4시간이 근거를 갖는 유일한 방법 |
| **5** | (별도 이슈) **Keycloak DB 통합** · **Helm 재적용 복구**(Q12) · **복구 스모크 검증**(Q13) | 이 EPIC 완료를 막지 않는다 |

> 초판의 순서 권고에서 **"검증을 UI·스케줄보다 앞에 둔다"** 가 핵심이었고, 그 판단이 값을 했다. 세 차례 리허설이 **구현과 CI 를 통과한 결함 16건**을 잡았고, 그중 8건은 실제 스택 위에서만 드러나는 것이었다. 검증을 마지막에 뒀다면 그 8건은 운영에서 처음 드러났을 것이다.

---

## 참고

- EPIC: [nullus-plan#75](https://github.com/cloud-nullus/nullus-plan/issues/75)
- 연계: [#68 보안 보강(BYOK)](https://github.com/cloud-nullus/nullus-plan/issues/68) · [#63 Alert 채널 1차 확장](https://github.com/cloud-nullus/nullus-plan/issues/63) · [#69 설치 간 용량 제어](https://github.com/cloud-nullus/nullus-plan/issues/69)
- **로컬 검증**: [`docs/50_운영/Nullus_백업복구_로컬_테스트_가이드.md`](../50_운영/Nullus_백업복구_로컬_테스트_가이드.md) — 설치부터 백업·복구까지, 자동 리허설 포함
- 관련 문서: `docs/20_아키텍처/OpenBao_시크릿_평면_구축_설계.md` · `docs/20_아키텍처/Nullus_DB_스키마.md` · `docs/70_전략/ROADMAP.md:363` · `deploy/csp/zadara/README.md:286` · `airgap/README.md`
