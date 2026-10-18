# CLI

`nullus` 명령줄 도구입니다.

!!! warning "릴리스에 바이너리가 없습니다"
    CLI 를 빌드하는 make 타깃도, 릴리스 자산도 없습니다(릴리스에는 SBOM ·
    취약점 리포트 · 체크섬만 올라갑니다). 지금 CLI 를 얻는 길은 소스에서 직접
    빌드하는 것뿐입니다.

    ```bash
    go build ./cmd/nullus
    ```

    CLI 사용 가이드의 "릴리스 아티팩트에서 받아 PATH 에 두면 끝" 은 아직 사실이
    아닙니다.

## 명령 트리 (구현된 전부)

```
nullus
  --server <url>        API 서버 주소
  -o, --output <fmt>    json (그 외는 사람이 읽는 표)

  version               CLI 버전 출력
  stack                 스택 조회·배포
    ls                  스택 목록 (GET /api/v1/stacks)
  mcp                   MCP 서버
    serve               stdio MCP 서버 기동
      --allow-write     변경 도구 3종 노출
```

!!! danger "가이드에 나오는 명령 상당수가 없습니다"
    `login` · `auth bootstrap issue|revoke` · `cluster` · `pipeline` ·
    `stack deploy|status|logs|rm|rollback|retry|template|config` 는 **등록되지
    않았습니다.** CLI 가이드와 MCP 가이드가 `nullus login` 으로 시작하라고 적지만
    그 명령은 없습니다.

    OIDC 로그인 **로직** 자체는 라이브러리에 구현돼 있습니다. 그걸 부르는 명령만
    없습니다. 그래서 인증은 `NULLUS_TOKEN` 환경변수로 넣어야 합니다.

## 설정과 토큰

우선순위는 **플래그 > 환경변수 > 파일** 입니다.

| 위치 | 내용 |
|------|------|
| `~/.nullus/config` | YAML, `server:` 키 |
| `~/.nullus/token` | 토큰 |
| `NULLUS_CONFIG_DIR` | 위 디렉터리를 다른 곳으로 |

!!! note "토큰 파일 권한을 검사합니다"
    `0600` 이 아니면 읽기를 거부하고 토큰 교체를 권합니다.

## 종료 코드

| 코드 | 뜻 |
|------|-----|
| 0 | 성공 |
| 1 | 그 외 오류 |
| 2 | 사용법 오류 또는 4xx |
| 3 | 인증 실패 (401·403) |
| 4 | 찾을 수 없음 (404) |
| 5 | 5xx 또는 전송 오류 |

!!! note "가이드의 6·7 은 존재하지 않습니다"
    CLI 가이드 표에 6(작업 실패)·7(`--wait` 타임아웃)이 있지만 그 코드를 내는
    분류가 정의돼 있지 않습니다. 해당 명령(`stack status --wait`)이 아직 없기
    때문입니다.

## 함께 들어 있는 바이너리 넷

| 바이너리 | 역할 | `make build` 가 만드나 |
|----------|------|---------------------|
| `cmd/api` | 플랫폼 API 서버 | ✅ |
| `cmd/nullus-bootstrap` | 무인·에어갭 설치용 Keycloak 자격 발급 (`issue` / `revoke`) | ✅ |
| `cmd/nullus` | CLI + MCP 서버 | ❌ |
| `cmd/token-source-sync` | 토큰 출처 백필 ([토큰 관리](../features/admin/tokens.md) 참고) | ❌ |
