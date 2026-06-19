## Nullus Phase 2 논의

### GitHub Project 보드 운영 기준

- Epic 카드: 최대 3일 이내에 끝나는 큰일감
- Task 카드: 1~2시간 안에 끝나는 세부일감
- Epic 1개 아래에 Task 3~7개 정도만 둔다.
- Task가 2시간을 넘길 것 같으면 새 Task로 다시 나눈다.
- 이번 문서는 GitHub Issue / Project 카드 제목으로 바로 복사할 수 있게 작성한다.

### 일정 가정

- 시작일: `2026-06-15 (월)`
- 종료 목표: `2026-07-31 (금)`
- 인원: 3명
- 개인 가용 시간: 주당 5시간
- 팀 총 가용 시간: 주당 15시간
- 운영 방식: 주 1회 백로그 점검 후 다음 주 카드 확정
- 처리 원칙:
  - 1명당 동시에 Epic 1개만 진행
  - 주간 기준 팀 전체 10~15시간 이내로만 카드 배치
  - Epic 1개는 보통 4~8시간, 큰 Epic은 10시간 내외로 본다.
  - 주간 기준 3명이 합쳐 Epic 2개 + 작은 Task 몇 개 정도가 현실적이다.
  - P1이 밀리면 P2/P3 신규 착수는 뒤로 민다.

### 주차 정의

| 주차 | 기간 | 비고 |
| --- | --- | --- |
| W1 | 06/15 ~ 06/21 | 시작 주 |
| W2 | 06/22 ~ 06/28 | 연결/검증 주 |
| W3 | 06/29 ~ 07/05 | 핵심 P1 계속 |
| W4 | 07/06 ~ 07/12 | 구현 마감/안정화 |
| W5 | 07/13 ~ 07/19 | P2 착수 |
| W6 | 07/20 ~ 07/26 | 설계/운영 점검 |
| W7 | 07/27 ~ 07/31 | 버퍼/마감 |

### 현재 프로젝트 기준 빠른 분석

| 항목 | 현재 상태 | 관련 코드/문서 |
| --- | --- | --- |
| OIDC | 세션 + OIDC Provider 구조는 있음. 설치 옵션화는 미완 | `cmd/api/main.go`, `configs/config.yaml`, `configs/config.authentik.yaml`, `internal/auth/**`, `scripts/runbook_local.sh` |
| 설정 Export | 백엔드 export API 있음. 프론트 진입점 없음 | `internal/stack/adapter/handler/export_handler.go`, `internal/stack/usecase/export_config.go`, `e2e/setup_test.go` |
| 설정 Import | 없음 | 신규 구현 필요 |
| OpenBao | 설치/게이트/토큰 소스 구조는 있음. 일부 라우트 미연결 | `internal/shared/secrets/openbao_store.go`, `internal/stack/usecase/install_stack.go`, `internal/admin/adapter/handler/token_source_handler.go`, `web/src/features/admin/api/admin-api.ts` |
| GitHub/GitHub Runner | GitLab 중심. GitHub 경로는 일부 문서/화면만 존재 | `README.md`, `docs/20_개발가이드/Nullus_CICD 흐름.md`, `internal/stack/adapter/helm/gitlab-runner.go` |
| CI/CD 조합 안정화 | GitLab + ArgoCD 위주. 테스트 조합 적음 | `web/src/features/cicd/**`, `internal/cicd/**`, `docs/guides/cicd-demo-guide.md` |
| Legacy 등록 | 없음 | 신규 설계 필요 |
| Deploy Preview | 프론트 미리보기 존재 | `web/src/features/stack/pages/stack-install-page.tsx`, `web/src/features/stack/utils/install-manifest-builders.ts` |
| Alert | 페이지/핸들러는 있으나 채널 확장 필요 | `internal/observability/**`, `web/src/features/observability/**` |
| Air-Gap | 스크립트/값 파일 다수 존재 | `airgap/**`, `airgap/install.sh` |
| OpenTofu/IaC | 없음 | 신규 설계 필요 |

---

## Epic 카드 목록

