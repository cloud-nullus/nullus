# Nullus v1.0 기능 목록

**작성일**: 2026-03-08  
**기반 문서**: nullus_PRD_1.2.md, 상세 기능 명세 및 시스템 아키텍처, 개발 마스터 플랜, 화면설계 프로토타입, Narwhal 분석 기반 Nullus 적용 항목  
**범위**: Phase 1 (DevOps) 전체 기능 + Phase 2-3 예정 기능 요약

## 변경 이력
- 2026-03-29: Empty Template/Storage 미선택 흐름, Alert Rule DB 동기화 편집, Home/Admin UX 보완사항 반영
- 2026-03-08: PRD v1.2 기반으로 갱신 (Narwhal 강화 요구사항: known-issues, 3-Phase, OIDC 반영 확인)

---

## 1. 전체 기능 요약

### 1.1 Phase 1 기능 (본 PRD 범위, v0.1)

| ID | 기능명 | 대상 사용자 | 예상 공수 | 우선순위 |
|---|---|---|---|---|
| `empty-template-v1` | Empty Template | 기본 선택 없음(모든 도구 미선택) | 커스텀 조합을 처음부터 직접 구성하려는 조직 | v1 |
| F0 | Organization 설정 등록 | Admin | 2주 | Must-have |
| F1 | K8S Cluster Configurations 등록 | DevOps Engineer | 1주 | Must-have |
| F2 | 노코드 기반 DevSecOps Stack 설정 UI | DevOps Engineer | 3주 | Must-have |
| F3 | DevSecOps Stack Golden Path 템플릿 제공 | DevOps Engineer | 1주 | Must-have |
| F4 | DevSecOps Stack 자동 설치/배포/이력 관리 | DevOps Engineer | 8주 | Must-have |
| F5 | CI/CD Pipeline 템플릿 제공 | DevOps Engineer | 1주 | Must-have |
| F6 | CI/CD Pipeline 배포/이력 관리 | Developer | 3주 | Must-have |
| F7 | 모니터링/알림 관리 | DevOps/Developer | 4주 | Must-have |
| F8 | DevSecOps Stack OSS 버전 호환성 관리 | DevOps Engineer | 1주 | Must-have |
| F9 | UI 권한 체계 | Admin | 3주 | Must-have |
| F10 | DevSecOps Stack 필요 Resource 예상량 계산 | DevOps Engineer | 2주 | Must-have |

**총 예상 공수**: 12주 (Active Engineer 6명 병렬 진행 기준)

### 1.2 Phase 2-3 예정 기능 (본 PRD 범위 외)

| Phase | 범위 | 주요 기능 | 목표 일정 |
|---|---|---|---|
| Phase 2 | DevSecOps (Security + Test) | SAST/DAST, 단위/E2E 테스트, CLI 도구, 설정 Export/Import, API 권한, Legacy Stack 등록 | 2026 Q3-Q4 |
| Phase 3 | InfraOps (Kubernetes 구축) | 클러스터 프로비저닝, 멀티 클러스터 관리, IaC 통합, Backstage 통합 | 2027+ (v1.0) |

---

## 2. 릴리스별 기능 배분

### 2.1 릴리스 전략

| 릴리스 | 시점 | 대상 | 품질 기준 |
|---|---|---|---|
| Alpha | W4 (2026-03-30) | 클라우드브로 코어 멤버 5~10명 | Happy Path 동작, 버그 허용 |
| Beta | W8 (2026-04-27) | 클라우드브로 전체 + 공개 모집 | 설치 성공률 ≥85%, P0 버그 0건 |
| v1 GA | W12 (2026-05-25) | 전체 사용자 (프로덕션 Ready) | 설치 성공률 ≥90%, 전 기능 완성 |

### 2.2 기능별 릴리스 로드맵

| 기능 | Alpha (W4) | Beta (W8) | v1 GA (W12) |
|---|---|---|---|
| F0: Organization 설정 | ○ 단일 Admin 생성, 세션 인증 | ○ 멤버 초대(링크), 기본 역할 부여 | ● Org 활성/비활성, 클러스터 접근 범위 |
| F1: K8S Cluster 등록 | ● kubeconfig 업로드, 연결 확인, CRUD | — 유지 | — 유지 |
| F2: 노코드 설정 UI | ○ 3단계 (Artifacts/Pipeline/Monitoring), Summary | ○ 5단계 전체 + 8단계 시각화 + 인스턴스 수 + Auto/Manual 리소스 | ● YAML 에디터 + 모드 동기화 |
| F3: Golden Path 템플릿 | ○ 1개 (GitLab All-in-One) | ○ 2개 (+GitLab+ArgoCD) | ● 3개 (+GitHub+Actions) |
| F4: 자동 설치/배포 | ○ Deploy + 실시간 로그 | ○ 롤백 + 헬스체크 + 연동 자동화 | ● 이력 관리 (diff, 스냅샷, 롤백) |
| F5: CI/CD 템플릿 | — | ○ 1개 (Web Backend) | ● 3개 (Web/Backend/Batch) |
| F6: CI/CD 배포/이력 | — | ○ 배포 + 기본 이력 + Developer Self-Service | ● 롤백 + diff + 상세 이력 |
| F7: 모니터링 배포/이력 | — | ○ 대시보드 + 지표 + Slack 알림 | ● 파이프라인 메트릭 + Email 알림 |
| F8: 버전 호환성 관리 | ○ 매트릭스 1개 | ○ 매트릭스 2개 | ● 매트릭스 3개 + 경고 UI |
| F9: UI 권한 체계 | — | — | ● RBAC + Keycloak 연동 |
| F10: 리소스 예상량 | ○ 정적 계산 (USD만) | ○ 통화 선택 (USD/KRW/CNY) | ● 동적 계산 + 비용 추정 |

(○ = 해당 릴리스에서 개발, ● = 완성, — = 해당 릴리스에서 작업 없음)

---

## 3. 기능 상세 명세

---

### F0: Organization 설정 등록

**목적**: Nullus를 팀 단위로 사용하기 위한 조직 관리  
**대상 사용자**: Admin

#### 수용 기준

- [ ] Organization 이름/슬러그/도메인 등록
- [ ] 기본 관리자 계정 지정 및 변경
- [ ] 멤버 초대(초대 링크) 및 기본 역할 부여 (admin/devops/developer)
  - 초대 링크 생성 → 공유 → 수락 플로우
  - 만료 기간 설정 가능 (기본 7일)
