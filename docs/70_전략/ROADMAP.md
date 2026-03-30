# Nullus Platform Roadmap

**최종 업데이트**: 2026-03-30
**현재 버전**: v0.2.0-alpha

> Nullus는 Kubernetes 기반 DevSecOps 자동화 플랫폼입니다.
> 검증된 OSS 조합(Golden Path)을 선택하고, 노코드 UI로 설정하고, 원클릭으로 배포합니다.

---

## 전체 로드맵 타임라인


```
2026 Q2                           2026 Q3-Q4           2027+
┌─────────── Phase 1 ──────────┐ ┌── Phase 2 ───────┐ ┌── Phase 3 ────────┐
│  DevOps                      │ │  DevSecOps       │ │  InfraOps         │
│  (CI/CD + Monitoring)        │ │  (Test,Security) │ │ (K8s Management)  │
│ W1─4   W5─8   W9─12  W13+    │ │  W25~W40         │ │  W41+             │
│ Alpha  Beta   v1.0   Cycles  │ │  v2.0            │ │  v3.0             │
│ 3/30   4/27   5/25   6/22~   │ │  2026 Q4         │ │  2027             │
└──────────────────────────────┘ └──────────────────┘ └───────────────────┘
```

---

## Phase 1: DevOps — CI/CD + Monitoring

Phase 1은 Nullus의 핵심 가치인 "Golden Path 기반 DevOps 플랫폼 자동 구축"을 완성하는 단계입니다. 12주 개발 + 4주 단위 피드백 사이클로 운영됩니다.

---

### v0.2.0-alpha (W1~W4) — 2026-03-30 ✅ 릴리스 완료

**목표**: 핵심 기능 전체 동작 확인, 내부 테스트 가능 수준

#### 구현 완료 기능

| 기능 | 상태 | 상세 |
|------|------|------|
| F0: Organization 관리 | ✅ 완료 | 생성/수정, 멤버 초대/역할 관리, 감사 로그 |
| F1: K8s 클러스터 등록 | ✅ 완료 | kubeconfig 업로드, AES-256-GCM 암호화, 연결 검증, NS 조회 |
| F2: 노코드 설정 UI | ✅ 완료 | 5단계 위자드 (템플릿→도구→네임스페이스→리소스→배포확인) |
| F3: Golden Path 템플릿 | ✅ 완료 | 3종 (GitLab All-in-One / GitLab+ArgoCD / GitHub+ArgoCD) |
| F4: 스택 자동 설치 | ✅ 완료 | 3-Phase Helm DAG Orchestrator, WebSocket 실시간 로그, 삭제(Helm uninstall) |
| F5: CI/CD 템플릿 | ✅ 완료 | 3종 (web / backend / batch) |
| F6: CI/CD 배포/이력 | ✅ 완료 | 매니페스트 생성 + K8s 적용, WebSocket 로그, 배포 이력 |
| F7: 모니터링/알림 | ✅ 완료 | Prometheus 프록시 대시보드, AlertRule CRUD, 알림 이력 |
| F8: 호환성 관리 | ✅ 완료 | DB 기반 호환성 매트릭스, 버전 검증 |
| F9: RBAC | ✅ 완료 | Admin/DevOps/Developer 3역할, Keycloak OIDC, DualAuth |
| F10: 리소스 관리 | ✅ 완료 | resource_defaults 테이블, OSS 별 기본값 CRUD |
| F11: 감사 로그 | ✅ 완료 | 관리자 작업 전건 기록 (audit_logs) |
| F12: Known Issues | ✅ 완료 | DB 기반 이슈 레지스트리 |

#### 아키텍처

- Clean Architecture + DDD 5개 Bounded Context (Admin, Stack, CI/CD, Observability, Auth)
- Go 1.26 + Echo v4 / React 19 + TypeScript 5.9 + Vite 8
- PostgreSQL 17 + pgx v5, 30+ 마이그레이션
- Helm SDK v3.20.1 + client-go
- Keycloak OIDC + Authentik 지원
- React 19 프론트엔드 15+ 페이지, TanStack Query 연동
- Playwright E2E 18+ 시나리오
- Docker + Helm 차트 배포

#### 품질 지표

| 지표 | 목표 | 현재 |
|------|------|------|
| 설치 성공률 | >70% | 측정 중 |
| Golden Path | 1+ | 3 (목표 초과) |
| P0 버그 | 허용 | — |
| 테스트 커버리지 | >30% | 측정 필요 |

---

