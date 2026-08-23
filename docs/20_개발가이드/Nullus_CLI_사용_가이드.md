# Nullus CLI 사용 가이드

- **작성일**: 2026-08-23
- **상태**: v1 설계 확정 기준 선행 작성 — **구현 진행 중** ([EPIC nullus-plan#59](https://github.com/cloud-nullus/nullus-plan/issues/59)). 릴리스 시점에 실제 동작과 대조해 현행화한다
- **설계 근거**: [CLI 컨셉](../11_기능설계/Nullus_CLI_컨셉.md) · [Automation 계약](../11_기능설계/Nullus_CLI_Automation_계약.md) · [stack.yaml 스키마](../11_기능설계/Nullus_Stack_YAML_스키마.md) · [MCP 설계](../11_기능설계/Nullus_MCP_설계.md)

`nullus`는 Nullus 플랫폼의 통합 CLI다. 웹 UI가 "탐색·설정 마법사(노코드)"라면 CLI는 **반복·자동화·헤드리스**를 맡는다 — 두 표면은 같은 `/api/v1/*` REST를 쓰므로 웹에서 만든 것을 CLI로 조작할 수 있고 그 반대도 된다.

**이런 경우에 CLI를 쓴다:**

| 상황 | 명령 |
|------|------|
| 스택 배포를 코드로 관리하고 CI에서 반복 실행 | `nullus stack deploy -f stack.yaml` |
| 브라우저 없는 환경(CI·air-gap)에서 플랫폼 조작 | `nullus auth bootstrap issue` + 임의 명령 |
| 배포 상태·로그를 터미널에서 바로 확인 | `nullus stack status`, `nullus stack logs -f` |
| AI 어시스턴트(Claude Code 등)에게 플랫폼 조작을 위임 | `nullus mcp serve` (§8) |

설치 마법사(대화형 탐색)·조직/멤버 관리·감사 로그 열람은 웹 UI가 1차 표면이다 — CLI 전용 명령은 없고 필요하면 `nullus api`(§7)로 접근한다.

---

## 1. 설치

단일 정적 바이너리로 배포된다(다중 OS/arch, air-gap 번들 동봉). 릴리스 아티팩트에서 받아 PATH에 두면 끝 — 런타임 의존성이 없다.

```bash
nullus version        # 설치 확인 (서버 버전과의 호환성 경고도 여기서)
```

## 2. 서버 연결 설정

설정 우선순위는 **플래그 > 환경변수 > 설정 파일** 순이다. CI에서는 파일 없이 환경변수만으로 동작한다.

| 방법 | 예 |
|------|----|
| 플래그 | `nullus --server https://nullus.example.com stack ls` |
| 환경변수 | `export NULLUS_SERVER=https://nullus.example.com` |
| 설정 파일 | `~/.nullus/config` → `server: https://nullus.example.com` |

## 3. 인증

경로가 두 가지다. 어느 쪽이든 얻은 토큰은 모든 명령에 자동으로 붙는다.

**사람 (대화형):**

```bash
nullus login          # OIDC 로그인 → 토큰을 ~/.nullus/token 에 캐시 (0600)
```

**무인 (CI·air-gap):**

```bash
export NULLUS_TOKEN=$(nullus auth bootstrap issue)   # 서비스 계정 토큰 발급 (멱등 — 재호출 안전)
# 사용 후:
nullus auth bootstrap revoke
```

- `NULLUS_TOKEN` 환경변수가 토큰 캐시보다 우선한다
- 토큰 파일 권한이 0600이 아니면 CLI가 **읽기를 거부한다** — 이미 노출됐을 수 있으므로 토큰 회전을 검토한 뒤 파일을 재생성하라
- 로컬 dev 모드(`auth.mode=session`)는 토큰 없이 동작한다

## 4. 핵심 워크플로 — 클러스터 등록부터 스택 배포까지

```bash
# 1) 클러스터 등록·검증
nullus cluster register -f kubeconfig.yaml
nullus cluster verify prod-01
nullus cluster ls

# 2) 스택 배포 (선언형 — 파일 하나로 생성+배포)
nullus stack deploy -f stack.yaml

# 3) 완료까지 대기 (exit code로 성패 판정 — CI 친화)
nullus stack status stk_abc123 --wait --timeout 30m

# 4) 로그·수습
nullus stack logs stk_abc123 -f        # 실시간 로그 (WebSocket)
nullus stack rollback stk_abc123 <version>
nullus stack retry stk_abc123
```

`stack.yaml`은 웹 마법사의 입력을 파일로 옮긴 것이다 — 필드 정의·전체 예시는 [stack.yaml 스키마](../11_기능설계/Nullus_Stack_YAML_스키마.md) 참조. 핵심 규칙 하나만 기억하면 된다: **시크릿(GitHub PAT 등)은 파일에 넣을 수 없고**, 배포 시 플래그/환경변수로만 전달한다:

```bash
NULLUS_SCM_TOKEN=ghp_xxx nullus stack deploy -f stack.yaml --ack-warnings
```

(`--ack-warnings`는 호환성 경고를 승인하고 진행하겠다는 이번 실행의 결정이다 — 파일에 넣어 영구화하지 않는다.)

## 5. 자동화(CI)에서 쓰기

CLI의 존재 이유가 이 절이다. 전체 규약은 [Automation 계약](../11_기능설계/Nullus_CLI_Automation_계약.md) 참조.

**Exit code로 분기한다** — "CLI가 실패했나"와 "작업이 실패했나"가 구분된다:

| Code | 의미 | CI에서의 처리 |
|------|------|--------------|
| 0 | 성공 | 다음 단계 진행 |
| 2 | 사용법·요청 오류 (스키마 오류 포함) | 파이프라인 수정 필요 |
| 3 | 인증·권한 | 토큰 발급/권한 확인 |
| 4 | 대상 없음 | ID·이름 확인 |
| 5 | 서버·연결 오류 | 인프라 확인, 재시도는 파이프라인이 결정 |
| 6 | **작업 실패** (배포가 failed로 종료) | 플랫폼에서 원인 조사 (`stack logs`) |
| 7 | `--wait` 타임아웃 (작업은 계속 진행 중) | 대기 연장 또는 후속 폴링 |

**스크립트 출력은 `-o json`으로 받는다** — stdout에는 데이터만, 진행 표시·경고는 전부 stderr로 가므로 파이프가 깨지지 않는다:

```bash
STACK_ID=$(nullus stack deploy -f stack.yaml -o json | jq -r .id)
nullus stack status "$STACK_ID" --wait -o json | jq .state
```

**프롬프트는 없다** — 확인이 필요한 파괴적 작업(rm 등)은 non-TTY에서 즉시 exit 2 하므로, CI에서는 `--yes`를 명시한다.

전형적인 CI 잡:

```bash
export NULLUS_SERVER=https://nullus.example.com
export NULLUS_TOKEN=$(nullus auth bootstrap issue)

nullus cluster verify prod-01
nullus stack deploy -f deploy/gitlab-argocd.yaml --ack-warnings
nullus stack status "$STACK_ID" --wait --timeout 40m
# exit code 가 그대로 잡의 성패가 된다
```

## 6. 조회·운영 명령 (2급)

```bash
nullus stack template ls | get <id>      # Golden Path 카탈로그
nullus compat ls                         # 호환성 매트릭스
nullus stack config get|set|preview <id> # 릴리스 values 편집
nullus token-source ls | rotate          # 토큰소스 운영 (reveal 은 기본 마스킹)
nullus alert ls | create | rm            # 알림 규칙
```

## 7. 전용 명령이 없는 API — `nullus api`

위 명령으로 커버되지 않는 REST(조직/멤버, 감사 로그 등)는 passthrough로 접근한다 — `gh api`와 같은 방식이다. 인증 헤더가 자동으로 붙고, 응답은 가공 없이 그대로 출력된다:

```bash
nullus api GET /api/v1/admin/organizations
nullus api POST /api/v1/stacks/stk_1/validate --data '{"tools":{...}}'
```

exit code 규약(§5)은 동일하게 적용된다. WebSocket 엔드포인트는 지원하지 않는다.

## 8. AI 어시스턴트 연동 (MCP)

`nullus mcp serve`는 Claude Code 등 MCP 클라이언트에게 플랫폼 조회·조작 tool을 제공한다. 프로젝트의 `.mcp.json`에:

```json
{
  "mcpServers": {
    "nullus": {
      "command": "nullus",
      "args": ["mcp", "serve"],
      "env": { "NULLUS_SERVER": "https://nullus.example.com" }
    }
  }
}
```

- 기본은 **읽기 전용**이다(스택/클러스터/템플릿 조회, 로그 tail). 배포·롤백 같은 변경 tool은 `args`에 `--allow-write`를 추가해야 나타난다 — 팀 공용 설정 파일에는 넣지 않기를 권고
- 인증은 CLI와 같은 토큰 캐시를 쓴다 — 미리 `nullus login` 해두거나 `NULLUS_TOKEN`을 env에 넣는다
- 시크릿은 tool 인자로 전달되지 않는다 (SCM PAT는 서버 프로세스의 `NULLUS_SCM_TOKEN` env에서만)

세부 tool 목록과 정책은 [MCP 설계](../11_기능설계/Nullus_MCP_설계.md), 클라이언트별 설정은 MCP 사용 가이드(트랙 B-4, 작성 예정) 참조.

## 9. 명령 요약

| 영역 | 명령 |
|------|------|
| 인증 | `login` · `auth bootstrap issue\|revoke` |
| 클러스터 | `cluster register -f \| verify \| ls \| get \| rm` |
| 스택 | `stack deploy -f \| status [--wait] \| logs -f \| ls \| rm \| rollback \| retry` |
| 파이프라인·앱 | `pipeline deploy <id>` · `app deploy -f` |
| 카탈로그·운영 | `stack template` · `stack config` · `compat` · `token-source` · `alert` |
| 범용 | `api <METHOD> <path>` · `mcp serve [--allow-write]` · `version` |

## 문제 해결

| 증상 | 확인 |
|------|------|
| `서버 주소가 없다` | `NULLUS_SERVER` 또는 `~/.nullus/config`의 `server:` 설정 (§2) |
| exit 3 (인증) | 토큰 만료 → `nullus login` 재실행 또는 bootstrap 재발급 (§3) |
| `토큰 파일 권한이 …` | 0600이 아닌 토큰 파일은 읽지 않는다 — 회전 검토 후 재로그인 (§3) |
| 배포가 400으로 거부 | 호환성 경고 미승인(`DEPLOY_COMPAT_WARN_UNACK`) → 내용 확인 후 `--ack-warnings` (§4) |
| 오류 문의 시 | 오류 메시지의 `trace_id`를 함께 전달하면 서버 로그와 대조된다 |
