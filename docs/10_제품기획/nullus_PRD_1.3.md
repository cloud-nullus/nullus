# Nullus Platform PRD

**작성자**: Nullus 팀
**작성일**: 2026-02-24
**버전**: 1.3
**상태**: 진행중
**마지막 업데이트**: 2026-03-14
---

## 변경 이력

| 버전 | 날짜 | 변경 내용 | 작성자 |
| --- | --- | --- | --- |
| 1.0 | 2026-02-11 | 초안 작성 | Nullus 팀 |
| 1.1 | 2026-02-24 | 페르소나 세분화, Phase1 기능 요구사항 수정 | Nullus 팀 |
| 1.2 | 2026-03-08 | Narwhal(dasomel/narwhal) 레퍼런스 분석 기반 설치 엔진, 호환성, SSO 요구사항 보강 | Nullus 팀 |
| 1.3 | 2026-03-14 | proto4 반영 (3역할 체계, 한글 메뉴 통일, i18n), 기술 스택 확정, 마일스톤 상태 갱신, UI/UX 구현계획·디자인시스템 문서 연동 | Nullus 팀 |
---

## 독자 가이드

- **임원/의사결정자**: 섹션 1, 2 읽기 (10분)
- **엔지니어**: 섹션 4, 5, 7, 8 집중 (30분)
- **디자이너**: 섹션 3, 4 집중 (20분)
- **DevOps Engineer**: 섹션 3, 4, 5, 6 필독 (40분)
- **전체**: 모든 섹션 (60분)

---

## 1. 개요 (Executive Summary)

### 한 문장 요약

새로운 프로젝트를 시작하는 DevOps Engineer가 검증된 CI/CD 베스트 프랙티스 조합(Golden Path)을 선택하고, 즉시 Kubernetes 기반 DevSecOps 파이프라인을 구축할 수 있도록 노코드 UI와 자동 설치 기능을 제공하는 오픈소스 플랫폼이다.

### 문제 정의

현재 플랫폼 엔지니어와 DevOps 팀들은 다음과 같은 문제를 겪고 있다:

- **높은 구축 비용**: 10-30개의 오픈소스 도구를 수동으로 통합하여 플랫폼을 구축하는 데 6-18개월 소요
- **버전 호환성 문제**: 각 도구의 버전업마다 연동 테스트 필요, 예측 불가능한 호환성 이슈 (KT 클라우드 OKD 사례, 삼성 SDS SPCC 실패)
- **표준화 부재**: 각 팀/조직마다 다른 도구 조합 사용, 베스트 프랙티스 부재
- **문서화 부족**: 커스텀 통합은 문서화되지 않아 유지보수 어려움
- **반복적 작업**: 프로젝트마다 유사한 파이프라인을 처음부터 구축

### 솔루션 개요

Nullus는 다음을 제공한다:

1. **검증된 Golden Path**: 프로덕션 검증된 CI/CD 도구 조합 템플릿 제공
    - GitHub/GitLab + GitHub Actions/GitLab CI → Argo CD → Prometheus + Grafana 등
2. **노코드 설정**: 웹 UI에서 체크박스/드롭다운 방식으로 파이프라인 구성
3. **자동 설치**: "한 줄 명령어"로 Kubernetes 클러스터에 전체 스택 자동 배포
4. **버전 호환성 보장**: 테스트 완료된 도구 버전 조합만 제공
5. **커스터마이징 지원**: 필요 시 특정 도구나 설정 변경 가능

### 기대 효과

| 지표 | 현재 | Nullus 적용 후 |
| --- | --- | --- |
| **플랫폼 구축 시간** | 6-18개월 | 며칠 (설정) + 1-2시간 (설치) |
| **필요 인력** | 5-10명 (플랫폼 팀) | 1-2명 (DevOps Engineer) |
| **버전 업그레이드 리스크** | 높음 (호환성 미검증) | 낮음 (테스트 완료된 조합) |
| **유지보수 부담** | 높음 (커스텀 통합) | 낮음 (표준화된 구성) |

---

## 2. 배경 및 목표

### 배경

### 2.1 시장 상황

- **플랫폼 엔지니어링 부상**: 토스, 카카오, 현대자동차 등 국내 대기업도 IDP(Internal Developer Platform) 구축 중
- **오픈소스 폭발**: CNCF 200개 이상 프로젝트, 선택의 어려움 증가
- **DevOps 성숙도 격차**: 대기업은 전담 플랫폼 팀 보유, 중소기업은 리소스 부족

### 2.2 왜 지금 Nullus인가?

- **경제적 압박**: 클라우드 비용 증가 → FinOps 중요성 증대
- **속도 경쟁**: "빠른 배포 = 시장 경쟁력" → DevOps 자동화 필수
- **AI 시대**: AI 코드 생성 시대 대비, 표준화된 플랫폼 필요
- **지식 주권**: 해외 유료 솔루션(GitHub Enterprise, GitLab Ultimate 등) 의존도 감소

### 2.3 Nullus 프로젝트의 비전

- **오픈소스 코어 프로젝트**: 클라우드브로 커뮤니티의 중심 프로젝트 (Hugging Face의 Transformers와 같은 역할)
- **CNCF 제출 목표**: 글로벌 커뮤니티로 확장
- **벤더 중립성**: 특정 클라우드/벤더에 종속되지 않음

### 비즈니스 목표

**Objective 1: 플랫폼 구축 시간 단축**

- Key Result 1.1: 플랫폼 구축 시간 평균 90% 감소 (설문 데이터)
- Key Result 1.2: 90% 이상의 사용자가 1시간 이내 첫 파이프라인 설치 완료

**Objective 2: 오픈소스 커뮤니티 성장**

- Key Result 2.1: GitHub Stars 1,000+ (6개월)
- Key Result 2.2: 기여자 50명 이상 (6개월)
- Key Result 2.3: 3개 이상 프로덕션 배포 검증 (6개월)

**Objective 3: 글로벌 인지도 확보**

- Key Result 3.1: CNCF Sandbox 제출 (12개월)
- Key Result 3.2: 해외 컨퍼런스 발표 2회 이상 (KubeCon, AWS re:Invent 등)

### 사용자 니즈

- **빠른 시작**: "오늘 프로젝트 시작해서 내일 첫 배포하고 싶다"
- **검증된 조합**: "어떤 도구를 선택해야 할지 모르겠다"
- **자동화**: "매번 수동 설정하는 게 번거롭다"
- **유연성**: "우리 회사 환경에 맞게 일부는 바꾸고 싶다"
- **비용 가시성**: "플랫폼 운영 비용이 얼마나 드는지 모르겠다"

