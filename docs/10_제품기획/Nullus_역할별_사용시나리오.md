# Nullus 역할별 사용 시나리오

**작성일**: 2026-03-22
**기반 문서**: nullus_PRD_1.3.md, Nullus_기능목록.md, Nullus_메뉴체계.md
**범위**: Phase 1 (DevOps) 3역할 체계 기반 사용 시나리오

---

## 1. 역할 체계 개요

Nullus는 3단계 RBAC 역할을 사용한다.

| 역할 | 대상 | 핵심 책임 | 초기 화면 |
|------|------|----------|-----------|
| **Admin** | 플랫폼 관리자 | 조직/인원/인프라 관리 | 관리 > 조직 |
| **DevOps Engineer** | DevOps/Platform Engineer | DevSecOps 스택 구성, 배포, 운영 | 데브섹옵스 스택 > 스택 템플릿 |
| **Developer** | 애플리케이션 개발자 | 앱 배포, 모니터링 조회 | CI/CD > CI/CD 템플릿 |

### 역할별 메뉴 가시성

| 대메뉴 | Admin | DevOps Engineer | Developer |
|--------|:-----:|:---------------:|:---------:|
| 데브섹옵스 스택 | O | O | - |
| CI/CD | O | O | O |
| 관측성 | O | O | O |
| 관리 (조직, 사용자, 클러스터) | O | - | - |
| 사용자 (로그아웃) | O | O | O |

---

## 2. Admin 시나리오

Admin은 Nullus 플랫폼의 초기 세팅과 조직/인프라 관리를 담당한다.

### A1. Organization 생성 및 설정

**관련 기능**: F0 (Organization 설정 등록)

1. 관리 > 조직 페이지 진입
2. Organization 이름, 슬러그, 도메인 등록
3. 기본 관리자 계정 지정
4. Organization 활성 상태 설정

**수용 기준**: Org 정보가 DB에 저장되고, 이후 멤버 초대 및 클러스터 등록이 가능해진다.

### A2. 멤버 초대 및 역할 부여

**관련 기능**: F0, F9 (Organization 설정, UI 권한 체계)

1. 관리 > 사용자 관리 진입
2. 초대 링크 생성 (만료 기간 기본 7일)
3. 링크를 대상 멤버에게 공유
4. 멤버가 링크 수락 시 역할(Admin / DevOps Engineer / Developer) 부여
5. 기존 사용자 이메일 검색을 통한 즉시 추가도 가능

**수용 기준**: 초대받은 멤버가 부여된 역할에 해당하는 메뉴만 볼 수 있다.

### A3. 멤버 역할 변경 및 제거

**관련 기능**: F0, F9

1. 관리 > 사용자 관리 진입
2. 대상 멤버의 역할을 변경하거나 비활성화/제거
3. 역할 변경 시 즉시 메뉴 가시성 및 API 접근 권한 반영

### A4. Kubernetes 클러스터 등록

**관련 기능**: F1 (K8S Cluster Configurations 등록)

1. 관리 > 클러스터 관리 진입
2. kubeconfig 파일 업로드
3. 파일 파싱 후 context 추출, AES-256-GCM으로 암호화하여 DB 저장
4. 클러스터 이름 입력 (유니크 검증)
5. 클러스터 타입 선택 (pipeline / target)

**수용 기준**: 클러스터가 등록되고, DevOps Engineer가 스택 설치 시 해당 클러스터를 선택할 수 있다.

### A5. 클러스터 연결 검증

**관련 기능**: F1

1. 클러스터 관리 > 등록된 클러스터 선택
2. "연결 테스트" 클릭
3. kubeconfig 복호화 후 K8s API 호출(`kubectl version`)
4. 연결 상태 갱신: connected / pending / unreachable / auth_failed
5. 연결 성공 시 네임스페이스 목록 캐시

### A6. Organization 활성/비활성 전환

**관련 기능**: F0

1. 관리 > 조직 > 상태 전환
2. 비활성 시 소속 멤버의 플랫폼 접근 차단

### A7. Keycloak RBAC 연동 관리 (v1)

**관련 기능**: F9

1. Keycloak realm 생성
2. groups client scope 생성 + mapper 추가
3. 앱별 클라이언트 생성 (7개 앱: GitLab, ArgoCD, Grafana 등)
4. 각 앱 OIDC 설정
5. K8s API Server OIDC 연동

**권한 매핑**:

| Keycloak Role | GitLab | Argo CD | Grafana |
|---------------|--------|---------|---------|
| admin | Admin | Admin | Admin |
| devops | Maintainer | Read-only | Editor |
| developer | Reporter | Read-only | Viewer |