### v0.2.0-beta (W5~W8) — 목표: 2026-04-27

**목표**: 안정성 강화, 테스트 자동화, 운영 준비 기반 마련

#### 주차별 마일스톤

**W5 (3/31~4/6)**: Alpha 피드백 반영 + 롤백 강화

- [ ] Alpha 사용성 피드백 수집 및 버그 수정
- [ ] 스택 설치 실패 시 자동 롤백 로직 보강
- [ ] 스택 헬스체크 강화 (Pod readiness + liveness)
- [ ] 연동 설정 자동화 (GitLab ↔ ArgoCD Webhook)

**W6 (4/7~4/13)**: 테스트 커버리지 집중

- [ ] Go 단위 테스트 커버리지 50% 달성
- [ ] 핵심 UseCase 통합 테스트 (testcontainers)
- [ ] Playwright E2E CI 자동화 (GitHub Actions)
- [ ] CI/CD 파이프라인 배포 안정성 개선

**W7 (4/14~4/20)**: 프로덕션 준비

- [ ] 프로덕션 배포 가이드 작성
- [ ] Keycloak SSO 실환경 검증 (GitLab, Grafana, ArgoCD 연동)
- [ ] API 에러 핸들링 일관성 강화 (retryable, trace_id 추가)
- [ ] 프론트엔드 접근성(a11y) 개선
- [ ] 성능 프로파일링 및 병목 개선

**W8 (4/21~4/27)**: Beta 안정화 + 릴리스

- [ ] 코드 프리즈 (4/22)
- [ ] 회귀 테스트 전체 실행
- [ ] Go 테스트 커버리지 70% 달성
- [ ] Beta 릴리스 (4/27)

#### Beta 품질 게이트

| 지표 | 목표 |
|------|------|
| 설치 성공률 | ≥85% |
| 설치 소요 시간 | ≤2시간 |
| P0 버그 | 0 |
| Golden Path | 3종 검증 완료 |
| 테스트 커버리지 | >50% (Go) |
| E2E 테스트 | CI 자동화 |

---

### v1.0.0 GA (W9~W12) — 목표: 2026-05-25

**목표**: 프로덕션 배포 준비 완료, 3+ 조직 검증

#### 주차별 마일스톤

**W9 (4/28~5/4)**: 이력 관리 + 멀티 클러스터

- [ ] 스택 설정 변경 시 스냅샷 자동 저장
- [ ] 버전별 diff API 완성 (side-by-side 비교)
- [ ] 특정 버전으로 롤백 기능
- [ ] 멀티 클러스터 동시 배포 지원 시작

**W10 (5/5~5/11)**: 전체 기능 완성

- [ ] Keycloak OIDC SSO 완전 통합
- [ ] YAML 에디터 ↔ 노코드 UI 양방향 동기화
- [ ] 사용자 알림 설정 구현 (Slack, Email, Webhook)
- [ ] Audit 로그 검색/필터/내보내기
- [ ] CI/CD 파이프라인 롤백 + diff

**W11 (5/12~5/18)**: 프로덕션 검증

- [ ] 3+ 조직 실환경 배포 검증
- [ ] 운영 런북 작성
- [ ] Helm 차트 프로덕션 hardening (PDB, NetworkPolicy, HPA)
- [ ] 보안 감사 및 취약점 스캔 통합
- [ ] API Rate Limiting 세분화

**W12 (5/19~5/25)**: GA 안정화 + 릴리스

- [ ] 코드 프리즈 (5/19)
- [ ] 최종 회귀 테스트
- [ ] 사용자 문서 사이트 공개
- [ ] v1.0 GA 릴리스 (5/25)

#### v1.0 GA 품질 게이트

| 지표 | 목표 |
|------|------|
| 설치 성공률 | ≥90% |
| 설치 소요 시간 | ≤2시간 |
| P0 버그 | 0 |
| Golden Path | 3종 전체 검증 |
| 테스트 커버리지 | >70% |
| 프로덕션 배포 | 3+ 조직 |

#### v1.0 GA 전체 기능 목록

