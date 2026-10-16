# Nullus 백업 키 자재 에스크로 런북

> 백업본이 있어도 **키를 잃으면 복구할 수 없는** 네 가지 비밀값을 어디에 어떻게 보관하고, 돌리고, 잃었을 때 무엇을 할 수 있는지 적는다.
> 설계: [`Nullus_백업복구_설계.md`](../11_기능설계/Nullus_백업복구_설계.md) §1.3 · §5 · §12.1 — 카드: nullus-plan#85 (nullus-plan#75 의 B2-2)

작성일: 2026-10-12

**이 문서는 운영자를 위한 것이다.** 백업을 뜨고 되돌리는 절차는 [백업/복구 사용 가이드](./Nullus_백업복구_사용_가이드.md)에 있다. 이 문서는 그 절차가 **되게 하는 조건** — 키 — 만 다룬다.

두 원칙이 전부를 결정한다.

1. **백업본 안에, 그 백업본을 여는 키를 넣지 않는다**(설계 §5.1). 넣으면 백업본 하나가 새는 순간 전부 샌다. 그래서 키는 백업과 **다른 곳**에 사람이 보관한다.
2. **플랫폼은 키를 다루는 절차를 제공하지만 키를 대신 보관하지 않는다**(설계 §5.2). 플랫폼이 보관하면 플랫폼 소실 = 키 소실이라 백업의 의미가 사라진다.

---

## 1. 키 자재 4종

| 키 | 형식 | 무엇을 여는가 | 런타임 위치 | 배포 때 넣는 곳 | 잃으면 |
|---|---|---|---|---|---|
| **`ENCRYPTION_KEY`** | 정확히 32바이트 | `clusters.kubeconfig`(AES-256-GCM) — 등록된 클러스터에 닿는 유일한 열쇠(설계 §1.3) | 컨트롤 플레인 Secret `<release>` 의 `encryption-key` → 파드 env `ENCRYPTION_KEY` | 차트 `secrets.encryptionKey`(Zadara 는 GitHub Actions secret `NULLUS_ENCRYPTION_KEY` 가 `--set`) | 복구해도 **어떤 클러스터도 조작할 수 없다** — 모든 클러스터 재등록 |
| **백업 봉인 키** | 정확히 32바이트, `ENCRYPTION_KEY`·DB 비밀번호와 **다른 값**(같으면 기동이 막힌다) | 백업 산출물 전부(AES-256-GCM 스트리밍). 매니페스트만 평문 | Secret `<release>` 의 `backup-seal-key` → env `NULLUS_BACKUP_SEAL_KEY`; 키 ID 는 `config.backup.sealKeyId`(기본 `platform-key-v1`) | 차트 `secrets.backupSealKey` + `config.backup.sealKeyId` | **모든 백업본이 열리지 않는다** — 네 키 중 가장 치명적 |
| **스택 OpenBao unseal key** | 스택마다 1조각(`KeyShares=1`), 스택 네임스페이스 Secret `openbao-unseal-keys` 의 `key1`(+ `root-token`) | 그 스택의 금고. 사이드카가 이 Secret 으로 자동 unseal 한다 | 대상 클러스터 — **밖으로 안 나간다**(설계 §5.3) | 설치가 만든다(`openbao-init` Job). 사람이 넣는 값이 아니다 | 클러스터와 함께 잃으면 금고 영구 봉인. 단, **스택 백업(`ns_resources`)이 이 Secret 을 담는다** — 백업 + 봉인 키가 있으면 복구로 돌아온다 |
| **백업 목적지 자격증명** | S3 호환 access key / secret key | 백업본이 놓인 오브젝트 스토리지 | Secret `<release>` 의 `backup-destination-secret-key` → env `NULLUS_BACKUP_DESTINATION_SECRET_KEY`; access key 는 ConfigMap `backup.destination.access_key` | 차트 `config.backup.destination.accessKey` + `secrets.backupDestinationSecretKey` | 백업본을 **가져올 수 없다**. 백업본 자체는 무사 — 스토리지 관리자가 재발급하면 끝 |

> **root token 은 보관하지 않는다.** unseal key 만 있으면 `bao operator generate-root` 로 다시 만든다(`openbao-bootstrap.go`). 플랫폼이 스택 금고에 붙을 때도 토큰이 아니라 Kubernetes Auth(컨트롤러 ServiceAccount, `StackSecretResolver`)를 쓴다 — 보관할 토큰이 없다.
>
> **플랫폼 전역 금고를 따로 두었다면**(`OPENBAO_ADDR`/`OPENBAO_TOKEN` 으로 등록한 것) 그 unseal key 와 토큰도 같은 규칙으로 보관한다. 차트는 전역 금고를 깔지 않으므로 이 문서의 기본 가정에는 없다.

---

## 2. 보관 규칙

