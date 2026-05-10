# Contributing to Nullus

Nullus에 기여해주셔서 감사합니다! 본 문서는 개발 워크플로우, 코드 스타일, 테스트 가이드를 제공합니다.

## 개발 환경 설정

### 사전 요구사항

- Go 1.24+
- Node.js 22+
- Docker + Docker Compose
- Git

### 로컬 환경 구성

1. 저장소 클론:
```bash
git clone https://github.com/cloud-nullus/draft.git
cd draft
```

2. 개발 환경 시작:
```bash
make dev
```

3. 환경 변수 확인:
```bash
cat .env.example
```

필요시 `.env` 파일을 생성하여 환경 변수를 커스터마이징할 수 있습니다.

4. 백엔드 및 프론트엔드 실행:
```bash
# Terminal 1: 백엔드
make run

# Terminal 2: 프론트엔드
make web-dev
```

## 브랜치 전략

메인 브랜치는 `main`이며, 다음 패턴을 따릅니다:

```
main (프로덕션)
  ├── feat/<module>/<description>      # 새 기능
  ├── fix/<module>/<description>       # 버그 수정
  └── refactor/<module>/<description>  # 리팩토링
```

### 예시

```bash
# 새 기능 브랜치 생성
git checkout -b feat/stack/install-workflow

# 버그 수정 브랜치 생성
git checkout -b fix/cicd/rollback-pvc

# 리팩토링 브랜치 생성
git checkout -b refactor/auth/keycloak-integration
```

## 커밋 메시지 컨벤션

커밋 메시지는 다음 형식을 따릅니다:

```
<type>(<module>): <description>

<body (optional)>
```

### Type

- `feat`: 새 기능 추가
- `fix`: 버그 수정
- `refactor`: 코드 구조 개선 (기능 변경 없음)
- `test`: 테스트 추가 또는 수정
- `docs`: 문서 추가 또는 수정
- `chore`: 빌드, 의존성 등 기타 변경

### Module

- `stack`: Stack Management 모듈
- `cicd`: CI/CD Pipeline 모듈
- `auth`: Auth 모듈
- `admin`: Organization/Admin 모듈
- `observability`: Observability 모듈
- `shared`: 공유 코드
- (해당 사항 없으면 생략)

### 예시

```
feat(stack): 스택 설치 워크플로우 5단계 구현

- Artifacts 탭: 레지스트리 선택
- Pipeline Tools 탭: CI/CD 플랫폼 선택
- Monitoring Tools 탭: 모니터링 도구 선택
- 실시간 Configuration Summary 표시

fix(cicd): 파이프라인 배포 롤백 시 PVC 보존

- Helm uninstall 시 --keep-history 추가
- PVC 자동 삭제 방지

test(admin): 클러스터 등록 통합 테스트 추가

refactor(auth): Keycloak OIDC 통합 정리

docs: README.md 개발 환경 섹션 추가
```

## 코드 리뷰 기준

Pull Request를 제출하기 전에 다음을 확인하세요:

### Architecture

- [ ] Clean Architecture 레이어 위반 없음
  - Domain 레이어가 외부 프레임워크/DB에 의존하지 않음
  - UseCase가 Repository 인터페이스에만 의존
  - Handler가 UseCase를 통해서만 비즈니스 로직에 접근
- [ ] 모듈 간 직접 의존 없음
  - 다른 모듈의 `internal/` 패키지를 import하지 않음
  - 공유 타입은 `internal/shared/` 또는 `pkg/`에 정의

### Code Quality

- [ ] 코드 스타일 준수
  - Go: `gofmt`, `goimports`, `golangci-lint` 통과
  - TypeScript: `eslint`, `prettier` 통과
  ```bash
  make lint          # Go 린트
  npm run lint       # TypeScript 린트 (web 디렉토리에서)
  ```
- [ ] 테스트 포함
  - 새 기능은 테스트부터 작성 (TDD)
  - 버그 수정 시 재현하는 테스트 먼저 작성
  - 테스트 커버리지 목표: >70%
- [ ] 도메인 용어 일관성
  - PRD/기능분해도의 용어를 코드에 그대로 반영
  - Ubiquitous Language 사용

## 테스트 가이드

### TDD 사이클

모든 새 기능은 다음 사이클을 따릅니다:

```
1. RED    — 실패하는 테스트 작성
2. GREEN  — 테스트를 통과시키는 최소 코드 작성
3. REFACTOR — 중복 제거 및 코드 정리 (테스트는 계속 통과)
```

### Go 테스트

#### 테스트 명명 규칙

```go
func TestStackService_Install_Success(t *testing.T) { ... }
func TestStackService_Install_ClusterNotFound(t *testing.T) { ... }
func TestStackService_Install_IncompatibleVersion(t *testing.T) { ... }
```

#### 테스트 레이어별 가이드