| 기능 | 범위 |
|------|------|
| Organization | 생성/수정/활성화·비활성화, 멤버 초대·역할 관리, 클러스터 접근 범위 설정 |
| Cluster | 등록/삭제/검증, kubeconfig AES-256-GCM 암호화, 멀티 클러스터 |
| Stack Config | 5단계 노코드 UI + YAML 에디터 양방향 동기화, Monaco Editor |
| Golden Path | 3종 템플릿 (GitLab All-in-One / GitLab+ArgoCD / GitHub+ArgoCD) |
| Stack Deploy | 3-Phase DAG, 실시간 로그, 자동 롤백, 헬스체크, 재시도 |
| Stack History | 버전 스냅샷, diff, 특정 버전 롤백 |
| CI/CD | 3종 템플릿 (web/backend/batch), 배포 이력, 롤백, diff |
| Monitoring | Prometheus 대시보드, Grafana 자동 프로비저닝, 파이프라인 성공률 |
| Alerts | CRUD + Slack/Email/Webhook 발송, 알림 이력 |
| Compatibility | 3개 매트릭스, UI 경고, Recommended 뱃지 |
| RBAC | 3역할 + Keycloak OIDC SSO + OSS별 권한 매핑 |
| Resources | 동적 리소스 계산, 비용 추정 (AWS/GCP, USD/KRW/CNY) |
| Audit | 전건 기록, 검색/필터/내보내기 |

---

### v1.x 피드백 사이클 (W13~W28) — 2026-05-26 ~ 2026-09-14

GA 릴리스 이후 4주 단위 피드백 사이클로 지속 개선합니다.

#### Cycle 1: v1.1 (W13~W16, 5/26~6/22)

- [ ] GA 배포 피드백 수집 및 반영
- [ ] 설치 성공률 93% 달성
- [ ] 설치 시간 ≤1.5시간으로 단축
- [ ] 커뮤니티 기여 가이드 정비

#### Cycle 2: v1.2 (W17~W20, 6/23~7/20)

- [ ] Golden Path 2~3종 추가 (Gitea 기반 경량 조합 등)
- [ ] 플러그인 시스템 기반 설계
- [ ] Stack 업그레이드 전략 (canary, blue-green)
- [ ] known-issues.yaml 패턴 70+ → 100+ 확장

#### Cycle 3: v1.3 (W21~W24, 7/21~8/17)

- [ ] GitOps 네이티브 통합 시작 (ArgoCD ApplicationSet)
- [ ] CLI 도구 (nullus-cli) 초기 버전
- [ ] 설정 내보내기/가져오기 (JSON/YAML)
- [ ] Backstage 플러그인 전환 가능성 검토

#### Cycle 4: v1.4 (W25~W28, 8/18~9/14) — Phase 2 준비

- [ ] 설치 성공률 95% 달성
- [ ] Phase 2 요구사항 정의 및 설계
- [ ] 레거시 스택 등록 기능 (기존 환경 인식)
- [ ] API 세분화 권한 (Fine-grained RBAC)

#### 피드백 사이클 품질 목표

| 지표 | Cycle 1 | Cycle 2 | Cycle 3 | Cycle 4 |
|------|---------|---------|---------|---------|
| 설치 성공률 | ≥93% | ≥93% | ≥95% | ≥95% |
| 설치 시간 | ≤1.5hr | ≤1.5hr | ≤1hr | ≤1hr |
| GitHub Stars | 200+ | 500+ | 700+ | 1,000+ |
| Weekly Active Installs | 20+ | 30+ | 40+ | 50+ |
| Contributors | 10+ | 30+ | 40+ | 50+ |
| NPS | >30 | >35 | >35 | >40 |

---

## Phase 2: DevSecOps — Security + Testing (2026 Q3~Q4)

Phase 2는 CI/CD 파이프라인에 보안(Security) 및 테스트(Testing) 레이어를 추가하여 DevOps → DevSecOps로 확장합니다.

### 목표

"Golden Path에 보안 검증과 자동 테스트가 내장된 진정한 DevSecOps 파이프라인 제공"

### 핵심 기능

#### Security 통합

| 기능 | 설명 | 우선순위 |
|------|------|----------|
| SAST 통합 | 정적 코드 분석 (Semgrep, SonarQube) | P0 |
| DAST 통합 | 동적 취약점 스캔 (ZAP, Nuclei) | P1 |
| Container Scanning | 이미지 취약점 스캔 (Trivy, Grype) | P0 |
| Secret Detection | 코드 내 비밀키 탐지 (GitLeaks, TruffleHog) | P0 |
| License Compliance | OSS 라이선스 검증 | P2 |
| Security Dashboard | 보안 취약점 통합 대시보드 | P1 |

#### Testing 통합