- [ ] Organization 활성/비활성 상태 관리
- [ ] Organization 단위 클러스터 접근 범위 설정

#### API 엔드포인트

| Method | Path | 설명 | 릴리스 |
|---|---|---|---|
| POST | `/api/v1/orgs` | Organization 생성 | Alpha |
| GET | `/api/v1/orgs/:orgId` | Organization 조회 | Alpha |
| PUT | `/api/v1/orgs/:orgId` | Organization 수정 | Beta |
| PUT | `/api/v1/orgs/:orgId/status` | 활성/비활성 전환 | v1 |
| POST | `/api/v1/orgs/:orgId/invites` | 초대 링크 생성 | Beta |
| GET | `/api/v1/orgs/:orgId/invites/:token` | 초대 링크 수락 | Beta |
| GET | `/api/v1/orgs/:orgId/members` | 멤버 목록 조회 | Beta |
| PUT | `/api/v1/orgs/:orgId/members/:userId` | 멤버 역할 변경 | v1 |
| DELETE | `/api/v1/orgs/:orgId/members/:userId` | 멤버 제거 | v1 |

#### 점진적 완성 계획

| 릴리스 | 범위 |
|---|---|
| Alpha | Org 생성 (name, slug), 단일 Admin 자동 지정, 세션 인증 |
| Beta | 멤버 초대 (링크 기반), 기본 역할 부여 |
| v1 | 활성/비활성 상태 전환, 클러스터 접근 범위 설정, 멤버 관리 UI 전체 |

---

### F1: K8S Cluster Configurations 등록

**목적**: Nullus가 도구를 설치할 대상 Kubernetes 클러스터 등록 및 관리  
**대상 사용자**: DevOps Engineer

#### 수용 기준

- [ ] 파이프라인 클러스터 / 타겟 클러스터 분리 등록
- [ ] Kubeconfig 업로드 및 유효성 검증 (AES-256-GCM 암호화 저장)
- [ ] 클러스터 이름, 네임스페이스, 엔드포인트, 인증 방식 저장
- [ ] 연결 상태 표시 (connected / pending / unreachable / auth_failed)
- [ ] 등록된 클러스터를 파이프라인 설정 단계에서 선택 가능
- [ ] 클러스터 접근 가능 Organization 설정

#### API 엔드포인트

| Method | Path | 설명 | 릴리스 |
|---|---|---|---|
| POST | `/api/v1/clusters` | 클러스터 등록 (kubeconfig 업로드) | Alpha |
| GET | `/api/v1/clusters` | 클러스터 목록 조회 | Alpha |
| GET | `/api/v1/clusters/:id` | 클러스터 상세 조회 | Alpha |
| PUT | `/api/v1/clusters/:id` | 클러스터 정보 수정 | Alpha |
| DELETE | `/api/v1/clusters/:id` | 클러스터 삭제 | Alpha |
| POST | `/api/v1/clusters/:id/verify` | 연결 검증 | Alpha |
| GET | `/api/v1/clusters/:id/namespaces` | 네임스페이스 목록 조회 | Alpha |

#### 등록 흐름

1. kubeconfig 파일 선택 → 파일 파싱, context 추출 → AES-256 암호화 후 DB 저장
2. 클러스터 이름 입력 → 유니크 검증
3. 클러스터 타입 선택 (pipeline / target)
4. "연결 테스트" 클릭 → kubeconfig 복호화 → `kubectl version` 실행 → 결과 반환
5. 연결 성공 → 상태 'connected' 업데이트, 네임스페이스 목록 캐시

#### 화면 참고

- proto3: Cluster Management 독립 페이지로 승격
- 클러스터 상태 카드 (연결됨/대기/미연결) 표시

---

### F2: 노코드 기반 DevSecOps Stack 설정 UI

**목적**: 웹 UI에서 노코드 방식으로 DevSecOps Stack 도구 선택 및 구성  
**대상 사용자**: DevOps Engineer  
**와이어프레임**: proto2/index.html (프로토타입 완성)

#### 수용 기준

- [ ] **5단계 설정 워크플로우 제공** (PRD v1.1 기준, proto3에서 6→5탭 변경)

| Step | 탭명 | 설명 |
|---|---|---|
| 1 | Artifacts | Package Registry, Source Repository, Container Registry, Storage Backend 선택 |
| 2 | Pipeline Tools | CI/CD 플랫폼, CD 도구 선택 |
| 3 | Monitoring Tools | 수집/조회 도구 선택 |
| 4 | Logging Tools | 수집/조회 도구 선택 |
| 5 | Resources | 팀 규모/워크로드 입력 → 필요 리소스 자동 계산 |

- [ ] **각 탭별 도구 옵션**

| 탭 | 카테고리 | 옵션 | 기본값 |
|---|---|---|---|
| Artifacts | Package Registry | GitLab, Harbor | GitLab |
| Artifacts | Source Repository | GitLab, GitHub, Gitea | GitLab |
| Artifacts | Container Registry | GitLab Registry, Harbor, Docker Hub | GitLab Registry |
| Artifacts | Storage Backend | MinIO (기본), AWS S3 (대안) | MinIO |
| Pipeline Tools | CI/CD Platform | GitLab CI, GitHub Actions, Jenkins | GitLab CI |
| Pipeline Tools | CD Tool | Argo CD, Flux, Tekton | Argo CD |
| Monitoring | Collection | Prometheus, Thanos | Prometheus |
| Monitoring | Visualization | Grafana | Grafana |
| Logging | Collection | OpenTelemetry, Loki | OpenTelemetry |
| Logging | Query & Search | OpenSearch | OpenSearch |
| Resources | 입력 항목 | 개발자 수, 동시 러너 수, 커밋 수, 빌드 빈도 | 10/4/50/medium |

> **Phase 2 백로그 (이관 항목)**: Nexus, JFrog Artifactory, Bitbucket, Amazon ECR, GCS, Azure Blob Storage, CircleCI, Spinnaker, Datadog, Elasticsearch 등 상용/벤더 종속 도구는 Phase 1 범위에서 제외하며, 필요 시 Phase 2 이후 검토함.