| 항목 | 규칙 | 왜 |
|---|---|---|
| **장소** | 키마다 **두 곳** — ① 조직의 비밀 관리 도구(팀 금고, 접근 로그가 남는 것) ② 오프라인 사본(봉인한 봉투·금고의 USB). **백업 목적지와 같은 계정·같은 스토리지에는 두지 않는다** | 한 곳이 사라져도 남아야 하고, 백업본과 열쇠가 한 곳에서 함께 새지 않아야 한다(원칙 1) |
| **분리** | `ENCRYPTION_KEY` 와 봉인 키는 **다른 보관함**(다른 금고 항목·다른 봉투). 같은 종이에 적지 않는다 | 하나를 잃을 때 둘 다 잃지 않는다(설계 §5.2). 코드도 두 값이 같으면 기동을 막는다 |
| **주체** | 릴리스 담당 + 인프라 담당 **2인**. 열람은 둘 중 하나, 변경(회전·폐기)은 둘이 함께 | 한 사람의 부재가 복구를 막지 않고, 한 사람의 실수가 키를 바꾸지 못하게 |
| **기록** | 키마다 아래 서식 한 줄. **값은 적지 않고 지문(sha256 앞 16자)만** 적는다 | 어느 백업이 어느 키로 잠겼는지(`encryption.key_id`), 보관본이 지금 값과 같은지 값 없이 대조할 수 있다 |
| **GitHub Actions secret 은 보관이 아니다** | `NULLUS_ENCRYPTION_KEY` 같은 CI secret 은 **읽어 낼 수 없다**. 배포 입력일 뿐이다 | CI secret 에만 있는 키는 러너와 컨트롤 플레인이 함께 사라지는 순간 없어진다 |
| **외부 노출 금지** | 채팅·이슈·PR·커밋 메시지·로그에 값을 적지 않는다. 터미널에서 꺼낸 값은 히스토리(`HISTCONTROL`)·클립보드를 지운다 | 키는 백업본보다 오래 산다 |

에스크로 기록 서식(팀 금고의 메모 또는 운영 위키):

| 키 | 키 ID / 대상 | 지문(sha256 16자) | 생성일 | 보관 ① | 보관 ② | 마지막 검증 | 담당 |
|---|---|---|---|---|---|---|---|
| ENCRYPTION_KEY | 컨트롤 플레인 `nullus` | `…` | 2026-… | 팀 금고 "nullus/encryption-key" | 금고 봉투 #1 | 2026-… | … |
| 봉인 키 | `platform-key-v1` | `…` | | 팀 금고 "nullus/backup-seal-key" | 금고 봉투 #2 | | |
| unseal key | 스택 `stk_…` / ns `nullus-…` | `…` | | 팀 금고 "nullus/openbao/<스택>" | — (백업본이 담는다) | | |
| 목적지 자격증명 | `<endpoint>/<bucket>` | `…` | | 팀 금고 "nullus/backup-destination" | 스토리지 관리자가 재발급 가능 | | |

---

## 3. 지금 당장 — 현재 값 반출 (첫 에스크로)

운영 중인 컨트롤 플레인에서 값을 꺼내 §2 의 두 곳에 넣는다. 설치 직후, 그리고 **첫 백업을 뜨기 전에** 한다 — 키 없이 뜬 백업은 열 수 없는 파일이다.

```bash
# 컨트롤 플레인 — 차트 릴리스 이름이 Secret 이름이다 (helm list -n <ns>)
NS=nullus; REL=nullus
for k in encryption-key backup-seal-key backup-destination-secret-key; do
  v="$(kubectl -n "$NS" get secret "$REL" -o jsonpath="{.data.$k}" | base64 -d)"
  printf '%-32s len=%s sha256=%s\n' "$k" "${#v}" "$(printf %s "$v" | shasum -a 256 | cut -c1-16)"
done
# 값 자체는 화면에 찍지 않는다. 금고에 넣을 때만:
#   kubectl -n "$NS" get secret "$REL" -o jsonpath='{.data.encryption-key}' | base64 -d | pbcopy   (붙여 넣은 뒤 pbcopy < /dev/null)

# 봉인 키 ID — 백업 매니페스트의 encryption.key_id 와 같아야 한다
kubectl -n "$NS" get configmap "$REL" -o yaml | grep seal_key_id

# 스택 금고 unseal key — 스택마다, 그 스택의 클러스터 컨텍스트로
STACK_NS=nullus-gitlab-argocd-sonarqube
kubectl --context <스택 클러스터> -n "$STACK_NS" get secret openbao-unseal-keys -o jsonpath='{.data.key1}' | base64 -d \
  | shasum -a 256 | cut -c1-16        # 지문만. 값은 금고에 넣을 때만 꺼낸다
```

