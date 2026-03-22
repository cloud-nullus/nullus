# Nullus PR/커밋 컨벤션 가이드

Nullus 프로젝트의 PR과 커밋 메시지 작성 규칙을 정의합니다. 일관된 컨벤션은 코드 리뷰를 효율화하고 변경 이력을 명확하게 유지합니다.

---

## 1. 브랜치 전략

### 브랜치 명명 규칙

```
main (프로덕션)
  ├── feat/<module>/<description>      # 기능 추가
  ├── fix/<module>/<description>       # 버그 수정
  ├── refactor/<module>/<description>  # 리팩터링 (기능 변경 없음)
  ├── test/<module>/<description>      # 테스트 추가/수정
  ├── docs/<description>               # 문서 작업
  └── chore/<description>              # 빌드, 설정 변경
```

### 예시

```
feat/stack/add-tools-wizard
fix/auth/token-refresh-loop
refactor/cicd/pipeline-state-machine
test/admin/cluster-registration-integration
docs/api-design-update
chore/upgrade-dependencies
```

### 규칙

- 모듈명은 소문자, 하이픈 구분 (예: `stack`, `cicd`, `admin`, `auth`, `observability`)
- 설명은 명령형 현재형 (예: `add`, `fix`, `update`, `refactor`)
- 브랜치는 작업 완료 후 머지하고 삭제

---

## 2. 커밋 메시지 형식

### 기본 형식

```
<type>(<module>): <description>

[선택] 본문
[선택] 바닥글
```

### 타입 (Type)

| 타입 | 설명 | 예시 |
|------|------|------|
| `feat` | 새 기능 추가 | `feat(stack): 스택 설치 워크플로우 5단계 구현` |
| `fix` | 버그 수정 | `fix(cicd): 파이프라인 배포 롤백 시 PVC 보존` |
| `refactor` | 리팩터링 (기능 변경 없음) | `refactor(stack): Install Engine 상태 머신 정리` |
| `test` | 테스트 추가/수정 | `test(admin): 클러스터 등록 통합 테스트 추가` |
| `docs` | 문서 추가/수정 | `docs(product): 역할별 시나리오 문서 갱신` |
| `chore` | 빌드, 설정, 의존성 변경 | `chore: Go 1.24 업그레이드` |
| `perf` | 성능 개선 | `perf(stack): Helm 설치 병렬화로 30% 단축` |
| `ci` | CI/CD 설정 변경 | `ci: GitHub Actions 워크플로우 추가` |

### 모듈 (Module)

| 모듈 | 설명 |
|------|------|
| `stack` | DevSecOps 스택 설치/관리 (Helm SDK) |
| `cicd` | CI/CD 파이프라인 템플릿/배포 |
| `admin` | 조직/사용자/클러스터 관리 |
| `auth` | 인증/권한 (Keycloak OIDC, JWT, RBAC) |
| `observability` | 모니터링/알림 (Prometheus, Alert) |
| `product` | 제품 기획 문서 (PRD, 시나리오) |
| `ui` | 프론트엔드 UI/UX (공통 컴포넌트, 스타일) |

**모듈이 없는 경우** (예: 문서, 설정, 의존성):
```
docs: PRD v1.3 업데이트
chore: 의존성 업그레이드
ci: GitHub Actions 워크플로우 추가
```

### 설명 (Description)

- **50자 이내** (GitHub 제목 표시 기준)
- 명령형 현재형 사용 (예: "추가한다" ❌ → "추가" ✅)
- 마침표 없음
- 한글 또는 영문 일관되게 사용

### 본문 (Body) — 선택

- 첫 줄 이후 빈 줄 추가
- 변경 이유와 영향을 설명
- 72자 이내로 줄 바꿈
- "무엇을" 보다 "왜"에 집중