- [ ] **파이프라인 8단계 시각화**
  - Develop → Build → Security → Test → Deploy → Operation → Monitoring → FinOps
  - 설정된 단계: 색상/글로우 강조, 미설정 단계: 흐림 처리 + "Phase 2에서 지원" 툴팁

- [ ] **도구별 인스턴스 수 설정**
  - 각 도구(GitLab Runner, Argo CD 등)에 대해 ± 버튼으로 인스턴스(복제본) 수 조절
  - 인스턴스 수에 따라 리소스 예상량 자동 재계산 (F10 연동)
  - 기본값: 도구별 1개 (Runner 등 스케일링 도구는 2~4개 기본)

- [ ] **실시간 Configuration Summary**
  - 오른쪽 패널에 선택한 모든 도구와 버전 표시
  - 탭/선택 변경 시 즉시 갱신

- [ ] **Empty Template / 미선택 조합 지원**
  - `Empty Template` 선택 시 Artifacts, Pipeline, Observability, Logging, Storage를 모두 미선택 상태로 시작
  - Storage Plan은 `미선택`, `기존 DB/Storage 연결`, `통합 DB/Storage 생성 연결`을 지원
  - Storage를 미선택한 경우 Stack 생성 요청에서 storage 블록을 제외하여 선택한 도구만 배포 가능

- [ ] **리소스 설정 모드 전환** (Auto / Manual)
  - **Auto Calculate**: 팀 규모/워크로드 입력값 기반 자동 계산 (기본값)
  - **Manual Config**: 개별 도구별 CPU/Memory/Storage 직접 입력
  - 모드 전환 시 토글 버튼으로 즉시 전환, 이전 값 보존
  - Manual 모드에서 도구별 리소스 슬라이더 또는 숫자 입력 제공

- [ ] **초보자용**: 체크박스/드롭다운 기반 설정 UI 제공
- [ ] **설정 내보내기 (Export)** (Phase 2 예정)
  - JSON 형식 Export (`exportAsJSON()`) — 현재 설정 전체를 JSON 파일로 다운로드
  - YAML 형식 Export (`exportAsYAML()`) — 현재 설정 전체를 YAML 파일로 다운로드

- [ ] **전문가용** (v1): YAML 에디터 (Monaco Editor)로 전체 설정 편집
  - YAML 문법/스키마 검증 및 오류 표시
  - 모드 전환 시 설정 값 동기화 (UI → YAML, YAML → UI)

#### 점진적 완성 계획

| 릴리스 | 범위 |
|---|---|
| Alpha | 3단계 (Artifacts, Pipeline, Monitoring), 노코드 UI만, Summary 패널 |
| Beta | 5단계 전체, 8단계 파이프라인 시각화, Resources 탭 통화 선택, 인스턴스 수 설정, Auto/Manual 리소스 모드 |
| v1 | YAML 에디터 (Monaco), 노코드 ↔ YAML 양방향 동기화, 스키마 검증 |

---

### F3: DevSecOps Stack Golden Path 템플릿 제공

**목적**: 검증된 CI/CD 도구 조합을 사전 정의하여 빠른 선택 지원  
**대상 사용자**: DevOps Engineer

#### 수용 기준

- [ ] 최소 3개 이상의 Golden Path 템플릿 제공
- [ ] 각 템플릿에 포함 정보: 도구 목록(버전 명시), 예상 설치 시간, 권장 사용 사례, 필요 리소스
- [ ] 템플릿 선택 후 즉시 다음 단계(커스터마이징 또는 설치)로 진행 가능

#### 템플릿 목록

| ID | 이름 | 도구 조합 | 대상 | 릴리스 |
|---|---|---|---|---|
| `empty-template-v1` | Empty Template | 기본 선택 없음(모든 도구 미선택) | 커스텀 조합을 처음부터 직접 구성하려는 조직 | v1 |
| `gitlab-allinone-v1` | GitLab All-in-One | GitLab CE + GitLab CI + GitLab Registry + MinIO + Argo CD + Prometheus + Grafana | 중견기업, 단일 플랫폼 선호 | Alpha |
| `gitlab-argocd-v1` | GitLab + Argo CD | GitLab CE + GitLab CI + Harbor + MinIO + Argo CD + Prometheus + Grafana | GitOps 중심 조직 | Beta |
| `github-argocd-v1` | GitHub + Argo CD | GitHub(외부) + GitHub Actions(외부) + Harbor + MinIO + Argo CD + Prometheus + Grafana | GitHub 사용 조직 | v1 |

#### 화면 참고

- proto3: Templates 페이지에서 "Use This Template" 선택 시 Install 화면으로 이동 + 프리셋 자동 적용
- Quick Start 템플릿 3종: Standard CI/CD, Full DevSecOps, Minimal Pipeline

---

### F4: DevSecOps Stack 자동 설치/배포/이력 관리

**목적**: 선택한 도구 조합을 Kubernetes 클러스터에 자동으로 설치  
**대상 사용자**: DevOps Engineer  
**핵심 컴포넌트**: Install Engine (Orchestrator, State Machine, Step Runner, Rollback Manager, Log Streamer)

#### 수용 기준

- [ ] 웹 UI "Deploy Pipeline" 버튼 (클러스터 설정 완료 시에만 활성화)
- [ ] 설치 순서 자동화 (의존성 DAG 기반):
  1. **OpenBao (Secret Control Plane)** → 2. Storage Backend (MinIO) → 3. Source Repository (GitLab CE) → 4. Container Registry → 5. CI Platform (Runner) → 6. CD Tool (Argo CD) → 7. Monitoring (Prometheus + Grafana) → 8. Logging (OTel + OpenSearch) → 9. Integration (도구 간 연동)
- [ ] 진행률 실시간 표시 (프로그레스 바 + WebSocket 로그 스트리밍)
- [ ] 설치 실패 시 자동 롤백 (역순 Helm uninstall + PVC 삭제 + Secret 정리)
- [ ] 설치 완료 후 헬스체크 자동 실행
- [ ] 설치 시간 < 2시간 (기본 템플릿 기준)
- [ ] **배포 스크립트 미리보기 (Deploy Script Preview)** (Phase 2 예정)
  - 현재 설정 기반으로 생성될 설치 스크립트(Bash) 미리보기 모달
  - 클립보드 복사 버튼 제공
