# OpenBao 시크릿 평면 구축 설계

**작성일**: 2026-07-28
**버전**: 1.0
**대상**: Platform Engineer, Backend Engineer, DevOps Engineer
**연관 문서**: `OpenBao_토큰_자동_갱신_설계.md`, `OpenBao_구현_체크리스트.md`, `Nullus_인프라_배포_설계.md`, `nullus_PRD_1.3.md`, `Nullus_DB_스키마.md`

---

## 0. 구현 상태 (2026-07-28)

kind 클러스터 대상 통합 테스트로 검증한 결과다. 재현 방법:

```bash
NULLUS_IT_KUBECONFIG=<path> NULLUS_IT_NAMESPACE=<prefix> \
  go test -tags=integration -timeout 35m ./internal/stack/adapter/helm/ -run TestIntegration_
```

### 0.1 구현·검증 완료

| 항목 | 검증 내용 |
|---|---|
| P1 공식 차트 전환 | StatefulSet + PVC Bound 확인. dev 모드 매니페스트 제거 |
| P1 init Job 멱등성 | 재실행 시 기존 unseal key 가 유지됨을 확인 |
| P1 unseal 사이드카 | 파드 삭제 후 자동 재개방 + 데이터 영속성 확인 |
| P1 StorageClass 선택 | `StackConfig.Storage.StorageClass`, 조회 API, 마법사 UI, preflight 검증 |
| P1 PVC/키 동시 삭제 | `data-openbao-0` + `openbao-unseal-keys` 를 삭제 대상에 등록 |
| P2 부트스트랩 Job | KV v2 를 `kv` 로 마운트, 리뷰어 토큰 미설정 확인, 정책·role 생성 |
| P2 mount 정리 | `kv`→`secret` 재작성 로직 제거, 경로 규약과 실제 마운트 일치 |
| P2 컨트롤 플레인 인증 | TokenRequest → k8s auth 로그인 → API server proxy 경유 KV 읽기/쓰기 |
| P2 Router 스택별 분리 | `(provider, stackID)` 키 + `StackSecretResolver` |
| P2 스케줄러 배선 | `TokenRotationScheduler` 를 `main.go` 에서 기동 (그전까지 미동작) |
| P3 ESO 설치 | 공식 차트, SecretStore Ready 확인(= ESO 가 OpenBao 로그인 성공) |
| P3 시크릿 프로비저닝 | 생성 → OpenBao → ExternalSecret → K8s Secret 값 일치 확인 |
| P3 최소권한 | ESO role 로 쓰기 시도 시 거부됨을 확인 |
| P3 `existingSecret` 전환 | PostgreSQL·MinIO·GitLab 차트가 프로비저닝된 Secret 을 참조. values 하드코딩 제거 |
| P3 object storage 연결 | ESO `template` 으로 연결 YAML 렌더링, 하드코딩 문자열 대체 확인 |
| P2 root token revoke | 인증 경로 증명(SecretStore Ready) 후 폐기, Secret 키 제거. 부트스트랩 재실행 경로 포함 |
| P3-SSO client secret | OpenBao → K8s Secret 복제 확인 (`sso-test-grafana` → `sso-test-grafana-oidc`) |
| P3-SSO client ID | 스택 단위 네임스페이싱 확인 (공용 realm 충돌 방지) |
| P3-SSO IdP 등록 | 3개 클라이언트가 secret push 방식으로 등록됨. confidential client 확인 |
| P3-SSO argocd-secret | admin 비밀번호 + OIDC secret 을 ESO 가 단독 소유 |
| 회전 후 반영 | `restart_required` 기반 rolling restart, provider 별 정책 단위 테스트 |
| P4 부트스트랩 자격 | **실제 Keycloak 26.0** 대상 — 발급/멱등 재발급 3회/폐기/멱등 폐기/폐기 후 재발급 전 주기 통과. 발급 토큰에 `admin` realm role 포함 확인 |
| P4 in-cluster kubeconfig | **실제 파드에서 실행** — kubeconfig 생성 후 그 자격으로 Kubernetes API 호출 성공 (v1.30.0) |
| P4 설치 스크립트 | 부트스트랩 발급 → 조직 조회 → 자기 클러스터 등록 → 스택 생성 → 배포 → `trap` 폐기 전 구간 통과. 401 강제 스텁으로 **4개 호출 전부 Bearer 인증 확인** |

**검증 중 발견해 수정한 결함 3건** — 모두 단발 설치가 아니라 재설치·다중 스택에서만 드러난다.

1. **ESO CRD 소유권 충돌** — CRD 는 클러스터 범위라 두 번째 스택 설치 시 Helm 이 실패한다. 존재하는 CRD 의 소유권을 현재 릴리스로 인수(adopt)하도록 했다. 설치를 건너뛰는 방식은 CRD 가 일부만 남은 상태에서 SecretStore 생성이 NotFound 로 실패하므로 채택하지 않았다.
2. **CRD Terminating 교착** — 삭제 직후 재설치하면 CRD 가 아직 Terminating 이라 CR 생성이 거부된다. 설치 전에 정리 완료를 기다린다.
3. **삭제 순서 결함** — ESO 오퍼레이터를 ExternalSecret 보다 먼저 uninstall 하면 finalizer 를 처리할 주체가 사라져 **네임스페이스와 CRD 가 영구 Terminating** 상태가 되고 같은 클러스터에 재설치가 불가능해진다. 커스텀 리소스를 오퍼레이터보다 먼저 지우도록 삭제 순서를 고쳤다.
4. **role 매핑 침묵 실패** — 부트스트랩 클라이언트 생성 후 UUID 재조회가 실패하면 realm role 매핑이 조용히 건너뛰어졌다. 토큰은 정상 발급되는데 Admin API 에서만 403 이 나므로 원인이 발급 시점과 멀어진다. 명시적으로 실패시키도록 고쳤다.
5. **헤더 단어 분리** — 설치 스크립트가 `${NULLUS_TOKEN:+-H "Authorization: Bearer $TOKEN"}` 를 비인용 확장해 헤더가 여러 인자로 쪼개졌다. 인증이 실리지 않은 채 요청이 나갔다. 배열 전달로 고쳤다.
6. **환경변수 export 순서** — heredoc 안의 python 이 읽는 `CLUSTER_ID` 를 heredoc 뒤에 export 해 `KeyError` 가 났다.

