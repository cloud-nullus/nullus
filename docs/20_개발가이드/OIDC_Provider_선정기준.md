# OIDC Provider 선정 기준 (Keycloak vs Authentik)

> EPIC: OIDC 설치 옵션화 1차 (nullus-plan#31). 결론: **기본 provider = Keycloak**, Authentik은 opt-in 후순위.

## 비교 기준

| 기준 | Keycloak | Authentik | 판정 |
|---|---|---|---|
| 리포 내 지원 성숙도 | `setup-keycloak.sh` + `keycloak-realm-export.json` + `configs/config.yaml` 기본값 | `setup-authentik.sh` + `configs/config.authentik.yaml` + 별도 `docker-compose.auth.yaml` | Keycloak |
| 로컬 풋프린트 | 컨테이너 1개 (`quay.io/keycloak/keycloak:26.0`, dev-mem) | 컨테이너 4개 (db/redis/server/worker) | Keycloak |
| 기동 시간 (runbook 대기값 기준) | ~60s | ~180s | Keycloak |
| 라이선스 | Apache-2.0 | MIT (Enterprise 기능은 유료) | Keycloak |
| 백엔드 어댑터 | `internal/auth/adapter/keycloak` — provider_factory 기본값 | `internal/auth/adapter/authentik` | 동등 (둘 다 구현됨) |
| JWKS 경로 호환 | jwt_middleware 기본 경로(`/protocol/openid-connect/certs`)와 일치 | issuer 경로 상이(`/application/o/<app>/`) — 미들웨어 보완 필요 (WIP: feat/auth/local-oidc-test-env) | Keycloak |
| K8s 운영 전환 | keycloak-operator / codecentric Helm 등 성숙 | 공식 Helm 차트 | Keycloak 우세 |

## 결정

1. runbook 기본값 `--auth=keycloak` — 팀 표준 로컬 개발 경로.
2. `--auth=authentik` — SSO 데모/멀티 provider 검증 시 opt-in.
3. `--auth=none` — IdP 없이 프론트 mock auth로 개발할 때 (리소스 절약).
4. Authentik 정식 승격(2차)은 JWKS 경로 분기 + production 모드 검증 완료가 선행 조건.