- [ ] **K8s Object Preview**
  - 배포 시 생성될 Kubernetes 오브젝트(Namespace, Deployments, Services, Ingress) YAML 미리보기
  - 탭별 오브젝트 구분 표시 (Namespace / Deployments / Services / Ingress)
  - proto3에서 구현 완료 (`renderK8sPreview()` — 각 탭 렌더링)
- [ ] **이력 관리**:
  - 설정 변경 시 버전 스냅샷 자동 저장
  - 버전별 변경자, 변경 시간, 변경 이유 기록
  - 이전 버전과의 diff 표시 (git diff 스타일)
  - 특정 버전으로 롤백 가능
- [ ] **OpenBao 연계 시크릿 주입**:
  - Stack/CI/CD/Observability 컴포넌트가 사용하는 토큰·비밀번호·client secret은 OpenBao에서 조회/주입
  - 원문 비밀값을 values 파일/소스코드/GitHub Actions 로그에 직접 노출하지 않음
  - Secret 회전 시 재배포 없이 반영 가능한 경로(ESO/CSI/sidecar 중 1개 이상) 제공
- [ ] **만료 토큰 자동 갱신(신규)**:
  - lease 기반 토큰은 만료 전 자동 renew
  - 고정 수명 토큰은 provider API 기반 reissue 후 OpenBao 경로 업데이트
  - 갱신 실패 시 백오프 재시도 + 임계 횟수 초과 알림(Slack/Email)

#### Narwhal 레퍼런스 기반 설치 엔진 강화

- **설치 순서 DAG 레퍼런스**: Narwhal의 실전 검증된 의존성 그래프를 레퍼런스 구현으로 활용
  - 3-Phase 프로비저닝 모델: Phase A (기반 인프라: OpenBao, Storage, DB, cert-manager) → Phase B (플랫폼 앱) → Phase C (연동: Webhook, ServiceMonitor, OIDC)
  - Phase 간 게이트 검증 (Phase A 완료 확인 후 Phase B 진행)
  - 참고: Narwhal 스크립트 번호 순서 (07-cnpg → 08-platform → ... → 14-bootstrap)
- **Helm Edge Case 자동 처리 (`known-issues.yaml`)**:
  - Narwhal CLAUDE.md의 70+ 실수 패턴을 코드화
  - CRD 262KB 초과 시 자동 `--server-side --force-conflicts` 전환
  - 비핵심 앱은 `--wait` 제거, `--timeout`만 사용
  - Helm values 사전 검증 (필수 필드 체크: Loki `bucketNames`, Grafana `assertNoLeakedSecrets` 등)
  - Secret 자동 생성 시 정확한 바이트 수 보장
- **노드 아키텍처 호환성**: 설치 전 노드 아키텍처 감지 → ARM64일 때 대체 이미지 자동 선택
- **레지스트리 우선순위 정책**: `ghcr.io > registry.k8s.io > quay.io > docker.io` 순서로 이미지 풀
- **Post-Install 헬스체크 강화**: Narwhal `verify-cluster.sh` (120+ 체크 항목) 패턴 참고, Phase별 분리 검증

#### 상태 머신

```
PENDING → VALIDATING → INSTALLING → CONFIGURING → HEALTHCHECK → COMPLETED
                            │             │
                            ▼             ▼
                         FAILED ←─── (실패 시)
                            │
                            ▼
                      ROLLING_BACK → ROLLED_BACK
```

#### API 엔드포인트

| Method | Path | 설명 | 릴리스 |
|---|---|---|---|
| POST | `/api/v1/deployments/stacks/:stackId/deploy` | 스택 배포 시작 | Alpha |
| GET | `/api/v1/deployments/stacks/:stackId/status` | 배포 상태 | Alpha |
| POST | `/api/v1/deployments/stacks/:stackId/rollback` | 배포 롤백 | Beta |
| WebSocket | `/ws/deployments/:deploymentId/logs` | 실시간 로그 | Alpha |
| GET | `/api/v1/stacks/:stackId/history` | 이력 조회 | v1 |
| GET | `/api/v1/stacks/:stackId/history/:versionId/diff` | 버전 diff | v1 |
| POST | `/api/v1/stacks/:stackId/rollback/:versionId` | 버전 롤백 | v1 |

#### 점진적 완성 계획

| 릴리스 | 범위 |
|---|---|
| Alpha | Deploy → 순차 설치 + 실시간 로그 + K8s Preview. 롤백/이력 없음 |
| Beta | 설치 실패 시 자동 롤백, 헬스체크, 연동 설정 자동화 |
| v1 | 설정 변경 시 스냅샷 저장, 버전별 diff, 특정 버전 롤백 |

---

### F5: CI/CD Pipeline 템플릿 제공

**목적**: 애플리케이션 배포를 위한 표준 CI/CD 파이프라인 템플릿 제공  
**대상 사용자**: DevOps Engineer

#### 수용 기준

- [ ] 최소 3개 이상의 파이프라인 템플릿 제공 (Web/Backend/Batch)
- [ ] 템플릿별 포함 단계/도구/변수 안내
- [ ] 템플릿 선택 후 배포 단계로 바로 이동 가능
- [ ] 템플릿 파라미터 입력 폼 제공 (Repo, 이미지명, 환경 변수 등)
- [ ] 템플릿 버전 관리 및 변경 이력 표시
- [ ] **CI/CD 설정 에디터 모달**
  - 파이프라인 설정(CI config)을 모달 내에서 직접 편집
  - 템플릿 적용 기능: Node.js, Docker, Kubernetes, Python 등 사전 정의 템플릿 선택 시 설정 자동 생성
  - 실시간 문법 검증 (Validation) 및 오류 표시
  - 저장(Save) 시 설정 반영

#### 템플릿 목록

| ID | 이름 | 대상 | 포함 단계 | 릴리스 |
|---|---|---|---|---|
| `web-backend-v1` | Web Backend | Spring Boot, Express, Django 등 | Build → Test → Image Build → Deploy | Beta |
| `web-frontend-v1` | Web Frontend | React, Vue, Next.js 등 | Build → Test → Static Build → Deploy (Nginx) | v1 |
| `batch-job-v1` | Batch Job | 크론 작업, 데이터 처리 | Build → Image Build → CronJob Deploy | v1 |

---

### F6: CI/CD Pipeline 배포/이력 관리