### 0.2 부분 구현 / 남은 과제

- **P4 에어갭 통합** — 구성 요소는 각각 실제 컴포넌트로 검증했다(0.1 참조). 다만 **에어갭 번들 전체를 빌드해 오프라인으로 돌리는 종단 검증은 미완**이다. 82개 이미지 번들 빌드가 필요해 이번 범위에서 수행하지 못했다. 남은 미검증 구간은 다음과 같다.
  - 번들 빌드/로드(`01`~`03`), 로컬 레지스트리 push(`10`~`12`)
  - `21-install-nullus.sh` 로 배포된 **실제 nullus-api 파드**에서의 self-register (in-cluster kubeconfig 생성 자체는 파드에서 검증했으나, API 핸들러 경유는 스텁으로 대체)
  - Keycloak 차트 조건부 의존성의 실제 렌더링/설치
  - 기존 우회 스크립트(`27`, `30`)는 폐기 예정 표시만 했고 제거하지 않았다
- **OpenBao HA / 클러스터 내부 TLS**
- **미지원 provider 회전 fallback** — 랜덤 문자열을 생성해 성공 처리하는 경로가 남아 있다

---

## 1. 목적

PRD 5.2와 인프라 배포 설계 8.3이 선언한 **OpenBao-first 원칙** — 원문 시크릿의 Source of Truth는 OpenBao이고 Kubernetes Secret은 파생 리소스 — 을 실제로 성립시키기 위한 구축 설계다.

`OpenBao_토큰_자동_갱신_설계.md`가 토큰의 **회전 정책**을 다룬다면, 이 문서는 그 전제가 되는 **시크릿 평면 자체의 구축**을 다룬다. 회전 컨트롤러가 아무리 정교해도 금고가 영속화되지 않고, 인증이 정적 토큰이며, 값이 앱까지 도달하는 경로가 없으면 회전은 의미를 갖지 못한다.

세 단계로 나눈다.

| Phase | 범위 | 해소되는 문제 |
|---|---|---|
| **P1. 운영 모드 전환** | dev 모드 → 영속 스토리지 + init + auto-unseal | 재시작 시 시크릿 전량 소실 |
| **P2. Kubernetes Auth** | 정적 토큰 → 단기 자격 + 최소권한 정책 | PRD 5.2 "정적 토큰 하드코딩 금지" 위반 |
| **P3. 주입 평면(ESO)** | OpenBao → K8s Secret → 앱 | SoT 선언이 성립하지 않는 상태 |

세 단계는 **P1 → P2 → P3 순서가 강제**된다. P1의 init이 root token을 만들어야 P2의 인증 설정이 가능하고, P2의 role이 있어야 P3의 ESO가 로그인할 수 있다.

---

## 2. 현재 상태와 문제

| 영역 | 현재 구현 | 문제 |
|---|---|---|
| 배포 | 자체 작성 Deployment, `server -dev -dev-root-token-id=root` | 인메모리 스토리지 — **파드 재시작 시 시크릿 전량 소실**. PVC 없음. 이미지 태그 `latest` |
| 초기화 | 없음 (dev 모드는 자동) | 운영 모드 전환 시 init 절차 부재 |
| Unseal | 없음 (dev 모드는 자동) | 운영 모드 전환 시 재시작마다 봉인 |
| 인증 | `OPENBAO_TOKEN` 정적 토큰을 `X-Vault-Token` 헤더로 직접 사용 | PRD 5.2 위반. 권한 분리 없음(단일 root) |
| 주입 | 없음 | 설치된 OSS는 OpenBao 값을 소비하지 않음 |
| 값의 방향 | Helm values에 비밀번호를 하드코딩하고 **사후에** 같은 문자열을 OpenBao에 기록 | OpenBao가 SoT가 아니라 사후 기록부. 동일 리터럴이 5개 파일에 중복 |
| 주소 해석 | 프로세스 시작 시 `OPENBAO_ADDR` 하나로 전역 1회 등록 | OpenBao는 스택마다 배포되는데 주소는 전역 1개 |

> 참고: 운영 런북(`Nullus_인프라_배포_설계.md` 11장)의 토큰 갱신 실패 대응 절차에는 "ESO/CSI 동기화 시각 확인" 단계가 이미 포함돼 있다. 즉 문서는 주입 평면의 존재를 전제하고 있으나 구현이 따라가지 않은 상태다.

---

## 3. 목표 아키텍처

