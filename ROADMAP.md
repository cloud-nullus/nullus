# Nullus Platform Roadmap

## Current: v0.1.0-alpha (Phase 1)

PRD v1.3 Phase 1 기능 전체 구현 완료.

- F0-F10 전체 기능 동작 (Organization, Cluster, Stack, CI/CD, Monitoring, RBAC, 리소스 추정)
- Clean Architecture + DDD 5개 Bounded Context
- Helm SDK 기반 스택 자동 설치 엔진 (3-Phase DAG Orchestrator)
- Keycloak OIDC 인증 + 3단계 RBAC (Admin / DevOps / Developer)
- React 19 프론트엔드 15개 페이지, TanStack Query API 연동
- GitHub Actions CI, Docker 빌드, Helm 차트

## Next: v0.2.0-beta

테스트 커버리지, 안정성, 운영 준비.

- Go 테스트 커버리지 70% 이상 달성
- Playwright E2E 테스트 CI 자동화
- 프로덕션 배포 가이드 작성
- Keycloak SSO 연동 실환경 검증 (GitLab, Grafana, ArgoCD)
- 프론트엔드 접근성(a11y) 개선
- API 에러 핸들링 일관성 강화
- 성능 프로파일링 및 병목 개선

## Future: v1.0.0 GA

프로덕션 배포 준비.

- 멀티 클러스터 지원
- Stack 업그레이드 전략 (canary, blue-green)
- 사용자 알림 설정 (Slack, Email, Webhook)
- Audit 로그 검색/필터/내보내기
- API Rate Limiting 세분화
- Helm 차트 프로덕션 hardening (PDB, NetworkPolicy, HPA)
- 보안 감사 및 취약점 스캔 통합
- 사용자 문서 (docs site)

## Long-term

- 플러그인 시스템 (커스텀 도구 추가)
- GitOps 네이티브 통합 (ArgoCD ApplicationSet)
- 멀티 테넌트 지원
- SaaS 호스티드 버전
- 커뮤니티 템플릿 마켓플레이스