| Priority | Epic 카드 제목 | 목표 | 예상 크기 | 예상 시간 | 추천 라벨 | 일정 | 권장 담당 | 비고 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| P1 | `EPIC: 프로젝트 소개 페이지 정리` | GitHub 첫인상 개선 | 1일 | 4h | `phase2`, `docs`, `epic` | W1 | 1명 | 빠르게 완료 가능 |
| P1 | `EPIC: 설정 Export UX 마무리` | 기존 export API를 실제 사용 가능 상태로 노출 | 1~2일 | 6h | `phase2`, `stack`, `epic` | W1 | 1명 | 빠르게 끝낼 수 있음 |
| P1 | `EPIC: OpenBao 라우트 연결` | 토큰 소스 API를 main에 연결하고 동작 검증 | 1~2일 | 5h | `phase2`, `security`, `epic` | W2 | 1명 | 구현체는 이미 있음 |
| P1 | `EPIC: CI/CD 조합 테스트 매트릭스 1차` | 현재 되는 조합/안 되는 조합 표준화 | 2일 | 6h | `phase2`, `cicd`, `qa`, `epic` | W2 | 1명 | 기능 추가보다 먼저 필요 |
| P1 | `EPIC: OIDC 설치 옵션화 1차` | Keycloak 우선, Runbook에서 선택 가능하게 정리 | 2~3일 | 8h | `phase2`, `auth`, `epic` | W3 | 1명 | Authentik은 후속 옵션 |
| P1 | `EPIC: 설정 Import 1차` | 파일 업로드 기반 import API + 최소 검증 | 3일 | 10h | `phase2`, `stack`, `epic` | W3~W4 | 1명 | Export 포맷 확정 필요 |
| P1 | `EPIC: GitHub Runner 안정화 조사/수정` | GitHub 경로의 막힌 부분 재현 및 수정 | 2~3일 | 8h | `phase2`, `cicd`, `epic` | W4 | 1명 | 테스트 환경 필요 |
| P2 | `EPIC: Deploy Preview 서버화 여부 결정` | dry-run API 필요성 결론 | 1일 | 3h | `phase2`, `stack`, `decision` | W5 | 1명 | 결정성 작업 |
| P2 | `EPIC: Alert 채널 1차 확장` | Telegram 또는 SMTP 한 채널 우선 연결 | 2~3일 | 8h | `phase2`, `observability`, `epic` | W5 | 1명 | Telegram 우선 검토 |
| P2 | `EPIC: 모바일/반응형 점검 1차` | 주요 페이지 깨짐 점검 및 작은 수정 | 2일 | 5h | `phase2`, `frontend`, `qa`, `epic` | W5 | 1명 | QA 성격 |
| P2 | `EPIC: Air-Gap 제품화 점검` | x86 기준 검증 포인트와 운영 문서 정리 | 2~3일 | 8h | `phase2`, `airgap`, `epic` | W6 | 1명 | 문서/검증 중심 |
| P2 | `EPIC: 자동 SSO 로그인 설계안` | 구현 전 범위/진입 흐름 정리 | 1~2일 | 4h | `phase2`, `auth`, `design` | W6 | 1명 | OIDC 선행 |
| P2 | `EPIC: OpenTofu 포함 여부와 최소 범위 정의` | 카카오클라우드용 범위 구체화 | 1~2일 | 4h | `phase2`, `infra`, `decision` | W7 버퍼 또는 8월 이관 | 1명 | 구현 전 결정 필요 |
| P3 | `EPIC: Legacy Stack 등록 설계안` | 대상/입력/검증 흐름 정리 | 2일 | 5h | `phase2`, `stack`, `design` | W7 | 1명 | 바로 구현하기엔 큼 |
| P3 | `EPIC: DevSecOps 보안 단계 설계안` | 도구 선정과 파이프라인 삽입 위치 정의 | 2~3일 | 6h | `phase2`, `security`, `design` | W7 | 1명 | 구현은 후반 |
| P3 | `EPIC: CLI 컨셉 문서화` | 범위와 명령 체계 확정 | 1일 | 3h | `phase2`, `cli`, `design` | W7 버퍼 또는 8월 이관 | 1명 | 구현 보류 가능 |
| P3 | `EPIC: API 레벨 권한 설계` | 리소스/행위 단위 RBAC 분해 | 2~3일 | 6h | `phase2`, `backend`, `auth`, `design` | W7 버퍼 또는 8월 이관 | 1명 | 복잡도 높음 |
| P3 | `EPIC: Webhook API 범위 정의` | inbound webhook 유즈케이스 정의 | 1일 | 3h | `phase2`, `backend`, `design` | W7 버퍼 또는 8월 이관 | 1명 | 설계 먼저 |
| P3 | `EPIC: Security Dashboard 설계` | 보안/게이트 결과 표시 방식 정의 | 1일 | 3h | `phase2`, `security`, `frontend`, `design` | 8월 이관 후보 | 1명 | |
| P3 | `EPIC: Test/Coverage Dashboard 설계` | 테스트 결과 노출 모델 정의 | 1일 | 3h | `phase2`, `qa`, `frontend`, `design` | 8월 이관 후보 | 1명 | |