---

## 3. DevOps Engineer 시나리오

DevOps Engineer는 DevSecOps 스택의 전체 라이프사이클(설정, 배포, 관리, 모니터링)을 담당한다.

### D1. Golden Path 템플릿으로 빠른 시작

**관련 기능**: F3 (Golden Path 템플릿 제공)

1. 데브섹옵스 스택 > 스택 템플릿 진입
2. 3종 템플릿 중 선택:
   - GitLab All-in-One: GitLab CE + GitLab CI + GitLab Registry + MinIO + Argo CD + Prometheus + Grafana
   - GitLab + Argo CD: GitLab CE + GitLab CI + Harbor + MinIO + Argo CD + Prometheus + Grafana
   - GitHub + Argo CD: GitHub(외부) + GitHub Actions(외부) + Harbor + MinIO + Argo CD + Prometheus + Grafana
3. "Use This Template" 클릭 시 프리셋 자동 적용
4. 스택 설치 화면으로 이동

### D2. 노코드 5단계 스택 설정

**관련 기능**: F2 (노코드 기반 DevSecOps Stack 설정 UI)

| 단계 | 탭명 | 설정 내용 |
|------|------|----------|
| 1 | Artifacts | Package Registry, Source Repository, Container Registry, Storage Backend 선택 |
| 2 | CI/CD | CI/CD Platform, CD Tool 선택 |
| 3 | Observability | Monitoring + Logging 도구 통합 선택 |
| 4 | Resources | 개발자 수, 동시 러너 수, 커밋 수, 빌드 빈도 입력 -> 자동 계산 |
| 5 | YAML View | 설정 YAML 미리보기 |

- 각 도구별 버전 드롭다운 제공
- 오른쪽 Configuration Summary 패널에서 선택 현황 실시간 확인
- 8단계 파이프라인 바(Develop -> Build -> Security -> Test -> Deploy -> Operation -> Monitoring -> FinOps) 시각화

### D3. 스택 원클릭 배포

**관련 기능**: F4 (DevSecOps Stack 자동 설치/배포/이력 관리)

1. 설정 완료 후 K8s Object Preview(Namespace, Deployments, Services, Ingress) 확인
2. "Deploy" 클릭
3. 3-Phase 순차 설치:
   - Phase A: 기반 인프라 (Storage, DB, cert-manager)
   - Phase B: 플랫폼 앱 (GitLab, Argo CD 등)
   - Phase C: 연동 (OIDC, Webhook, ServiceMonitor)
4. 실시간 로그 스트리밍(WebSocket) + 프로그레스 바
5. Phase 간 게이트 검증 (이전 Phase 완료 확인 후 다음 Phase 진행)
6. Post-Install 헬스체크 자동 실행
7. 상태 전환: PENDING -> VALIDATING -> INSTALLING -> CONFIGURING -> HEALTHCHECK -> COMPLETED

### D4. 배포 실패 시 자동 롤백

**관련 기능**: F4

1. 설치 중 실패 감지
2. 자동 롤백 시작 (역순 Helm uninstall)
3. PVC 기본 보존 (safe 모드), 명시적 확인 시에만 삭제 (destructive 모드)
4. 상태 전환: FAILED -> ROLLING_BACK -> ROLLED_BACK

**롤백 전략 (릴리스별)**:

| 단계 | 모드 | 설명 |
|------|------|------|
| Alpha | FULL | 전체 롤백만 지원 |
| Beta | FULL + RETRY | 전체 롤백 + 실패 단계 재시도 |
| v1.0 | FULL + PARTIAL + RETRY | 부분 롤백 (실패 컴포넌트만 선택적 롤백) |

### D5. 리소스 예상량 확인

**관련 기능**: F10 (Resource 예상량 계산)

1. Resources 탭에서 입력:
   - 개발자 수, 동시 러너 수, 커밋 수, 빌드 빈도
2. 자동 계산 결과:
   - CPU (cores), Memory (Gi), Storage (Gi), 예상 월 비용
3. 통화 선택: USD / KRW / CNY
4. Auto/Manual 모드 전환:
   - Auto Calculate: 입력값 기반 자동 계산
   - Manual Config: 도구별 CPU/Memory/Storage 직접 입력

### D6. 버전 호환성 확인

**관련 기능**: F8 (OSS 버전 호환성 관리)

