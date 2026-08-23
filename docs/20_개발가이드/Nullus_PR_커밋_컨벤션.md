# Nullus PR/커밋 컨벤션 가이드 (v3)

Nullus 프로젝트의 브랜치, PR, 커밋 작성 규칙을 정의합니다.
본 문서는 `Nullus_브랜치_관리_개선안.md`를 반영한 v2에 §4(CHANGELOG)를 더한 v3 기준입니다.

- 작성일: 2026-05-22 (v3 개정: 2026-08-09)
- 적용 대상: `cloud-nullus/draft`
- 선행 문서: `Nullus_브랜치_관리_개선안.md`

### v3 개정 사유

두 건 모두 실제로 사고가 난 뒤에 적었습니다.

1. **CHANGELOG 규칙이 아예 없었습니다** (§4 신설). #114(GitLab 저장소 프로비저닝과 Argo CD 연동)가 CHANGELOG를 전혀 건드리지 않고 머지되어, 릴리즈를 자를 때 기능 본체가 통째로 빠져 있었습니다. 커밋 36건을 손으로 대조해 17건을 뒤늦게 백필했습니다. 규칙과 함께 CI 검사(`📝 CHANGELOG Check`)를 붙였습니다.
2. **브랜치명 형식이 `CLAUDE.md`와 어긋나 있었습니다** (§1.2). 릴리즈 정책 §13-6이 "둘 중 한쪽으로 단일화"를 미해결 과제로 들고 있었고, 실제 브랜치는 두 형식이 34:33으로 섞여 있었습니다. 최근 브랜치가 전부 중첩형이고 `CLAUDE.md`도 그 형식이라 **중첩형으로 단일화**합니다.

---

## 1. 브랜치 전략

### 1.1 브랜치 타입 (3종)

브랜치 prefix는 아래 3개만 사용합니다.

- `feat`: 기능 추가
- `fix`: 버그 수정
- `chore`: 그 외 작업 (문서/테스트/리팩터링/CI/설정 포함)

`refactor`, `test`, `docs`, `perf`, `ci`는 **브랜치 타입이 아니라 커밋 타입**으로 유지합니다.

### 1.2 브랜치명 규칙

```text
<type>/<module>/<short-desc>
```

- 예: `feat/stack/tools-wizard`
- 길이: `type/` 이후 전체 60자 이내 권장
- 소문자 + 숫자 + `-` + `.`만 사용
- 모듈은 §1.3 화이트리스트에서 고른다. 여러 모듈이면 주 모듈 하나만 쓴다

> **v3 변경**: v2는 평면형(`feat/stack-tools-wizard`)이었으나 `CLAUDE.md`가 중첩형을 규정해 두 형식이 섞여 있었습니다(릴리즈 정책 §13-6). 최근 브랜치가 전부 중첩형이므로 중첩형으로 단일화합니다. 기존 평면형 브랜치는 그대로 두고, 새로 만드는 것만 이 형식을 따릅니다.

### 1.3 모듈 화이트리스트

브랜치/커밋 모듈은 아래 enum만 허용합니다.

- `stack`, `cicd`, `admin`, `auth`, `o11y`, `product`, `ui`, `shared`, `infra`, `cli`

주의:
- `observability`는 사용하지 않고 `o11y`로 통일합니다.
- `cli`는 CLI+MCP 트랙(`cmd/nullus`, `internal/cli`, `pkg/nullusclient`, MCP 서버) 작업에 사용합니다 (2026-08-23 추가 — CLI+MCP 구현 백로그 0-5).

### 1.4 브랜치 예시

```text
feat/stack/tools-wizard
fix/cicd/pipeline-rollback-pvc
fix/o11y/deploy-readiness-retries
chore/shared/upgrade-go-1.24
chore/product/prd-v1.3-update
```

---

## 2. 커밋 메시지 규칙

### 2.1 기본 형식

```text
<type>(<module>): <description>

[선택] 본문
[선택] 바닥글
```

### 2.2 커밋 타입

| 타입 | 설명 |
|---|---|
| `feat` | 새 기능 추가 |
| `fix` | 버그 수정 |
| `refactor` | 리팩터링 (동작 변경 없음) |
| `test` | 테스트 추가/수정 |
| `docs` | 문서 추가/수정 |
| `chore` | 빌드/설정/의존성/기타 관리 작업 |
| `perf` | 성능 개선 |
| `ci` | CI/CD 설정 변경 |

### 2.3 모듈 표기

- 원칙: 가능하면 모듈 포함 (`feat(stack): ...`)
- 공통 작업은 모듈 생략 가능 (`chore: ...`, `docs: ...`)
- 모듈을 쓸 때는 §1.3 화이트리스트만 사용

### 2.4 설명 작성 규칙

- 50자 이내 권장
- 명령형 현재형 사용
- 마침표 미사용
- 한글/영문 혼용 최소화

### 2.5 AI 작성 커밋 트레일러