```text
[Nullus 컨트롤 플레인]
  Backend API / Rotation Controller
        │ ① kubeconfig로 SA 단기 토큰 발급(TokenRequest)
        │ ② 그 JWT로 k8s auth 로그인 → client_token(TTL 1h)
        ▼
┌─────────────────────── 대상 클러스터 / <stack-ns> ───────────────────────┐
│                                                                          │
│   StatefulSet openbao (공식 차트)                                        │
│     ├ openbao         file storage + PVC                                 │
│     └ unseal-sidecar  Secret 마운트, 로컬 폴링 → 자동 unseal             │
│                                                                          │
│   Secret openbao-unseal-keys   ← RBAC로 읽기 대상 한정                   │
│                                                                          │
│   External Secrets Operator (공식 차트)                                  │
│     │ SecretStore(nullus-openbao) + ExternalSecret(N개)                  │
│     │ role=nullus-eso, 정책=nullus-eso-read, refreshInterval=5m          │
│     ▼                                                                    │
│   Kubernetes Secret (파생)                                               │
│     ▼                                                                    │
│   GitLab / ArgoCD / Runner / Registry ...  ← existingSecret 참조         │
└──────────────────────────────────────────────────────────────────────────┘
```

역할 구분:

- **OpenBao** — 원문 시크릿의 단일 진실 공급원
- **Nullus 백엔드** — 시크릿 생성·기록·회전 (write 권한)
- **ESO** — OpenBao → K8s Secret 복제 (read 전용)
- **OSS 구성요소** — K8s Secret 소비자. OpenBao를 직접 알지 못한다

---

## 4. Phase 1 — OpenBao 운영 모드 전환

### 4.1 배포 방식

자체 작성 Deployment를 폐기하고 **공식 Helm 차트**로 전환한다. Nullus의 다른 구성요소가 모두 Helm으로 설치되므로 일관되고, 차트가 StatefulSet·PVC·ServiceAccount·`system:auth-delegator` 바인딩을 모두 제공하므로 자체 매니페스트로 관리할 대상이 사라진다.

| 항목 | 값 |
|---|---|
| 차트 | `openbao/openbao` |
| 버전 고정 | 차트·앱 버전 모두 명시 고정 (`latest` 금지) |
| 모드 | `server.standalone.enabled=true` |
| 스토리지 | `storage "file"` + `server.dataStorage` (기본 5Gi) |
| 리스너 | `tls_disable = 1` (클러스터 내부 통신, TLS는 후속 과제) |
| injector | `injector.enabled=false` (주입은 ESO가 담당) |
| UI | `ui.enabled=true` |

`server.extraContainers`로 unseal 사이드카를 주입한다.

### 4.1.1 StorageClass 선택

기본 StorageClass가 없는 클러스터에서는 PVC가 Pending에 머물러 설치가 멈춘다. 설치 마법사에서 **StorageClass를 명시적으로 선택**하게 하여 이 실패를 설치 시작 전으로 앞당긴다.

**저장 위치는 스택 설정이다.** UI에서는 리소스 프로파일과 같은 화면에 배치하되, 값 자체는 `StackConfig.Storage.StorageClass`에 둔다.

| | 리소스 프로파일 (`org_resource_profiles`) | StorageClass |
|---|---|---|
| 범위 | 조직 단위 재사용 자산 | 클러스터마다 다름 |
| 의미 | 얼마나 (`storageRequestGi` 등) | 어디에서 |
| 저장 | `org_resource_profiles` | `StackConfig.Storage.StorageClass` |

프로파일에 StorageClass를 넣으면 같은 프로파일을 다른 클러스터에 적용할 때 존재하지 않는 SC를 참조하게 된다. UI 인접 배치와 데이터 소유를 분리한다.

**목록 조회 API 신설** — `GET /api/v1/admin/clusters/:id/storage-classes`. 등록된 kubeconfig로 대상 클러스터의 StorageClass를 조회해 다음을 반환한다.

| 필드 | 용도 |
|---|---|
| `name` | 선택 값 |
| `provisioner` | 식별 보조 |
| `is_default` | 기본 선택값 결정 (`storageclass.kubernetes.io/is-default-class`) |
| `reclaim_policy` | `Retain`이면 삭제 정책 경고 표시 (4.5) |
| `volume_binding_mode` | `WaitForFirstConsumer`면 PVC가 Pending으로 보이는 것이 정상임을 안내 |

UI 동작:

- 기본 StorageClass가 있으면 자동 선택한다
- 기본 StorageClass가 없으면 **선택을 필수로 강제**한다. 미선택 상태로 다음 단계를 진행할 수 없다
- `reclaimPolicy: Retain`인 SC를 선택하면 "스택 삭제 후에도 볼륨이 남습니다"를 안내한다 (4.5의 완전 파기 미보장과 연결)

**적용 범위는 스택 전체다.** 현재 PostgreSQL·MinIO·GitLab·OpenSearch 등은 StorageClass를 지정하지 않고 차트 기본값에 의존한다. 선택된 값은 OpenBao뿐 아니라 스택이 생성하는 모든 PVC에 적용한다. P1에서는 OpenBao에 우선 적용하고, 나머지 구성요소는 동일 필드를 참조하도록 순차 전환한다.

**preflight 검증**: 설치 시작 시 선택된 StorageClass가 대상 클러스터에 실제로 존재하는지 확인한다. 설정 저장 이후 SC가 삭제·변경됐을 수 있다.

### 4.2 초기화 (Init)

대상 클러스터 안의 Job이 수행한다. 백엔드가 `/v1/sys/init`을 직접 호출하는 방식도 가능하나, 설치 초기에는 `openbao.<access_domain>` 경로가 아직 준비 전일 수 있어 도달성이 불확실하다. Job 방식은 **unseal key가 대상 클러스터를 한 번도 벗어나지 않는다**는 속성도 함께 얻는다.

```text
1. GET /v1/sys/init → initialized 확인
2. initialized == true  → 즉시 정상 종료 (아무것도 하지 않음)
3. initialized == false → operator init 실행
4. 결과를 Secret(openbao-unseal-keys)으로 생성
```