| 기능 | 설명 | 우선순위 |
|------|------|----------|
| Unit Test Runner | 언어별 단위 테스트 실행기 | P1 |
| E2E Test Runner | Playwright/Cypress 통합 | P1 |
| Test Coverage Report | 커버리지 리포트 대시보드 | P2 |
| Quality Gate | 커버리지/보안 기준 미달 시 배포 차단 | P0 |

#### 인프라 확장

| 기능 | 설명 | 우선순위 |
|------|------|----------|
| nullus-cli | CLI 도구 (스택 관리, 배포, 로그 조회) | P1 |
| 설정 Export/Import | JSON/YAML 설정 파일 내보내기/가져오기 | P1 |
| Webhook API | 외부 시스템 연동용 Webhook | P2 |
| 레거시 스택 등록 | 기존 환경을 Nullus 관리 하에 등록 | P2 |

### 파이프라인 8단계 완성

```
Phase 1 구현:   Develop → Build ─────────────── Deploy → ─── → Monitoring ───────
Phase 2 추가:                    → Security → Test →      → Ops              → FinOps
                                    (SAST      (Unit       (Health    (Cost
                                     DAST       E2E         Check)    Analysis)
                                     Image      Coverage
                                     Scan)      Gate)
```

### 릴리스 계획

| 마일스톤 | 시기 | 내용 |
|----------|------|------|
| v2.0-alpha | 2026 Q3 초 | SAST + Container Scanning + Quality Gate 기본 |
| v2.0-beta | 2026 Q3 말 | DAST + CLI + Security Dashboard |
| v2.0 GA | 2026 Q4 | 전체 Security/Testing 통합 완료 |

### 기술 스택 추가

| 도구 | 용도 |
|------|------|
| Trivy | 컨테이너 이미지 취약점 스캔 |
| Semgrep / SonarQube | 정적 코드 분석 |
| OWASP ZAP | 동적 웹 취약점 스캔 |
| GitLeaks | 시크릿 탐지 |
| Allure Report | 테스트 리포트 통합 |

---

## Phase 3: InfraOps — Kubernetes 인프라 관리 (2027+)

Phase 3은 Nullus를 DevSecOps 플랫폼에서 완전한 플랫폼 엔지니어링 도구로 확장합니다. Kubernetes 클러스터 자체의 프로비저닝과 멀티 클러스터 관리를 포함합니다.

### 목표

"클러스터 생성부터 앱 배포, 운영까지 — 플랫폼 엔지니어링의 전체 라이프사이클 자동화"

### 핵심 기능

#### 클러스터 프로비저닝

| 기능 | 설명 | 우선순위 |
|------|------|----------|
| Cluster Provisioning | AWS EKS / GCP GKE / Azure AKS 자동 생성 | P0 |
| IaC 통합 | Terraform/Pulumi 기반 인프라 코드 관리 | P0 |
| Cluster Templates | 검증된 클러스터 구성 템플릿 (사이즈별, 환경별) | P1 |
| Node Group 관리 | 노드 그룹 자동 스케일링 설정 | P1 |

#### 멀티 클러스터 관리

| 기능 | 설명 | 우선순위 |
|------|------|----------|
| Fleet Management | 멀티 클러스터 통합 대시보드 | P0 |
| Cross-Cluster Deploy | 멀티 클러스터 동시 배포 | P1 |
| Cluster Federation | 클러스터 간 서비스 디스커버리 | P2 |
| Disaster Recovery | DR 전략 자동화 (Active-Passive, Active-Active) | P2 |

#### 플랫폼 엔지니어링

| 기능 | 설명 | 우선순위 |
|------|------|----------|
| Backstage 통합 | Backstage 플러그인으로 Nullus 기능 제공 | P1 |
| Self-Service Portal | 개발자 셀프서비스 포털 (환경 요청/승인 워크플로우) | P1 |
| Cost Management | FinOps — 클러스터/스택별 비용 분석 및 최적화 | P1 |
| Policy Engine | OPA/Gatekeeper 기반 거버넌스 정책 | P2 |

### 릴리스 계획

| 마일스톤 | 시기 | 내용 |
|----------|------|------|
| v3.0-alpha | 2027 Q1 | EKS/GKE 프로비저닝 + Terraform 통합 |
| v3.0-beta | 2027 Q2 | Fleet Management + Cross-Cluster Deploy |
| v3.0 GA | 2027 Q3 | 전체 InfraOps 통합, Backstage 플러그인 |

---

## 장기 비전 (2027~)

Nullus의 장기 목표는 DevOps 생태계의 중심 오픈소스 프로젝트로 자리매김하는 것입니다.

### 커뮤니티 & 생태계