### 성공 지표

| 지표 | 현재 (Baseline) | 3개월 목표 | 6개월 목표 |
| --- | --- | --- | --- |
| **GitHub Stars** | 0 | 100 | 1,000 |
| **주간 활성 설치** | 0 | 10 | 50 |
| **설치 성공률** | - | >85% | >90% |
| **첫 가치 도달 시간** | - | <2시간 | <1시간 |
| **NPS** | - | >30 | >40 |
| **웹사이트 방문자** | 0 | 1,000/월 | 5,000/월 |

---

## 3. 타겟 사용자

### 주요 페르소나1: Junior DevOps Engineer "미정"

**기본 정보**:

- 역할: DevOps Engineer / Platform Engineer
- 회사 규모: 50-500명 개발자 (중견 기업)
- 경험: Kubernetes 1년차, CI/CD 도구 경험 없음

**목표와 동기**:

- 새 프로젝트의 CI/CD 파이프라인을 빠르게 구축하고 싶다
- 검증된 베스트 프랙티스를 적용하고 싶다
- 반복 작업을 자동화하여 핵심 업무에 집중하고 싶다
- 팀에 표준화된 플랫폼을 제공하고 싶다

**고통 포인트**:

- **시간 부족**: "플랫폼 구축에만 몇 달을 쓸 수 없다"
- **선택의 어려움**: "GitLab vs GitHub Actions? Prometheus vs Thanos? 어떤 게 더 나을까?"
- **문서 부족**: "인터넷에 파편적인 정보만 있고, 전체를 아우르는 가이드가 없다"

**의사결정 기준**:

- **설치 용이성**: 1시간 안에 설치 가능한가?
- **검증 수준**: 프로덕션 사용 사례가 있는가?
- **커뮤니티**: 활발한 커뮤니티가 있는가? 이슈 해결이 빠른가?
- **비용**: 오픈소스인가? 숨은 비용은 없는가?

**시나리오 1: 신규 프로젝트 시작**:
```
미정이는 회사에서 새로운 마이크로서비스 프로젝트를 맡았다.
Kubernetes 클러스터는 이미 있지만, CI/CD 파이프라인은 없다.
미정이는 Nullus를 설치해서
1. "Golden Path" 템플릿 중 "GitHub + Argo CD + Prometheus" 선택
2. 팀 규모(20명), 예상 커밋 수(주 100회) 입력 → 필요 리소스 자동 계산
3. Kubernetes 클러스터 정보 입력
4. "Deploy" 버튼 클릭 → 1시간 후 전체 파이프라인 설치 완료
5. 다음 날부터 개발팀이 CI/CD 사용 시작
```

### 주요 페르소나2: Senior DevOps Engineer "민수"

**기본 정보**:

- 역할: DevOps Engineer
- 회사 규모: 4천명 개발자 (대기업)
- 경험: Kubernetes 5년차, CI/CD 도구 경험 많음

**목표와 동기**:

- 기존 프로젝트의 DevSecOps Stack을 중앙에서 제대로 관리하고 싶다
- 반복 작업을 자동화하여 핵심 업무에 집중하고 싶다
- 팀에 표준화된 플랫폼을 제공하고 싶다

**고통 포인트**:

- **시간 부족**: "플랫폼 구축에만 몇 달을 쓸 수 없다"
- **버전 지옥**: "Argo CD 업그레이드했더니 GitLab 연동이 안 된다"
- **반복 작업**: "매 프로젝트마다 똑같은 설정을 반복하고 있다"

**의사결정 기준**:

- **검증 수준**: 프로덕션 사용 사례가 있는가?
- **커스터마이징**: 우리 환경에 맞게 수정 가능한가?
- **커뮤니티**: 활발한 커뮤니티가 있는가? 이슈 해결이 빠른가?
- **비용**: 오픈소스인가? 숨은 비용은 없는가?

**시나리오 1: 기존 도구 일부 교체**:
```
민수의 팀은 이미 GitLab을 사용 중이지만, 모니터링 스택이 없다.
Nullus에서:
1. "Monitoring Only" 템플릿 선택
2. Prometheus + Grafana 조합 선택
3. GitLab과 연동 설정
4. 기존 클러스터에 추가 설치
→ 모니터링 스택만 추가로 구축 완료
```

**시나리오 2: 커스터마이징**:
```
민수의 회사는 보안 정책상 GitLab 대신 자체 Git 서버를 사용해야 한다.
Nullus에서:
1. 기본 템플릿 선택 후 "Customize" 모드 활성화
2. Source Repository를 "Custom Git Server"로 변경
3. 연동 설정 입력 (엔드포인트, 인증 정보)
4. 나머지 도구는 기본값 유지
5. 배포 → 커스터마이징된 파이프라인 구축 완료
```

### 부차 페르소나: Developer "지은"

**기본 정보**:

- 역할: Developer
- 목표: 빠르게 개발 환경 세팅하고 코드에만 집중하고 싶다
- 경험: 개발자 3년차

**시나리오 1: 기존 도구 일부 교체**:

- DevOps Engineer가 구축한 파이프라인을 그대로 사용
- Git push만 하면 자동 빌드/배포되는 환경
- Grafana 대시보드에서 애플리케이션 상태 모니터링


### 안티 페르소나 (이 제품을 위한 사용자가 아닌 사람)

**1. 개인 개발자 (1인)**

- 이유: 팀 플랫폼이 필요 없음, Vercel/Netlify 같은 PaaS로 충분
- 대안: GitHub Actions 단독 사용

**2. 소규모 스타트업 (<5명)**

- 이유: 플랫폼 운영 리소스 없음
- 대안: Heroku, AWS Amplify 등 Managed PaaS
- 추후 Phase에서 다룰 수 있을 듯

**3. Kubernetes 미사용 조직**

- 이유: Nullus는 Kubernetes 기반 (Phase 1 범위)
- 대안: Jenkins, CircleCI 등 VM 기반 CI/CD

---

## 4. 기능 요구사항

### Phase 구조

Nullus는 3개 Phase로 나뉘어 개발된다. 본 PRD는 **Phase 1**을 중점적으로 다룬다.

| Phase | 범위 | 목표 일정 |
| --- | --- | --- |
| **Phase 1** | DevOps (CI/CD + Monitoring) | 2026 Q2 (v1.0) |
| **Phase 2** | DevSecOps (Security + Test) | 2026 Q3-Q4 |
| **Phase 3** | InfraOps (Kubernetes 구축) | 2027+ (v1.0) |

---

### Phase 1: DevOps (v1.0 - 본 PRD 범위)