`ENCRYPTION_KEY`·봉인 키의 `len` 은 **32** 여야 한다. 다르면 지금 돌고 있는 값이 잘못된 것이다(기동 검사가 막았을 것이다 — 그래도 눈으로 본다).

> **에어갭·VM 설치**는 설치 스크립트에 넘긴 값이 곧 키다(`airgap/INSTALL.md` 의 `--set secrets.encryptionKey=…`, `deploy/csp/vm-cluster/runbook_csp.sh` 의 `ENCRYPTION_KEY`). 설치 머신의 셸 히스토리나 `~/.nullus-secrets` 파일이 유일한 사본이 되기 쉽다 — 설치가 끝나면 위 절차로 금고에 옮기고 파일은 지운다.

---

## 4. 회전

| 키 | 회전 방법 | 옛 백업본은 |
|---|---|---|
| **봉인 키** | ① 새 32바이트(`openssl rand -hex 16`)와 **새 키 ID**(`platform-key-v2`) 를 정한다 ② 차트 `secrets.backupSealKey`·`config.backup.sealKeyId` 를 바꿔 적용(API 재기동) ③ **즉시 전체 백업**(화면 "전체 — 플랫폼 + 모든 스택" 또는 `POST /admin/backups/sets`)을 뜬다 ④ 옛 키를 §2 서식에 옛 키 ID 와 함께 **보존 기간이 끝날 때까지** 남긴다 | 옛 키로만 열린다. 코드는 **설정된 키 하나**만 쓰고 키 링이 없다 — 옛 백업으로 복구해야 하면 `backup.seal_key`/`sealKeyId` 를 그 백업의 `manifest.encryption.key_id` 키로 **잠시 되돌려** 복구하고, 끝나면 새 키로 돌아온다. 어느 키인지는 `GET /admin/backups/:id` 의 `manifest.encryption.key_id` 가 말한다 |
| **`ENCRYPTION_KEY`** | 회전 도구가 없다. 바꾸는 순간 **기존 `clusters.kubeconfig` 가 모두 열리지 않는다.** 그래서 ① 회전 전 전체 백업 1회(옛 키로 — 그 백업을 복구하려면 옛 키가 필요하므로 옛 키도 §2 에 남긴다) ② 새 키 적용·재기동 ③ **모든 클러스터의 kubeconfig 재등록**(`PATCH /admin/clusters/:id` 에 kubeconfig 재제출, 화면 관리 → 클러스터) ④ 재등록 뒤 전체 백업 | 그 백업을 복구하면 kubeconfig 행도 옛 키 암호문으로 돌아온다 — **복구 뒤 다시 재등록**해야 한다. 유출 정황이 없으면 회전하지 않는 것을 권한다 |
| **unseal key** | 지원하지 않는다. 사이드카가 `openbao-unseal-keys` 의 `key1` 을 그대로 제출하므로 `bao operator rekey` 를 하려면 Secret 도 손으로 함께 갱신해야 한다 | 유출 정황이면 스택을 지우고 다시 깐다 — 금고의 자격증명은 설치가 다시 발급하고, 토큰 소스는 재등록한다. 옛 백업본의 금고는 옛 Secret 과 함께 담겨 있어 그대로 열린다 |
| **목적지 자격증명** | 스토리지에서 새 키 발급 → 차트 `config.backup.destination.accessKey`·`secrets.backupDestinationSecretKey` → 재기동 → 최근 백업에 `POST /admin/backups/:id/verify` 가 통과하면 옛 키 폐기 | 영향 없다 — 자격증명은 백업본을 잠그지 않는다 |

회전 뒤에는 §2 서식의 지문·생성일·마지막 검증을 갱신한다.

---

## 5. 잃었을 때 — 무엇까지 되살릴 수 있나

전제: 백업본은 목적지에 있다.