**예시:**
```
feat(stack): 스택 설치 워크플로우 5단계 구현

사용자가 웹 UI에서 노코드로 DevSecOps 스택을 설정할 수 있도록
5단계 Wizard를 구현합니다:
1. 클러스터 선택
2. Golden Path 템플릿 선택
3. 도구 버전 선택
4. 리소스 예상량 확인
5. 배포 실행

이를 통해 설치 프로세스를 단순화하고 사용자 경험을 개선합니다.
```

### 바닥글 (Footer) — 선택

이슈 참조, Breaking Changes 등을 기록합니다.

```
Closes #123
Refs #456
BREAKING CHANGE: API 응답 형식 변경
```

---

## 3. 실제 커밋 메시지 예시

### 기능 추가

```
feat(stack): 스택 설치 워크플로우 5단계 구현

사용자가 웹 UI에서 노코드로 DevSecOps 스택을 설정할 수 있도록
5단계 Wizard를 구현합니다.
```

```
feat(cicd): CI/CD 파이프라인 템플릿 추가

GitHub Actions, GitLab CI, Jenkins 3개 템플릿을 추가하고
각 템플릿별 설정 폼을 구현합니다.
```

### 버그 수정

```
fix(cicd): 파이프라인 배포 롤백 시 PVC 보존

배포 실패 시 롤백 중에 PVC가 삭제되는 버그를 수정합니다.
Helm rollback 전에 PVC를 별도로 보존하도록 변경합니다.
```

```
fix(auth): JWT 토큰 갱신 루프 해결

토큰 갱신 중 무한 루프가 발생하는 버그를 수정합니다.
갱신 시간을 토큰 만료 시간 5분 전으로 조정합니다.
```

### 리팩터링

```
refactor(stack): Install Engine 상태 머신 정리

Install Engine의 상태 전이 로직을 명확하게 정리합니다.
상태 머신 패턴을 적용하여 유지보수성을 개선합니다.
```

### 테스트

```
test(admin): 클러스터 등록 통합 테스트 추가

클러스터 등록 및 검증 전체 흐름을 테스트합니다.
testcontainers를 사용하여 실제 PostgreSQL에 연동합니다.
```

### 문서

```
docs(product): 역할별 사용 시나리오 문서 갱신

Admin, DevOps, Developer 역할별 주요 사용 시나리오를
구체적인 단계와 함께 문서화합니다.
```

```
docs: API 설계 문서 v1.2 업데이트

새로운 엔드포인트 추가 및 응답 형식 변경을 반영합니다.
```

### UI/UX

```
feat(ui): 스택 상세 패널에 배포 로그 추가

스택 목록에서 행을 클릭하면 우측 패널에 배포 로그를
실시간으로 표시합니다.
```

```
refactor(ui): 검색 박스 스타일 통일

모든 목록 페이지의 검색 박스를 동일한 스타일로 통일합니다.
```

---

## 4. PR (Pull Request) 작성 가이드

### PR 제목

커밋 메시지와 동일한 형식을 사용하거나, 더 간결한 요약형을 사용합니다.

```
feat(stack): 스택 설치 워크플로우 5단계 구현
```

또는

```
[feat/stack] 스택 설치 워크플로우 5단계 구현
```

### PR 본문 구조

```markdown
## Summary
- 무엇을 변경했는가? (1-3줄)
- 왜 변경했는가?

## Changes
- 변경 사항 1 (파일/모듈 단위)
- 변경 사항 2
- ...

## Related Issues
- Closes #123
- Refs #456

## Testing
- 테스트 방법 설명
- 테스트 결과 (스크린샷, 로그 등)

## Checklist
- [ ] 테스트 추가/수정됨
- [ ] 문서 업데이트됨
- [ ] 타입 체크 통과 (TypeScript) / 린트 통과 (Go)
- [ ] 코드 리뷰 준비 완료
```

### PR 본문 예시