**전제 조건**: Kubernetes 클러스터는 이미 구축되어 있음 (kubeconfig로 연결)

**기능 우선순위 요약**

| 기능 | 예상 공수(병렬 진행) |
| --- | --- |
| 기능 0: Organization 설정 등록 | 2주 |
| 기능 1: K8S Cluster Configurations 등록 | 1주 |
| 기능 2: 노코드 기반 Nullus DevSecOps Stack 설정 UI | 3주 |
| 기능 3: DevSecOps Stack Golden Path 템플릿 제공 | 1주 |
| 기능 4: DevSecOps Stack 자동 설치/배포/이력 관리 | 8주 |
| 기능 5: CI/CD Pipeline 템플릿 제공 | 1주 |
| 기능 6: CI/CD Pipeline 배포/이력 관리 | 3주 |
| 기능 7: 모니터링 Pipeline 배포/이력 관리 | 4주 |
| 기능 8: DevSecOps Stack OSS 버전 호환성 관리 | 1주 |
| 기능 9: UI 권한 체계 | 3주 |
| 기능 10: DevSecOps Stack 필요 Resource 예상량 계산 | 2주 |
|  | **12주 (약 3개월, Active Engineer 6명)** |

---
### 기능 0: Organization 설정 등록

**사용자 대상**: Admin

**사용자 스토리**: Nullus를 팀 단위로 사용하기 위해, Organization을 생성하고 기본 정보를 등록하고 싶다.

**수용 기준**:

- [ ]  Organization 이름/슬러그/도메인 등록
- [ ]  기본 관리자 계정 지정 및 변경
- [ ]  멤버 초대(초대 링크) 및 기본 역할 부여
- [ ]  Organization 활성/비활성 상태 관리
- [ ]  Organization 단위 클러스터 접근 범위 설정

---

### 기능 1: K8S Cluster Configurations 등록

**사용자 대상**: DevOps Engineer

**사용자 스토리**: 클러스터 설정을 등록하고 관리하고 싶다.

**수용 기준**:

- [ ]  파이프라인 클러스터/타겟 클러스터를 분리 등록
- [ ]  Kubeconfig 업로드 및 유효성 검증
- [ ]  Kubeconfig 서버 DB 저장 (AES-256-GCM 암호화)
- [ ]  클러스터 이름, 네임스페이스, 엔드포인트, 인증 방식 저장
- [ ]  연결 상태 표시 (연결됨/대기/미설정)
- [ ]  등록된 클러스터를 파이프라인 설정 단계에서 선택 가능
- [ ]  클러스터 접근 가능 Organization 설정

---

### 기능 2: 노코드 기반 Nullus DevSecOps Stack 설정 UI

**사용자 대상**: DevOps Engineer

**사용자 스토리**: Nullus DevSecOps Stack을 설정하기 위해, 웹 UI에서 노코드 방식으로 도구를 선택하고 싶다. 전문가는 YAML을 수정할 수 있다.

**수용 기준**:

- [ ]  **5단계 설정 워크플로우 제공**:
    1. **Artifacts**: Package Registry, Source Repository, Container Registry, Storage Backend 선택
    2. **Pipeline Tools**: CI/CD 플랫폼, CD 도구 선택
    3. **Monitoring Tools**: 수집/조회 도구 선택
    4. **Logging Tools**: 수집/조회 도구 선택
    5. **Resources**: 팀 규모/워크로드 입력 → 필요 리소스 자동 계산
- [ ]  **각 단계별 상세 요구사항**:
    - **Artifacts 탭**:
        - Package Registry: GitLab(기본), Nexus, JFrog Artifactory, Harbor
        - Source Repository: GitLab(기본), GitHub, Gitea
        - Container Registry: GitLab Container Registry(기본), Harbor, Docker Hub
        - Storage Backend: MinIO(기본), AWS S3, GCS
        - 각 도구별 버전 선택 드롭다운 제공
    - **Pipeline Tools 탭**:
        - CI/CD Platform: GitLab CI(기본), GitHub Actions, Jenkins
        - CD Tool: Argo CD(기본), Flux
        - 단일 선택 (라디오 버튼)
    - **Monitoring Tools 탭**:
        - Collection: Prometheus(기본), Thanos
        - Query & Visualization: Grafana(기본)
        - 각 도구별 버전 선택
    - **Logging Tools 탭**:
        - Collection: OpenTelemetry(기본), Loki
        - Query & Search: OpenSearch(기본), Elasticsearch
        - 각 도구별 버전 선택
    - **Resources 탭**:
        - 입력 항목: 개발자 수, 동시 러너 수, 커밋 수, 빌드 빈도
        - 자동 계산: CPU (cores), Memory (Gi), Storage (Gi), 예상 월 비용
        - 통화 선택: USD, KRW, CNY
- [ ]  **파이프라인 8단계 시각화**:
    - Develop → Build → Security → Test → Deploy → Operation → Monitoring → FinOps
    - 세팅된 단계는 색상/글로우로 강조, 미세팅 단계는 흐림 처리
- [ ]  **실시간 Configuration Summary**:
    - 오른쪽 패널에 선택한 모든 도구와 버전 표시
    - 탭/선택 변경 시 즉시 갱신
- [ ]  **초보자용**: 체크박스/드롭다운 기반 설정 UI 제공
- [ ]  **전문가용**: YAML 에디터로 전체 설정 편집 가능
    - [ ]  YAML 문법/스키마 검증 및 오류 표시
- [ ]  모드 전환 시 설정 값 동기화 (UI → YAML, YAML → UI)


**와이어프레임**: proto4 (최신, 3역할 지원, i18n, 기능분해도 검증 완료)

---

### 기능 3: DevSecOps Stack Golden Path 템플릿 제공

**사용자 대상**: DevOps Engineer

**사용자 스토리**: 검증된 CI/CD 도구 조합을 빠르게 선택하기 위해, 사전 정의된 Golden Path 템플릿을 제공받고 싶다.

**수용 기준**:

- [ ]  최소 3개 이상의 DevSecOps Golden Path 템플릿 제공
    - Template 1: GitHub + GitHub Actions + Argo CD + Prometheus + Grafana
    - Template 2: GitLab + GitLab CI + Argo CD + Prometheus + Grafana
    - Template 3: GitLab All-in-One + Prometheus + Grafana (외부 구성)
        - 모니터링 도구가 GitLab 내장이 아닌 별도 Helm 차트로 설치됨을 명시
- [ ]  각 템플릿은 다음 정보 포함:
    - 포함된 도구 목록 (버전 명시)
    - 예상 설치 시간
    - 권장 사용 사례
    - 필요 리소스 (CPU/Memory/Storage)