**init 멱등성은 코드로 강제한다.** 스택 설치에는 스텝 재시도 경로가 존재하므로(`install_stack.go`), 문서 경고만으로는 부족하다. 2단계 분기를 빠뜨리면 재설치·재시도가 기존 금고를 무효화하고 이전 unseal key를 못 쓰게 만든다. **P1에서 사고 확률이 가장 높은 지점이다.**

**key shares / threshold**: 기본 `1/1`. auto-unseal 구성에서는 threshold를 채우는 데 필요한 모든 조각이 어차피 같은 Secret에 존재하므로, 분할이 런타임 보안을 높이지 않고 복잡도만 늘린다. 오프라인 백업본을 여러 관리자에게 분산 보관하려는 조직을 위해 `key_shares`/`key_threshold`를 설치 옵션으로 노출한다.

### 4.3 Auto-unseal

외부 KMS(AWS KMS, GCP KMS, Transit)를 사용하지 않는다. 온프레미스 설치가 기본 시나리오이며 KMS 인프라를 전제할 수 없다.

**OpenBao 파드 내 사이드카** 방식으로 구현한다.

```text
loop:
  GET  127.0.0.1:8200/v1/sys/seal-status
  if sealed:
      POST 127.0.0.1:8200/v1/sys/unseal   (threshold 만큼 반복)
  sleep 10s
```

별도 CronJob이 `kubectl exec`로 unseal하는 방식과 비교한 근거:

| | 외부 CronJob | 파드 내 사이드카 |
|---|---|---|
| 복구 시간 | 스케줄 주기까지 대기 | ~10초 |
| 필요 RBAC | `pods`, `pods/exec` | 없음 (로컬 호출) |
| 다중 파드(HA) | 셀렉터로 지정한 파드만 처리 | 파드마다 자연 동작 |
| 키 접근 | 환경변수 주입 | 볼륨 마운트 |

사이드카는 Secret을 **볼륨으로만** 받는다. ServiceAccount에 Secret read 권한을 부여하지 않는다.

멱등성: 이미 unsealed면 아무것도 하지 않는다.

### 4.4 Unseal Key 보관 정책

**대상 클러스터의 Kubernetes Secret(`openbao-unseal-keys`)에 보관한다.**

| 선택지 | 채택 | 사유 |
|---|---|---|
| 대상 클러스터 K8s Secret | ✅ | 자동 unseal 가능. Nullus가 고객 마스터키의 보관자가 되지 않아 제품 책임 범위가 좁다 |
| Nullus 컨트롤 플레인 DB(암호화) | ✗ | `ENCRYPTION_KEY` 하나가 전 고객 금고의 상위 키가 된다. 컨트롤 플레인 침해 시 영향 범위가 전체로 확대 |
| 운영자 수동 보관(자동 unseal 없음) | ✗ | 파드 재시작마다 수동 개입 필요. 자동화 제품의 UX와 충돌하고, 설치 중 재시작 시 설치가 멈춘다 |

동반 통제:

- Secret 접근 Role은 `resourceNames`로 `openbao-unseal-keys` 하나만 허용한다. 네임스페이스 전체 Secret read 권한을 가진 주체를 만들지 않는다
- 설치 완료 화면에서 unseal key와 root token을 **1회 표시하고 다운로드를 제공**해 오프라인 백업을 유도한다
- 조회 사실은 audit 로그에 기록한다

### 4.5 스택 삭제 시 데이터 처리

**스택 삭제 시 OpenBao PVC와 `openbao-unseal-keys` Secret을 함께 삭제한다.** 보존하지 않는다.

보존이 위험한 이유는 데이터 손실 방지 효과보다 **재설치 차단 부작용**이 크기 때문이다.

| 잔존 상태 | 재설치 시 증상 |
|---|---|
| PVC만 남음 | init Job이 `initialized == true`를 보고 초기화를 건너뛴다. 그러나 금고를 열 unseal key는 이전 설치와 함께 사라져 **영구 sealed** 상태가 되고 preflight gate에서 설치가 정지한다 |
| Secret만 남음 | 새로 초기화된 금고에 이전 키를 제출하게 되어 unseal이 실패한다. 증상 동일 |

따라서 **둘은 항상 같은 생애주기를 갖는다.** 부분 실패로 한쪽만 남지 않도록 삭제 경로에서 두 리소스를 모두 best-effort로 처리한다.

구현 시 주의사항:

- **라벨 기반 정리에 의존하지 않는다.** 현재 삭제 로직은 `nullus.io/stack-name` 셀렉터로 `pvc`·`secret`을 정리하지만, 이 라벨은 게이트웨이 번들 매니페스트에만 부여된다. Helm 차트가 생성하는 OpenBao 리소스는 셀렉터에 걸리지 않는다. 차트 values로 공통 라벨을 주입하거나, 명시적 삭제 대상 목록에 추가해야 한다
- **PV reclaim policy를 보장할 수 없다.** StorageClass가 `Retain`이면 PVC 삭제 후에도 PV와 실제 볼륨이 남는다. 제품이 완전 파기를 보장할 수 없는 영역이므로, 운영 문서에 명시하고 필요 시 운영자가 PV를 직접 확인하도록 안내한다
- **soft delete와 비대칭이 발생한다.** 스택 레코드는 `deleted_at` 기반 soft delete로 이력이 보존되지만 금고는 복구되지 않는다. 삭제 확인 UI에서 "OpenBao에 저장된 시크릿이 영구 삭제되며 복구할 수 없습니다"를 명시한다

