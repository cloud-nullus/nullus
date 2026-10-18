# 참고

<div class="nullus-cards" markdown>

[**권한 표**<span>세 역할이 각각 무엇에 접근할 수 있나. 사이드바·라우트·서버 세 겹의 대조표.</span>](rbac.md)

[**환경 변수**<span>서버가 실제로 읽는 변수와 기본값.</span>](config.md)

[**CLI**<span>`nullus` 명령 트리와 종료 코드.</span>](cli.md)

[**MCP**<span>AI 어시스턴트용 도구 9종.</span>](mcp.md)

[**API**<span>엔드포인트 그룹 구조와 권한.</span>](api.md)

[**기여하기**<span>브랜치·커밋·PR 규칙과 테스트 전략.</span>](contributing.md)

</div>

## 이 문서의 출처

이 사이트의 내용은 **저장소의 코드**에서 뽑았습니다.

| 무엇 | 출처 |
|------|------|
| 화면 목록·권한 | `web/src/app/routes.tsx` |
| 사이드바 | `web/src/components/layout/nav-model.tsx` |
| 화면 이름 | `web/src/i18n/ko.json` |
| 로그인 후 첫 화면 | `web/src/features/auth/role-landing.ts` |
| API 경로·권한 | Go 라우터 등록부 (`cmd/api/main.go`, 각 모듈의 handler) |
| 환경변수 | `internal/shared/config/config.go`, `configs/config.yaml` |
| 기본 데이터 | `db/migrations/` |
| 버전 | `go.mod`, `web/package.json`, `.github/workflows/`, `Dockerfile` |

틀린 곳을 찾으면 [이슈로 알려 주세요](https://github.com/cloud-nullus/nullus/issues).
