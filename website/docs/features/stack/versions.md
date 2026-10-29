# 스택 버전

<span class="route">/stack/version</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>

어떤 도구 버전 조합이 검증됐는지 보는 호환성 매트릭스입니다.
`/stack/versions` 도 같은 화면입니다.

![도구 버전 호환성 매트릭스](../../assets/screens/stack-versions-light.png#only-light)
![도구 버전 호환성 매트릭스](../../assets/screens/stack-versions-dark.png#only-dark)

## 표

컬럼이 **GitLab · Argo CD · Prometheus · Grafana · OpenTelemetry · K8s · 상태**
7개로 고정돼 있습니다. 매트릭스 ID 에 `gitlab-argocd` 가 들어가면 `권장` 배지가
붙습니다. `스택 또는 도구 검색...` 으로 좁힐 수 있습니다.

!!! warning "GitLab 이 들어간 매트릭스만 보입니다"
    표가 매트릭스 이름에 `gitlab` 이 들어간 것만 걸러서 그립니다. GitHub·Gitea
    기반 매트릭스가 등록돼 있어도 이 화면에는 **나타나지 않습니다.**
    전체 목록은 [스택 버전 관리](#스택-버전-관리) 화면에서 봅니다.

## 현재 스택 검증

!!! warning "이 버튼은 지금 동작하지 않습니다"
    버튼이 스택 ID 자리에 문자열 `current` 를 넣어 호출하고, 서버는 그걸 저장된
    스택을 보는 모드로 해석합니다. 그런데 검증 유스케이스가 **스택 저장소 없이**
    조립돼 있어 `"persisted validate mode requires a stack repository"` 로 400 이
    돌아옵니다.

    같은 이유로 `POST /api/v1/stacks/{stackId}/validate` 를 저장된 스택 모드로
    부르는 **모든 호출**이 실패합니다. 설치 위자드가 쓰는 클라이언트 쪽 검사는
    영향을 받지 않습니다.

## 스택 버전 관리

<span class="route">/admin/stack-versions</span>
<span class="role role-admin">admin</span>

매트릭스를 등록·수정하는 관리자 화면입니다. 사이드바에서는 "데브섹옵스 스택"
그룹 안에 있지만 주소는 `/admin/` 입니다.

왼쪽에 매트릭스 목록(이름, `verified`·`untested`·`unsupported` 배지, ID,
`K8s <범위> · 도구 N개`)이 있고, 오른쪽 상세에는 Kubernetes 지원 범위, 도구
표(이름·Helm·App·아키텍처·최소 K8s·등급), 클러스터 호환성 표(클러스터·노드
아키텍처·호환성)가 나옵니다. 클러스터마다 **재판독** 버튼으로 노드 아키텍처를
다시 읽어 옵니다(`POST /api/v1/admin/clusters/{id}/refresh-discovery`).

검색·상태 필터 바는 **매트릭스가 5개를 넘을 때만** 나타납니다.

!!! warning "새 매트릭스·편집·삭제는 404 입니다"
    세 버튼이 `/api/v1/admin/compatibility/matrices` 를 부르는데, 그 라우트를
    등록하는 함수가 서버에서 **한 번도 호출되지 않습니다.** 등록했더라도 관리
    기능 옵션이 빠져 있어 아무 일도 하지 않습니다.

    지금 매트릭스를 추가하는 길은 **마이그레이션 시드**뿐입니다.

## 업그레이드 번들

<span class="route">/admin/stack-versions?tab=upgrade-bundles</span>
<span class="role role-admin">admin</span>

같은 화면의 두 번째 탭 **업그레이드 번들** 입니다. 스택 목록의 **버전 업그레이드** 탭이
후보로 보여 주는 **검증된 업그레이드 경로**(출발 차트 버전 → 대상 차트 버전)를 등록하고
관리합니다. 번들은 플랫폼 전체의 것이라 조직에 묶이지 않습니다. 지금 업그레이드할 수
있는 도구는 Argo CD 하나입니다.

표에는 도구 · 경로(출발 → 대상, 앱 버전) · 위험도 · 백업 · 고치는 CVE · 상태 · 등록일이
나오고, 상태마다 갈 수 있는 작업 버튼이 붙습니다.

| 상태 | 뜻 | 갈 수 있는 곳 |
|------|----|-------------|
| **초안** | 후보에 보이지 않습니다 | **검증됨으로**(리허설 확인 필요) · **회수** |
| **검증됨** | 출발 버전을 쓰는 모든 스택에 후보로 보입니다 | **폐기** · **회수** |
| **폐기 예정** | 후보에서 빠졌습니다 | **검증됨으로**(리허설 확인 필요) · **회수** |
| **회수됨** | 거둔 경로입니다 | — (되돌릴 수 없습니다) |

**새 번들** 로 등록합니다. 도구를 고르면 Helm 릴리스 이름 · 차트 이름 · 차트 저장소가
채워지고, 출발 · 대상 차트 버전과 앱 버전, 위험도, 예상 중단, 릴리스 노트, 고치는 CVE,
지원 아키텍처, **실행 전 24시간 이내 백업을 요구** 를 적습니다. 등록 상태를 **검증됨** 으로
두면 **이 경로를 실클러스터에서 리허설했다** 에 표시해야 등록됩니다 — 표시하지 않으면
서버가 거부합니다(`UPGRADE_BUNDLE_REHEARSAL_REQUIRED`). 리허설 없이 실행 가능한 경로가
생기지 않게 하는 장치입니다.

!!! note "서버가 검사하는 것"
    지원 도구인지, 출발 · 대상 차트 버전을 읽을 수 있고 대상이 더 높은지, 차트 저장소가
    `https` · `http` · `oci` 인지, CVE 가 `CVE-YYYY-NNNN` 꼴인지 봅니다. 같은 ID 나 같은
    경로(도구 · 출발 · 대상)가 이미 있으면 409 입니다. 대상 앱 버전은 업그레이드 뒤 상태
    검사가 서버 이미지 태그와 비교하는 값이라, 틀리면 업그레이드가 되돌려집니다.

등록과 상태 변경은 감사 로그(`register_upgrade_bundle` · `change_upgrade_bundle_status`)에
남습니다. 리허설 절차와 번들의 뜻은 설계 문서 `Nullus_OSS_업그레이드_설계.md` §12 에 있습니다.

## 호출하는 API

| 동작 | 엔드포인트 |
|------|-----------|
| 매트릭스 목록 | `GET /api/v1/stacks/compatibility` |
| 스택 검증 | `POST /api/v1/stacks/{stackId}/validate` |
| 노드 재판독 | `POST /api/v1/admin/clusters/{id}/refresh-discovery` |
| 번들 목록 · 등록 | `GET` · `POST /api/v1/admin/upgrade-bundles` |
| 번들 상태 변경 | `PATCH /api/v1/admin/upgrade-bundles/{id}/status` |