- [ ]  템플릿 선택 후 즉시 다음 단계(커스터마이징 또는 설치)로 진행 가능

**와이어프레임**: 참고 - 기획단계/아키텍처/화면설계/proto2

---

### 기능 4: DevSecOps Stack 자동 설치/배포/이력 관리

**사용자 대상**: DevOps Engineer

**사용자 스토리**: 복잡한 수동 설정 없이 파이프라인을 자동으로 설치하기 위해, "Deploy" 버튼 한 번으로 전체 스택이 배포되기를 원한다.

**수용 기준**:

- [ ]  웹 UI "Deploy Pipeline" 버튼
    - 전제 조건: 클러스터 설정 완료 시에만 활성화
    - 클릭 시 백그라운드 배포 시작
    - 진행률 실시간 표시 (프로그레스 바 + 로그 스트리밍)
- [ ]  설치 순서 자동화:
    1. 스토리지 백엔드 (MinIO 등) 설치
    2. 레지스트리 (GitLab, Harbor 등) 설치
    3. CI/CD 플랫폼 설치
    4. CD 도구 (Argo CD) 설치
    5. 모니터링 스택 설치
    6. 로깅 스택 설치
    7. 연동 설정 자동 구성
- [ ]  설치 실패 시 자동 롤백

#### 롤백 전략
| 단계 | 모드 | 설명 |
|------|------|------|
| Alpha | FULL | 전체 롤백만 지원 (설치된 모든 컴포넌트 제거 후 재설치) |
| Beta | FULL + RETRY | 전체 롤백 + 실패 단계 재시도 |
| v1.0 | FULL + PARTIAL + RETRY | 부분 롤백 지원 (실패 컴포넌트만 선택적 롤백) |

- 롤백 시 PVC(Persistent Volume Claim)는 기본적으로 보존 (safe 모드)
- 명시적 확인 후에만 PVC 삭제 가능 (destructive 모드)

- [ ]  설치 완료 후 헬스체크 자동 실행
- [ ]  설치 시간 < 2시간 (default 템플릿 기준, Kubernetes 클러스터 사양에 따라 변동)
- [ ]  이력 관리:
    - [ ]  설정 변경 시 버전 스냅샷 자동 저장
    - [ ]  버전별 변경자, 변경 시간, 변경 이유 기록
    - [ ]  이전 버전과의 diff 표시 (git diff 스타일)
    - [ ]  특정 버전으로 롤백 가능
- [ ] **Narwhal 레퍼런스 기반 설치 엔진 강화**:
    - 3-Phase 프로비저닝: Phase A (기반 인프라: Storage, DB, cert-manager) → Phase B (플랫폼 앱) → Phase C (연동: OIDC, Webhook, ServiceMonitor)
    - Phase 간 게이트 검증 (이전 Phase 완료 확인 후 다음 Phase 진행)
    - `known-issues.yaml`: Narwhal CLAUDE.md의 70+ Helm edge case 패턴 코드화
    - CRD 크기 초과(262KB) 시 자동 `--server-side --force-conflicts` 전환
    - 비핵심 앱 `--wait` 제거, `--timeout`만 사용
    - 노드 아키텍처 감지 → ARM64 대체 이미지 자동 선택
    - 레지스트리 우선순위: `ghcr.io > registry.k8s.io > quay.io > docker.io`
    - Post-Install 헬스체크: Narwhal verify-cluster.sh (120+ 항목) 패턴 참고

---

### 기능 5: CI/CD Pipeline 템플릿 제공

**사용자 대상**: DevOps Engineer

**사용자 스토리**: 반복되는 애플리케이션 배포를 빠르게 시작하기 위해, 표준 CI/CD 파이프라인 템플릿을 선택하고 적용하고 싶다.

**수용 기준**:

- [ ]  최소 3개 이상의 파이프라인 템플릿 제공 (Web/Backend/Batch 등)
- [ ]  템플릿별 포함 단계/도구/변수 안내
- [ ]  템플릿 선택 후 배포 단계로 바로 이동 가능
- [ ]  템플릿 파라미터 입력 폼 제공 (Repo, 이미지명, 환경 변수 등)
- [ ]  템플릿 버전 관리 및 변경 이력 표시

---

### 기능 6: CI/CD Pipeline 배포/이력 관리

**사용자 대상**: Developer

**사용자 스토리**: 애플리케이션 파이프라인을 빠르게 배포하기 위해, 필요한 Kubernetes 오브젝트가 자동 생성되고 배포 이력이 관리되기를 원한다.

**수용 기준**:

- [ ]  파이프라인 배포 시 필수 Kubernetes Object 자동 생성
    - Namespace, Deployment, Service, Ingress/Gateway, Secret, PV/PVC
- [ ]  파이프라인 배포 기록(버전, 배포 시간, 결과) 저장
- [ ]  배포 실패 시 이전 버전으로 롤백 지원
- [ ]  배포 이력 조회 및 상태 필터링 제공
- [ ]  버전별 변경자, 변경 시간, 변경 이유 기록
- [ ]  이전 버전과의 diff 표시 (git diff 스타일)
- [ ]  특정 버전으로 롤백 가능

---

### 기능 7: 모니터링/알림 관리

**사용자 대상**: DevOps Engineer, Developer

**사용자 스토리**: 파이프라인과 애플리케이션 상태를 한눈에 확인하기 위해, 기본 모니터링 기능을 제공받고 싶다.

**수용 기준**:

- [ ]  기본 대시보드 제공 (클러스터/파이프라인/애플리케이션)
- [ ]  핵심 지표 수집 (CPU, Memory, Storage, 파이프라인 성공률)
- [ ]  알림 연동 기본값 제공 (Slack/Email 중 1개 이상)

---

### 기능 8: DevSecOps Stack OSS 버전 호환성 관리

**사용자 대상**: DevOps Engineer

**사용자 스토리**: 도구 간 호환성 문제를 피하기 위해, 테스트 완료된 버전 조합만 선택하고 싶다.

**수용 기준**:

- [ ]  **호환성 매트릭스 관리**:
    - `templates/compatibility/compatibility-matrix.yaml` 파일에 테스트 완료된 버전 조합 정의
    - 예: GitLab 16.7 + Argo CD 2.9 + Prometheus 2.48 = ✅ 검증 완료
- [ ] **Chart 버전 / App 버전 분리 관리**:
    - `templates/compatibility/compatibility-matrix.yaml`에 `helm_version`과 `app_version` 필드 분리
    - 예: Traefik v39.0.0 (chart) / v3.6.7 (app)
    - Narwhal VERSIONS.md의 실제 버전 매핑을 초기 매트릭스 시드 데이터로 활용