| 목표 | 시기 | 상세 |
|------|------|------|
| CNCF Sandbox 제출 | 2027 Q1 | CNCF 커뮤니티 인정, 생태계 통합 |
| 플러그인 마켓플레이스 | 2027 Q2 | 커뮤니티 기여 템플릿/도구 마켓 |
| 글로벌 문서화 | 2027 Q2 | 영문/한글/중문 사용자 문서 |
| 컨퍼런스 발표 | 2027 | KubeCon, CNCF 밋업 참여 |

### 제품 확장

| 목표 | 시기 | 상세 |
|------|------|------|
| SaaS 호스티드 버전 | 2027 Q3 | 설치 없이 Nullus 사용 (멀티 테넌트) |
| 멀티 테넌트 강화 | 2027 Q3 | 조직별 완전 격리, 리소스 쿼터 |
| Enterprise 기능 | 2027 Q4 | SSO 연동 강화, Audit 고급 기능, SLA |
| AI 기반 자동화 | 2028 | 장애 자동 진단, 리소스 최적화 추천 |

### 성공 지표 (장기)

| 지표 | 2027 Q2 | 2027 Q4 | 2028 |
|------|---------|---------|------|
| GitHub Stars | 5,000+ | 10,000+ | 20,000+ |
| Monthly Active Installs | 500+ | 2,000+ | 5,000+ |
| Enterprise 고객 | 10+ | 30+ | 100+ |
| 커뮤니티 Contributors | 100+ | 200+ | 500+ |
| Golden Path 수 | 10+ | 20+ | 30+ |

---

## 팀 구성

| 역할 | 인원 | 책임 | 크리티컬 패스 |
|------|------|------|---------------|
| BE Lead (BE-1) | 1 | API 설계, 설치 엔진, 이력 관리 | W1~W12 전구간 |
| BE (BE-2) | 1 | 클러스터 연결, 헬스체크, RBAC, Helm 검증 | W2~W8 |
| FE Lead (FE-1) | 1 | 설정 UI 워크플로우, 실시간 로그, YAML 에디터 | W2~W12 전구간 |
| FE (FE-2) | 1 | 대시보드, 템플릿 UI, 모니터링, RBAC UI | W6~W12 |
| DevOps | 1 | Helm 차트, Golden Path 정의, CI/CD 파이프라인 | W2~W4 (크리티컬) |
| Full-stack/QA | 1 | 통합 테스트, E2E 자동화, 문서, 릴리스 | W1~W12 전구간 |

---

## 의사결정 기록

| 결정 | 선택 | 대안 | 사유 |
|------|------|------|------|
| Phase 1 범위 | DevOps (CI/CD + Monitoring) | 전체 DevSecOps | 12주 내 검증 가능한 MVP |
| Golden Path 수 | 3종 (Alpha 1 → Beta 2 → GA 3) | 5종 이상 | 품질 > 양 |
| 인증 방식 | Keycloak OIDC | Auth0, Firebase | OSS, 자체 호스팅, OSS 권한 매핑 |
| 설치 엔진 | Helm SDK | ArgoCD ApplicationSet | Helm이 K8s 표준, 직접 제어 가능 |
| 아키텍처 | Clean Architecture + DDD | 레이어드 | 모듈 독립성, 향후 마이크로서비스 전환 |
| 프론트엔드 | React 19 + TypeScript | Vue, Svelte | Backstage 플러그인 전환 가능성 |
| 데이터베이스 | PostgreSQL + JSONB | MongoDB | 트랜잭션 안정성, JSONB 유연성 |
| 실시간 통신 | WebSocket | SSE, gRPC Stream | 양방향 통신, 재연결 처리 용이 |

---

## 리스크 & 대응

| 리스크 | 영향 | 대응 |
|--------|------|------|
| Helm 설치 실패율 | 사용자 이탈 | known-issues 패턴 DB + 자동 재시도 + 롤백 |
| 버전 호환성 이슈 | 설치 후 장애 | Compatibility Matrix + 검증된 조합만 Recommended |
| 12주 내 전체 기능 완성 | 품질 저하 | Feature flag로 범위 조절 (YAML sync, Keycloak은 Cycle로 이관 가능) |
| 커뮤니티 참여 저조 | 성장 정체 | 빠른 오픈소스 공개 + 기여 가이드 + 영문 문서 |
| 대규모 클러스터 성능 | 엔터프라이즈 부적합 | v1.x에서 성능 프로파일링 + 최적화 |