| 레이어 | 테스트 유형 | 도구 | 범위 |
|--------|-----------|------|------|
| **Domain** | 단위 테스트 | `testing` | 비즈니스 규칙, 순수 함수, 외부 의존 없음 |
| **UseCase** | 단위 + 모킹 | `testing`, `testify/mock` | Repository 인터페이스 모킹 |
| **Handler** | 통합 테스트 | `httptest` | HTTP 요청/응답 검증 |
| **Repository** | 통합 테스트 | `testcontainers` | 실제 DB 연동 (테스트 컨테이너) |

#### 테스트 실행

```bash
# 전체 테스트
make test

# 특정 패키지 테스트
go test ./internal/stack/... -v

# 커버리지 리포트
make test-cover
```

#### DB 연동 테스트 (E2E)

`make dev`로 Docker 인프라가 기동 중이어야 합니다.

```bash
# 전체 E2E 시나리오 실행
go test ./e2e/ -v -count=1

# DB 연동 테스트만 실행
go test ./e2e/ -run TestDBIntegration -v
```

#### 벤치마크

```bash
go test -bench=. -benchmem ./...

# 특정 패키지 벤치마크
go test -bench=. -benchmem ./internal/stack/...
```

#### 커버리지

```bash
make test-cover
```

`coverage.html`이 생성됩니다. 브라우저에서 열어 라인별 커버리지를 확인하세요. 목표: **>70%**.

#### 예시: Stack 설치 테스트

```go
package usecase_test

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// Mock Repository
type MockStackRepository struct {
	mock.Mock
}

func (m *MockStackRepository) Save(s *domain.Stack) error {
	args := m.Called(s)
	return args.Error(0)
}

// Test
func TestInstallStack_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockStackRepository)
	mockRepo.On("Save", mock.Anything).Return(nil)

	service := usecase.NewInstallStackService(mockRepo)
	stack := &domain.Stack{
		Name: "test-stack",
		// ...
	}

	// Act
	err := service.Install(stack)

	// Assert
	assert.NoError(t, err)
	mockRepo.AssertCalled(t, "Save", stack)
}
```

### TypeScript 테스트

#### 테스트 명명 규칙

```typescript
describe('useInstallStack', () => {
  it('should deploy stack when cluster is connected', () => { ... })
  it('should show error when cluster is unreachable', () => { ... })
})
```

#### 테스트 실행

```bash
make web-test

# 특정 파일 테스트
npm run test -- src/features/stack/hooks/useInstallStack.test.ts

# Watch 모드
npm run test -- --watch
```

#### 커버리지

```bash
cd web
npx vitest run --coverage
```

`coverage/` 디렉토리에 HTML 리포트가 생성됩니다.

### Playwright E2E 테스트

`web/` 디렉토리에서 실행합니다. 프론트엔드 개발 서버(`make web-dev`)와 백엔드 서버(`make run`)가 모두 실행 중이어야 합니다.

```bash
cd web

# headless 실행
npm run e2e

# 브라우저 표시하며 실행
npm run e2e:headed

# 결과 리포트 열기
npm run e2e:report
```

#### 예시: Stack 설치 훅 테스트

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { useInstallStack } from './useInstallStack'

describe('useInstallStack', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should deploy stack when cluster is connected', async () => {
    // Arrange
    const mockApi = vi.spyOn(api, 'deployStack').mockResolvedValue({
      id: 'stack-1',
      status: 'deploying',
    })

    // Act
    const { result } = renderHook(() => useInstallStack())
    await result.current.deploy({ clusterId: 'cluster-1' })

    // Assert
    await waitFor(() => {
      expect(mockApi).toHaveBeenCalled()
    })
  })
})
```

## 인프라 관련 주의사항

### 포트 충돌 해결

`make dev` 실행 시 로컬 서비스와 포트가 충돌할 수 있습니다.

| 서비스 | 호스트 포트 | 충돌 시 확인 명령 |
|--------|------------|-----------------|
| PostgreSQL | 5433 | `lsof -i :5433` |
| MinIO API | 9000 | `lsof -i :9000` |
| MinIO 콘솔 | 9001 | `lsof -i :9001` |
| Redis | 6380 | `lsof -i :6380` |

포트를 점유한 프로세스가 있으면 종료하거나, `docker-compose.dev.yaml`의 ports 항목을 수정합니다.

```yaml
# 예: PostgreSQL 포트를 5434로 변경
ports:
  - "5434:5432"
```

포트를 변경한 경우 `Makefile`의 `DB_URL`도 함께 수정해야 합니다.

### 개발 환경 초기화

볼륨 데이터를 포함하여 완전히 초기화하려면:

```bash
make dev-clean   # Docker 볼륨 포함 삭제
make dev         # 재기동 + 마이그레이션 재실행
```

## 코드 스타일

### Go

#### 포맷팅

```bash
# 자동 포맷팅
gofmt -w ./...
goimports -w ./...