**목적**: 애플리케이션 파이프라인을 배포하고 이력을 추적 관리  
**대상 사용자**: Developer

#### 수용 기준

- [ ] **Developer Self-Service Deploy** (proto3 구현 기반)
  - 역할 전환 시 Developer 전용 화면 제공 (DevOps → Developer 모드 전환)
  - 애플리케이션 템플릿 선택: react-spa, next-app, express-api, spring-boot, python-fastapi 등
  - 카테고리별 필터: Frontend, Backend, Full-stack
  - 앱 배포 위자드 (App Wizard):
    1. 앱 이름 입력
    2. Git Repository URL 입력
    3. 대상 클러스터 선택 (드롭다운)
    4. 네임스페이스 선택 (드롭다운)
    5. 리소스 설정 (CPU/Memory 슬라이더)
    6. 환경 변수 설정 (Key-Value 동적 추가/삭제)
  - 매니페스트 미리보기 (YAML) 후 Deploy 실행
- [ ] 파이프라인 배포 시 필수 Kubernetes Object 자동 생성
  - Namespace, Deployment, Service, Ingress/Gateway, Secret, PV/PVC, ServiceAccount
- [ ] 파이프라인 배포 기록 (버전, 배포 시간, 결과) 저장
- [ ] 배포 실패 시 이전 버전으로 롤백 지원
- [ ] 배포 이력 조회 및 상태 필터링 제공
- [ ] 버전별 변경자, 변경 시간, 변경 이유 기록
- [ ] 이전 버전과의 diff 표시 (git diff 스타일)
- [ ] 특정 버전으로 롤백 가능

#### API 엔드포인트

| Method | Path | 설명 | 릴리스 |
|---|---|---|---|
| POST | `/api/v1/pipelines` | 파이프라인 생성 (템플릿 + 파라미터) | Beta |
| GET | `/api/v1/pipelines` | 파이프라인 목록 | Beta |
| POST | `/api/v1/pipelines/:id/deploy` | 파이프라인 배포 실행 | Beta |
| GET | `/api/v1/pipelines/:id/deployments` | 배포 이력 조회 | Beta |
| GET | `/api/v1/pipelines/:id/deployments/:did` | 배포 상세 (K8s 오브젝트 목록) | v1 |
| POST | `/api/v1/pipelines/:id/rollback/:did` | 특정 버전으로 롤백 | v1 |
| GET | `/api/v1/pipelines/:id/deployments/:did/diff` | 이전 버전과 diff | v1 |

#### 점진적 완성 계획

| 릴리스 | 범위 |
|---|---|
| Beta | 파이프라인 생성/배포, 기본 이력 (버전/시간/결과/상태 필터링) |
| v1 | 롤백, diff, 변경자/사유 기록, K8s 오브젝트 상세 조회 |

---

### F7: 모니터링/알림 관리

**목적**: 설치된 스택과 사용자 애플리케이션의 상태를 모니터링  
**대상 사용자**: DevOps Engineer, Developer

#### 수용 기준

- [ ] 기본 대시보드 제공 (클러스터/파이프라인/애플리케이션)
  - Cluster Health (CPU/MEM/Storage 사용률)
  - Pipeline Status (성공/실패 건수, 성공률, 평균 빌드 시간)
  - Tool Health (각 도구별 Running/Warning/Error 상태)
- [ ] 핵심 지표 수집 (CPU, Memory, Storage, 파이프라인 성공률)
- [ ] 알림 연동 기본값 제공 (Slack/Email 중 1개 이상)
  - 이벤트: tool_down, high_cpu, high_memory, storage_warning, pipeline_failure
- [ ] Alert Rule 편집 시 Edit 팝업에서 DB 최신 값을 단건 조회 후 수정하고, Save 뒤 즉시 목록에 반영
- [ ] Alert Rule 임계값을 Warning / Critical 2단계로 관리

#### API 엔드포인트

| Method | Path | 설명 | 릴리스 |
|---|---|---|---|
| GET | `/api/v1/monitoring/dashboards` | 대시보드 데이터 | Beta |
| GET | `/api/v1/monitoring/metrics/summary` | 메트릭 요약 | Beta |
| POST | `/api/v1/monitoring/alerts/config` | 알림 설정 | Beta |

#### 점진적 완성 계획

| 릴리스 | 범위 |
|---|---|
| Beta | 기본 대시보드 (클러스터+도구 상태), 핵심 지표, Slack 알림 |
| v1 | 파이프라인 성공률 메트릭, Grafana 대시보드 자동 프로비저닝 확장, Email 알림 |

#### 현재 기준 추가 개발 필요 항목 (Observability)

- [ ] **Alert Rule 모델 고도화**: `condition` 문자열 중심에서 `metric_name + operator + threshold + window` 구조로 확장
- [ ] **메트릭 기반 공통 평가 엔진**: 항목별 하드코딩 없이 `metric_name`만으로 룰 평가 가능한 evaluator 구현
- [ ] **알림 채널 확장 구조**: Slack/Email 공통 인터페이스로 추상화하고 재시도/백오프 정책 추가
- [ ] **Alert History 고도화**: fired/resolved 상태 전이, 중복 알림 억제(dedup), ack/silence 기능 추가
- [ ] **대시보드 API 세분화**: summary 외에 시계열/범위 조회(`from`, `to`, `step`) API 추가
- [ ] **권한 분리**: 조회(Viewer)와 규칙 관리(DevOps/Admin) 권한 분리 및 감사 로그 연동
- [ ] **테스트 보강**: metric evaluator 단위 테스트 + 채널 어댑터 통합 테스트 + e2e 시나리오(룰 생성→발생→해제)
- [ ] **운영 설정**: 룰 평가 주기, 채널 타임아웃, 알림 rate limit를 환경변수/설정 파일로 관리

---

### F8: DevSecOps Stack OSS 버전 호환성 관리

**목적**: 도구 간 버전 호환성을 사전 검증하여 설치 실패 방지  
**대상 사용자**: DevOps Engineer

#### 수용 기준

- [ ] `templates/compatibility/compatibility-matrix.yaml`에 테스트 완료된 버전 조합 정의
- [ ] 검증 API 제공 (도구+버전 조합 → compatible/untested 판별)
- [ ] 비검증 조합 선택 시 경고 (v1)
- [ ] 권장 버전 자동 선택 + "Recommended" 뱃지 표시 (v1)