### 4.6 preflight gate 승격

현재 OpenBao 헬스 게이트는 실패해도 경고만 남기고 통과한다(`install_stack.go`). 다음을 **차단 조건**으로 승격한다.

- OpenBao 응답 없음
- `sealed == true`가 지정 시간 내 해소되지 않음
- 시크릿 엔진/인증 백엔드 미구성 (P2 완료 이후)

`OpenBao_구현_체크리스트.md` 6절의 "배포 전 OpenBao preflight gate 구현"에 해당한다.

---

## 5. Phase 2 — Kubernetes Auth

### 5.1 신원 목록

| 신원 | 위치 | 인증 경로 | 정책 |
|---|---|---|---|
| Nullus 백엔드 + 회전 컨트롤러 | 컨트롤 플레인 | TokenRequest → k8s auth | `nullus-controller-write` |
| External Secrets Operator | 대상 클러스터 | 자기 SA 토큰 → k8s auth | `nullus-eso-read` |
| unseal 사이드카 | 대상 클러스터 | **인증 불필요** (unseal은 미인증 엔드포인트) | 없음 |

### 5.2 컨트롤 플레인의 인증 경로

Nullus 백엔드는 대상 클러스터 **밖**에서 동작하므로 제출할 ServiceAccount 토큰이 없다. kubeconfig로 대상 클러스터의 SA 단기 토큰을 발급받아 사용한다.

```text
① kubeconfig → TokenRequest API로 nullus-controller SA 토큰 발급 (수명 10분)
② POST /v1/auth/kubernetes/login  {role: nullus-controller, jwt: <SA 토큰>}
③ OpenBao가 TokenReview로 검증
④ 정책이 바인딩된 client_token 수신 (TTL 1h)
⑤ 캐시. 만료 10분 전 재로그인
```

새로운 정적 비밀이 생기지 않는다. kubeconfig가 이미 Source of Truth이고, 거기서 파생되는 단기 자격만 사용한다. `client-go`가 이미 의존성에 포함돼 있어 TokenRequest를 바로 사용할 수 있다.

### 5.3 auth/kubernetes 설정

```text
bao auth enable kubernetes
bao write auth/kubernetes/config kubernetes_host="https://kubernetes.default.svc"
```

**`token_reviewer_jwt`와 `kubernetes_ca_cert`는 설정하지 않는다.** 생략하면 OpenBao가 자기 파드의 로컬 ServiceAccount 토큰(`/var/run/secrets/kubernetes.io/serviceaccount/token`)을 매 요청마다 새로 읽어 TokenReview에 사용한다. kubelet이 만료 전 자동 로테이션하므로 파드가 살아있는 한 유효하다.

정적 리뷰어 토큰을 박을 경우 발생하는 두 가지 문제를 처음부터 회피한다.

- Kubernetes 1.24+ 에서 약 1년 후 만료 → 모든 k8s 로그인이 실패
- 토큰이 특정 파드에 바인딩되어 **파드 재시작 시 무효화** → `403 permission denied`

TokenReview 호출에는 OpenBao ServiceAccount의 `system:auth-delegator` 권한이 필요하다. 공식 차트가 이를 제공하므로 4.1의 차트 전환이 선행되면 별도 작업이 없다. 자체 매니페스트를 유지할 경우 ClusterRoleBinding을 직접 생성해야 하며, 누락 시 증상이 "ESO 로그인 실패"로만 나타나 원인 파악이 어렵다.

### 5.4 Policy / Role

OpenBao 인스턴스는 스택 단위로 배포되고 스택은 조직 단위이므로, 경로를 조직까지 좁혀도 운영 부담이 없다.

```hcl
# nullus-controller-write — 백엔드 / 회전 컨트롤러
path "kv/data/nullus/{env}/{org_id}/*"     { capabilities = ["create", "update", "read"] }
path "kv/metadata/nullus/{env}/{org_id}/*" { capabilities = ["read", "list"] }

# nullus-eso-read — External Secrets Operator
path "kv/data/nullus/{env}/{org_id}/*"     { capabilities = ["read"] }
path "kv/metadata/nullus/{env}/{org_id}/*" { capabilities = ["read", "list"] }
```

```text
bao write auth/kubernetes/role/nullus-controller \
  bound_service_account_names=nullus-controller \
  bound_service_account_namespaces=<stack-ns> \
  policies=nullus-controller-write ttl=1h

bao write auth/kubernetes/role/nullus-eso \
  bound_service_account_names=external-secrets \
  bound_service_account_namespaces=<stack-ns> \
  policies=nullus-eso-read ttl=1h
```

**`delete`/`destroy`는 어디에도 부여하지 않는다.** 회전은 KV v2 위에 새 버전을 쌓는 방식이며, 그래야 회전 실패 시 이전 버전으로 되돌릴 수 있다.

### 5.5 시크릿 엔진 마운트 정리

두 가지를 함께 처리한다.

**① prod 모드는 시크릿 엔진이 자동 마운트되지 않는다.** dev 모드는 KV v2를 자동으로 붙여주지만 운영 모드는 빈 상태로 뜬다. 부트스트랩에서 명시적으로 enable해야 한다.

**② 경로 규약과 실제 마운트가 어긋나 있다.** 문서·DB의 규약은 `kv/nullus/...`인데, `internal/shared/secrets/openbao_store.go`가 mount `kv`를 `secret`으로 재작성해 실제로는 `secret/data/nullus/...`로 나간다. 마운트를 **`kv`라는 이름으로 enable하고 재작성 로직을 제거**해 규약과 실제를 일치시킨다. 정책 경로를 확정하기 전에 정리돼야 한다.