AI 도구가 작성한 커밋은 아래 trailer를 권장합니다.

```text
Co-Authored-By: Claude <noreply@anthropic.com>
AI-Tool: <도구 이름>          # 예: claude-code, codex, gemini-cli
AI-Session: <session-id>
```

`AI-Tool` 에는 **도구 이름**을 씁니다. 모델명을 적으면 모델이 바뀔 때마다 이 문서와 과거 커밋이 어긋납니다.

이 정보는 PR 자동 라벨링(`ai-authored`)과 메타 체크에 사용됩니다.

---

## 3. PR 작성 가이드

### 3.1 기본 원칙

- **1 작업 = 1 브랜치 = 1 PR**
- 분리는 필수가 아니라 예외적 허용입니다.
- 연속 패치가 필요하면 별도 브랜치/PR을 허용합니다.

### 3.2 PR 제목

커밋 메시지 형식을 그대로 사용합니다.

```text
feat(stack): 스택 설치 워크플로우 5단계 구현
```

### 3.3 PR 본문 템플릿

```markdown
Dispatcher: @<github-handle>   <!-- AI 작성 PR인 경우 필수 -->
AI-Tool: <tool-name>           <!-- AI 작성 PR인 경우 권장 -->
Scope: <이 PR이 다루는 범위 1줄>

## Summary
- 무엇을 변경했는가?
- 왜 변경했는가?

## Changes
- 주요 변경 1
- 주요 변경 2

## Related Issues
- Closes #123

## Testing
- 실행한 테스트와 결과
```

### 3.4 PR 크기 가이드

- 권장: changed lines 500 이내
- 500 초과 시 리뷰어/dispatcher가 분리 요청 가능

---

## 4. CHANGELOG

> 릴리즈 관점의 규정은 `Nullus_릴리즈 정책.md` §4.2에 있습니다. 이 절은 **작성자가 매 PR에서 무엇을 어떻게 쓰는가**만 다룹니다. 규칙이 어긋나면 릴리즈 정책이 우선합니다.

### 4.1 원칙

**동작이 바뀌는 PR은 `CHANGELOG.md`를 함께 고친다.** 나중에 몰아서 쓰지 않는다.

몰아서 쓰면 반드시 빠진다. #114는 CHANGELOG 없이 머지되어 GitLab 저장소 프로비저닝과 Argo CD 연동이라는 기능 본체가 릴리즈 노트에서 통째로 누락됐고, 릴리즈를 자르는 시점에 커밋 36건을 손으로 대조해 17건을 백필해야 했습니다. 그 시점에는 이미 무엇이 왜 바뀌었는지 아는 사람이 없습니다.

### 4.2 어디에 쓰는가

항상 `## [Unreleased]` 아래에 씁니다. 릴리즈된 섹션(`## [0.4.0-alpha]` 등)은 **릴리즈 담당 외에는 건드리지 않습니다** — 이미 발행된 GitHub Release 본문과 어긋나기 때문입니다.

분류는 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)를 따릅니다.

| 헤딩 | 쓰는 경우 |
|---|---|
| `### Added` | 새 기능 |
| `### Changed` | 기존 동작 변경 |
| `### Deprecated` | 곧 제거될 기능 |
| `### Removed` | 제거된 기능 |
| `### Fixed` | 버그 수정 |
| `### Security` | 보안 수정 |

### 4.3 항목 작성 규칙

```markdown
- **<증상 중심 한 줄 제목>**: <원인은 무엇이었고, 그래서 무슨 일이 벌어졌고, 무엇을 했는지>
```

- 증상부터 씁니다. 읽는 사람은 자기가 겪은 현상으로 찾습니다 — "`installDAG` 수정"이 아니라 "GitLab+Argo CD 배포가 MinIO에서 멈추던 문제"
- 원인을 남깁니다. 같은 함정에 다시 빠지는 것을 막는 것이 이 파일의 가장 큰 값입니다
- 코드 식별자는 백틱으로 감쌉니다
- 한국어, `~다` 체
- PR 번호는 선택입니다. 본문만으로 이해되면 굳이 붙이지 않습니다

### 4.4 CI 검사

`Lint Review` 워크플로의 `📝 CHANGELOG Check`(`scripts/check_changelog.py`)가 두 가지를 봅니다.

1. **항목 누락** — 동작이 바뀌는 파일을 고쳤는데 `CHANGELOG.md`를 건드리지 않으면 실패합니다.
2. **릴리즈 중복** — `[Unreleased]`의 항목이 이미 릴리즈된 섹션에도 있으면 실패합니다. 릴리즈를 자른 뒤 `main`을 되머지할 때 실제로 발생했습니다.

같은 검사를 로컬에서도 돌릴 수 있습니다.

```bash
python3 scripts/check_changelog.py                        # 구조만
git diff --name-only origin/main...HEAD \
  | python3 scripts/check_changelog.py --changed -        # 누락까지
```

