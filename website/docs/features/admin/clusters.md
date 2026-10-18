# 클러스터 관리

<span class="route">/admin/clusters</span>
<span class="role role-admin">admin</span>

스택을 설치할 Kubernetes 클러스터를 등록하고 연결을 확인하는 화면입니다.
Nullus 를 쓰기 시작할 때 **가장 먼저** 하는 일입니다.

![등록된 클러스터와 연결 상태](../../assets/screens/admin-clusters-light.png#only-light)
![등록된 클러스터와 연결 상태](../../assets/screens/admin-clusters-dark.png#only-dark)

## 클러스터 등록하기

1. **Register** 창을 엽니다.
2. **Name** — 2자 이상.
3. **Cluster Type** 체크박스 — `DevSecOps Stack Cluster`(pipeline) ·
   `Target Cluster`(target). 하나 이상 골라야 합니다.
4. **Cloud Provider** 선택 — 11개 중 하나. 기본값은 온프레미스.
5. **Endpoint URL** — 선택.
6. **kubeconfig** — 붙여넣거나 파일을 올립니다.
7. **Verify Connection** 을 눌러 `connected` 가 나와야 **Register 버튼이
   열립니다.**
8. Register 를 누르면 저장 후 자동으로 한 번 더 검증합니다.

### kubeconfig 를 넣는 두 가지 방법

| 방법 | 동작 |
|------|------|
| 붙여넣기 | `Paste kubeconfig content…` 텍스트 영역 |
| 파일 업로드 | `.yaml`·`.yml`·`.conf`, 1 MiB 이하. **파일을 보내는 게 아니라** 브라우저가 읽어서 위 텍스트 영역을 채웁니다 |

수정할 때는 kubeconfig 를 비워 두면 기존 값을 유지합니다.

!!! note "kubeconfig 는 파싱하지 않습니다"
    서버는 context 를 뽑아내거나 내용을 해석하지 않습니다. base64 인지 먼저
    보고, 아니면 YAML 원문으로 받아 **문서 전체를 AES-256-GCM 으로 암호화해**
    저장합니다. 암호화 키는 32바이트 `ENCRYPTION_KEY` 입니다.

!!! warning "클러스터 이름 중복을 막지 않습니다"
    코드에도 검사가 없고 데이터베이스에도 유니크 제약이 없습니다. 같은 이름을
    여러 개 등록할 수 있습니다.

## 연결 검증이 보는 것

검증은 `kubectl version` 을 돌리는 게 아니라, 클라이언트 라이브러리로
**서버 버전 조회 + 노드 목록 조회**를 10초 안에 해 보는 것입니다.

| 버튼 | 엔드포인트 | 동작 |
|------|-----------|------|
| 창 안의 Verify Connection | `POST /api/v1/admin/clusters/verify` | kubeconfig 만 받아 확인하고 **아무것도 저장하지 않습니다**. 실패하면 502. `endpoint` 를 보내도 무시합니다 |
| 등록된 클러스터 검증 | `POST /api/v1/admin/clusters/{id}/verify` | 저장된 kubeconfig 를 복호화해 확인. 실패하면 재판독 후 **한 번 더 시도**합니다. 성공하면 상태와 노드 아키텍처를 저장합니다 |

성공 응답은 `status`, `version`, `node_architectures`, `node_count` 입니다.

## 노드 재판독 (refresh-discovery)

`POST /api/v1/admin/clusters/{id}/refresh-discovery` 가 kubeconfig 를 복호화해
다시 조회하고 연결 상태와 노드 아키텍처를 저장합니다.

등록할 때, kubeconfig 를 바꿔 수정할 때도 자동으로 한 번 돕니다.

!!! note "이 화면에는 재판독 버튼이 없습니다"
    노드 아키텍처를 보여 주지도 않습니다. 재판독 버튼은
    [스택 버전 관리](../stack/versions.md#스택-버전-관리) 화면에 있습니다.

## 연결 상태 값

| 값 | 언제 |
|----|------|
| `pending` | 등록 직후 |
| `connected` | 검증·재판독 성공 |

!!! danger "연결 실패가 기록되지 않습니다"
    코드에는 `unreachable` · `auth_failed` · `connection_failed` 도 정의돼
    있지만, 실제로 쓰이는 것은 `connection_failed` 하나이고 **그 값이 데이터베이스
    타입에 들어 있지 않습니다.** 그래서 실패를 저장하려는 쓰기가 DB 에서
    거부되고, 행은 **이전 상태를 그대로 유지**합니다. API 는 200 을 돌려줍니다.

    즉 연결이 끊긴 클러스터가 화면에서 계속 `connected`(또는 `pending`) 로 보일
    수 있습니다. 상태를 믿기보다 **검증 버튼을 직접 눌러** 확인하세요.

## 상세 패널

**Available Resources** 카드는 5초마다
`GET /api/v1/admin/clusters/{id}/monitoring-summary` 를 다시 읽습니다.

!!! warning "Namespace 와 Auth Method 행은 저장된 값이 아닙니다"
    클러스터 종류에 따라 화면에 박아 둔 값을 보여 줍니다 — pipeline 이면
    `nullus-system` / "Kubeconfig (ServiceAccount)", target 이면 `default` /
    "Kubeconfig". 실제로 저장되는 항목은 이름·종류·클라우드 제공자·엔드포인트·
    연결 상태·조직·노드 아키텍처입니다.

## 클러스터 안에서 스스로 등록하기

`POST /api/v1/admin/clusters/self-register` 는 **kubeconfig 를 받지 않습니다.**
파드에 마운트된 ServiceAccount 로 kubeconfig 를 만들어 자기 클러스터를
등록합니다. 클러스터 안에서 돌고 있지 않으면 412 를 줍니다.

이름 기본값은 `nullus-self`, 조직 ID 는 필수입니다. 같은 이름이 이미 있으면
그것을 돌려줍니다(멱등).

화면에서 부르는 곳은 없고, 에어갭 설치 스크립트가 씁니다.

## 호출하는 API

`GET /api/v1/admin/clusters` · `GET …/{id}` (수정 창을 채우려고 **복호화된
kubeconfig 를 응답에 담습니다**) · `PATCH …/{id}` · `DELETE …/{id}` ·
`POST …/verify` · `POST …/{id}/verify` · `POST …/{id}/refresh-discovery` ·
`GET …/{id}/monitoring-summary`

등록돼 있지만 이 화면이 쓰지 않는 것: `GET …/{id}/namespaces` ·
`GET …/{id}/storage-classes` · `GET …/{id}/pods` — 스택 설치 위자드가 씁니다.