### 5.6 Root Token 폐기

부트스트랩 완료 후 root token을 revoke한다.

복구 경로가 보장되므로 안전하다 — unseal key threshold를 충족하면 `operator generate-root`로 언제든 재발급할 수 있다. 4.4의 키 보관 정책이 이를 가능하게 한다.

### 5.7 스택별 접속 경로

현재 백엔드는 프로세스 시작 시 `OPENBAO_ADDR` 하나로 provider를 전역 1회 등록한다(`cmd/api/main.go`). 경로에는 `org_id`가 들어가지만 **주소는 전역 1개**여서, 사실상 "OpenBao는 시스템에 하나"를 가정한 구조다.

인증을 클러스터별 자격으로 전환하면 이 가정과 충돌하므로 함께 정리한다.

- Router의 키를 `provider` → **`(provider, stackID)`** 로 확장
- 스택별 지연 생성 + 클라이언트 캐시
- 접속 경로는 **API server proxy** 사용

```text
/api/v1/namespaces/{ns}/services/openbao:8200/proxy/v1/...
```

kubeconfig만으로 도달하므로 컨트롤 플레인 쪽 DNS 해석이나 인증서 배포가 필요 없다. 게이트웨이 도메인(`openbao.<access_domain>`)은 UI 접속용으로 유지한다.

로컬 개발 환경은 정적 토큰 방식을 유지한다. 토큰 획득 전략을 인터페이스로 분리해 `static`(로컬) / `kubernetes`(운영) 두 구현을 두고, 허용 범위는 dev 전용으로 문서화한다.

---

## 6. Phase 3 — 주입 평면 (ESO)

### 6.1 External Secrets Operator 설치

공식 Helm 차트(`external-secrets/external-secrets`)로 설치한다. 컨트롤러·웹훅·cert-controller를 코드 수정 없이 그대로 사용하며, Nullus가 작성하는 것은 `SecretStore`/`ExternalSecret` 리소스뿐이다.

버전은 고정하고 에어갭 번들에 포함한다(10장).

### 6.2 SecretStore

스택이 단일 네임스페이스에 설치되므로 `SecretStore` 하나로 충분하다. `ClusterSecretStore`는 사용하지 않는다.

```yaml
apiVersion: external-secrets.io/v1
kind: SecretStore
metadata:
  name: nullus-openbao
  namespace: <stack-ns>
spec:
  provider:
    vault:
      server: "http://openbao.<stack-ns>.svc.cluster.local:8200"
      path: "kv"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "nullus-eso"
          serviceAccountRef:
            name: "external-secrets"
```

### 6.3 시크릿 생성 방향 전환

**P3의 본질은 ESO 설치가 아니라 값의 방향을 뒤집는 것이다.**

```text
[현재]  Helm values에 비밀번호 하드코딩 → 설치 → 사후에 같은 문자열을 OpenBao에 기록

[목표]  Nullus가 랜덤 생성 → OpenBao write → ExternalSecret → K8s Secret → Helm이 existingSecret 참조
```

현재 `nullus-gitlab-password` 등 동일한 리터럴이 `values.go`, `helm-values.go`, `object-storage-buckets.go`, `token_source_inputs.go` 등 5개 지점에 중복돼 있다. 전환과 함께 제거한다.

인프라 배포 설계 7장의 values 예시에는 `existingSecret` 필드가 이미 정의돼 있다. 코드가 그 방향을 따라가지 않았을 뿐이다.

**부수 효과 — Secret 소유권 충돌이 대부분 사라진다.** ESO가 생성하는 Secret은 `creationPolicy: Owner`로 ESO가 소유자가 되어 주기적으로 덮어쓴다. 같은 이름의 Secret을 다른 주체가 만들면 충돌하지만, `existingSecret` 패턴에서는 Helm 차트가 Secret을 만들지 않고 참조만 하므로 소유자가 ESO 하나로 통일된다.

> **예외 — `argocd-secret`.** ArgoCD는 하나의 Secret에 admin 비밀번호 해시와 OIDC client secret을 함께 담으며, 차트가 `configs.secret.extra`로 이를 생성한다. `existingSecret` 치환이 성립하지 않으므로 **ESO가 Secret 전체를 소유하고 차트의 생성을 끈다**(`configs.secret.createSecret: false`). 두 값을 하나의 ExternalSecret에 함께 담는다. 상세는 `Nullus_OSS_SSO_자동로그인_설계.md` 7.7을 따른다.

### 6.4 시크릿 지도

`provisioning_secrets` 스텝이 생성·저장할 목록이며, ExternalSecret 작성의 입력이 된다.

| OpenBao 경로 | 생성 방식 | K8s Secret | 소비자 | 회전 후 재시작 |
|---|---|---|---|---|
| `.../storage/postgresql/password` | 랜덤 32B | `nullus-postgresql` | PostgreSQL, GitLab | 필요 |
| `.../artifacts/minio/root-password` | 랜덤 32B | `nullus-minio` | MinIO, GitLab(object storage) | 필요 |
| `.../artifacts/gitlab/root-password` | 랜덤 32B | `gitlab-initial-root-password` | GitLab | 필요 |
| `.../artifacts/{registry}/token` | provider 발급 | `*-registry-auth` (dockerconfigjson) | Job Pod | 불필요 |
| `.../pipeline/{ci}/runner-token` | provider 발급 | `gitlab-runner-secret` | Runner | **필요** |
| `.../pipeline/argocd/admin-password` | 랜덤 + bcrypt | `argocd-secret` | ArgoCD | 필요 |
| `.../auth/{client_id}/client-secret` | 랜덤 32B | OSS별 상이 (6.7) | Grafana/ArgoCD/Harbor/MinIO/GitLab | 필요 |

