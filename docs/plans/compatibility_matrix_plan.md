# 스택 버젼 호환성 매트릭스(Compatibility Matrix) 고도화 계획

**작성일시**: 2026-04-12
**기능 ID**: F8 (DevSecOps Stack OSS 버전 호환성 관리)
**소속 메뉴**: 데브섹옵스 스택 > 스택 버전 관리

---

## 1. 달성 목표 (Goal)

Nullus 시스템의 핵심 지향점인 **"안전하고 검증된 배포"**를 달성하기 위해, 수십 종의 OSS 도구들이 서로 충돌 없이 안착할 수 있는 조합을 시드 데이터로 구축합니다.
사용자(DevOps, 개발자)가 임의의 버전을 조합 시 발생할 수 있는 에러(K8s 버전 미달, CRD 충돌 등)를 **배포 위저드(Pre-Deploy Gate) 단계에서 사전 차단(Hard Block)하거나 경고(Warn)** 함으로써, 배포 성공률 90% 이상을 확보하는 것을 목표로 합니다.

## 2. 주요 기능 명세 (Features)

1. **상태 머신 게이트 (Pre-Deploy Gate)**
   * `pass`: 검증이 완료된 완전한 Golden Path 조합 (Recommended)
   * `warn`: 검증되지 않은 임의의 버전 조합 (사용자 명시적 승인 필요)
   * `fail`: 호환성이 깨지는 것으로 알려진 치명적 조합 (배포 버튼 활성화 불가)
2. **이원화된 버전 체크**
   * Helm 차트 관점의 `helm_version`과 내부 컨테이너 애플리케이션 관점의 `app_version`을 분리 검증 (예: `bitnami/postgresql` 차트 버전과 `Postgres 16` 엔진 버전 동시 확인)
3. **Narwhal 시드 데이터베이스 주입**
   * 오픈소스 프로젝트 Narwhal의 `VERSIONS.md`에 명시된 하드 트레이닝 데이터(각 분기별 검증 버전 차트)를 매트릭스로 활용
4. **아키텍처 및 제약사항 예외 처리**
   * 배포 타겟 쿠버네티스의 워커 노드 아키텍처(ARM64 등)를 판독하고, 지원하지 않는 이미지(Harbor 일부 등) 선택 시 대안 이미지를 강제하거나 Fail 처리.

---

## 3. 사용자별 사용 시나리오 (Use Cases)

| 페르소나 | 시나리오 요약 |
| :--- | :--- |
| **DevOps Engineer** | **UC 1. 골든 패스(Golden Path) 템플릿 생성**<br>데브옵스 엔지니어는 사내 개발팀을 위한 표준 CI/CD 환경을 조합할 때, 호환성 매트릭스에 `pass` 처리된(Recommended) 버전만을 선택하여 '표준 스택 템플릿' 카탈로그를 작성한다. 배포 실패 리스크를 원천 제거한다. |
| **Developer** | **UC 2. 사전 배포 경고의 확인 및 동의**<br>개발자가 위저드를 통해 신규 최신 버전의 인하우스 앱용 DB를 배포하려 했으나, 매트릭스상 검증되지 않은 조합으로 판별된다. 게이트에서 `warn`이 발생하고, 개발자는 "위험을 감수하고 설치합니다"라는 서약 체크 박스를 누른 뒤에야 배포를 진행할 수 있다. |
| **Admin** | **UC 3. 사내 전용 매트릭스 관리**<br>`스택 버전 관리` 메뉴에 진입하여 시스템이 제공하는 기본 매트릭스 외에, 내부 보안팀이 결재한 버전을 신규 `pass` 매트릭스로 직접 등록/관리한다. |

---

## 4. 현재 구현 현황 

*   **프론트엔드 (UI Layer): [🟢 완성도 높음]**
    *   `stack-install-page.tsx`에 `PreDeployCompatibilityGate` 로직 구비 완료.
    *   사용자의 선택 폼(K8s, MinIO, Postgres 등)을 실시간으로 감지, `useCompatibilityMatrix()` API 훅을 통해 점수 판별.
    *   `fail` 시 에러 토스트 노출 및 하드 블록, `warn` 시 명시적 동의(Acknowledgment 체크) UI 적용 완료.
    *   다국어(`ko.json`, `en.json`) 번역 및 메시지 매핑 완료.

*   **백엔드 (API Layer) 및 DB: [🟡 진행률 약 30%]**
    *   데이터베이스 스키마 및 더미/하드코딩된 API 규격 구성 중.
    *   실제 Narwhal Seed 데이터베이스(수십 종의 도구 매핑 테이블) 미완성.

---

## 5. 잔여 개발 아이템 (Backlog 및 계획)

v1 GA 전까지 이 기능을 프로덕션 수준으로 끌어올리기 위한 남은 작업 목록입니다.

### 백엔드 & 인프라 (Backend & Infra)
- [ ] **Task 1:** `compatibility_matrices` DB 테이블 고도화 (`helm_version`, `app_version`, `min_k8s_version` 등 스키마 세분화)
- [ ] **Task 2:** `GET /api/v1/compatibility/matrix` API에 Narwhal 기반의 **Golden Path 3종 조합을 Seed Data로 영속화**하여 제공.
- [ ] **Task 3:** 쿠버네티스 클러스터 Discovery 로직 내 ARM64 (Node Architecture) 체크 및 파라미터 전달 로직.

### 프론트엔드 (Frontend)
- [ ] **Task 4:** `데브섹옵스 스택 > 스택 버전 관리` 메뉴를 위한 **어드민 뷰 페이지 (목록/상세 그리드)** 신규 제작.
- [ ] **Task 5:** Golden Path 조합(매트릭스 3종)을 유저가 단축 버튼 클릭으로 한 번에 `Auto Select` 할 수 있는 UI 적용.

### DevOps / QA
- [ ] **Task 6:** 3종의 Golden Path 조합을 실제 EKS / GKE 환경에서 CI 배포 후 성공 여부 검증.
- [ ] **Task 7:** `warn` 상태에서 강제 진행 후 배포 실패 시 복구되는(Retry/Rollback) 통합 E2E 테스트 추가.