---

### 기능 9: UI 권한 체계

**사용자 대상**: Platform Engineer, Developer Admin

**사용자 스토리**: Admin으로서, 팀별로 기능 접근을 통제하기 위해, 역할 기반 권한을 관리하고 싶다.

**수용 기준**:

- [ ]  Role 기반 접근 제어 (Admin/DevOps Engineer/Developer 3역할 제공)
- [ ]  대메뉴 단위 접근 권한 설정 (대메뉴 - 사용자 관리 포함)
- [ ]  사용자 관리 화면 제공 (역할 부여, 비활성화)
- [ ]  OSS별 권한 매핑 지원 (Keycloak 활용)

**역할별 메뉴 가시성 (proto4 확정)**:

| 역할 | 표시 메뉴 | 초기 화면 |
|------|-----------|-----------|
| Admin | 관리(조직, 사용자 관리, 클러스터 관리), 사용자(로그아웃) | 조직 페이지 |
| DevOps Engineer | 데브섹옵스 스택, CI/CD, 관측성, 관리, 사용자 (전체) | 스택 설치 페이지 |
| Developer | CI/CD, 관측성, 사용자 (Admin/DevSecOps 스택 숨김) | CI/CD 템플릿 페이지 |
- [ ] **Keycloak OIDC 자동 설정** (Narwhal 레퍼런스):
    - Keycloak realm 생성 → groups client scope 생성 → 앱별 클라이언트 생성 → OIDC 설정 → K8s API Server OIDC 연동
    - 알려진 SSO 이슈 사전 처리: `groups` scope 자동 생성, self-signed cert 처리, CA cert 마운트
    - Narwhal `11-keycloak.sh` 스크립트를 Go 코드 전환의 구현 명세로 활용

---

### 기능 10: DevSecOps Stack 필요 Resource 예상량 계산

**사용자 대상**: DevOps Engineer

**사용자 스토리**: 설치 전 인프라 요구사항을 파악하기 위해, 예상 리소스 계산 결과를 보고 싶다.

**수용 기준**:

- [ ]  입력 항목: 개발자 수, 동시 러너 수, 커밋 수, 빌드 빈도
- [ ]  자동 계산: CPU (cores), Memory (Gi), Storage (Gi), 예상 월 비용

---

### Phase 2로 넘길 기능 요구사항

- **기존 Legacy DevOps Stack 등록**
- **API 권한**: API 레벨 권한 및 토큰 정책 정의
- **CLI 도구 제공**: `nullus` CLI (init/validate/deploy/status/delete)
- **설정 내보내기/불러오기**: `nullus-config.yaml` 기반 Export/Import
- **배포 스크립트 미리보기**: 실제 실행될 스크립트 표시 및 Dry-run
- **Multi Cloud 환경 제공**
- **OSS 버전 호환성 관리**:
- [ ]  **호환되지 않는 조합 선택 시 경고**:
    - UI에 경고 메시지 표시: "이 조합은 테스트되지 않았습니다. 계속하시겠습니까?"
    - 강제 진행 가능하지만 위험 경고
- [ ]  **권장 버전 자동 선택**:
    - 도구 선택 시 호환되는 최신 버전 자동 선택
    - "Latest (Recommended)" 레이블 표시
- [ ]  **호환성 테스트 자동화** (개발팀 내부):
    - 새 버전 출시 시 자동으로 조합 테스트
    - CI 파이프라인에서 호환성 검증

---

## 5. 비기능 요구사항

### 5.1 성능

- **파이프라인 설치 시간**: < 2시간 (기본 템플릿, 클러스터 사양: 8 vCPU, 16GB RAM 기준)
- **웹 UI 응답 시간**: < 500ms (페이지 로드), < 100ms (탭 전환)
- **배포 시작 시간**: Deploy 버튼 클릭 후 < 10초 내 배포 시작, 전체 설치는 백그라운드
- **설치 성공률**: ≥90% (v1 GA 기준) — Narwhal의 70+ Helm edge case 패턴을 `known-issues.yaml`로 코드화하여 달성

### 5.2 보안

- **Kubeconfig 보안**:
    - Kubeconfig는 서버 측 DB에 AES-256-GCM으로 암호화 저장
    - 전송 시 TLS 1.3 필수
    - 접근 권한은 프로젝트 Owner/Admin만 허용
    - 민감 정보(API 토큰, OIDC client secret, webhook secret, DB credential 등)는 **OpenBao를 1차 저장소(Source of Truth)** 로 저장
    - Kubernetes Secret에는 원문 비밀값을 직접 저장하지 않고, OpenBao 연계 주입 결과(단기 캐시/참조용)만 허용
    - 배포 순서는 OpenBao 선배포를 원칙으로 하며, 이후 모든 OSS 연동 토큰은 OpenBao 경유로 주입
- **RBAC**:
    - Nullus가 생성하는 ServiceAccount는 최소 권한 원칙
    - Namespace 격리 지원
- **취약점 스캔**:
    - 제공하는 컨테이너 이미지는 Trivy 스캔 완료
    - CVE 발견 시 24시간 이내 패치

#### OpenBao-first 시크릿 관리 원칙 (신규)

- Phase A(기반 인프라)에서 OpenBao를 먼저 배포하고 health check를 완료해야 Phase B/C를 진행한다.
- OIDC/SCM/Registry/Alert(Webhook) 관련 credential은 OpenBao path 정책으로 관리한다.
- 애플리케이션은 정적 토큰 하드코딩을 금지하며, Kubernetes auth 기반 short-lived token 사용을 기본으로 한다.
- 비밀값 회전(rotate)은 운영 표준 절차로 정의하고, 회전 후 무중단 재기동 전략을 함께 제공한다.

### 5.3 확장성

- **멀티 클러스터 지원**: Phase 3에서 추가 (현재 단일 클러스터만 지원)
- **동시 설치**: 최대 10개 파이프라인 동시 설치 지원 (CLI + 웹 UI 합산)
- **팀 규모**: 최대 500명 개발자 지원 (Phase 1 기준)

### 5.4 신뢰성

- **가동시간**: 웹 UI 99.0% (커뮤니티 베타 기간)
- **자동 롤백**: 설치 실패 시 이전 상태로 자동 복구
- **백업**: 설정 파일 Git 백업 권장 (사용자 책임)

### 5.5 유지보수성

- **코드 품질**:
    - 테스트 커버리지 >70%
    - 린터 규칙 준수 (ESLint, Prettier, golangci-lint 등)
