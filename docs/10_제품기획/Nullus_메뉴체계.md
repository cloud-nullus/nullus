# Nullus 메뉴 체계

**작성일**: 2026-03-08  
**용도**: proto3, 기능목록, **기능분해도(Nullus_기능분해도.csv)** 간 메뉴 명칭 통일

---

## 1. 통일된 메뉴 구조

### 1.1 사이드바 메뉴 (최상위 → 하위)

| 대메뉴 | 하위 메뉴 | 메뉴 ID (data-page) | 설명 | 기능분해도 중분류 |
|--------|-----------|---------------------|------|-------------------|
| **데브섹옵스 스택** | 스택 템플릿 | templates | Golden Path 템플릿 선택 | Golden Path 템플릿 |
| | 스택 설치 | install | 5단계 설정 워크플로우 + Deploy | 노코드 설정 UI, 스택 생성/배포, 리소스 예상량 계산 |
| | 스택 목록 | list | 구성된 스택 목록 (검색/필터/정렬) | 스택 목록 관리 |
| | 스택 이력 | history | 스택 변경 이력 + diff + 롤백 | 스택 이력 관리 |
| | 스택 버전 관리 | compatibility | OSS 호환성 매트릭스 | OSS 버전 호환성 |
| **CI/CD** | CI/CD 템플릿 | cicdtemplates | 파이프라인 템플릿 목록 | 파이프라인 템플릿 |
| | CI/CD 목록 | cicdlist | 생성된 파이프라인 목록 | 파이프라인 관리 |
| | CI/CD 이력 | cicdhistory | 파이프라인 배포 이력 | 파이프라인 배포 |
| **관측성** | 모니터링 대시보드 | monitoring | Cluster/Pipeline/Tool Health | 모니터링 |
| | 알림 규칙 | alertlist | 알림 규칙 목록 | 알림 관리 |
| | 알림 이력 | alerthistory | 알림 발생 이력 | 알림 관리 |
| **관리** | 조직 | organization | 조직 정보 등록/수정 | 조직 관리 |
| | 사용자 관리 | users | 역할 부여/비활성화 | 사용자 관리 |
| | 클러스터 관리 | clusters | 클러스터 등록/수정/상태 | 클러스터 관리 |
| **사용자** | 로그아웃 | — | 로그아웃 | 인증 |

### 1.2 역할 전환 시 추가 화면

| 화면 | 설명 | 표시 조건 | 기능분해도 |
|------|------|-----------|------------|
| 앱 배포 | Developer Self-Service 배포 위자드 | Developer 역할 선택 시 | Developer Self-Service (CIC_040) |

### 1.3 공통 화면 (메뉴 외)

| 화면 | 설명 | 기능분해도 |
|------|------|------------|
| 홈 | 역할별 요약 대시보드 | USR 홈/대시보드 |
| 로그인 | 세션/Keycloak OIDC | USR 인증 |
| 다국어 | UI 언어 전환 (en/ko) | USR 다국어 (NULLUS_USR_030_010) |

---

## 2. 명칭 통일 규칙

### 2.1 대메뉴

| 통일 명칭 | 이전/혼용 표현 | 기능분해도 대분류 코드 |
|-----------|----------------|------------------------|
| 데브섹옵스 스택 | DevSecOps Stack | DSS |
| CI/CD | — | CIC |
| 관측성 | Observability | OBS |
| 관리 | Admin | ADM |
| 사용자 | User | USR |

### 2.2 하위 메뉴

| 통일 명칭 | 이전/혼용 표현 |
|-----------|----------------|
| 스택 템플릿 | DevSecOps Stack Template |
| 스택 설치 | DevSecOps Stack Install, Install DevSecOps |
| 스택 목록 | DevSecOps Stack List |
| 스택 이력 | DevSecOps Stack History |
| 스택 버전 관리 | DevSecOps Stack Version Management |
| CI/CD 템플릿 | CI/CD Template |
| CI/CD 목록 | CI/CD List |
| CI/CD 이력 | CI/CD History |
| 모니터링 대시보드 | Monitoring Dashboard |
| 알림 규칙 | Alert Rule List |
| 알림 이력 | Alert History |
| 조직 | Organization |
| 사용자 관리 | User Management |
| 클러스터 관리 | Cluster Management |
| 로그아웃 | Log out |