### 주차별 추천 배치

| 주차 | 담당자 A | 담당자 B | 담당자 C | 주간 총시간 |
| --- | --- | --- | --- | --- |
| W1 | `EPIC: 프로젝트 소개 페이지 정리` 4h | `EPIC: 설정 Export UX 마무리` 6h | 버퍼/운영 Task 3~4h | 13~14h |
| W2 | `EPIC: OpenBao 라우트 연결` 5h | `EPIC: CI/CD 조합 테스트 매트릭스 1차` 6h | W1 후속 Task 3~4h | 14~15h |
| W3 | `EPIC: OIDC 설치 옵션화 1차` 5h 착수 | `EPIC: 설정 Import 1차` 5h 착수 | CI/CD 후속 또는 버그 수정 4~5h | 14~15h |
| W4 | `EPIC: OIDC 설치 옵션화 1차` 3h 마감 | `EPIC: 설정 Import 1차` 5h 마감 | `EPIC: GitHub Runner 안정화 조사/수정` 5h 착수 | 13h |
| W5 | `EPIC: GitHub Runner 안정화 조사/수정` 3h 마감 | `EPIC: Alert 채널 1차 확장` 5h 착수 | `EPIC: Deploy Preview 서버화 여부 결정` 3h + 반응형 점검 4h 착수 | 15h |
| W6 | `EPIC: Alert 채널 1차 확장` 3h 마감 | `EPIC: Air-Gap 제품화 점검` 5h 착수 | `EPIC: 자동 SSO 로그인 설계안` 4h | 12h |
| W7 | `EPIC: Air-Gap 제품화 점검` 3h 마감 | `EPIC: Legacy Stack 등록 설계안` 5h | `EPIC: DevSecOps 보안 단계 설계안` 5h | 13h |

### 7월 말까지 완료 목표

| 구분 | 카드 |
| --- | --- |
| 7월 내 완료 목표 | 프로젝트 소개 페이지 정리, 설정 Export UX 마무리, OpenBao 라우트 연결, CI/CD 조합 테스트 매트릭스 1차, OIDC 설치 옵션화 1차, 설정 Import 1차, GitHub Runner 안정화 조사/수정, Alert 채널 1차 확장, 모바일/반응형 점검 1차, Deploy Preview 서버화 여부 결정, Air-Gap 제품화 점검, 자동 SSO 로그인 설계안, Legacy Stack 등록 설계안, DevSecOps 보안 단계 설계안 |
| 버퍼 또는 8월 이관 후보 | OpenTofu 포함 여부와 최소 범위 정의, CLI 컨셉 문서화, API 레벨 권한 설계, Webhook API 범위 정의, Security Dashboard 설계, Test/Coverage Dashboard 설계 |

---

## Task 카드 목록

아래 제목은 GitHub Issue 카드 제목으로 바로 복사해도 됩니다.

