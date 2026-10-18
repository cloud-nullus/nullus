# CI/CD 이력

<span class="route">/cicd/history</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>
<span class="role role-developer">developer</span>

배포 기록을 시간순으로 보는 화면입니다.

![파이프라인 배포 이력](../../assets/screens/cicd-history-light.png#only-light)
![파이프라인 배포 이력](../../assets/screens/cicd-history-dark.png#only-dark)

## 표 컬럼

| 컬럼 | 내용 |
|------|------|
| (펼치기) | 행 선택 |
| Pipeline | 파이프라인 이름 |
| Version | 배포된 버전 |
| Status | 상태 배지 |
| Deployer | 실행한 사람 |
| Started At | 시작 시각 |
| Completed At | 완료 시각 |

툴바에 상태 선택과 검색(파이프라인 이름·실행자)이 있습니다.

주소에 `?pipeline=<id>` 가 붙어 들어오면 "이 파이프라인으로 필터됨" 칩이 뜨고,
**필터 해제** 를 누르면 주소를 고쳐 쓰고 화면을 새로 읽습니다.

오른쪽 상세 패널은 같은 여섯 항목(Pipeline · Version · **Triggered By** · Status ·
Started At · Completed At)을 반복합니다 — 단계별 데이터가 서버에는 있지만 이
화면에는 일부러 연결하지 않았습니다.

!!! warning "`cancelled` 는 절대 걸리지 않습니다"
    상태 선택에 `cancelled` 가 있는데, 배포 상태 값은 `pending` · `running` ·
    `success` · `failed` · `rolled_back` 다섯뿐입니다. 상태 필터는 브라우저에서
    돕니다.

## 호출하는 API

```
GET /api/v1/cicd/deployments          # 파이프라인 이름 해석용으로 /pipelines 도 함께
```

응답 항목은 `id`, `pipelineId`, `pipelineName`, `version`, `status`,
`triggeredBy`, `startedAt`, `completedAt` 입니다.

서버 쪽 필터는 `pipeline_id` 만 받습니다. 그리고 `pipeline_id` 가 주어지면
**CI 서버에서 실행 기록을 먼저 동기화한 뒤** 응답합니다.

!!! note "롤백 기능은 화면에서 빠졌습니다"
    롤백 API 라우트가 서버에 등록돼 있지 않고, 이 화면에서도 버튼이 제거됐습니다.
