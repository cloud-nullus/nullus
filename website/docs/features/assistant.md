# AI 어시스턴트

<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>
<span class="role role-developer">developer</span>

모든 화면의 오른쪽 아래(단축키 배지 위) **AI 어시스턴트** 버튼으로 엽니다. 스택 · 클러스터 · 설치 상태를
자연어로 물으면, 서버가 [MCP](../reference/mcp.md) 의 조회 도구로 확인해서 답합니다. 모델은 기본으로
Upstage Solar 입니다.

!!! note "서버에 키가 없으면 버튼이 없습니다"
    `NULLUS_ASSISTANT_API_KEY`(차트: `secrets.assistantApiKey`)가 있어야 켜집니다. 키가 없으면
    `GET /api/v1/assistant/status` 가 `{"enabled": false}` 를 돌려주고 화면은 버튼을 그리지 않습니다.

## 물어볼 수 있는 것

대화가 비어 있으면 추천 질문 세 개가 나옵니다 — **실패한 스택이 있어?** · **마지막 스택 설치는 왜
실패했어?** · **어떤 스택 템플릿이 있어?**

모델은 아래 도구를 골라 부르고, 그 결과를 근거로 답합니다. 답 아래에 **사용한 도구** 줄이 붙고, 실패한
도구에는 "(실패)" 가 붙습니다.

| 도구 | 쓰는 곳 |
|------|---------|
| `stack_list` | 스택 목록과 상태 |
| `stack_status` | 배포 진행률 · 실패한 단계와 사유 |
| `stack_logs_tail` | 설치 로그의 마지막 줄들 — 설치 실패 원인 |
| `cluster_list` | 등록된 클러스터(kubeconfig 같은 비밀은 빼고) |
| `template_list` | 스택 템플릿 카탈로그 |
| `compat_check` | 도구 조합의 호환성 |

## 질문한 사람의 권한으로만 봅니다

도구는 질문한 사용자의 인증을 그대로 실어 API 를 부릅니다. 그래서 화면에서 못 보는 것은 어시스턴트도
못 봅니다 — 예를 들어 developer 는 스택 · 클러스터 API 권한이 없어, 어시스턴트가 "볼 권한이 없다" 고
답합니다([권한 표](../reference/rbac.md)).

## 조회만 합니다

배포 · 롤백 같은 변경 도구는 어시스턴트의 도구 목록에 아예 없습니다. 변경이 필요하면 화면의 어디서 하면
되는지 안내합니다.

## 외부로 나가는 데이터

!!! warning "도구 결과가 외부 모델로 전송됩니다"
    켜 두면 위 도구의 결과(스택 설정 · 상태 · 설치 로그 일부)가 설정한 모델(기본 Upstage)로 나갑니다.
    패널 아래에도 그렇게 적혀 있습니다. 원하지 않으면 키를 넣지 않거나, 사내 OpenAI 호환 서버를
    `NULLUS_ASSISTANT_BASE_URL` 로 지정합니다.

- 보내기 전에 문자열 안의 `Bearer` 토큰 · JWT · GitLab 토큰(`glpat-`) · AWS 액세스 키 · 개인 키 블록 ·
  `password=` 류 값을 가립니다
- 대화는 서버에 저장하지 않습니다. 화면이 들고 있다가 질문마다 통째로 보내므로, 새로 고치면 사라집니다

## 켜기

| 환경 변수 | 차트 값 | 기본값 |
|-----------|---------|--------|
| `NULLUS_ASSISTANT_API_KEY` | `secrets.assistantApiKey` | 없음 — 꺼짐 |
| `NULLUS_ASSISTANT_BASE_URL` | `config.assistant.baseURL` | `https://api.upstage.ai/v1` |
| `NULLUS_ASSISTANT_MODEL` | `config.assistant.model` | `solar-pro4` |

## 한계

- 대화는 20턴, 한 턴은 4,000자까지입니다. 질문 하나에 도구는 6라운드까지 부릅니다
- 도구 결과 하나가 12,000자를 넘으면 앞부분만 모델에 넘깁니다
- 답은 다 만들어진 뒤 한 번에 나옵니다(스트리밍 없음)
- 도구 호출은 API 자신을 루프백(127.0.0.1)으로 부르므로, 모든 사용자의 도구 호출이 IP 상한(기본 분당 600회)을
  함께 씁니다

## 호출하는 API

`GET /api/v1/assistant/status` · `POST /api/v1/assistant/chat`