경로 접두사는 `kv/nullus/{env}/{org_id}/` 규약을 따른다.

ArgoCD 관리자 비밀번호는 **해시와 평문을 분리 저장**한다. 실제 로그인에 사용되는 값은 `argocd-secret`의 bcrypt 해시이고, OpenBao에는 운영자 조회용 평문 사본을 둔다. Admin `reveal` API가 이 사본을 참조한다.

### 6.5 ExternalSecret

`refreshInterval: 5m`, `creationPolicy: Owner`를 기본으로 한다. 레지스트리 인증처럼 형식 변환이 필요한 경우 `target.template`으로 `kubernetes.io/dockerconfigjson`을 생성한다.

OpenBao 값 변경은 최대 5분 내 반영되며, 즉시 반영이 필요하면 강제 동기화 어노테이션을 사용한다.

### 6.6 회전 후 반영

시크릿 지도의 "회전 후 재시작" 열이 반영 전략의 스펙이다. 소비 방식에 따라 재시작 필요 여부가 달라진다.

- **Runner** — `config.toml`을 기동 시 1회만 렌더링한다. K8s Secret이 갱신돼도 이미 떠 있는 파드는 이전 값을 유지하므로 **회전 주체가 rolling restart까지 책임진다**
- **ArgoCD repository Secret** — 매 요청 시점에 읽으므로 재시작이 필요 없다

스택 구성요소 메타데이터에 `restart_required` 플래그로 반영하고, 회전 컨트롤러가 이를 참조한다. 상세는 `OpenBao_토큰_자동_갱신_설계.md` 8장을 따른다.

### 6.7 OIDC client secret 연계

OSS SSO 연동에 사용하는 OIDC client secret도 이 평면에서 관리한다. PRD 5.2가 "OIDC client secret은 OpenBao 경유로만 주입"을 규정하고 있으나, 현재는 `airgap/helm/stack-values/*.yaml`에 `*-dev-secret` 리터럴 5종이 하드코딩돼 있고 코드 생성 values에는 OIDC 설정 자체가 없다.

**Nullus가 생성해 Keycloak에 push한다.** Keycloak이 생성한 값을 읽어오지 않는다 — OpenBao가 SoT여야 하고, Keycloak이 유실돼도 복원 가능해야 하기 때문이다.

```text
provisioning_secrets   random(32) → OpenBao write → ExternalSecret → K8s Secret
        ↓
provisioning_sso       OpenBao read → Keycloak upsert(clientId, secret, redirectURIs)
        ↓
installing_{oss}       K8s Secret 참조
```

`provisioning_sso`는 **Keycloak 기동 이후**여야 하므로 설치 스텝 의존성에 제약이 하나 추가된다(7장).

전체 설계 — Keycloak 조달 경로, client ID 네임스페이싱, Go 프로비저너 보강, 코드 생성 values의 OIDC 주입 — 는 `Nullus_OSS_SSO_자동로그인_설계.md` 7장을 따른다.

---

## 7. 설치 스텝 배선

```text
installing_openbao              P1  공식 차트 + PVC + unseal 사이드카
  ↓
openbao_init                    P1  init Job (멱등)
  ↓
openbao_bootstrap               P2  engine / auth / policy / role / root revoke
  ↓
installing_external_secrets     P3  ESO 차트
  ↓
provisioning_secrets            P3  랜덤 생성 → OpenBao write → SecretStore/ExternalSecret → K8s Secret 생성 대기
  ↓
provisioning_sso                P3  OpenBao read → Keycloak client upsert (Keycloak 기동 이후)
  ↓
installing_postgresql / minio / gitlab / argocd / runner ...   existingSecret 참조
```

`provisioning_secrets`에는 **K8s Secret 생성 대기가 반드시 포함된다.** ExternalSecret을 apply해도 ESO가 실제 Secret을 만들기까지 시간이 걸리며, 그 전에 후속 Helm 설치가 `existingSecret`을 참조하면 파드가 기동에 실패한다.

기존 스텝 의존성 그래프(`install_stack.go`)에서 Phase B 구성요소들의 `deps`에 `provisioning_secrets`를 추가한다.

---

## 8. 코드 변경 지점

| 파일 | 작업 | Phase |
|---|---|---|
| `internal/stack/adapter/helm/manifest-builders.go` | 자체 OpenBao 매니페스트 제거 → 차트 설치로 전환 | P1 |
| `internal/stack/adapter/helm/openbao-init.go` (신규) | init Job 매니페스트 + 완료 대기 + 멱등 분기 | P1 |
| `internal/stack/usecase/install_stack.go` | 스텝 추가 및 deps 재배선, 헬스 게이트 차단 조건 승격 | P1, P3 |
| `internal/shared/secrets/openbao_store.go` | 토큰 획득 전략 인터페이스 분리(static / kubernetes), mount 재작성 로직 제거 | P2 |
| `internal/shared/secrets/k8s_auth.go` (신규) | TokenRequest → login → client_token 캐시/갱신 | P2 |
| `internal/shared/secrets/store.go` | Router 키를 `(provider, stackID)`로 확장 | P2 |
| `cmd/api/main.go` | 전역 등록 제거 → resolver 주입 | P2 |
| `internal/admin/scheduler/token_rotation.go` | 전역 provider 확인 → 스택별 해석. **스케줄러 기동 배선 추가** | P2 |
| `internal/stack/adapter/helm/openbao-bootstrap.go` (신규) | engine/auth/policy/role 부트스트랩 Job | P2 |
| `internal/stack/adapter/helm/external-secrets.go` (신규) | ESO 차트 설치 스텝 | P3 |
| `internal/stack/adapter/helm/secret-provisioning.go` (신규) | 생성 → write → ExternalSecret → 대기 | P3 |
| `internal/auth/adapter/keycloak/client.go`, `sso_provisioner.go` | client secret 파라미터, **upsert 전환**, PKCE/webOrigins, 파이프라인 배선 | P3 |
| `internal/stack/port/` | SSO 프로비저너 인터페이스 정의 (모듈 직접 의존 회피) | P3 |
| `internal/stack/adapter/helm/values.go`, `helm-values.go`, `object-storage-buckets.go` | 하드코딩 비밀번호 → `existingSecret` 참조 | P3 |
| `internal/stack/usecase/token_source_inputs.go` | 하드코딩 bootstrap 문자열 및 placeholder 값 제거 | P3 |
| `airgap/images/images.txt` | ESO 이미지 추가, OpenBao 태그 고정 | P1, P3 |