### 1. `EPIC: OIDC 설치 옵션화 1차`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| `TASK: oidc-onesync 관련 브랜치/설정 흔적 조사` | 1h | 참고 구현 포인트 정리 |
| `TASK: config.yaml 과 config.authentik.yaml 차이 비교` | 1h | 옵션화 대상 정리 |
| `TASK: runbook_local.sh 의 OIDC 입력 포인트 찾기` | 1h | Runbook 변경 위치 파악 |
| `TASK: Keycloak 우선 설치 옵션 초안 작성` | 1h | 결정안 초안 |
| `TASK: Runbook 변수 추가 및 기본값 반영` | 1~2h | 스크립트 수정 |
| `TASK: OIDC 선택 방법 운영 문서 반영` | 1h | 사용자 가이드 |
| `TASK: OIDC 로컬 부팅/로그인 smoke test` | 1~2h | 동작 확인 |

### 2. `EPIC: 설정 Export UX 마무리`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| `TASK: 현재 export API 응답 형식 확인` | 1h | JSON/YAML 스펙 확인 |
| `TASK: 프론트 export 진입 위치 결정` | 1h | 화면 위치 결정 |
| `TASK: stack-api.ts 에 export 호출 추가` | 1h | API 함수 |
| `TASK: 목록 또는 상세 화면에 export 버튼 추가` | 1~2h | UI 반영 |
| `TASK: 파일명 및 포맷 선택 UX 정리` | 1h | 사용성 개선 |
| `TASK: export 프론트 테스트 1~2개 추가` | 1~2h | 회귀 방지 |

### 3. `EPIC: 설정 Import 1차`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| `TASK: export 포맷 기준 import 범위 정의` | 1h | 스코프 결정 |
| `TASK: import request/response 스키마 설계` | 1h | API 초안 |
| `TASK: import 백엔드 usecase 뼈대 추가` | 1~2h | 기본 구조 |
| `TASK: import handler 및 route 추가` | 1~2h | 엔드포인트 |
| `TASK: import 최소 필수 필드 검증 추가` | 1~2h | 유효성 검사 |
| `TASK: import 메모리 또는 단위 테스트 작성` | 1~2h | 테스트 |
| `TASK: import 프론트 업로드 UI 초안 추가` | 1~2h | UI 초안 |
| `TASK: import 성공 후 이동 및 메시지 처리` | 1h | UX 마무리 |

### 4. `EPIC: OpenBao 라우트 연결`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| `TASK: TokenSourceHandler 의존성 생성 경로 확인` | 1h | wiring 체크 |
| `TASK: cmd/api/main.go 에 token source handler 등록` | 1h | 라우트 연결 |
| `TASK: admin-api.ts 와 서버 경로 일치 여부 검증` | 1h | API 정합성 |
| `TASK: token source 목록 조회 수동 테스트` | 1h | 정상 응답 확인 |
| `TASK: rotate approve reveal 최소 테스트 추가` | 1~2h | 회귀 방지 |

### 5. `EPIC: GitHub Runner 안정화 조사/수정`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| `TASK: GitHub 관련 코드 및 문서 검색` | 1h | 영향 범위 정리 |
| `TASK: GitHub Runner 미배포 재현 절차 정리` | 1h | 재현 문서 |
| `TASK: Runner values 또는 step 누락 지점 확인` | 1~2h | 원인 파악 |
| `TASK: GitHub Runner 가장 작은 수정 적용` | 1~2h | 코드 수정 |
| `TASK: GitHub 경로 smoke test` | 1~2h | 검증 결과 |
| `TASK: README 에 GitHub 지원 범위 반영` | 1h | 문서화 |

### 6. `EPIC: CI/CD 조합 테스트 매트릭스 1차`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| `TASK: 현재 지원 조합 문서화` | 1h | 기준표 |
| `TASK: 최소 테스트 조합 3~5개 선정` | 1h | 우선순위 표 |
| `TASK: 각 조합 입력값 템플릿 정리` | 1~2h | 재현 가능한 입력 세트 |
| `TASK: 성공 실패 미지원 상태 표 작성` | 1~2h | 운영 매트릭스 |
| `TASK: 대표 실패 케이스 1건 이슈화` | 1h | 후속 작업 기반 |

### 7. `EPIC: 프로젝트 소개 페이지 정리`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| `TASK: GitHub org profile 또는 README 대상 위치 확인` | 1h | 수정 대상 확인 |
| `TASK: 프로젝트 한 줄 소개와 핵심 기능 정리` | 1h | 콘텐츠 초안 |
| `TASK: README 또는 org profile 반영` | 1~2h | 문서 반영 |
| `TASK: 링크 및 이미지 동작 확인` | 1h | 검수 |