### 4.5 면제

아래만 고친 PR은 검사가 자동으로 넘어갑니다.

- `docs/`, `README.md`, `LICENSE`, `.gitignore`
- 테스트 전용: `e2e/`, `*_test.go`, `*.test.ts(x)`, `*.spec.ts(x)`

그 밖에 정말 필요 없다고 판단되면 PR에 `no-changelog` 라벨을 붙입니다. 이때도 **중복 검사는 그대로 돕니다**. 라벨은 리뷰어가 보게 되므로 판단 근거를 PR 본문에 남기십시오.

---

## 5. AI 작업자 운영 규칙

### 5.1 책임 분리

- AI는 브랜치 생성/커밋/PR 생성 가능

### 5.2 범위 통제

- PR description에 적은 scope 밖 변경 금지
- 개인 메타 파일 커밋 금지:
  - `.omc/`
  - `.claude/state/`
  - `MEMORY.md`
  - `*.plan.md`
  - `notepad.*`

### 5.3 검증

- CI 통과는 필요조건
- 사람 reviewer 1명 이상 승인 필수
- 보안 민감 모듈(`auth`, `admin`, 향후 `billing`)은 maintainer 승인 필수

---

## 6. 코드 리뷰 기준

리뷰어는 아래를 우선 확인합니다.

- 아키텍처 레이어 위반 여부 (Clean Architecture)
- 모듈 경계 위반 여부 (직접 internal import 금지)
- 테스트 유의미성 (신규 기능/버그 재현 포함)
- 변경 범위와 PR scope 일치 여부
- 문서/마이그레이션/보안 영향 반영 여부

---

## 7. 머지 전략

### 7.1 머지 방식

- 기본: **Squash and Merge**
- 머지 후 브랜치 자동 삭제를 기본 정책으로 사용

### 7.2 머지 전 체크

- CI 통과
- 사람 승인 1명 이상
- 충돌 해결 완료
- 커밋/PR 제목 컨벤션 준수

---

## 8. 브랜치 수명주기 (SLA)

| 상태 | 기준 | 조치 |
|---|---|---|
| 머지 완료 | 즉시 | head branch 자동 삭제 |
| PR inactive | 14일 | `stale` 라벨 + 알림 |
| PR inactive | 30일 | PR 자동 close (브랜치 유지) |
| 미머지 브랜치 inactive | 60일 | 정리 이슈 생성 후 7일 내 삭제 결정 |

---

## 9. 개발 워크플로우 예시

### 9.1 기능 추가

```bash
# 1) 브랜치 생성
git switch -c feat/stack/tools-wizard

# 2) 테스트 작성 (TDD)
# 3) 구현

# 4) CHANGELOG [Unreleased] 에 항목 추가 (§4)
python3 scripts/check_changelog.py

# 5) 커밋
git add .
git commit -m "feat(stack): 스택 설치 워크플로우 5단계 구현"

# 6) 푸시 및 PR 생성
git push -u origin feat/stack/tools-wizard
```

### 9.2 버그 수정

```bash
# 1) 브랜치 생성
git switch -c fix/cicd/pipeline-rollback-pvc

# 2) 실패 테스트 추가
# 3) 수정

# 4) 검증
go test ./... -count=1

# 5) CHANGELOG [Unreleased] 의 Fixed 에 항목 추가 (§4)

# 6) 커밋 및 PR
git commit -m "fix(cicd): 파이프라인 롤백 시 PVC 보존"
git push -u origin fix/cicd/pipeline-rollback-pvc
```

### 9.3 로컬 stale 브랜치 정리

```bash
# 원격 삭제 반영 + 로컬 stale 브랜치 정리
git fetch --prune
git branch -vv | awk '/: gone]/{print $1}' | xargs -r git branch -D
```

---

## 10. FAQ

### Q1. 문서 작업인데 브랜치는 `docs/...`를 쓰면 안 되나요?

A. v2에서는 브랜치 타입을 3개만 사용하므로 `chore/...`를 사용합니다.
커밋 타입은 `docs`를 그대로 사용해도 됩니다.

### Q2. `observability`와 `o11y` 중 무엇을 쓰나요?

A. `o11y`만 사용합니다.

### Q3. AI가 작성한 PR은 자동 머지 가능한가요?

A. 아닙니다. `ai-authored` PR은 사람 승인 후 머지합니다.

### Q4. 여러 모듈이 함께 바뀌면 어떻게 하나요?

A. 주 모듈 기준으로 커밋/브랜치명을 정하고, PR 본문에 영향 모듈을 명시합니다.

---

## 11. 참고 자료

- `docs/20_개발가이드/Nullus_브랜치_관리_개선안.md`
- `CLAUDE.md`
- [Conventional Commits](https://www.conventionalcommits.org/)
- [GitHub Flow](https://docs.github.com/en/get-started/quickstart/github-flow)