1. 데브섹옵스 스택 > 스택 버전 관리 진입
2. 호환성 매트릭스 조회 (Chart 버전 / App 버전 분리 표시)
3. 비검증 조합 선택 시 경고 표시
4. 권장 버전 자동 선택 + "Recommended" 뱃지

### D7. 스택 이력 관리 및 롤백

**관련 기능**: F4

1. 데브섹옵스 스택 > 스택 이력 진입
2. 버전별 스냅샷 목록 조회
3. 이전 버전과 diff 비교 (git diff 스타일)
4. 변경자, 변경 시간, 변경 이유 확인
5. 특정 버전으로 롤백 실행

### D8. CI/CD 파이프라인 템플릿 적용

**관련 기능**: F5 (CI/CD Pipeline 템플릿 제공)

1. CI/CD > CI/CD 템플릿 진입
2. 템플릿 선택:
   - Web Backend (Spring Boot, Express, Django)
   - Web Frontend (React, Vue, Next.js)
   - Batch Job (크론 작업, 데이터 처리)
3. 파라미터 입력 (Repo URL, 이미지명, 환경 변수)
4. CI/CD 설정 에디터에서 편집 (Node.js, Docker, K8s, Python 템플릿)
5. 배포 실행

### D9. 모니터링 대시보드 확인

**관련 기능**: F7 (모니터링/알림 관리)

1. 관측성 > 모니터링 대시보드 진입
2. 확인 가능 지표:
   - Cluster Health: CPU/Memory/Storage 사용률
   - Pipeline Status: 성공/실패 건수, 성공률, 평균 빌드 시간
   - Tool Health: 각 도구별 Running/Warning/Error 상태

### D10. 알림 규칙 설정

**관련 기능**: F7

1. 관측성 > 알림 규칙 진입
2. 알림 이벤트 설정:
   - tool_down, high_cpu, high_memory, storage_warning, pipeline_failure
3. 알림 채널 연동: Slack / Email

### D11. 기존 스택에 도구 추가 설치

**관련 기능**: F2, F3  
**페르소나**: Senior DevOps "민수" 시나리오

> **Note**: 이 시나리오는 Phase 2로 이관되었습니다. Phase 1에서는 새 스택 생성만 지원합니다.

1. 이미 GitLab을 사용 중이지만 모니터링 스택이 없는 상황
2. 스택 설치에서 Monitoring 관련 도구만 선택 (Prometheus + Grafana)
3. 기존 클러스터에 추가 설치
4. 모니터링 스택 구축 완료

### D12. 커스터마이징 배포

**관련 기능**: F2  
**페르소나**: Senior DevOps "민수" 시나리오

1. 기본 템플릿 선택
2. 보안 정책에 맞게 특정 도구 변경 (예: Source Repository를 Custom Git Server로)
3. 연동 설정 입력 (엔드포인트, 인증 정보)
4. 나머지 도구는 기본값 유지
5. 배포 실행

### D13. YAML 에디터 직접 편집 (v1)

**관련 기능**: F2

1. 노코드 UI에서 기본 설정 완료
2. YAML 에디터(Monaco Editor)로 전환
3. 노코드 UI <-> YAML 양방향 동기화
4. YAML 문법/스키마 검증 및 오류 표시
5. 저장 후 배포

---

## 4. Developer 시나리오

Developer는 DevOps Engineer가 구축한 파이프라인 위에서 앱을 배포하고 모니터링한다. 인프라 설정에는 접근하지 않는다.

### V1. Self-Service 앱 배포

**관련 기능**: F6 (CI/CD Pipeline 배포/이력 관리)

1. CI/CD > 앱 배포 위자드 진입
2. 앱 템플릿 선택 (카테고리별 필터: Frontend / Backend / Full-stack):
   - react-spa, next-app, express-api, spring-boot, python-fastapi 등
3. 앱 배포 위자드 5단계:
   1. 앱 이름 입력
   2. Git Repository URL 입력
   3. 대상 클러스터 선택 (드롭다운)
   4. 네임스페이스 선택 (드롭다운)
   5. 리소스 설정 (CPU/Memory 슬라이더)
   6. 환경 변수 설정 (Key-Value 동적 추가/삭제)
4. 매니페스트 미리보기 (YAML) 확인
5. Deploy 실행
6. 필수 K8s Object 자동 생성: Namespace, Deployment, Service, Ingress/Gateway, Secret, PV/PVC, ServiceAccount

### V2. 배포 이력 조회

**관련 기능**: F6

1. CI/CD > CI/CD 이력 진입
2. 배포 기록 조회: 버전, 배포 시간, 결과(성공/실패), 상태
3. 타입/상태 필터링으로 원하는 이력 검색
4. 변경자, 변경 시간, 변경 이유 확인

