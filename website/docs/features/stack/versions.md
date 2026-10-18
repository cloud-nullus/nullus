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

## 호출하는 API

| 동작 | 엔드포인트 |
|------|-----------|
| 매트릭스 목록 | `GET /api/v1/stacks/compatibility` |
| 스택 검증 | `POST /api/v1/stacks/{stackId}/validate` |
| 노드 재판독 | `POST /api/v1/admin/clusters/{id}/refresh-discovery` |
