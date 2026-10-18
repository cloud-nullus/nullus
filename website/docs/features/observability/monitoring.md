# 모니터링 대시보드

<span class="route">/observability/monitoring</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>
<span class="role role-developer">developer</span>

클러스터·스택·앱의 상태를 한 화면에서 봅니다. 먼저 **클러스터나 스택을 고르면**
그에 맞는 탭이 열립니다.

## 쓰는 법

1. 화면 맨 위 **클러스터** 또는 **스택** 드롭다운에서 볼 대상을 고릅니다. 아무것도
   고르지 않으면 "Select a Cluster or Stack above to begin" 만 보입니다.
2. 아래 탭에서 보고 싶은 관점을 고릅니다.
3. 지표는 **5초마다** 새로 읽습니다. 범위 필(`1h`·`6h`·`24h`·`7d`)로 차트 구간을
   바꿀 수 있습니다.

## 세 개의 탭

| 탭 | 열리는 조건 | 보여 주는 것 |
|----|------------|-------------|
| **Cluster** | 클러스터를 골랐을 때 | 클러스터 전체의 파드·CPU·메모리, 파이프라인 성공/실패 |
| **Stack** | 스택을 골랐을 때 | 그 스택 도구별 자원 사용량과 도구 건강 상태 |
| **CI/CD** | **고른 클러스터에 파이프라인이 실제로 있을 때만** | 파이프라인·배포 집계, 앱 파드의 실시간 자원과 로그 |

### Cluster 탭

- KPI 카드 — `Running Pods`, `Pods`, `CPU`, `Memory`
- 차트 — `CPU Usage`(영역), `Memory Usage`(영역), `Pod Status`(원형), `Pipeline Success (this week)`(막대)
- `Pod Status` 원형을 누르면 **파드 목록 모달**이 열립니다. 컬럼은 Namespace · Pod · Status · Ready · Node · Tool 입니다.

### Stack 탭

- **Resource Trend** 카드 — 스코프 필(`All` + 설치된 OSS 별), 범위 필(`Live 5s`·`1h`·`6h`·`24h`·`7d`)
- KPI 카드 — `Current CPU`, `Current Memory`, `Current Storage`, `Ready Pods`
- 차트 — `CPU (Request / Limit / Current)`, `Memory (Request / Limit / Current)`, `OSS Pod Coverage`, `Pod Status`(도넛)
- **Tool Health** 카드 — 설치 리소스 표(Kind · Resource · Ready/Desired · Status)와 도구별 파드 상세 표(Pod · Status · Ready · CPU Req/Limit · Mem Req/Limit · Restarts)
- 관리자에게는 첫 방문 시 **스택 컴포넌트 연결** 패널이 떠서, 설치된 도구 URL 을 임베드 탭으로 추가할 수 있습니다.

### CI/CD 탭

- KPI 카드 — `Total Pipelines`, `Pipeline Success / Failed`, `Total Deployments`, `Running Deployments`
- `App CPU (Live)` · `App Memory (Live)` 차트와 앱 로그 패널
- `Deployment Timeline`(성공/실패 막대)

## 임베드 탭 (Manage Tabs)

관리자는 **Manage Tabs** 로 iframe 탭을 직접 추가·정렬·삭제할 수 있습니다. Grafana
대시보드처럼 외부 화면을 이 자리에 붙이는 용도입니다. 관리자가 아니면 `View only`
배지가 붙습니다.

!!! warning "임베드 탭 설정은 브라우저에만 저장됩니다"
    화면에는 "Changes apply to all users after saving" 이라고 적혀 있지만, 실제
    저장 위치는 **그 브라우저의 localStorage** 입니다. 다른 사람이나 다른 기기에는
    전달되지 않고, 브라우저 데이터를 지우면 함께 사라집니다. 문구가 구현과
    어긋나 있는 상태입니다.

!!! warning "CI/CD 탭의 기본 임베드는 공개 데모 주소입니다"
    CI/CD 탭은 기본 탭으로 `play.grafana.org` 의 공개 데모 대시보드를 자동으로
    넣습니다. 조직 데이터가 아니므로, 쓰시려면 Manage Tabs 에서 자기 Grafana
    주소로 바꾸세요.

## 지표는 어디서 오나 — Prometheus 가 아닙니다

이 화면의 숫자는 Prometheus 질의 결과가 아닙니다. 서버가 **클러스터 kubeconfig 로
Kubernetes 에 직접 붙어** 모읍니다.

| 값 | 출처 |
|-----|------|
| 파드·워크로드 목록과 상태 | Kubernetes API (client-go) |
| CPU·메모리 사용량 | `kubectl top pod` (metrics-server) |
| 스토리지 사용량 | kubelet `nodes/{node}/proxy/stats/summary` |

!!! danger "차트의 시계열은 서버에 저장되지 않습니다"
    `CPU Usage`·`Memory Usage` 같은 추이 차트는 **브라우저가 5초마다 모은 표본**을
    그린 것입니다. 새로고침하면 비어 있고, 어제 값을 되돌려 볼 수 없습니다.
    범위 필에 `7d` 가 있지만 7일치 보존 데이터가 있다는 뜻은 아닙니다. 장기
    추이가 필요하면 스택에 설치된 Grafana 를 직접 쓰세요.

## 호출하는 API

| 용도 | 엔드포인트 |
|------|-----------|
| 클러스터 목록(필터) | `GET /api/v1/admin/clusters?page=1&page_size=500` |
| 스택 목록(필터) | `GET /api/v1/stacks` |
| 스택 스냅샷 (5초 폴링) | `GET /api/v1/stacks/{stackId}/monitoring` |
| 클러스터 요약 (폴백) | `GET /api/v1/admin/clusters/{id}/monitoring-summary` |
| 파이프라인·배포 | `GET /api/v1/cicd/pipelines`, `GET /api/v1/cicd/deployments` |
| 앱 파드 사용량·로그 | `GET /api/v1/stacks/{stackId}/workloads`, `GET /api/v1/stacks/{stackId}/workloads/logs?tailLines=` |

!!! warning "developer 는 지표를 못 받을 수 있습니다"
    이 화면은 developer 에게도 열려 있지만, 화면이 부르는
    `/api/v1/admin/clusters` 는 **admin 전용**이고 `/api/v1/stacks*` 는
    **admin·devops 전용**입니다. 인증을 켠 환경에서 developer 가 열면 필터와 지표가
    403 으로 비어 보입니다. 인증을 끈 개발 모드에서는 동작합니다. 화면 권한과 API
    권한이 어긋나 있는 상태입니다.