### 8. `EPIC: Alert 채널 1차 확장`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| 현재 알림 저장/전송 경로 확인 | 1h | 구조 파악 |
| Telegram vs SMTP 중 1차 채널 결정 | 1h | 결정안 |
| notifier 인터페이스 확장 포인트 확인 | 1h | 변경 범위 |
| 채널 구현체 1개 추가 | 1~2h | 기능 구현 |
| 설정값/API 노출 최소 반영 | 1~2h | 사용 가능 상태 |
| 간단한 전송 테스트 추가 | 1~2h | 검증 |

### 9. `EPIC: Air-Gap 제품화 점검`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| `airgap/` 스크립트 목록과 역할 정리 | 1h | 구성표 |
| x86 검증 필요 스크립트 선별 | 1h | 검증 계획 |
| 번들 크기/이미지/차트 목록 점검 | 1~2h | 자산 목록 |
| 라이선스 고지 필요 항목 정리 | 1h | 체크리스트 |
| 운영 문서 보강 포인트 정리 | 1h | 문서 TODO |
| 실제 설치 검증 1회 또는 검증 절차 문서화 | 1~2h | 검증 기록 |

### 10. `EPIC: 모바일/반응형 점검 1차`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| 우선 페이지 3개 선정 (`home`, `stack install`, `cicd list`) | 1h | 점검 범위 |
| 3개 페이지 모바일 폭 깨짐 확인 | 1~2h | 이슈 목록 |
| 가장 작은 CSS 수정 1~2건 적용 | 1~2h | UI 개선 |
| 스크린샷으로 before/after 남기기 | 1h | 검증 자료 |

### 11. `EPIC: Deploy Preview 서버화 여부 결정`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| 현재 프론트 preview가 만드는 결과와 실제 배포 입력 비교 | 1h | 차이 분석 |
| 서버 dry-run이 필요한 실패 유형 정리 | 1h | 필요성 판단 |
| 구현/미구현 두 안의 장단점 기록 | 1h | 의사결정 자료 |
| 결론 문서화 | 1h | 결정 완료 |

### 12. `EPIC: 자동 SSO 로그인 설계안`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| 대상 OSS 링크 진입 흐름 정리 | 1h | 사용자 흐름 |
| 현재 로그인 방식과 충돌 포인트 확인 | 1h | 제약사항 |
| Keycloak 기준 우선 설계안 작성 | 1~2h | 설계 초안 |
| 구현 난이도/선행조건 정리 | 1h | 일정 판단 |

### 13. `EPIC: OpenTofu 포함 여부와 최소 범위 정의`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| 회의록의 OpenTofu/Terraform 언급 정리 | 1h | 근거 정리 |
| 카카오클라우드 배포 시 필요한 최소 리소스 정의 | 1~2h | 범위 초안 |
| Phase 2 포함/제외 기준안 작성 | 1h | 의사결정안 |
| Page 1 배포 필수 여부 결론 | 1h | 결정 완료 |

### 14. `EPIC: Legacy Stack 등록 설계안`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| 우선 대상 시스템 1~2개 선정 | 1h | 스코프 |
| "새로 설치"와 "기존 연결" 차이 정리 | 1h | 요구사항 |
| 최소 등록 정보 정의 | 1h | 필드 목록 |
| 검증/연결 테스트 흐름 정리 | 1~2h | 플로우 초안 |
| API/UI 초안 스케치 | 1~2h | 설계안 |

### 15. `EPIC: DevSecOps 보안 단계 설계안`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| SAST/DAST/Container/Secret Scan 후보 도구 정리 | 1~2h | 비교표 |
| GitLab 기준 파이프라인 삽입 위치 정리 | 1h | 단계도 |
| 결과를 Nullus가 어디까지 저장할지 정의 | 1h | 데이터 범위 |
| Quality Gate 초안 작성 | 1h | 정책 초안 |
| Phase 2 후반 구현 순서 제안 | 1h | 실행 순서 |