# 또는 Makefile 사용
make lint
```

#### 명명 규칙

- 패키지: 소문자 (예: `stack`, `cicd`)
- 구조체/인터페이스: PascalCase (예: `Stack`, `StackRepository`)
- 변수/함수: camelCase (예: `installStack`, `getStack`)
- 상수: SCREAMING_SNAKE_CASE (예: `MAX_RETRIES`)

#### 예시

```go
package stack

// Entity
type Stack struct {
	ID    string
	Name  string
	Tools []Tool
}

// Interface
type StackRepository interface {
	Save(ctx context.Context, s *Stack) error
	Get(ctx context.Context, id string) (*Stack, error)
}

// Service
type InstallStackService struct {
	repo StackRepository
}

func NewInstallStackService(repo StackRepository) *InstallStackService {
	return &InstallStackService{repo: repo}
}

func (s *InstallStackService) Install(ctx context.Context, stack *Stack) error {
	// 비즈니스 로직
	return s.repo.Save(ctx, stack)
}
```

### TypeScript/React

#### 포맷팅

```bash
cd web

# ESLint + Prettier
npm run lint:fix

# 또는 개별 실행
npm run eslint -- --fix
npm run prettier -- --write
```

#### 명명 규칙

- 컴포넌트: PascalCase (예: `StackInstaller`, `ConfigStep`)
- 훅: camelCase with `use` prefix (예: `useInstallStack`, `useCluster`)
- 변수/함수: camelCase (예: `installStack`, `getStack`)
- 상수: SCREAMING_SNAKE_CASE (예: `MAX_RETRIES`)
- 파일: kebab-case (예: `stack-installer.tsx`, `use-install-stack.ts`)

#### 예시

```typescript
// src/features/stack/components/StackInstaller.tsx
import { useState } from 'react'
import { useInstallStack } from '../hooks/useInstallStack'

interface StackInstallerProps {
  clusterId: string
}

export function StackInstaller({ clusterId }: StackInstallerProps) {
  const [isLoading, setIsLoading] = useState(false)
  const { deploy } = useInstallStack()

  const handleInstall = async () => {
    setIsLoading(true)
    try {
      await deploy({ clusterId })
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <button onClick={handleInstall} disabled={isLoading}>
      {isLoading ? 'Installing...' : 'Install Stack'}
    </button>
  )
}
```

## Pull Request 프로세스

1. 브랜치 생성 및 기능 구현
2. 테스트 작성 및 실행 (테스트 통과 확인)
3. 코드 리뷰 기준 확인
4. 커밋 메시지 작성 (컨벤션 준수)
5. PR 제출

### PR 템플릿

```markdown
## 설명
<!-- 변경 사항을 간단히 설명합니다 -->

## 유형
- [ ] 새 기능
- [ ] 버그 수정
- [ ] 리팩토링
- [ ] 문서 업데이트

## 변경 사항
<!-- 주요 변경 사항 목록 -->
- Change 1
- Change 2

## 테스트 방법
<!-- 테스트 방법을 설명합니다 -->

## 체크리스트
- [ ] 테스트를 작성했습니다
- [ ] 테스트가 통과합니다
- [ ] 코드 스타일을 준수합니다
- [ ] 문서를 업데이트했습니다
- [ ] 커밋 메시지가 컨벤션을 따릅니다
```

## 문서 작성

### README/문서 구조

- 간결한 제목 사용
- 코드 예시는 언어 명시 (```go, ```typescript)
- 명령어는 code block 사용
- 내부 링크는 상대 경로 사용

### 예시

```markdown
## Backend Setup

### 데이터베이스 마이그레이션

데이터베이스 스키마를 준비합니다:

```bash
make migrate-up
```

#### 마이그레이션 상태 확인

```bash
make migrate-status
```
```

## 일반적인 개발 워크플로우

### 새 기능 구현

```bash
# 1. 기능 브랜치 생성
git checkout -b feat/stack/new-feature

# 2. 테스트 작성 (TDD)
# internal/stack/usecase/new_feature_test.go 작성

# 3. 기능 구현
# internal/stack/usecase/new_feature.go 구현

# 4. 테스트 실행
make test

# 5. 코드 스타일 확인
make lint

# 6. 커밋
git add .
git commit -m "feat(stack): new feature implementation"

# 7. PR 제출
git push origin feat/stack/new-feature
```

### 버그 수정

```bash
# 1. 버그 수정 브랜치 생성
git checkout -b fix/stack/bug-name

# 2. 버그를 재현하는 테스트 작성
# internal/stack/usecase/bug_test.go 작성

# 3. 버그 수정
# 테스트가 통과할 때까지 수정

# 4. 전체 테스트 실행
make test

# 5. 커밋 및 PR 제출
```

## 지원 및 문의

- **Issues**: 버그 리포트 및 기능 요청
- **Discussions**: 아이디어 및 질문
- **GitHub**: [cloud-nullus/draft](https://github.com/cloud-nullus/draft)

## 라이선스

Nullus는 Apache License 2.0 하에 배포됩니다. 기여함으로써 귀하는 이 라이선스에 동의하게 됩니다.