#### API 엔드포인트

| Method | Path | 설명 | 릴리스 |
|---|---|---|---|
| GET | `/api/v1/compatibility/matrix` | 매트릭스 조회 | Alpha |
| POST | `/api/v1/compatibility/validate` | 조합 검증 | Alpha |

#### 호환성 매트릭스 예시

```yaml
matrices:
  - id: "gitlab-allinone-v1"
    name: "GitLab All-in-One"
    status: "verified"
    kubernetes: { min: "1.26", max: "1.30", recommended: "1.28" }
    tools:
      source_repository: { name: "gitlab-ce", helm_version: "10.7.x", app_version: "17.7.x" }
      cd_tool: { name: "argocd", helm_version: "7.7.x", app_version: "2.13.x" }
      monitoring_collection: { name: "prometheus", helm_version: "26.0.x", app_version: "3.1.x" }
      # ...
```

#### Narwhal 기반 매트릭스 확장

- **Chart 버전 / App 버전 분리**: 호환성 매트릭스에 `helm_version`과 `app_version`을 분리 관리
  - 예: Traefik v39.0.0 (chart) / v3.6.7 (app), Loki 6.52.0 (chart) / 3.6.4 (app)
- **Narwhal 시드 데이터**: Narwhal VERSIONS.md의 실제 버전 매핑을 초기 매트릭스 시드 데이터로 활용

#### 점진적 완성 계획

| 릴리스 | 범위 |
|---|---|
| Alpha | 매트릭스 1개 (GitLab All-in-One), API 검증 |
| Beta | 매트릭스 2개, 설정 UI에서 비검증 조합 콘솔 경고 |
| v1 | 매트릭스 3개, UI 경고 팝업, 권장 버전 자동 선택, "Recommended" 뱃지 |

---

### F9: UI 권한 체계

**목적**: 역할 기반 접근 제어로 팀별 기능 통제  
**대상 사용자**: Admin  
**릴리스**: v1 (전체 구현)

#### 수용 기준

- [ ] Role 기반 접근 제어 (Admin / DevOps Engineer / Developer)
- [ ] 대메뉴 단위 접근 권한 설정 (사용자 관리 포함)
- [ ] 사용자 관리 화면 제공 (역할 부여, 비활성화)
- [ ] OSS별 권한 매핑 지원 (Keycloak 활용)
- [ ] 토큰 회전 관리 권한 분리 (Admin만 rotate/approve/pause/resume 가능)
- [ ] **고위험 조회 Step-up 인증**:
  - 관리자 토큰/시크릿 조회(reveal)는 재인증(비밀번호 재입력 또는 OIDC step-up) 후에만 허용
  - 재인증 성공 세션의 유효시간은 짧게 제한(예: 5분)
  - 조회 이력(사용자/시각/대상 path)은 감사 로그에 필수 기록

#### RBAC 매핑

| 기능 영역 | Admin | DevOps Engineer | Developer |
|---|---|---|---|
| Organization 관리 | ✅ | ❌ | ❌ |
| 사용자 관리 | ✅ | ❌ | ❌ |
| 클러스터 등록/삭제 | ✅ | ❌ | ❌ |
| 클러스터 조회 | ✅ | ✅ | ✅ |
| 스택 설정 생성/수정 | ✅ | ✅ | ❌ |
| 스택 설정 조회 | ✅ | ✅ | ✅ |
| 스택 배포 | ✅ | ✅ | ❌ |
| 파이프라인 배포 | ✅ | ✅ | ❌ |
| 배포 이력 조회 | ✅ | ✅ | ✅ |
| 모니터링 조회 | ✅ | ✅ | ✅ |
| 알림 설정 | ✅ | ✅ | ❌ |

#### Keycloak 연동 (v1)

```
Keycloak Role "admin"    → GitLab Admin + Argo CD Admin + Grafana Admin
Keycloak Role "devops"   → GitLab Maintainer + Argo CD Read-only + Grafana Editor
Keycloak Role "developer" → GitLab Reporter + Argo CD Read-only + Grafana Viewer
```

#### Narwhal Keycloak OIDC 통합 레퍼런스

- Narwhal의 7-app Keycloak OIDC 연동 플로우를 구현 레퍼런스로 활용
- 구현 플로우: Keycloak realm 생성 → groups client scope 생성 → 7개 클라이언트 생성 → 각 앱별 OIDC 설정 → K8s API Server OIDC 연동
- 알려진 SSO 이슈:
  - `groups` scope를 realm 레벨에서 생성 + mapper 추가 + 전체 클라이언트에 default scope 할당 필요 (미수행 시 `invalid_scope` 에러)
  - ArgoCD SSO: `x509: certificate signed by unknown authority` → self-signed cert 처리 필요
  - Headlamp: `oidc-skip-issuer-tls-verify` 플래그 없음 → CA cert 직접 마운트
- 참고: Narwhal `11-keycloak.sh` 스크립트 → Go 코드 전환의 사실상 구현 명세서

---

### F10: DevSecOps Stack 필요 Resource 예상량 계산

**목적**: 설치 전 인프라 요구사항을 사전에 파악  
**대상 사용자**: DevOps Engineer

#### 수용 기준

- [ ] 입력 항목: 개발자 수, 동시 러너 수, 커밋 수, 빌드 빈도
- [ ] 자동 계산: CPU (cores), Memory (Gi), Storage (Gi), 예상 월 비용
- [ ] 통화 선택: USD, KRW, CNY

#### 리소스 기본값 테이블 (도구별)

| 도구 | CPU | Memory | Storage |
|---|---|---|---|
| GitLab CE | 4 | 8 Gi | 30 Gi |
| GitLab Runner | 2 | 4 Gi | 10 Gi |
| Argo CD | 1 | 2 Gi | 5 Gi |
| Prometheus | 1 | 4 Gi | 20 Gi |
| Grafana | 0.5 | 1 Gi | 5 Gi |
| MinIO | 0.5 | 1 Gi | 50 Gi |
| OpenTelemetry | 0.5 | 1 Gi | 0 Gi |
| OpenSearch | 2 | 4 Gi | 30 Gi |
| **기본 합계** | **11.5** | **25 Gi** | **150 Gi** |

