# MCP

Nullus 를 AI 어시스턴트에 붙이는 Model Context Protocol 서버입니다.

```bash
nullus mcp serve
nullus mcp serve --allow-write   # 변경 도구까지
```

전송은 **stdio 전용**입니다.

!!! note "CLI 가이드는 이 기능을 "미구현" 으로 적고 있습니다"
    실제로는 구현돼 있습니다 — 도구 9종이 등록됩니다. 문서가 기능을 과소
    보고하고 있습니다.

!!! warning "리소스와 프롬프트는 등록하지 않습니다"
    도구(tool)만 노출합니다.

## 읽기 도구 6종 (항상)

| 도구 | 입력 | 호출하는 API |
|------|------|-------------|
| `stack_list` | 없음 | `GET /api/v1/stacks` |
| `stack_status` | `stack_id` | `GET /api/v1/stacks/{id}/status` |
| `cluster_list` | 없음 | `GET /api/v1/admin/clusters` |
| `template_list` | 없음 | `GET /api/v1/stacks/templates` |
| `compat_check` | `stack_id`, `tools?` | `POST /api/v1/stacks/{id}/validate` |
| `stack_logs_tail` | `stack_id`, `lines?`(기본 100) | `GET /api/v1/stacks/{id}/deploy/logs/tail` |

`cluster_list` 응답에서는 민감한 키를 **값 가림이 아니라 키 자체를 삭제**합니다 —
`kubeconfig`, `kubeconfig_encrypted`, `token`, `password`, `secret`.

## 변경 도구 3종 (`--allow-write` 일 때만)

| 도구 | 입력 | 동작 |
|------|------|------|
| `stack_deploy` | `stack`(stack.yaml v1alpha1), `acknowledge_warnings?` | 스택을 만들고 배포합니다 |
| `stack_rollback` | `stack_id`, `version_id`, `reason?` | 롤백 요청 |
| `pipeline_deploy` | `pipeline_id` | 파이프라인 실행 |

`--allow-write` 대신 `NULLUS_MCP_ALLOW_WRITE=true` 도 됩니다. 플래그를 명시하면
환경변수를 이깁니다(`--allow-write=false` 로 끌 수 있음).

!!! note "SCM 토큰은 도구 인자로 받지 않습니다"
    `stack_deploy` 는 입력에서 비밀성 항목을 제거하고, SCM PAT 는 서버 프로세스의
    `NULLUS_SCM_TOKEN` 에서 읽어 배포 요청에만 넣습니다.

!!! warning "`stack_rollback` 은 지금 실패합니다"
    [스택 이력](../features/stack/history.md#롤백)에 적은 것과 같은 이유로 서버의
    롤백 경로가 아직 맞물려 있지 않습니다.

## 기동 조건

토큰이 없으면 서버를 띄우지 않고 **종료 코드 3** 으로 끝납니다.

!!! warning "그때 안내하는 명령이 존재하지 않습니다"
    오류 메시지가 `nullus login` 과 `nullus auth bootstrap issue` 를 안내하지만
    [둘 다 구현되지 않은 명령](cli.md#명령-트리-구현된-전부)입니다. 실제 해법은
    `NULLUS_TOKEN` 환경변수입니다.

기동할 때 버전 차이를 한 번 확인하고, 다르면 표준 오류로 경고만 하고 계속
진행합니다. 시작 배너와 경고는 모두 표준 오류로 나갑니다.