### V3. 배포 롤백

**관련 기능**: F6

1. CI/CD 이력에서 이전 버전 선택
2. 현재 버전과 diff 비교 (git diff 스타일)
3. 롤백 실행 확인
4. 이전 버전으로 복구

### V4. 모니터링 대시보드 조회 (읽기 전용)

**관련 기능**: F7

1. 관측성 > 모니터링 대시보드 진입
2. 애플리케이션 상태 확인 (Cluster Health, Pipeline Status)
3. Grafana 대시보드에서 상세 지표 조회
4. 알림 설정은 불가 (읽기 전용)

### V5. Git Push 자동 빌드/배포

**관련 기능**: F6  
**페르소나**: Developer "지은" 시나리오

1. DevOps Engineer가 구축한 CI/CD 파이프라인 활용
2. Git push 수행
3. 자동 빌드/배포 파이프라인 실행
4. 빌드/배포 결과 확인

---

## 5. RBAC 권한 매트릭스

| 기능 영역 | Admin | DevOps Engineer | Developer |
|----------|:-----:|:---------------:|:---------:|
| Organization 관리 | O | - | - |
| 사용자 관리 | O | - | - |
| 클러스터 등록/삭제 | O | - | - |
| 클러스터 조회 | O | O | O |
| 스택 설정 생성/수정 | O | O | - |
| 스택 설정 조회 | O | O | O |
| 스택 배포 | O | O | - |
| 파이프라인 배포 | O | O | - |
| 앱 Self-Service 배포 | - | - | O |
| 배포 이력 조회 | O | O | O |
| 모니터링 조회 | O | O | O |
| 알림 설정 | O | O | - |

---

## 6. 역할 간 협업 흐름

```
Admin                    DevOps Engineer              Developer
  |                           |                           |
  |-- Org 생성 ------------->|                           |
  |-- 클러스터 등록 -------->|                           |
  |-- 멤버 초대(DevOps) ---->|                           |
  |                           |-- Golden Path 선택        |
  |                           |-- 5단계 스택 설정         |
  |                           |-- Deploy (원클릭)         |
  |                           |-- 모니터링/알림 설정       |
  |                           |-- CI/CD 템플릿 적용       |
  |                           |-- 멤버 초대(Dev) -------->|
  |                           |                           |-- Self-Service 앱 배포
  |                           |                           |-- Git push -> 자동 빌드/배포
  |                           |                           |-- 모니터링 대시보드 조회
  |                           |                           |-- 배포 이력/롤백
```

**협업 순서**:

1. **Admin**이 Organization을 생성하고, K8s 클러스터를 등록한다
2. **Admin**이 DevOps Engineer를 멤버로 초대한다
3. **DevOps Engineer**가 Golden Path 템플릿을 선택하고, 스택을 설정/배포한다
4. **DevOps Engineer**가 CI/CD 템플릿을 적용하고, 모니터링/알림을 설정한다
5. **DevOps Engineer**가 Developer를 멤버로 초대한다
6. **Developer**가 Self-Service로 앱을 배포하고, Git push로 자동 빌드/배포한다
7. **Developer**가 모니터링 대시보드에서 애플리케이션 상태를 확인한다

---

## 부록

### A. 페르소나 요약

| 페르소나 | 역할 | 경험 | 회사 규모 | 핵심 니즈 |
|---------|------|------|----------|----------|
| Junior DevOps "미정" | DevOps Engineer | K8s 1년차, CI/CD 경험 없음 | 50-500명 (중견기업) | 검증된 조합으로 빠르게 구축 |
| Senior DevOps "민수" | DevOps Engineer | K8s 5년차, CI/CD 경험 많음 | 4천명 (대기업) | 중앙 관리, 커스터마이징, 버전 관리 |
| Developer "지은" | Developer | 개발 3년차 | - | 인프라 신경 안 쓰고 코드에 집중 |

### B. 관련 문서

| 문서 | 경로 |
|------|------|
| PRD v1.3 | `docs/10_제품기획/nullus_PRD_1.3.md` |
| 기능 목록 | `docs/10_제품기획/Nullus_기능목록.md` |
| 메뉴 체계 | `docs/10_제품기획/Nullus_메뉴체계.md` |
| UI/UX 구현계획 | `docs/40_UI_UX/Nullus_UI_UX_구현계획.md` |
| 디자인 시스템 | `docs/40_UI_UX/Nullus_디자인시스템.md` |
