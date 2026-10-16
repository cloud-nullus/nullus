# Nullus MCP 사용 가이드

- **작성일**: 2026-09-26
- **상태**: v1 구현 기준 ([EPIC nullus-plan#61](https://github.com/cloud-nullus/nullus-plan/issues/61) 트랙 B, [nullus#279](https://github.com/cloud-nullus/nullus/issues/279))
- **설계 근거**: [MCP 설계](../11_기능설계/Nullus_MCP_설계.md) · [Automation 계약](../11_기능설계/Nullus_CLI_Automation_계약.md) · [CLI 사용 가이드 §8](./Nullus_CLI_사용_가이드.md)

`nullus mcp serve`는 AI 어시스턴트(Claude Code 등 MCP 클라이언트)에게 Nullus 조회·조작 tool을 stdio로 제공한다. 원칙은 CLI와 같다 — **`/api/v1/*` REST의 얇은 클라이언트**이며, 서버가 판정하고 tool은 결과를 전달할 뿐이다.

---

## 1. 빠른 시작

전제: `nullus` 바이너리가 PATH에 있고, 서버 주소·토큰이 준비됐다 (CLI와 같은 설정을 공유한다 — [CLI 가이드 §2·§3](./Nullus_CLI_사용_가이드.md)).

```bash
nullus login          # 토큰을 ~/.nullus/token 에 캐시 (무인 환경은 NULLUS_TOKEN env)
```

프로젝트 루트의 `.mcp.json`:

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

이대로면 **읽기 전용 tool 6종**만 노출된다. 클라이언트를 재시작하면 어시스턴트가 `stack_list`·`stack_status` 등을 바로 쓸 수 있다.

## 2. Tool 표면 v1

이름은 `대상_행위` 순 snake_case. 모든 출력은 JSON(text content)이고, 실패는 MCP `isError`에 HTTP 상태·`trace_id`가 담긴다 — 어시스턴트가 원인을 추적할 단서다.

### 읽기 tool (기본 활성)

| tool | 입력 | 설명 |
|------|------|------|
| `stack_list` | (없음) | 스택 목록 — 이름·상태·클러스터·네임스페이스 |
| `stack_status` | `stack_id` | 배포 상태·진행률·실패 스텝과 사유 |
| `cluster_list` | (없음) | 등록된 클러스터 목록 — kubeconfig 등 시크릿성 필드는 **응답에서 제거** |
| `template_list` | (없음) | 스택 템플릿(Golden Path) 카탈로그 |
| `compat_check` | `stack_id`, `tools?`(map) | 도구 조합 호환성 검증 — POST지만 시맨틱은 조회 |
| `stack_logs_tail` | `stack_id`, `lines?`(기본 100) | 설치 로그 최근 N줄 (`GET /stacks/:id/deploy/logs/tail`) |

### 변경 tool (옵트인 시에만 등록, §3)

| tool | 입력 | 설명 |
|------|------|------|
| `stack_deploy` | `stack`([stack.yaml v1alpha1](../11_기능설계/Nullus_Stack_YAML_스키마.md) 구조 객체), `acknowledge_warnings?` | 스택 생성 + 배포. 호환성 게이트는 서버 판정을 따른다 |
| `stack_rollback` | `stack_id`, `version_id`, `reason?` | 설정(config) 버전 롤백. Helm 롤백은 설치 엔진 자동 동작이라 tool 없음 |
| `pipeline_deploy` | `pipeline_id` | CI/CD 파이프라인 배포 트리거 |

retry/continue는 v1에 없다 — 실패 재시도는 사람 확인이 필요한 작업이라 `stack_status`로 확인한 사용자가 웹/CLI로 판단한다 (설계 §2).

## 3. 변경 tool 옵트인 — `--allow-write`

변경 tool 3종은 기본 **비활성**이며, 미허용 시 `list_tools`에 아예 나타나지 않는다 — "호출하면 거부"가 아니라 표면에서 제거돼 모델이 시도조차 못 한다.

| 방법 | 예 |
|------|----|
| 플래그 | `args: ["mcp", "serve", "--allow-write"]` |
| env | `"env": { "NULLUS_MCP_ALLOW_WRITE": "true" }` |

플래그가 명시되면 env를 이긴다 — 공용 env가 켜져 있어도 `--allow-write=false`로 끌 수 있다.

**정책 권고:**

- 팀 공용 설정(리포지토리에 커밋되는 `.mcp.json`)에는 넣지 않는다 — 개인 로컬 설정에서만 켠다
- CI·무인 환경에서는 켜지 않는다 — 자동화된 변경은 exit code로 성패를 판정할 수 있는 CLI(`nullus stack deploy`)가 맞는 표면이다
- 켜더라도 배포·롤백은 서버 RBAC이 최종 판정한다 — 토큰 권한 밖 호출은 403이 그대로 `isError`로 돌아온다

## 4. 인증·시크릿 경로

- **토큰**: `nullus login` 캐시(`~/.nullus/token`) → `NULLUS_TOKEN` env 순으로 찾는다. 없으면 기동하지 않고 stderr 안내 후 exit 3 — 모든 tool이 실패하는 서버를 띄우지 않기 위해서다. 무인 환경은 bootstrap 토큰(`nullus auth bootstrap issue`)을 쓴다
- **SCM PAT**: `stack_deploy`의 GitHub PAT는 tool 인자에 없다 — 서버 프로세스의 `NULLUS_SCM_TOKEN` env에서만 읽어 **deploy 요청 본문**(`source_control.personal_access_token`)에 주입한다. 모델 컨텍스트·대화 로그에 시크릿이 남지 않고, 스택 생성 본문(평문 JSONB로 저장되는 `stacks.config`)에도 실리지 않는다
- 어시스턴트가 stack 객체에 토큰을 섞어 보내도 전송 전에 제거된다. 읽기 응답의 시크릿성 필드(kubeconfig·token·password 등)도 키 자체를 지우고 반환한다

```json
{
  "mcpServers": {
    "nullus": {
      "command": "nullus",
      "args": ["mcp", "serve", "--allow-write"],
      "env": {
        "NULLUS_SERVER": "https://nullus.example.com",
        "NULLUS_SCM_TOKEN": "ghp_..."
      }
    }
  }
}
```

## 5. 운영 특성

- **stdout은 프로토콜 전용**이다 — 시작 배너·버전 스큐 경고·오류 안내는 전부 stderr로 나간다 (Automation 계약 §2). MCP 클라이언트 로그 화면에서 stderr를 확인하라
- 버전 스큐는 시작 시 1회 검사해 stderr 경고만 남긴다 — 서버 도달 실패로 기동을 막지 않는다 (tool 호출이 각자 실패를 보고한다)
- 서버 주소 해석 우선순위는 CLI와 같다: `--server` 플래그 > `NULLUS_SERVER` env > `~/.nullus/config`

## 6. 문제 해결

| 증상 | 원인·해법 |
|------|----------|
| 서버가 바로 종료, stderr에 "로그인 토큰이 없다" (exit 3) | `nullus login` 실행 또는 `.mcp.json` env에 `NULLUS_TOKEN` 추가 |
| exit 2, "서버 주소가 없다" | `.mcp.json` env에 `NULLUS_SERVER` 추가 (또는 `--server` 플래그) |
| 변경 tool이 목록에 안 보임 | 정상 — `--allow-write` 옵트인이 없으면 표면에서 제거된다 (§3) |
| stderr에 "서버 버전 확인 실패" 경고 | 서버 미도달이어도 기동은 계속된다 — 주소·네트워크를 확인하라 |
| tool 호출이 403 `isError` | 토큰 권한 밖 — 서버 RBAC 판정이다. 관리자에게 역할을 확인하라 |
| `stack_deploy`가 경고로 차단 (`DEPLOY_COMPAT_WARN_UNACK`) | 호환성 경고 인지 후 진행하려면 `acknowledge_warnings: true` |

## 7. 수동 검증 (MCP Inspector)

```bash
npx @modelcontextprotocol/inspector -- nullus mcp serve --server https://nullus.example.com
```

Inspector UI에서 initialize → List Tools → tool 호출을 확인한다. `--allow-write`를 붙였다 떼면 tool 수가 9 ↔ 6으로 바뀌는 것으로 옵트인 정책을 검증할 수 있다.