```markdown
## Summary
사용자가 웹 UI에서 노코드로 DevSecOps 스택을 설정할 수 있도록
5단계 Wizard를 구현합니다. 이를 통해 설치 프로세스를 단순화합니다.

## Changes
- `web/src/features/stack/components/StackWizard.tsx` — 5단계 Wizard 컴포넌트
- `web/src/features/stack/hooks/useInstallStack.ts` — 스택 설치 로직
- `internal/stack/usecase/install_stack.go` — 백엔드 설치 로직
- `internal/stack/adapter/handler/stack_handler.go` — HTTP 핸들러

## Related Issues
- Closes #42 (스택 설치 워크플로우)

## Testing
- 로컬 개발 서버에서 5단계 Wizard 동작 확인
- 각 단계별 유효성 검사 확인
- 배포 실행 후 Helm 설치 확인

## Checklist
- [x] 테스트 추가됨 (useInstallStack.test.ts)
- [x] 문서 업데이트됨 (API 설계)
- [x] 타입 체크 통과
- [x] 코드 리뷰 준비 완료
```

### PR 크기 가이드

| 크기 | 파일 변경 | 권장사항 |
|------|---------|---------|
| 소 (S) | 1-3개 | 빠른 리뷰, 즉시 머지 가능 |
| 중 (M) | 4-10개 | 표준 리뷰 시간 |
| 대 (L) | 10+개 | 가능하면 분리 권장 |

**대형 PR 분리 전략:**
- 기능을 여러 PR로 나누기 (예: 백엔드 → 프론트엔드)
- 리팩터링과 기능 추가 분리
- 문서와 코드 분리

---

## 5. 코드 리뷰 기준

리뷰어는 다음 항목을 확인합니다:

### 아키텍처

- [ ] Clean Architecture 레이어 위반 없는가?
  - Domain 레이어에 프레임워크 import 없음
  - UseCase는 Repository 인터페이스에만 의존
  - Handler는 UseCase를 호출
  
- [ ] 모듈 간 직접 의존 없는가?
  - 다른 모듈의 `internal/` 패키지 import 없음
  - 공유 타입은 `internal/shared/` 또는 `pkg/`에 위치
  
- [ ] 도메인 용어가 일관되게 사용되는가?
  - PRD/기능분해도의 용어를 코드에 반영
  - 변수명, 함수명이 도메인 언어를 따름

### 테스트

- [ ] 테스트가 함께 포함되어 있는가?
  - 새 기능: 단위 테스트 + 통합 테스트
  - 버그 수정: 버그를 재현하는 테스트
  
- [ ] 테스트가 의미 있는가?
  - 단순 통과만 하는 테스트 아님
  - 엣지 케이스 포함
  
- [ ] 테스트 커버리지가 충분한가?
  - Domain: 100% 목표
  - UseCase: 핵심 시나리오 커버

### 코드 품질

- [ ] 타입 안정성이 확보되었는가?
  - TypeScript: `any`, `@ts-ignore` 없음
  - Go: 에러 처리 누락 없음
  
- [ ] 코드가 읽기 쉬운가?
  - 함수/변수명이 명확
  - 복잡한 로직에 주석 있음
  
- [ ] 중복 코드가 없는가?
  - 공통 로직 추출
  - 유틸리티 함수 재사용

### 문서

- [ ] 문서가 업데이트되었는가?
  - API 변경: API 설계 문서 업데이트
  - 새 기능: README 또는 가이드 문서 추가
  - 주요 변경: CHANGELOG 업데이트

---

## 6. 머지 전략

### 머지 방식

**기본: Squash and Merge**

```
PR의 모든 커밋을 하나의 커밋으로 압축하여 main에 머지합니다.
```

**이유:**
- main 브랜치의 커밋 이력을 깔끔하게 유지
- 각 PR이 하나의 논리적 단위로 표현
- 롤백이 간단함

### 머지 커밋 메시지

Squash and Merge 시 커밋 메시지는 **PR 제목**을 사용합니다.

```
feat(stack): 스택 설치 워크플로우 5단계 구현
```

본문은 PR 본문의 Summary를 간략히 포함할 수 있습니다.

### 머지 전 체크리스트