---

## 3. 기능분해도 매핑

기능분해도(Nullus_기능분해도.csv)의 대분류/중분류와 메뉴/화면 매핑

| 대분류 | 대분류 코드 | 중분류 | 연결 메뉴/화면 | 대표 기능 ID |
|--------|-------------|--------|----------------|--------------|
| 조직 | ORG | 시스템 최초 설정 | — (백엔드) | NULLUS_ORG_000_xxx |
| | | Organization 관리 | 관리 > 조직 | NULLUS_ORG_010_xxx |
| | | 클러스터 접근 범위 | 관리 > 조직 (하위) | NULLUS_ORG_020_xxx |
| | | 멤버 관리 | 관리 > 조직/사용자 관리 | NULLUS_ORG_030_xxx |
| 클러스터 | CLU | 클러스터 관리 | 관리 > 클러스터 관리 | NULLUS_CLU_010_xxx |
| | | Kubeconfig 관리 | 관리 > 클러스터 관리 (등록/수정 시) | NULLUS_CLU_020_xxx |
| | | 클러스터 메타정보 | 관리 > 클러스터 관리 | NULLUS_CLU_030_xxx |
| | | 클러스터 선택 | 데브섹옵스 스택 > 스택 설치 | NULLUS_CLU_040_010 |
| | | Organization 접근 | 관리 > 클러스터 관리 (하위) | NULLUS_CLU_040_020/030 |
| DevSecOps 스택 | DSS | Golden Path 템플릿 | 데브섹옵스 스택 > 스택 템플릿 | NULLUS_DSS_010_xxx |
| | | 노코드 설정 UI | 데브섹옵스 스택 > 스택 설치 | NULLUS_DSS_020_xxx |
| | | 스택 생성/배포 | 데브섹옵스 스택 > 스택 설치 | NULLUS_DSS_030_xxx |
| | | 스택 목록 관리 | 데브섹옵스 스택 > 스택 목록 | NULLUS_DSS_040_xxx |
| | | 스택 이력 관리 | 데브섹옵스 스택 > 스택 이력 | NULLUS_DSS_050_xxx |
| | | OSS 버전 호환성 | 데브섹옵스 스택 > 스택 버전 관리 | NULLUS_DSS_060_xxx |
| | | 리소스 예상량 계산 | 데브섹옵스 스택 > 스택 설치 (Resources 탭) | NULLUS_DSS_070_xxx |
| CI/CD | CIC | 파이프라인 템플릿 | CI/CD > CI/CD 템플릿 | NULLUS_CIC_010_xxx |
| | | 파이프라인 관리 | CI/CD > CI/CD 목록 | NULLUS_CIC_020_xxx |
| | | 파이프라인 배포 | CI/CD > CI/CD 이력 | NULLUS_CIC_030_xxx |
| | | Developer Self-Service | 앱 배포 (역할 전환 시) | NULLUS_CIC_040_xxx |
| 관측성 | OBS | 모니터링 | 관측성 > 모니터링 대시보드 | NULLUS_OBS_010_xxx |
| | | 알림 관리 | 관측성 > 알림 규칙, 알림 이력 | NULLUS_OBS_020_xxx |
| 관리 | ADM | 조직 관리 | 관리 > 조직 | NULLUS_ADM_010_xxx |
| | | 사용자 관리 | 관리 > 사용자 관리 | NULLUS_ADM_020_xxx |
| | | 클러스터 관리 | 관리 > 클러스터 관리 | NULLUS_ADM_030_xxx |
| 사용자 | USR | 홈/대시보드 | 홈 | NULLUS_USR_010_xxx |
| | | 인증 | 로그인, 로그아웃 | NULLUS_USR_020_xxx |
| | | 다국어 | UI 언어 전환 (en/ko) | NULLUS_USR_030_010 |