- **문서화**:
    - API 문서 (OpenAPI 3.0)
    - 개발자 가이드 ([CONTRIBUTING.md](http://contributing.md/))
    - 사용자 가이드 (docs/ 디렉토리)

### 5.6 접근성

- **웹 UI**: WCAG 2.1 AA 부분 준수 (키보드 네비게이션, 색상 대비 4.5:1 이상)
- **다국어**: Phase 1에서 영어/한국어(en/ko) 동시 지원, localStorage 영속화 (proto4에서 i18n 구현 완료)

### 5.7 호환성

- **Kubernetes 버전**: 1.26+ (최소), 1.28+ (권장)
- **브라우저**: Chrome, Firefox, Safari, Edge (최신 2개 버전)
- **OS** (CLI): Linux (Ubuntu 22.04+, RHEL 8+), macOS 12+, Windows 10+
- **UI 반응형**: Phase 1은 데스크톱 브라우저(1280px 이상)를 대상으로 하며, 모바일/태블릿 반응형 UI는 Phase 2 범위로 검토합니다.

### 5.8 Nullus 플랫폼 배포 방법

- **Alpha/Beta**: Docker Compose (단일 노드, 개발/테스트용)
- **v1.0 GA**: Helm Chart (프로덕션 K8s 클러스터 배포)
- **최소 요구사항**: K8s 1.27+, PostgreSQL 18+, 2 vCPU / 4GB RAM
- **설치 명령어 예시**:
  ```bash
  helm repo add nullus https://charts.nullus.io
  helm install nullus nullus/nullus-platform
  ```

---

## 6. 명시적 제외 사항

### Phase 1에서 제외 (Phase 2-3에서 재검토)

- ❌ **멀티 클러스터 관리**: 단일 클러스터에 집중 (Phase 3에서 추가)
    - 이유: 초기 범위 축소, 복잡도 관리
    - 향후 계획: Phase 2에서 재검토
- ❌ **클러스터 프로비저닝**: 기존 Kubernetes 클러스터 가정
    - 이유: Terraform/OpenTofu로 클러스터 생성은 Phase 3 범위
    - 향후 계획: v1.0에서 IaC 통합
- ❌ **Security/Test 자동화**: DevSecOps의 일부만 지원 (Phase 2에서 추가)
    - Phase 1: Develop, Build, Deploy, Monitoring만 지원
    - Phase 2: Security (SAST, DAST), Test (Unit, E2E) 추가
    - 이유: 범위 축소로 빠른 출시 우선
- ❌ **AI 기반 자동 최적화**: 결정론적 워크플로우 우선
    - 이유: AI는 보조 도구로만 고려, 안정성 우선
    - 향후 계획: v1.0+ 보조 기능으로 검토 (예: 리소스 최적화 추천)
- ❌ **GUI 관리 콘솔**: 웹 UI는 "설정 + 설치"에만 집중
    - 이유: 배포 후 관리는 Grafana, Argo CD 등 각 도구의 UI 사용
    - 향후 계획: Backstage 통합으로 대체 검토 (v1.0+)
- ❌ **Windows 네이티브 지원** (Kubernetes): WSL2로 우회 사용
    - 이유: Kubernetes 생태계가 Linux 중심
    - CLI는 Windows 지원, 하지만 클러스터는 Linux 기반
- ❌ **Database 자동 프로비저닝**: 사용자가 직접 DB 관리
    - 이유: PostgreSQL, MySQL 등은 사용자 책임
    - Nullus는 파이프라인만 관리

### 영구 제외

- ❌ **상용 도구 통합**: 오픈소스만 지원 (GitHub Enterprise, GitLab Ultimate 등 제외)
    - 이유: 벤더 중립성, 오픈소스 철학

---

## 7. 타임라인 및 마일스톤

| 마일스톤 | 날짜 | 주요 산출물 | 상태 |
| --- | --- | --- | --- |
| **M0: PRD 완료** | 2026-02-28 | PRD v1.0~v1.2, 기술 스택 확정 (React/Go/PostgreSQL) | ✅ 완료 |
| **M1: 설계 완료** | 2026-03-08 | 아키텍처 문서, API 설계, DB 스키마, 기능분해도, 메뉴체계 | ✅ 완료 |
| **M2: UI 프로토타입** | 2026-03-14 | proto4 완성 (3역할, 15페이지, i18n, 기능분해도 검증), UI/UX 구현계획·디자인시스템 문서 | ✅ 완료 |
| **M3: 호환성 매트릭스** | 2026-03-22 | 3개 Golden Path 템플릿, 호환성 테스트 완료 | ⚪ 예정 |
| **M4: 자동 설치 로직** | 2026-03-29 | Helm 기반 자동 설치 | ⚪ 예정 |
| **M5: Alpha 릴리스** | 2026-03-30 | Alpha, 내부 테스트 (클라우드브로 커뮤니티) | ⚪ 예정 |
| **M6: Beta 릴리스** | 2026-04-27 | Beta, 공개 베타 테스트 | ⚪ 예정 |
| **M7: v1.0 GA** | 2026-05-25 | v1.0 정식 릴리스, 3+ 프로덕션 배포 검증 | ⚪ 예정 |

---

## 8. 종속성 및 위험

### 8.1 기술적 종속성

| 종속성 | 설명 | 완화 전략 |
| --- | --- | --- |
| **Kubernetes 1.26+** | Nullus는 Kubernetes 필수 | 최소 버전 1.26 고정, 호환성 테스트 자동화 |
| **Helm 3.0+** | 자동 설치에 Helm 사용 | Helm 미설치 시 자동으로 설치 스크립트 제공 |
| **Gateway API (선택)** | Ingress 대신 Gateway API 권장 | Ingress 폴백 지원 (기본값은 Ingress) |

### 8.2 팀 종속성

| 팀/조직 | 필요한 작업 | 일정 | 완화 전략 |
| --- | --- | --- | --- |
| **클라우드브로 커뮤니티** | 베타 테스트 참여 | M6 (2026-04) | 얼리 어답터 모집 캠페인 |
| **CNCF** | Sandbox 신청 검토 | M8 이후 | 글로벌 전문가 리뷰 먼저 (파코 등) |
| **외부 기여자** | 도구 플러그인 개발 | M8 이후 | 명확한 기여 가이드 제공 |

### 8.3 위험 관리

| 위험 | 가능성 | 영향 | 완화 전략 |
| --- | --- | --- | --- |
| **Kubernetes API 변경** | 중 | 높음 | 최소 K8s 버전 고정, 호환성 테스트 자동화, K8s 릴리스 노트 모니터링 |
| **커뮤니티 채택 실패** | 중 | 치명적 | 베타 프로그램, 클라우드브로 커뮤니티 활용, 컨퍼런스 발표 |
| **팀 리소스 부족** | 높음 | 높음 | 범위 축소 (Phase 분할), 외부 기여자 모집, 파트타임 기여 허용 |
| **버전 호환성 테스트 부담** | 중 | 중 | CI 자동화, 제한적 버전만 지원 (최신 2-3개 버전), 커뮤니티 피드백 활용 |
| **경쟁 프로젝트 (Kratix 등)** | 낮음 | 중 | 차별화 포인트 강조 (노코드 UI, 한국어 지원, 클라우드브로 커뮤니티), 오픈소스 철학 |
| **도구 라이선스 변경** | 낮음 | 높음 | CNCF 프로젝트 우선 사용, Apache 2.0/MIT 라이선스 확인, 대체 도구 미리 파악 |
| **Helm Edge Case 누적** | 높음 | 높음 | `known-issues.yaml` 패턴 DB 구축, Narwhal CLAUDE.md 70+ 패턴 초기 시드, CI 자동 테스트 |
| **ARM64 호환성** | 중 | 중 | 설치 전 노드 아키텍처 감지, 대체 이미지 자동 선택 (Harbor 등 ARM64 미지원 도구) |
| **Bitnami 이미지 상용화** | 중 | 중 | 레지스트리 우선순위 정책 (`ghcr.io > registry.k8s.io > quay.io > docker.io`), 대체 이미지 사전 매핑 |
| **Docker Hub Rate Limit** | 높음 | 중 | 레지스트리 우선순위 정책으로 Docker Hub 의존도 최소화, 프록시 캐시 레지스트리 권장 |

---

## 9. 오픈 이슈 및 의사결정 필요 사항

### 해결된 이슈 (v1.3에서 확정)

- [x] **이슈 1: 웹 UI 프레임워크** → **React 19 + TypeScript** 확정
    - 근거: 생태계 최대, shadcn/ui + Tailwind CSS 4 조합, Backstage 전환 가능
    - 상세: `docs/40_UI_UX/Nullus_UI_UX_구현계획.md` 참조
- [x] **이슈 2: CLI 개발 언어** → **Go** 확정
    - 근거: Kubernetes 클라이언트 라이브러리 네이티브, 단일 바이너리 배포
- [x] **이슈 3: 백엔드 데이터베이스** → **PostgreSQL 18+** 확정
    - 근거: 확장성, JSONB 지원, pgvector 활용 가능
- [x] **이슈 4: 역할 체계** → **Admin / DevOps Engineer / Developer** 3역할 확정
    - 근거: proto4에서 검증 완료, 기능분해도와 1:1 매핑, PRD v1.2의 Admin/Operator/Viewer에서 변경

### 해결된 이슈 (Narwhal 레퍼런스 기반)

- [x] **레지스트리 정책**: `ghcr.io > registry.k8s.io > quay.io > docker.io` 우선순위 채택 (Bitnami 상용화, Docker Hub rate limit 대응)
- [x] **설치 순서 DAG**: Narwhal의 실전 검증된 의존성 그래프를 레퍼런스 구현으로 활용 (CNPG → MetalLB → Traefik → cert-manager → ... → Keycloak → ArgoCD)
- [x] **SSO 통합 접근법**: Narwhal의 7-app Keycloak OIDC 연동 패턴을 기능 9 구현 명세로 채택
- [x] **Helm 배포 전략**: 대형 CRD는 `--server-side --force-conflicts`, 비핵심 앱은 `--wait` 제거

### 미해결 질문

- [ ]  **Q1: Backstage 통합 vs 독립 실행?**
    - 현재: 독립 웹 앱으로 개발
    - 향후: Backstage 플러그인으로 전환 가능한가?
    - 검토 일정: Phase 2
- [ ]  **Q2: 멀티 테넌시 지원?**
    - 여러 팀이 하나의 Nullus 인스턴스를 공유할 수 있는가?
    - Phase 1: 단일 팀만 지원
    - Phase 3에서 재검토
- [ ]  **Q3: FinOps 도구 선택?**
    - Kubecost vs OpenCost
    - Phase 1에는 리소스 계산만 제공
    - Phase 2에서 FinOps 통합 검토

---

## 10. 성공 지표 및 출시 계획

### 10.1 성공 지표 (AARRR)

### Acquisition (획득)

- **GitHub Stars**: 1,000+ (6개월)
- **웹사이트 방문자**: 5,000/월
- **Docker Hub Pulls**: 10,000+ (6개월)
- **YouTube 튜토리얼 조회수**: 5,000+ (3개월)

### Activation (활성화)

- **설치 성공률**: >90%
- **첫 가치 도달 시간**: <1시간 (파이프라인 설치 완료)
- **튜토리얼 완료율**: >70% (사용자가 문서 따라하기)

### Retention (유지)

- **v1.0 → v1.1 업그레이드율**: >80%
- **주간 활성 설치**: 50+
- **월간 활성 사용자**: 200+ (CLI + 웹 UI)

### Referral (추천)

- **NPS**: >40
- **오가닉 가입**: >50% (검색 유입, 입소문)
- **컨퍼런스 발표**: 2회 이상 (KCD Korea, AWS Summit 등)

### Revenue (수익)

- **오픈소스 스폰서 펀딩**: $50K (6개월)
    - GitHub Sponsors, Open Collective
- **기업 후원**: 1개 이상 (삼성전자, NHN, 카카오 등)

### 10.2 출시 계획

### Alpha (Alpha)

- **날짜**: 2026-03-30
- **대상**: 클라우드브로 커뮤니티 (초대제)
- **목표**:
    - 기본 기능 검증
    - 버그 발견 및 수정
    - 사용자 피드백 수집
- **배포 채널**: GitHub Release (pre-release), Discord 공지

### Beta (Beta)

- **날짜**: 2026-04-27
- **대상**: 공개 베타 (누구나 참여)
- **목표**:
    - 프로덕션 환경 테스트 (3개 이상 조직)
    - 성능 튜닝
    - 문서화 완성
- **배포 채널**:
    - GitHub Release
    - Docker Hub
    - 공식 웹사이트 (docs)
    - 블로그 포스트 (한국어 + 영어)

### GA (v1.0)

- **날짜**: 2026-05-25
- **대상**: 전체 사용자 (프로덕션 Ready)
- **목표**:
    - Stable 버전 제공
    - 3개 이상 프로덕션 배포 검증 완료
    - CNCF Sandbox 신청 준비
- **배포 채널**:
    - GitHub Release (Latest)
    - Docker Hub (latest tag)
    - Homebrew Tap
    - 공식 블로그 릴리스 노트
    - 컨퍼런스 발표 (KCD Korea 2026)

### 10.3 마케팅 계획

**Phase 1: 인지도 확보 (M0-M4)**

- 블로그 시리즈: "DevOps 플랫폼 구축기" (한국어)
- YouTube 튜토리얼: "Nullus로 10분 만에 CI/CD 구축"
- 클라우드브로 커뮤니티 발표

**Phase 2: 커뮤니티 확장 (M5-M8)**

- KCD Korea 2026 발표
- AWS Summit Korea 발표
- CNCF 블로그 기고 (영어)
- 해외 개발자 유입 (Reddit, Hacker News)

**Phase 3: 글로벌 진출 (M8+)**

- KubeCon North America 2027 제출
- CNCF Sandbox 신청
- 파트너십 (CNCF 멤버사, 클라우드 벤더)

---
## 11. Action Item

### 회의

- 2025.12.21: 킥오프 미팅 (프로젝트 비전, 핵심 가치)
- 2026.01.04: 아키텍처 설계 논의 (기술 스택, 데이터베이스)
- 2026.01.29: Phase 정의 (DevOps → DevSecOps → InfraOps)
- 2026.02.03: 방향성 논의
- 2026.02.10: 방향성 논의 (정규민님 합류)
- 2026.02.12: 오프라인 미팅, 방향성 논의, 기술스택 결정
- 2026.02.20: 페르소나 정의 
- 2026.02.24: Phase 1 Must-have Item 정의 (고동환님, 장미영님 합류) 
- 2026.03.03(예정): DevOps Best Practice 정의, UI Prototype 검토 

### 2월

- [v]  GitHub Organization 설정 완료 (cloud-nullus)
- [ ]  PRD 리뷰
- [ ]  Golden Path 템플릿 정의 (GitLab All-in-One)
- [ ]  M1 마일스톤 구체화 (아키텍처 문서 템플릿)

### 3월

- [x]  Phase1 PRD 완성 (v1.3, 2026-03-14)
- [x]  UI 프로토타입 완성 (proto4, 15페이지, 3역할, i18n)
- [x]  기술 스택 결정 (React 19, Go, PostgreSQL)
- [x]  아키텍처 하이레벨 설계 완료 (상세 기능 명세 및 시스템 아키텍처 문서)
- [x]  UI/UX 구현계획 및 디자인시스템 문서 작성 완료
- [ ]  역할 분담 (FE, BE, DevOps, ...)
- [ ]  M3~M4 완료 및 Alpha 릴리스 (3/30)

### 4월

- [ ]  Beta 릴리스 (4/27)
- [ ]  사용자 피드백 반영 및 안정화

### 5월

- [ ]  v1.0 GA 릴리스 (5/25)
- [ ]  프로덕션 배포 검증 (3+ 사례)

### 6월

- [ ]  CNCF Sandbox 신청 준비 및 커뮤니티 확장

--

## 부록

### A. 용어집

- **Golden Path**: 검증된 베스트 프랙티스 기반의 도구 조합 템플릿
- **Phase**: Nullus 프로젝트의 개발 단계 (Phase 1: DevOps, Phase 2: DevSecOps, Phase 3: InfraOps)
- **Nullus DevSecOps Stack**: Nullus를 통해 구축한 DevSecOps Stack

### B. 참고 자료

### 벤치마킹

- R2Devops: GitLab 기반 CI/CD 관리 플랫폼
- Plumber: CLI 기반 CI/CD 자동화 도구
- OpsFlow: AWS 배포 플랫폼
- Kratix: Kubernetes 기반 플랫폼 조율 프레임워크

### 기술 문서

- Kubernetes Gateway API: [https://gateway-api.sigs.k8s.io/](https://gateway-api.sigs.k8s.io/)
- Argo CD: [https://argo-cd.readthedocs.io/](https://argo-cd.readthedocs.io/)
- CNCF Landscape: [https://landscape.cncf.io/](https://landscape.cncf.io/)

### Narwhal 레퍼런스
- Narwhal GitHub: [https://github.com/dasomel/narwhal](https://github.com/dasomel/narwhal) — Vagrant 기반 K8s IDP 프로비저닝 도구
- Narwhal 레포지토리 분석: `기획단계/아키텍처/개발계획/Narwhal 레포지토리 분석.md`
- Narwhal 분석 기반 Nullus 적용 항목: `기획단계/아키텍처/개발계획/Narwhal 분석 기반 Nullus 적용 항목.md`

### C. 프로토타입

- **웹 UI 프로토타입 (최신)**: `기획단계/아키텍처/화면설계/proto4/`
    - 3역할 체계 (Admin / DevOps Engineer / Developer)
    - 15개 페이지 JS 모듈 (pages/ 디렉토리)
    - 5단계 설정 워크플로우 (Artifacts → Pipeline → Monitoring → Logging → Resources)
    - 한글 메뉴 통일 (Nullus_메뉴체계.md 기준)
    - i18n 다국어 지원 (en/ko, localStorage 영속화)
    - 기능분해도 검증 완료 (약 60개 기능 ID 반영, proto4_기능분해도_검증.md)
    - 다크/라이트 테마, 사이드바 접기/펼치기
    - Deploy Script Preview, K8s Object Preview 모달
- **이전 프로토타입 이력**:
    - proto1: 기본 6탭 워크플로우, 8단계 파이프라인 바, 다크 테마
    - proto2: 오픈소스 스택 수 자유 조정, 투입 리소스 조절 기능
    - proto3: 6→5탭 단순화, Cluster Management 독립 페이지, 2역할 기반 UX

### D. UI/UX 문서

- **UI/UX 구현계획**: `docs/40_UI_UX/Nullus_UI_UX_구현계획.md`
    - React 19 + TypeScript + Tailwind CSS 4 + shadcn/ui 기술 스택
    - 19개 페이지별 구현 계획 (5 Phase, 10주)
    - 공통 컴포넌트 라이브러리 (12개 컴포넌트)
    - 프로젝트 디렉토리 구조
- **디자인시스템**: `docs/40_UI_UX/Nullus_디자인시스템.md`
    - Dark Mode (OLED) + Data-Dense Dashboard 스타일
    - 컬러 팔레트 (다크/라이트), 타이포그래피 (Inter/Pretendard/Fira Code)
    - 레이아웃 토큰, 컴포넌트 스펙, Font Awesome→Lucide 아이콘 매핑
    - 반응형 브레이크포인트, 애니메이션 가이드, 접근성 체크리스트