`internal/admin/scheduler/token_rotation.go`의 `TokenRotationScheduler`는 현재 정의부 외에 **호출되는 곳이 없어 실제로 동작하지 않는다.** P2에서 Router 구조를 손볼 때 기동 배선을 함께 추가한다.

---

## 9. 보안 트레이드오프 (필수 고지)

외부 KMS를 사용하지 않는 구성이므로 다음 트레이드오프가 존재하며, **릴리스 노트와 설치 가이드에 명시한다.**

> 대상 클러스터가 침해되면 `openbao-unseal-keys` Secret도 함께 노출되어 금고가 열릴 수 있다. 이는 KMS 인프라를 전제하지 않는 온프레미스 설치를 지원하기 위해 감수한 선택이다.
>
> 완화 조치:
> - `openbao-unseal-keys` Secret에 대한 RBAC를 `resourceNames` 단위로 제한한다
> - 설치 시 발급된 unseal key와 root token을 오프라인에 백업한다
> - 네임스페이스 전체 Secret read 권한을 가진 주체를 만들지 않는다
>
> KMS / Transit 기반 auto-unseal은 후속 옵션으로 제공 예정이다.

**확장 포인트**: seal 설정을 차트 values로 주입 가능한 구조로 유지한다. 이후 KMS/Transit 옵션을 추가할 때 배포 구조를 변경하지 않아도 된다.

---

## 10. 에어갭

에어갭 번들(`airgap/`)에 다음을 반영한다.

- ESO 이미지 3종(controller, webhook, cert-controller) 및 차트 추가
- ESO용 values를 `airgap/helm/stack-values/`에 추가
- **OpenBao 이미지 태그 고정** — 현재 `airgap/images/images.txt`에 `openbao/openbao:latest`가 등재돼 있다. 에어갭 번들은 재현 가능해야 하므로 mutable 태그를 허용하지 않는다

`airgap/scripts/00-generate-images.sh`의 드리프트 검사(`MODE=check`) 대상에 포함되는지 확인한다.

---

## 11. 호환성 / 마이그레이션

**기존 설치본과 호환되지 않는 변경이다.**

- P1: dev 모드는 인메모리 스토리지이므로 보존할 데이터가 없다. 마이그레이션 없이 전환 가능하다
- P3: 비밀번호가 고정 리터럴에서 랜덤 생성으로 바뀌므로 기존 스택은 자동 이관되지 않는다

두 변경을 **동일 릴리스의 breaking change로 묶어 한 번에 처리**하고, `Nullus_릴리즈 정책.md`에 따라 CHANGELOG에 영향 범위를 명시한다.

스택 삭제 시 데이터 처리 정책은 4.5를 따른다 — PVC와 unseal-keys Secret을 함께 삭제하며, 복구되지 않는다.

---

## 12. 결정 사항 요약

1. OpenBao·ESO 모두 **공식 Helm 차트**를 사용한다. 자체 매니페스트를 유지하지 않는다
2. 외부 KMS를 사용하지 않으며, unseal key는 **대상 클러스터의 Kubernetes Secret**에 보관한다
3. auto-unseal은 **OpenBao 파드 내 사이드카**로 구현한다
4. init은 클러스터 내 Job이 수행하며 **멱등성을 코드로 강제**한다
5. 인증은 **Kubernetes Auth**로 통일하고, 컨트롤 플레인은 TokenRequest로 발급한 SA 단기 토큰을 사용한다
6. `token_reviewer_jwt`·`kubernetes_ca_cert`는 **설정하지 않는다**
7. 부트스트랩 완료 후 **root token을 revoke**한다
8. 시크릿은 **생성 → OpenBao → ESO → K8s Secret → `existingSecret` 참조** 순서로 흐른다
9. 회전 후 반영은 **소비자별 재시작 필요 여부**에 따라 분기한다
10. Secret Router는 **스택 단위로 분리**하고 API server proxy로 접속한다
11. 스택 삭제 시 **PVC와 unseal-keys Secret을 함께 삭제**한다. 한쪽만 잔존하면 재설치가 불가능해지므로 보존하지 않는다

---

## 13. 후속 과제

이 문서의 범위 밖이며 별도로 다룬다.

- OpenBao HA 구성 및 클러스터 내부 TLS
- KMS / Transit auto-unseal 옵션
- provider별 차등 회전 주기 정책
- 백업/복구(snapshot) 절차 및 정기 복구 리허설
- kubeconfig 암호화 키의 OpenBao 이관 (`Nullus_DB_스키마.md` 13.2.1)
