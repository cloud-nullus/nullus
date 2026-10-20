# AI 어시스턴트

<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>
<span class="role role-developer">developer</span>

모든 화면의 오른쪽 아래(단축키 배지 위) **AI 어시스턴트** 버튼으로 엽니다. 스택 · 클러스터 · 설치 상태를
자연어로 물으면, 서버가 [MCP](../reference/mcp.md) 의 조회 도구로 확인해서 답합니다. 답하는 모델은
조직의 관리자가 고릅니다 — Upstage Solar · OpenAI · Anthropic Claude · Google Gemini · OpenAI 호환 사내 서버.

!!! note "관리자가 켜기 전에는 버튼이 없습니다"
    **관리 › AI 어시스턴트** 에서 공급자와 API 키를 등록하고 켜야 보입니다(아래 [켜기](#켜기)). 조직마다
    따로이고, 꺼져 있으면 `GET /api/v1/assistant/status` 가 `{"enabled": false}` 를 돌려주고 화면은 버튼을
    그리지 않습니다.

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
    켜 두면 위 도구의 결과(스택 설정 · 상태 · 설치 로그 일부)가 고른 공급자로 나갑니다. 패널 아래에도
    그렇게 적혀 있습니다. 원하지 않으면 켜지 않거나, 공급자로 **OpenAI 호환(사내 서버)** 를 골라 사내에서
    도는 모델(vLLM · Ollama 등)을 씁니다.

- 보내기 전에 문자열 안의 `Bearer` 토큰 · JWT · GitLab 토큰(`glpat-`) · AWS 액세스 키 · 개인 키 블록 ·
  `password=` 류 값을 가립니다
- 대화는 서버에 저장하지 않습니다. 화면이 들고 있다가 질문마다 통째로 보내므로, 패널을 닫았다 열면 이어지고
  새로 고치면 사라집니다

## 켜기

<span class="route">/admin/assistant</span>
<span class="role role-admin">admin</span>

**관리 › AI 어시스턴트** 에서 조직의 공급자 · 모델 · API 키를 등록합니다.

| 공급자 | 기본 모델 | 주소 |
|--------|-----------|------|
| Upstage Solar | `solar-pro4` | `https://api.upstage.ai/v1` |
| OpenAI | 직접 입력 | `https://api.openai.com/v1` |
| Anthropic Claude | `claude-sonnet-5-5` | `https://api.anthropic.com` |
| Google Gemini | 직접 입력 | `https://generativelanguage.googleapis.com/v1beta/openai` |
| OpenAI 호환(사내 서버) | 직접 입력 | 직접 입력 — 키는 없어도 됩니다 |

1. 공급자를 고르고, 필요하면 모델 이름과 주소를 넣습니다(비우면 위 기본값)
2. **API 키** 를 넣습니다. 저장한 뒤에는 다시 보이지 않고 끝 네 자리만 보입니다 — 칸을 비워 두고 저장하면
   저장된 키를 그대로 씁니다. 공급자를 바꾸면 새 공급자의 키를 넣어야 합니다
3. **연결 테스트** 로 그 설정으로 모델에 한 번 물어 봅니다(저장하지 않습니다)
4. **이 조직에서 어시스턴트 켜기** 를 체크하고 **저장** 합니다

키는 서버의 `ENCRYPTION_KEY` 로 암호화해 둡니다 — 그 값이 없으면 키를 저장하지 않습니다. 저장 · 삭제는
감사 로그에 남고(키 원문은 남기지 않음), **설정 삭제** 는 키까지 지우고 어시스턴트를 끕니다.

## 한계

- 대화는 20턴, 한 턴은 4,000자까지입니다. 질문 하나에 도구는 6라운드 · 라운드당 8회 · 모두 20회까지 부릅니다
- 질문 하나에 55초까지 씁니다. 넘으면 "질문을 좁혀 다시 물어보세요" 라고 알립니다
- 도구 결과 하나가 12,000자를 넘으면 앞과 끝만 모델에 넘깁니다(설치 로그는 끝에 실패 원인이 있습니다)
- 답은 다 만들어진 뒤 한 번에 나옵니다(스트리밍 없음)
- 도구 호출은 API 자신을 루프백(127.0.0.1)으로 부르므로, 모든 사용자의 도구 호출이 IP 상한(기본 분당 600회)을
  함께 쓰고, 질문한 사람의 사용자 레이트리밋에도 잡힙니다

## 호출하는 API

`GET /api/v1/assistant/status` · `POST /api/v1/assistant/chat` · 관리:
`GET · PUT · DELETE /api/v1/admin/assistant` · `POST /api/v1/admin/assistant/test`