#### 점진적 완성 계획

| 릴리스 | 범위 |
|---|---|
| Alpha | 정적 계산 (도구별 기본값 합산만), USD만 |
| Beta | 통화 선택 (USD/KRW/CNY) |
| v1 | 동적 계산 (스케일링 팩터), 클라우드별 비용 추정, 그래프 표시 |

---

## 4. 화면 구성 요약

### 4.1 주요 화면 목록

| 화면 | 설명 | 프로토타입 |
|---|---|---|
| Home | 대시보드 홈 (역할별 요약 정보 표시) | proto3 |
| 로그인 | Admin 로그인 (세션 기반, v1에서 Keycloak OIDC) | — |
| Cluster Management | 클러스터 등록/수정/상태 관리 (proto3에서 독립 페이지 승격) | proto3 |
| Install DevSecOps | 5단계 설정 워크플로우 + Configuration Summary + Deploy 버튼 | proto2, proto3 |
| DevSecOps Stack Templates | Golden Path 템플릿 카드 선택 → 프리셋 자동 적용 | proto3 |
| DevSecOps Stack List | 구성된 스택 목록 카드 (검색/필터/정렬/New Stack) | proto1, proto2, proto3 |
| DevSecOps Stack History | 스택 변경 이력 + diff 뷰어 + 배포 로그 모달 + 롤백 확인 모달 | proto3 |
| DevSecOps Stack Version Management | 스택 버전별 관리 (스냅샷 목록, 버전 비교) | proto3 |
| CI/CD Template | CI/CD 파이프라인 템플릿 목록 (검색 기능 포함) | proto3 |
| CI/CD List | 생성된 CI/CD 파이프라인 목록 (검색/필터) | proto3 |
| CI/CD History | CI/CD 배포 이력 (타입/상태 필터) | proto3 |
| 모니터링 대시보드 | Cluster Health (CPU/MEM/Storage 바 차트) + Pipeline Status + Tool Health | proto3 |
| Alert Rule List | 알림 규칙 목록 (메뉴 정의, 상세 구현은 Phase 2) | proto3 (메뉴만) |
| Alert History | 알림 발생 이력 (메뉴 정의, 상세 구현은 Phase 2) | proto3 (메뉴만) |
| Organization 관리 | 조직 정보 등록/수정 | — |
| 사용자 관리 | 역할 부여/비활성화 (v1) | proto3 |
| Developer Deploy | Developer Self-Service 앱 배포 위자드 (역할 전환 시 표시) | proto3 |
- Home 화면에서는 8단계 파이프라인 중 사용하지 않는 `Operation` 항목을 제거하여 현재 기능 범위와 일치하도록 정리함
- Organization 화면의 `Add User`는 별도 404 경로가 아니라 실제 `User Management` 라우트(`/admin/users`)로 연결함

### 4.2 공통 UI/UX 요소

- **다크/라이트 테마 전환**
  - 다크 테마(기본): 배경 #0f1419 / #1a1d29, 강조색 #ffd700
  - 라이트 테마: 밝은 배경, 동일 강조색 체계
  - 토글 버튼으로 전환, `localStorage`에 선택 상태 영속화
- **8단계 파이프라인 바**: Develop → Build → Security → Test → Deploy → Operation → Monitoring → FinOps
- **사이드바 내비게이션**
  - 섹션 구조: DEVSECOPS STACK (Template, Install, List, History, Version Management) / CI/CD (Template, List, History) / OBSERVABILITY (Monitoring Dashboard, Alert Rule List, Alert History) / ADMIN (Organization, User Management, Cluster Management) / USER (Log out)
  - **접기/펼치기 토글**: 아이콘만 표시 모드 ↔ 전체 표시 모드, `localStorage` 영속화
  - Cluster Configuration 모달: 사이드바에서 직접 클러스터 설정 접근 가능
- **Configuration Summary 패널**: 오른쪽 고정, 실시간 갱신
- **토스트 알림 시스템**: 작업 완료/오류/경고 시 우측 상단 토스트 메시지 표시, 자동 소멸
- **검색/필터/정렬 공통 패턴**
  - Stack List: 이름 검색 + 상태 필터 + 정렬
  - CI/CD List: 이름 검색 + 상태 필터
  - CI/CD Template: 키워드 검색
  - CI/CD History: 타입 필터 + 상태 필터
  - Developer Template: 카테고리 필터 (Frontend / Backend / Full-stack)
- **반응형**: 데스크톱(1200px+), 태블릿(768-1199px), 모바일(767px 이하)
- **역할 기반 UX** (proto3): DevOps Engineer / Developer 역할 전환 (사이드바 역할 전환기로 즉시 전환)

### 4.3 프로토타입 진화 이력

| 버전 | 주요 변경 |
|---|---|
| proto1 | 기본 6탭 워크플로우, 8단계 파이프라인 바, 다크 테마, 통화 선택 |
| proto2 | 오픈소스 스택 수 자유 조정, 투입 리소스 조절 기능 추가 |
| proto3 | 6→5탭 단순화, Cluster Management 독립 페이지, 역할 기반 UX 분리, K8s 오브젝트 프리뷰 |

---

## 5. 기술 스택

| 계층 | 기술 | 선택 이유 |
|---|---|---|
| Frontend | React 19 + TypeScript | 생태계 최대, Backstage 전환 가능 |
| 상태 관리 | Zustand | 경량, 보일러플레이트 최소 |
| 스타일링 | Tailwind CSS + shadcn/ui | 다크 테마, 빠른 UI 개발 |
| YAML 에디터 | Monaco Editor (v1) | VS Code 동일 엔진, YAML 스키마 검증 |
| Backend | Go 1.24+ | K8s 클라이언트 네이티브, 단일 바이너리 |
| 웹 프레임워크 | Echo v4 | 경량, 고성능 |
| 실시간 통신 | WebSocket (gorilla/websocket) | 설치 로그 양방향 스트리밍 |
| Database | PostgreSQL 18+ | 확장성, JSONB 지원, pgvector 활용 가능 |
| 인증 (Alpha~Beta) | 세션 기반 (gorilla/sessions) | 빠른 구현, 단순 |
| 인증 (v1) | Keycloak OIDC | SSO, RBAC, OSS 권한 매핑 |
| 설치 엔진 | Helm Go SDK + client-go | K8s 네이티브, Helm 차트 프로그래밍 제어 |
| CI/CD | GitHub Actions | 자체 빌드/테스트/릴리스 |
| API 문서 | OpenAPI 3.0 (swaggo/swag) | Go 구조체 자동 생성 |