| 잃은 것 | 되살릴 수 있는 것 | 못 하는 것 | 할 일 |
|---|---|---|---|
| **봉인 키** | 없다. 매니페스트(무엇이 들어 있나)만 읽힌다 | 모든 백업본 | 지금 운영 중인 것이 유일한 데이터다. **즉시** 새 키(새 키 ID)로 전체 백업을 뜨고 옛 백업은 폐기한다. 이 문서 §2 를 다시 한다 |
| **`ENCRYPTION_KEY`** | 플랫폼 DB·Keycloak·금고·스택 데이터 전부(복구는 **어떤** 32바이트 키라도 설정돼 있으면 시작한다 — §6.1 0단계는 존재만 본다) | 등록된 클러스터의 kubeconfig 복호화 → 스택을 조작·설치·삭제할 수 없다 | 새 키를 설정하고 복구한 뒤 **모든 클러스터를 재등록**한다(§4). 스택 기록은 `cluster_id` 로 그대로 묶여 있다 |
| **unseal key** (클러스터도 함께 소실) | 스택 백업에 `openbao-unseal-keys` Secret 이 들어 있다 — `stack_only`/세트 복구가 Secret 과 PVC 를 함께 되살리고 사이드카가 unseal 한 뒤 KV 를 import 한다 | 백업본도 없으면 금고 영구 봉인 | 복구한다. 백업이 없으면 스택을 다시 깔고 토큰 소스를 재등록한다 |
| **목적지 자격증명** | 전부 — 자격증명은 데이터를 잠그지 않는다 | 재발급 전까지 가져올 수 없다 | 스토리지 관리자가 재발급 → 차트 반영(§4) |
| **`ENCRYPTION_KEY` + 봉인 키 함께** | 없다 | 전부 | 그래서 둘을 **다른 보관함**에 둔다(§2) |
| **컨트롤 플레인 클러스터 통째** (키는 보관함에 있음) | 전부 — 새 클러스터에 차트를 같은 `secrets.*` 값으로 깔고 세트 복구 | — | 보관함의 값으로 차트를 깔고 [사용 가이드 §4](./Nullus_백업복구_사용_가이드.md) 의 세트 복구 |

---

## 6. 키 자재는 아니지만 복구에 함께 필요한 것

이것들은 잃어도 **데이터가 잠기지는 않지만**, 없으면 복구 절차가 서지 않는다. 같은 금고에 둔다.

| 것 | 어디 | 비고 |
|---|---|---|
| 플랫폼 DB 비밀번호 | 차트 `secrets.dbPassword`(Secret `db-password`) | `pg_restore` 와 API 가 쓴다 |
| Keycloak DB 비밀번호 · Keycloak admin 비밀번호 | `config.backup.keycloakDatabase.password` · `config.keycloak.adminPassword` | Keycloak DB 복원과 realm 설정 |
| 컨트롤 플레인 클러스터의 kubeconfig / 접속 수단 | 운영자 보관 | 차트를 다시 깔 곳 |
| 차트 버전과 values 파일 | 저장소 태그 + 환경별 values(`deploy/csp/*/values-*.yaml`) | 매니페스트 `helm_releases` 와 같은 버전으로 되돌려야 한다 |
| 백업 목적지 주소·버킷·prefix | `config.backup.destination.*` | 자격증명과 함께 |

---

## 7. 분기 점검 — 복구 리허설 체크리스트에 더하는 항목

[로컬 테스트 가이드](./Nullus_백업복구_로컬_테스트_가이드.md)의 리허설을 분기마다 돌릴 때, 키에 대해 다음을 함께 확인한다. **보관본이 지금 값과 같은지**를 값 없이 확인하는 것이 핵심이다.

- [ ] §3 명령으로 지금 돌고 있는 세 키의 지문을 뜨고, §2 서식의 지문과 **같은지** 본다. 다르면 누가 언제 바꿨는지 찾는다 — 기록 없는 회전은 사고다
- [ ] 봉인 키: 가장 최근 백업에 `POST /admin/backups/:id/verify` 가 통과하는가. 매니페스트 `encryption.key_id` 가 설정의 `sealKeyId` 와 같은가
- [ ] `ENCRYPTION_KEY`: 등록된 클러스터마다 `POST /admin/clusters/:id/verify` 가 `connected` 인가 — kubeconfig 가 열린다는 뜻이다
- [ ] unseal key: 스택마다 `openbao-unseal-keys` Secret 이 있고(`kubectl -n <ns> get secret openbao-unseal-keys`) 금고가 `Sealed: false` 인가(`kubectl -n <ns> exec openbao-0 -c openbao -- bao status`). 보관함의 지문과 같은가
- [ ] 목적지 자격증명: 복구 응답의 `prerequisites`/`verify` 가 목적지에 닿는가
- [ ] 오프라인 사본(봉투·USB)이 제자리에 있고 읽히는가 — 1년에 한 번은 실제로 열어 지문을 대조한다
- [ ] §2 서식의 "마지막 검증" 을 오늘로 갱신한다

---

## 참고

- 설계 §1.3(왜 `ENCRYPTION_KEY` 가 백업보다 먼저인가) · §5(시크릿 취급 정책) · §5.3(unseal key 가 금고와 같은 곳에 있다) · §12.1-1(이 런북)
- [백업/복구 사용 가이드](./Nullus_백업복구_사용_가이드.md) · [로컬 테스트 가이드](./Nullus_백업복구_로컬_테스트_가이드.md)
- 차트: `deploy/helm/nullus/templates/secret.yaml`(네 키의 Secret 키 이름) · `values.yaml` 의 `secrets` 블록
- #68(BYOK): 봉인 키를 사용자 제공 키로 감싸는 결정은 그 카드가 가진다 — 바뀌면 §4 봉인 키 행이 바뀐다