### 16. `EPIC: CLI 컨셉 문서화`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| CLI 목표 사용자 정의 | 1h | 타깃 정의 |
| 명령 후보 5개 정리 | 1h | 커맨드 초안 |
| API 재사용 방식 정리 | 1h | 구조 초안 |
| 구현 보류 사유와 선행조건 문서화 | 1h | 의사결정 기록 |

### 17. `EPIC: API 레벨 권한 설계`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| 현재 그룹 권한과 실제 기능 불일치 목록 작성 | 1~2h | 갭 분석 |
| 보호가 필요한 액션 목록 정리 | 1h | 액션 표 |
| 리소스/행위 기준 정책표 작성 | 1~2h | RBAC 초안 |
| 적용 순서 제안 | 1h | 단계적 계획 |

### 18. `EPIC: Webhook API 범위 정의`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| 어떤 외부 이벤트를 받을지 정의 | 1h | 유즈케이스 |
| 인증 방식 후보 정리 | 1h | 보안 초안 |
| 최소 엔드포인트 초안 작성 | 1h | API 초안 |
| Phase 2 우선순위 재판단 | 1h | 결정 |

### 19. `EPIC: Security Dashboard 설계`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| 필요한 지표 5개 내외 정의 | 1h | 메트릭 목록 |
| 기존 observability UI 재사용 가능성 확인 | 1h | 재사용 판단 |
| 최소 화면 스케치 | 1h | 와이어 초안 |

### 20. `EPIC: Test/Coverage Dashboard 설계`

| Task 카드 제목 | 예상 시간 | 결과물 |
| --- | --- | --- |
| 표시할 테스트 결과 항목 정의 | 1h | 요구사항 |
| coverage 저장 위치와 수집 방식 초안 | 1h | 데이터 흐름 |
| 기존 페이지 재사용 여부 판단 | 1h | 설계 초안 |

---

## 추천 진행 순서

### 보드 첫 컬럼에 넣을 추천 순서

1. 프로젝트 소개 페이지 정리
2. 설정 Export UX 마무리
3. OpenBao 라우트 연결
4. CI/CD 조합 테스트 매트릭스 1차
5. OIDC 설치 옵션화 1차

### 그 다음 묶음

1. 설정 Import 1차
2. GitHub Runner 안정화 조사/수정
3. Alert 채널 1차 확장
4. Air-Gap 제품화 점검

### 설계 먼저 필요한 묶음

1. Legacy Stack 등록 설계안
2. OpenTofu 포함 여부와 최소 범위 정의
3. DevSecOps 보안 단계 설계안
4. API 레벨 권한 설계

### 이번 문서 사용 방식

- Epic 카드 하나를 `In Progress`로 옮기면, 연결된 Task 카드도 1~2개만 같이 옮긴다.
- Epic이 3일을 넘길 것 같으면 즉시 Epic을 둘로 나눈다.
- Task 완료 기준은 "코드 반영", "문서 반영", "결정 완료" 중 하나가 명확해야 한다.

### 주간 백로그 미팅 체크리스트

| 체크 항목 | 설명 |
| --- | --- |
| 지난주 Epic 완료 여부 | 3일 넘긴 Epic은 즉시 분할 |
| Task 잔량 | 2시간 넘는 Task는 재분할 |
| 다음 주 담당자 배정 | 각자 Epic 1개 이상 동시 진행 금지 |
| 버퍼 사용 여부 | 미완 Epic이 있으면 신규 Epic 착수 제한 |
| 7월 말 마감 영향도 | P1 미완이면 P2/P3 착수 보수적으로 조정 |

### GitHub Project 추천 필드

| 필드 | 추천 값 |
| --- | --- |
| Status | `Backlog`, `Todo`, `In Progress`, `Review`, `Done` |
| Type | `Epic`, `Task`, `Decision`, `Bug` |
| Priority | `P1`, `P2`, `P3` |
| Area | `auth`, `stack`, `cicd`, `airgap`, `frontend`, `docs`, `security` |
| Size | `1h`, `2h`, `1d`, `2d`, `3d` |
| Target Week | `W1` ~ `W7` |