---

## 6. 비기능 요구사항

| 분류 | 요구사항 | 기준 |
|---|---|---|
| 성능 | 파이프라인 설치 시간 | < 2시간 (8 vCPU, 16GB RAM 기준) |
| 성능 | 웹 UI 응답 시간 | < 500ms (페이지 로드), < 100ms (탭 전환) |
| 성능 | 배포 시작 시간 | Deploy 클릭 후 < 10초 내 배포 시작 |
| 보안 | Kubeconfig | AES-256 암호화 저장, 메모리에서 복호화 |
| 보안 | 민감 정보 | Kubernetes Secret 저장 |
| 보안 | RBAC | ServiceAccount 최소 권한 원칙, Namespace 격리 |
| 보안 | 이미지 스캔 | Trivy 스캔 완료, CVE 24시간 내 패치 |
| 확장성 | 동시 설치 | 최대 10개 파이프라인 동시 설치 |
| 확장성 | 팀 규모 | 최대 500명 개발자 (Phase 1) |
| 신뢰성 | 가동시간 | 웹 UI 99.0% (베타 기간) |
| 신뢰성 | 자동 롤백 | 설치 실패 시 이전 상태로 자동 복구 |
| 유지보수 | 테스트 커버리지 | >70% |
| 유지보수 | 코드 품질 | ESLint, Prettier, golangci-lint 준수 |
| 접근성 | 웹 UI | WCAG 2.1 AA 부분 준수 |
| 호환성 | Kubernetes | 1.26+ (최소), 1.28+ (권장) |
| 호환성 | 브라우저 | Chrome, Firefox, Safari, Edge (최신 2버전) |

---

## 7. Phase 1 명시적 제외 사항

| 제외 항목 | 이유 | 향후 계획 |
|---|---|---|
| 멀티 클러스터 관리 | 초기 범위 축소 | Phase 3 (v1.0) |
| 클러스터 프로비저닝 | Terraform/OpenTofu → Phase 3 | Phase 3 (v1.0) |
| Security/Test 자동화 | SAST, DAST, E2E | Phase 2 (v0.5) |
| AI 기반 자동 최적화 | 안정성 우선 | v1.0+ |
| GUI 관리 콘솔 | 각 도구 UI 사용 (Grafana, Argo CD 등) | Backstage 통합 검토 |
| CLI 도구 | `nullus` CLI (init/validate/deploy/status/delete) | Phase 2 (v0.5) |
| 설정 내보내기/불러오기 | Export/Import 모두 Phase 2 | Phase 2 (v0.5) |
| 배포 스크립트 미리보기 | Deploy Script Preview 및 Dry-run 실행 | Phase 2 (v0.5) |
| Multi Cloud 환경 | 복잡도 관리 | Phase 2 (v0.5) |
| 상용 도구 통합 | 오픈소스만 지원 (벤더 중립성) | 영구 제외 |

---

## 부록

### A. 관련 문서 목록

| 문서 | 경로 | 설명 |
|---|---|---|
| PRD v1.2 | `기획단계/아키텍처/nullus_PRD_1.2.md` | 최신 제품 요구사항 정의서 |
| PRD v1.1 | `기획단계/아키텍처/nullus_PRD_1.1.md` | 이전 제품 요구사항 정의서 |
| PRD v1.0 | `기획단계/아키텍처/nullus_PRD_1.0.md` | 초기 제품 요구사항 정의서 |
| PMF 정의 | `기획단계/아키텍처/nullus_PMF.md` | Product-Market Fit 정의 및 달성 조건 |
| 상세 기능 명세 | `기획단계/아키텍처/개발계획/Nullus 상세 기능 명세 및 시스템 아키텍처.md` | API, 데이터 모델, 아키텍처 상세 |
| 개발 마스터 플랜 | `기획단계/아키텍처/개발계획/Nullus 개발 마스터 플랜.md` | 12주 + 피드백 루프 개발 계획 |
| 경쟁 분석 | `기획단계/아키텍처/개발계획/Nullus 플랫폼 경쟁 분석 및 PRD 검토 보고서.md` | 시장 경쟁 환경 분석 |
| Day 0 체크리스트 | `기획단계/아키텍처/개발계획/Nullus Day 0 프로젝트 착수 체크리스트.md` | 프로젝트 착수 준비 작업 |
| 화면설계 요구사항 | `기획단계/아키텍처/화면설계/ui/needs.md` | UI/UX 기획 요구사항 |
| 프로토타입 proto1 | `기획단계/아키텍처/화면설계/proto1/` | 초기 프로토타입 (6탭, 8단계 바) |
| 프로토타입 proto2 | `기획단계/아키텍처/화면설계/proto2/` | 개선 프로토타입 (리소스 조절) |
| 프로토타입 proto3 | `기획단계/아키텍처/화면설계/proto3/` | 최신 프로토타입 (5탭, 역할 기반 UX) |
| Narwhal 레포지토리 분석 | `기획단계/아키텍처/개발계획/Narwhal 레포지토리 분석.md` | Narwhal(dasomel/narwhal) 오픈소스 분석 |
| Narwhal 분석 기반 Nullus 적용 항목 | `기획단계/아키텍처/개발계획/Narwhal 분석 기반 Nullus 적용 항목.md` | Narwhal 패턴의 Nullus 적용 가능 항목 정리 |

### B. 용어집

| 용어 | 설명 |
|---|---|
| Golden Path | 검증된 베스트 프랙티스 기반의 도구 조합 템플릿 |
| DevSecOps Stack | Nullus를 통해 구축한 DevSecOps 도구 조합 |
| Install Engine | 스택 설정을 받아 K8s 클러스터에 도구를 설치하는 핵심 엔진 |
| Compatibility Matrix | 테스트 완료된 도구 버전 조합 정의 파일 |
| Phase | 개발 단계 (Phase 1: DevOps, Phase 2: DevSecOps, Phase 3: InfraOps) |
| IDP | Internal Developer Platform, 내부 개발자 플랫폼 |
| CNCF | Cloud Native Computing Foundation |