- [ ] 모든 테스트 통과
- [ ] 코드 리뷰 승인 (최소 1명)
- [ ] CI/CD 파이프라인 성공
- [ ] 충돌 해결됨
- [ ] 커밋 메시지 형식 확인

---

## 7. 개발 워크플로우 예시

### 기능 추가 (feat)

```bash
# 1. 브랜치 생성
git checkout -b feat/stack/add-tools-wizard

# 2. 테스트 작성 (TDD)
# web/src/features/stack/hooks/useInstallStack.test.ts 작성

# 3. 구현
# web/src/features/stack/hooks/useInstallStack.ts 구현
# internal/stack/usecase/install_stack.go 구현

# 4. 커밋
git add .
git commit -m "feat(stack): 스택 설치 워크플로우 5단계 구현

사용자가 웹 UI에서 노코드로 DevSecOps 스택을 설정할 수 있도록
5단계 Wizard를 구현합니다."

# 5. PR 생성
git push origin feat/stack/add-tools-wizard
# GitHub에서 PR 생성

# 6. 리뷰 및 머지
# 리뷰어 승인 후 Squash and Merge
```

### 버그 수정 (fix)

```bash
# 1. 브랜치 생성
git checkout -b fix/cicd/pipeline-rollback-pvc

# 2. 버그 재현 테스트 작성
# internal/cicd/usecase/deploy_pipeline_test.go에 실패 테스트 추가

# 3. 버그 수정
# internal/cicd/usecase/deploy_pipeline.go 수정

# 4. 테스트 통과 확인
go test ./internal/cicd/... -v

# 5. 커밋
git commit -m "fix(cicd): 파이프라인 배포 롤백 시 PVC 보존

배포 실패 시 롤백 중에 PVC가 삭제되는 버그를 수정합니다.
Helm rollback 전에 PVC를 별도로 보존하도록 변경합니다."

# 6. PR 생성 및 머지
```

---

## 8. 자주 묻는 질문 (FAQ)

### Q1. 커밋을 여러 개 만들었는데 PR은 어떻게 하나요?

**A.** Squash and Merge를 사용합니다. GitHub에서 PR을 머지할 때 "Squash and Merge" 옵션을 선택하면 모든 커밋이 하나로 압축됩니다.

### Q2. 커밋 메시지를 잘못 작성했어요. 수정할 수 있나요?

**A.** 아직 푸시하지 않았다면:
```bash
git commit --amend -m "올바른 메시지"
```

이미 푸시했다면 PR에서 수정하고, 머지 시 올바른 메시지로 Squash and Merge합니다.

### Q3. 모듈이 여러 개 변경되었어요. 어떻게 커밋하나요?

**A.** 주요 모듈을 선택합니다. 예를 들어 `stack`과 `admin` 모듈이 모두 변경되었다면:
```
feat(stack): 스택 설치 워크플로우 구현

admin 모듈의 클러스터 관리 기능도 함께 개선합니다.
```

또는 여러 PR로 분리하는 것을 권장합니다.

### Q4. 문서만 변경했어요. 모듈을 지정해야 하나요?

**A.** 아니요. 문서 변경은 모듈을 생략합니다:
```
docs: API 설계 문서 v1.2 업데이트
```

특정 모듈의 문서라면:
```
docs(stack): 스택 설치 가이드 추가
```

### Q5. 의존성을 업그레이드했어요. 어떻게 커밋하나요?

**A.** `chore` 타입을 사용합니다:
```
chore: Go 1.24 업그레이드
chore: React 19 업그레이드
chore(deps): 보안 패치 적용
```

---

## 9. 참고 자료

- [CLAUDE.md](../CLAUDE.md) — 아키텍처 원칙 및 개발 규칙
- [Conventional Commits](https://www.conventionalcommits.org/) — 커밋 메시지 표준
- [GitHub Flow](https://guides.github.com/introduction/flow/) — 브랜치 전략
- [Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html) — 아키텍처 원칙
