# Nullus 브랜치 관리 개선안

`Nullus_PR_커밋_컨벤션.md`(v1) 운영 결과 origin에 브랜치 27개가 누적되었고, 모듈명 불일치(`o11y` vs `observability`), 컨벤션 외 브랜치(`phase1`, `feat/phase-2`, `feat/f8-compatibility-matrix`) 가 발생했습니다.

**전제 — 작업 분담의 현실**: 본 프로젝트는 **AI 코딩 도구(Claude Code/OMC, Cursor, Copilot 등)가 다수 작업을 수행**하고, 사람은 소수입니다. 따라서 본 컨벤션은 "AI 가 기본 작업자, 사람은 방향 설정·게이트·안전성 보증" 이라는 역할 분담을 전제로 설계합니다. §2.5 는 AI 작업자가 따라야 할 규칙을, §2.6 은 사람만 할 수 있는 일과 사람이 반드시 게이트로 작용해야 하는 일을 정의합니다.

본 문서 구성: 원인 진단(§1), 컨벤션 v2 개정안(§2 — AI 운영 §2.5, 사람 책임 §2.6), 자동화 워크플로우(§3 — PR meta-check / CODEOWNERS), 현 origin 브랜치 정리 리스트(§4), 도입 순서·측정 지표(§5~§6).

- 작성일: 2026-05-11
- 적용 대상: `cloud-nullus/draft`
- 선행 문서: [Nullus_PR_커밋_컨벤션.md](./Nullus_PR_커밋_컨벤션.md)

---

## 1. 원인 진단

| # | 원인 | 근거 |
|---|---|---|
| 1 | 분리 권장이 곱셈 효과를 만든다 | v1 §4 "대형 PR 분리 전략"이 백엔드/프론트엔드, 리팩터링/기능, 문서/코드 3축 분리를 명시 → 기능 1개에 PR 3~4개 |
| 2 | 자동 삭제 정책 부재 | v1 §1에 "머지 후 삭제"만 적혀 있고, GitHub `delete-branch-on-merge`·cleanup workflow·`fetch --prune` 가이드 없음 |
| 3 | 타입 8종 × 모듈 7종 = 56 조합 | 작성자마다 `refactor` vs `chore`, `o11y` vs `observability` 분류가 갈림 (실제 발생) |
| 4 | 2단계 nesting 강제 | `feat/<module>/<description>` 형식은 autocomplete·`git branch --list` 가독성 저하 |
| 5 | Stacked / Draft PR 미언급 | 동기 의존 작업도 독립 브랜치로 떠야 함 |
| 6 | 만료/정리 SLA 없음 | 14일 inactive, 60일 머지 후 자동 삭제 같은 수치 기준 부재 |

> **주의**: 동일 모듈 연속 패치(`fix/o11y/*` 4개) 자체는 **결함이 아닙니다.** 작업자(사람·AI)가 비동기·병렬로 투입되는 환경에서는 브랜치를 합칠 수 없습니다. 진짜 문제는 "머지된 브랜치가 origin에 잔존"이고, 이건 자동 삭제(§3)로 해결합니다.

---

## 2. 컨벤션 v2 개정안

### 2.1 브랜치 타입 축소 (8 → 3)

브랜치 prefix는 **`feat` / `fix` / `chore`** 3종만 사용. `refactor`·`test`·`docs`·`perf`·`ci`는 **커밋 타입으로만** 유지하고 브랜치는 `chore`로 흡수합니다.

| v1 브랜치 prefix | v2 브랜치 prefix | v2 커밋 타입은 그대로 유지 |
|---|---|---|
| `feat/` | `feat/` | `feat` |
| `fix/` | `fix/` | `fix` |
| `refactor/` | **`chore/`** | `refactor` |
| `test/` | **`chore/`** | `test` |
| `docs/` | **`chore/`** | `docs` |
| `chore/` | `chore/` | `chore` |
| `perf/` | (사례 시 `feat` 또는 `fix`) | `perf` |
| `ci/` | **`chore/`** | `ci` |

### 2.2 브랜치명 평면화 + 모듈 화이트리스트

```
v1:  feat/<module>/<description>      # 2단계 nesting
v2:  <type>/<module>-<short-desc>     # 1단계, 60자 이내
```

**모듈 화이트리스트(enum 고정)**: `stack`, `cicd`, `admin`, `auth`, `o11y`, `product`, `ui`, `shared`, `infra`
- `observability` → **`o11y`** 로 통일 (v1의 혼용 사례 차단)
- 화이트리스트 외 모듈은 lint에서 reject

**예시**
```
feat/stack-tools-wizard
fix/cicd-pipeline-rollback-pvc
fix/o11y-deploy-readiness-retries
chore/upgrade-go-1.24
chore/docs-prd-v1.3-update
```

### 2.3 PR 분리 정책 조정

v1 §4 "대형 PR 분리 전략"은 **약화**합니다.

- **기본 원칙: 1 작업 = 1 브랜치 = 1 PR.** 분리는 "리뷰 비용 > 합치는 비용"일 때만 — 분리를 "장려"에서 "허용"으로 톤다운.
- **연속 패치는 묶지 않는다.** 작업자(사람·AI)가 비동기·병렬로 다르면 동일 모듈이라도 별도 브랜치가 자연스럽습니다. 대신 §3 자동화로 머지 즉시 청소되므로 누적되지 않습니다. (`fix/o11y/*` 4개 패턴은 v2에서도 허용)
- **단일 작업자가 동기적으로 이어가는 경우만** 추가 커밋으로 누적 후 squash.
- **의존성 있는 작업은 Stacked PR** — base 브랜치를 지정해 트리를 만들고, 머지는 순차로. 비동기 다중 작업자에는 권장하지 않음.
- **실험·WIP는 origin push 금지** — 로컬 또는 fork에서 작업.

### 2.4 브랜치 수명주기 SLA

| 상태 | SLA | 조치 |
|---|---|---|
| 머지 완료 | **즉시** | GitHub `delete-branch-on-merge` 자동 삭제 |
| PR 열린 채 14일 inactive | 14일 | `stale` 라벨 + 작성자 멘션 봇 댓글 |
| PR 열린 채 30일 inactive | 30일 | PR 자동 close (브랜치는 유지) |
| 브랜치만 존재, 60일 commit 없음 | 60일 | cleanup 워크플로우가 알림 issue 생성 → 7일 후 삭제 |

### 2.5 AI 작업자 운영 지침

**전제**: Claude Code(OMC), Cursor, GitHub Copilot 등 AI 코딩 도구가 브랜치를 직접 만들고 push 하는 일이 일상화되었습니다. AI도 §2.1~2.4 규칙을 그대로 따르되, AI 특유의 비동기성·범위 무한확장 경향·머지 권한 부재를 보완하기 위해 다음 규칙을 추가합니다.

#### A. 디스패처(사람) 책임

- AI가 만든 모든 브랜치/PR에는 **사람 dispatcher 1명을 owner로 명시**합니다. PR description 첫 줄에 `Dispatcher: @<github-handle>` 표기.
- AI가 만든 브랜치의 lint/stale 알림·삭제 issue 는 dispatcher 에게 라우팅됩니다.
- AI는 **머지 권한이 없습니다.** Approve·merge는 사람만. `ai-authored` 라벨이 붙은 PR은 auto-merge 비활성화(§3.1).

#### B. 브랜치/커밋 식별

- 브랜치명은 §2.2 규칙을 그대로 따릅니다 (`feat/stack-tools-wizard`). **AI 여부는 브랜치명에 표시하지 않습니다** — lint·grep 일관성 유지를 위해.
- AI 작성 commit 은 trailer 로 식별합니다:
  ```
  Co-Authored-By: Claude <noreply@anthropic.com>
  AI-Tool: claude-opus-4-7
  AI-Session: <session-id>   # 가능한 경우
  ```
- 위 trailer 또는 봇 author 가 감지되면 PR 에 **`ai-authored` 라벨이 자동 부착**됩니다(§3.5).

#### C. 동시성·격리

- 여러 AI 에이전트가 같은 모듈을 동시 작업할 때는 **git worktree 격리 필수** (OMC: `isolation: "worktree"`, Claude Code subagent 의 worktree mode).
- 같은 모듈에 다른 AI 작업을 dispatcher 가 중복 투입하지 않습니다. 충돌 시 dispatcher 큐로 직렬화.
- AI 세션 1개 = 브랜치 1개 원칙. 세션이 끝나면 PR 까지 완료해 두고, 추가 작업은 새 세션·새 브랜치.

#### D. 작업 범위 제한 (scope creep 차단)

- 1 PR = 1 작업: AI 는 PR description 에 적힌 scope 외 변경을 만들지 않습니다 ("정리 좀 같이 했어요" 금지).
- **메타데이터·세션 산출물은 commit 금지**: `.omc/`, `.claude/state/`, `MEMORY.md`(개인용), `*.plan.md`, `notepad.*` → `.gitignore` 에 등록.
- AI 가 임의로 새 문서(`SUMMARY.md`, `CHANGES.md` 등)를 추가하지 않습니다. 변경 설명은 PR description 에만.
- 리뷰어가 "out-of-scope" 지적 시 AI 는 해당 변경을 즉시 별도 PR 로 분리합니다.

#### E. 검증·머지 흐름

- AI 가 작성한 PR 의 CI 통과는 **필요조건이지 충분조건이 아닙니다.** 사람 reviewer 1명 이상 approve 필수.
- AI 작성 테스트만 있는 PR(구현 + 테스트 모두 AI)은 "self-verify 단독" 으로 표시하고, 사람이 테스트의 의미성·엣지 케이스를 확인합니다.
- 보안·인증·결제 관련 모듈(`auth`, 향후 `billing`)은 AI 단독 PR 머지 금지 — 사람 author 또는 사람 pair-review 필수.

#### F. Stale 처리 라우팅

- AI 가 만든 미머지 stale 브랜치 책임은 dispatcher 에게 있습니다:
  - 14일 inactive → cleanup 봇이 dispatcher 멘션
  - 30일 inactive → PR auto-close (브랜치 유지)
  - 60일 inactive → cleanup workflow 가 dispatcher 멘션 issue 생성 → 7일 후 삭제

#### G. 권장 워크플로우 (Claude Code / OMC 기준)

```bash
# 1. dispatcher 가 작업 정의 후 AI 세션 시작
#    OMC delegation 시 isolation: "worktree" 로 격리
git worktree add ../draft-ai-stack-wizard feat/stack-tools-wizard

# 2. AI 가 작업 + 커밋 (trailer 자동 부착)
#    Claude Code 는 Co-Authored-By 를 기본 추가

# 3. AI 가 push 후 PR 생성. dispatcher 가 description 첫 줄에 명시:
#    Dispatcher: @dasomel
#    AI-Tool: claude-opus-4-7
#    Scope: <PR 의 범위 1줄>

# 4. CI + 사람 1명 approve → Squash and Merge → 브랜치 자동 삭제
```

#### H. 금지 사항 요약

| 행동 | 사유 |
|---|---|
| AI 가 직접 머지·force push | 권한 분리 |
| AI 가 main 에 직접 push | branch protection 우회 |
| AI 가 `.omc/`, 세션 메모 commit | 개인 메타데이터 |
| AI 가 PR scope 외 파일 변경 | scope creep |
| AI 가 다른 AI PR 을 approve | 사람 검증 누락 |
| AI 가 stale 브랜치 자체 정리 | dispatcher 권한 |

### 2.6 사람(Dispatcher / Reviewer / Maintainer) 책임 영역

AI 가 대부분의 코드 작업을 수행하는 환경에서 사람의 역할은 "작업 실행" 이 아니라 **"방향 설정 · 게이트 통과 · 안전성 보증"** 입니다. 본 절은 AI 에게 위임할 수 없는 일과, 위임 가능하더라도 사람이 반드시 게이트로 작용해야 하는 일을 명시합니다.

#### A. 사람 단독 수행 (AI 위임 금지)

다음 영역은 AI 가 PR 로 제안할 수는 있어도 **실행·승인 권한은 사람에게만** 있습니다.

| 영역 | 사유 |
|---|---|
| 제품 방향·우선순위 결정 | 비즈니스 의사결정 |
| 작업 정의 (issue 작성, scope 결정, AI 디스패치) | 책임 추적의 시작점 |
| PR description 의 `Dispatcher: @<handle>` 표기 | 책임 추적 |
| PR approve / merge / close | 권한 분리 |
| `main` branch protection 변경 | 안전 장치 무력화 방지 |
| GitHub repo 설정 변경 (visibility, collaborators, secrets) | 보안 |
| CODEOWNERS 변경 | 권한 변경 (PR 은 AI 가능, approve 는 maintainer) |
| Production deploy 승인 | 운영 책임 |
| External communication (사용자/조직 공지, release note 발행) | 대외 책임 |
| 본 컨벤션 문서·v1 컨벤션·`.github/workflows/` 변경 승인 | 메타 룰 |

#### B. 사람 게이트 필수 (AI 가 작성하더라도 사람이 통과 결정)

| 게이트 | AI 가 할 수 있는 일 | 사람이 결정·검증하는 일 |
|---|---|---|
| 보안 민감 모듈 (`auth`, `admin`, 향후 `billing`) | 구현·테스트 작성 | 위협 모델 검증, approve |
| Schema / DB migration | migration 파일 작성 | 운영 데이터 영향, 다운타임 평가 |
| Breaking API change | 변경 작성 | 외부 영향, 버전 정책 |
| 의존성 메이저 업그레이드 | upgrade PR 작성 | 회귀 위험, 일정 |
| CI/CD workflow 변경 | YAML 작성 | secret 권한·branch protection 영향 |
| 신규 모듈 추가 (화이트리스트 확장) | 디렉토리 구조 제안 | 모듈 경계·이름 결정 |
| Performance 회귀 (>10%) PR | 변경·벤치마크 작성 | 트레이드오프, rollback 판단 |
| 동일 모듈 다중 AI 동시 작업 | 각자 작업 진행 | 디스패처가 직렬화·worktree 분배 |

#### C. AI 단독으로 진행 가능 (사람은 PR review · 머지만)

다음은 위 B 의 예외를 제외하고 AI 가 PR 까지 자율적으로 진행할 수 있습니다.

- 일반 모듈의 `feat` (기능 추가)
- 영향 범위가 단일 모듈인 `fix` (버그 수정)
- `refactor` / `test` 보강
- 문서 갱신 (`docs/20_개발가이드/` 와 본 문서 제외)
- 의존성 마이너·패치 업그레이드

#### D. Dispatcher 일일 routine

AI 작업자를 띄우는 사람의 1일 단위 책임:

1. **아침**: 어제 띄운 AI PR 의 CI · `pr-meta-check` 결과 확인
2. **세션 시작 전**: scope·모듈·다른 AI 작업과의 충돌 가능성 확인 후 worktree 격리 분배
3. **AI push 직후**: PR description 첫 줄에 `Dispatcher: @<handle>` + `Scope:` 한 줄 추가
4. **Review**: 도메인 적합성·Clean Architecture layer 위반·테스트 의미성 확인. **CI 통과 ≠ 머지 가능** 으로 간주
5. **머지**: Squash and Merge, 자동 삭제 확인
6. **저녁**: stale 봇 멘션 처리 (재개 / close / `wip-pinned`)

#### E. Maintainer 주간 routine

CODEOWNERS · repo admin 권한 보유자의 주간 책임:

1. cleanup workflow 가 만든 "60일 미머지" issue 검토 → 7일 내 결정
2. 컨벤션 외 브랜치 발생 시 dispatcher 와 원인 분석. lint 가 통과시켰다면 lint 패턴 보강
3. §6 측정 지표 수집 (월 1회): origin 브랜치 수, AI PR dispatcher 표기율, scope creep 분리 재요청 건수
4. 본 문서 / v1 컨벤션 / workflow 의 분기별 업데이트

#### F. 권한 매트릭스 요약

| 작업 | AI | Dispatcher | Maintainer |
|---|---|---|---|
| 브랜치 생성·push | ✅ | ✅ | ✅ |
| PR 생성 | ✅ | ✅ | ✅ |
| PR approve | ❌ | ✅ | ✅ |
| 일반 모듈 머지 | ❌ | ✅ | ✅ |
| 보안 모듈 머지 (`auth`/`admin`/`billing`) | ❌ | ❌ | ✅ |
| `main` direct push | ❌ | ❌ | ❌ (protection) |
| Branch protection 변경 | ❌ | ❌ | ✅ |
| CODEOWNERS / secret / repo settings | ❌ | ❌ | ✅ |
| Stale 브랜치 즉시 삭제 | ❌ | ❌ | ✅ |
| `wip-pinned` 라벨 부여 | ❌ | ✅ (own PR) | ✅ (any) |
| 본 문서 / v1 컨벤션 / workflow 수정 PR | ✅ | ✅ | ✅ |
| 위 PR approve | ❌ | ❌ | ✅ |
| Production deploy 승인 | ❌ | ❌ | ✅ |
| 외부 release / 공지 | ❌ | ❌ | ✅ |

#### G. 사람 1명 운영 가능성 (병목 방지)

AI 가 다수 작업을 수행하는 환경에서 dispatcher 1명이 병목이 되지 않도록:

- **PR 크기 상한**: AI PR 은 changed lines 500 라인 이내 권장. 초과 시 분리 요청.
- **자동화 우선**: lint·meta-check·stale 처리는 사람이 아닌 workflow 가 처리 (§3).
- **CODEOWNERS 위임**: 모듈별 maintainer 2명 이상 지정해 단일 장애점 제거.
- **Read-only 라벨링은 봇**: 봇이 `ai-authored`·`stale`·`needs-dispatcher` 라벨을 자동 부여, 사람은 의사결정에만 집중.
- **사람 의사결정 SLA**: AI PR push 후 48 시간 내 dispatcher review (없으면 자동 stale).

---

## 3. 자동화 설정 (Option 2)

### 3.1 GitHub 저장소 설정

`Settings → General → Pull Requests`:
- [x] **Automatically delete head branches** (Squash and Merge 시 자동 삭제)
- [x] Allow squash merging (default)
- [ ] Allow merge commits (off)
- [ ] Allow rebase merging (off — Stacked PR은 별도 가이드)

`Settings → Branches → Branch protection rule (main)`:
- Require pull request reviews before merging — **1 approval** (사람만, AI bot review는 카운트 제외)
- Require status checks to pass — `lint / branch-name`, `lint / pr-meta`, `ci / test`
- Require linear history (on)
- Restrict who can push to matching branches — bot 계정 제외

`Settings → Code security → Code review limits` (AI 관련):
- `ai-authored` 라벨이 붙은 PR 은 auto-merge 비활성화 (§3.5 workflow 가 자동 부착·차단).
- `auth` / `billing` 등 보안 민감 모듈은 CODEOWNERS 로 사람 reviewer 강제.

### 3.2 브랜치명 lint workflow

`.github/workflows/branch-name-lint.yml`

```yaml
name: branch-name-lint

on:
  pull_request:
    types: [opened, edited, synchronize, reopened]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - name: Validate branch name
        env:
          BRANCH: ${{ github.head_ref }}
        run: |
          pattern='^(feat|fix|chore)/(stack|cicd|admin|auth|o11y|product|ui|shared|infra)-[a-z0-9.-]{2,60}$'
          if [[ ! "$BRANCH" =~ $pattern ]]; then
            echo "::error::Branch '$BRANCH' violates Nullus convention."
            echo "Expected: <feat|fix|chore>/<module>-<short-desc>"
            echo "Modules : stack|cicd|admin|auth|o11y|product|ui|shared|infra"
            exit 1
          fi
          echo "OK: $BRANCH"
```

### 3.3 Stale PR / 브랜치 정리 workflow

`.github/workflows/stale.yml` — PR 정리 (`actions/stale`)

```yaml
name: stale

on:
  schedule:
    - cron: '0 1 * * *'   # daily 01:00 UTC
  workflow_dispatch:

permissions:
  pull-requests: write
  issues: write

jobs:
  stale:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/stale@v9
        with:
          days-before-pr-stale: 14
          days-before-pr-close: 16   # 14 + 16 = 30
          stale-pr-label: stale
          stale-pr-message: |
            이 PR은 14일간 활동이 없어 stale 처리되었습니다.
            16일 내 업데이트 없으면 자동 close 됩니다 (브랜치는 유지).
          close-pr-message: |
            30일간 비활동으로 자동 close 합니다. 재개하려면 reopen 하세요.
          exempt-pr-labels: 'pinned,security,wip-pinned'
          days-before-issue-stale: -1
          days-before-issue-close: -1
```

`.github/workflows/cleanup-stale-branches.yml` — 머지된/오래된 브랜치 삭제

```yaml
name: cleanup-stale-branches

on:
  schedule:
    - cron: '0 2 * * 1'   # weekly Mon 02:00 UTC
  workflow_dispatch:

permissions:
  contents: write
  issues: write

jobs:
  cleanup:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }

      - name: Delete merged branches older than 7 days
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          git fetch --prune origin
          cutoff=$(date -u -d '7 days ago' +%s)
          for ref in $(git for-each-ref --format='%(refname:short)|%(committerdate:unix)' \
                       refs/remotes/origin --merged origin/main); do
            name="${ref%%|*}"
            ts="${ref##*|}"
            [[ "$name" == "origin/main" || "$name" == "origin/HEAD" ]] && continue
            short="${name#origin/}"
            if (( ts < cutoff )); then
              echo "Deleting merged stale: $short"
              gh api -X DELETE "repos/${{ github.repository }}/git/refs/heads/$short" || true
            fi
          done

      - name: Notify unmerged stale branches (>60d)
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          cutoff=$(date -u -d '60 days ago' +%s)
          {
            echo "다음 브랜치는 60일 이상 commit 이 없습니다."
            echo "7일 내 업데이트 없으면 다음 주 실행에서 삭제됩니다."
            echo ""
            for ref in $(git for-each-ref \
                          --format='%(refname:short)|%(committerdate:unix)|%(authorname)' \
                          refs/remotes/origin --no-merged origin/main); do
              name="${ref%%|*}"
              rest="${ref#*|}"
              ts="${rest%%|*}"
              author="${rest##*|}"
              [[ "$name" == "origin/main" || "$name" == "origin/HEAD" ]] && continue
              if (( ts < cutoff )); then
                printf -- "- \`%s\` (last commit: %s, author: %s)\n" \
                  "${name#origin/}" "$(date -u -d @$ts +%F)" "$author"
              fi
            done
            echo ""
            echo "유지가 필요하면 \`wip-pinned\` 라벨로 보호하세요."
          } > /tmp/cleanup-body.md
          if grep -q '^- ' /tmp/cleanup-body.md; then
            gh issue create \
              --title "[branch-cleanup] 60일 이상 비활동 미머지 브랜치 검토 요청" \
              --label "branch-cleanup" \
              --body-file /tmp/cleanup-body.md
          fi
```

### 3.5 PR meta-check workflow (AI 작업자 대응)

`.github/workflows/pr-meta-check.yml` — PR description 의 `Dispatcher:` 명시 여부와 AI commit trailer 감지로 `ai-authored` 라벨을 자동 부착/차단합니다.

```yaml
name: pr-meta-check

on:
  pull_request:
    types: [opened, edited, synchronize, reopened, ready_for_review]

permissions:
  pull-requests: write
  contents: read

jobs:
  meta:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
          ref: ${{ github.event.pull_request.head.sha }}

      - name: Detect AI authorship
        id: detect
        env:
          BASE: ${{ github.event.pull_request.base.sha }}
          HEAD: ${{ github.event.pull_request.head.sha }}
        run: |
          ai=false
          if git log "$BASE..$HEAD" --format='%(trailers:key=AI-Tool,valueonly)%(trailers:key=Co-Authored-By,valueonly)' \
             | grep -Eiq 'claude|copilot|cursor|gpt|gemini|anthropic'; then
            ai=true
          fi
          echo "ai=$ai" >> "$GITHUB_OUTPUT"

      - name: Require Dispatcher tag when AI-authored
        if: steps.detect.outputs.ai == 'true'
        env:
          BODY: ${{ github.event.pull_request.body }}
        run: |
          if ! grep -Eiq '^Dispatcher:[[:space:]]*@[A-Za-z0-9_-]+' <<<"$BODY"; then
            echo "::error::AI-authored PR requires 'Dispatcher: @<github-handle>' on the first line of the description."
            exit 1
          fi

      - name: Apply ai-authored label
        if: steps.detect.outputs.ai == 'true'
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          gh pr edit ${{ github.event.pull_request.number }} --add-label "ai-authored"

      - name: Block auto-merge for ai-authored
        if: steps.detect.outputs.ai == 'true'
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          gh pr merge ${{ github.event.pull_request.number }} --disable-auto || true
```

이 workflow 는:
- AI commit trailer(`AI-Tool`, `Co-Authored-By` 에 claude/copilot/cursor/gpt/gemini/anthropic 포함) 감지 시 `ai-authored` 라벨 부착
- AI PR 에 `Dispatcher: @<handle>` 가 없으면 CI fail (머지 차단)
- AI PR 의 auto-merge 비활성화 (사람 approve 강제)

### 3.6 CODEOWNERS (보안 민감 모듈)

`.github/CODEOWNERS`

```
# 보안 민감 모듈은 사람 리뷰 강제
internal/auth/      @cloud-nullus/security-reviewers
internal/admin/     @cloud-nullus/admin-reviewers
.github/workflows/  @cloud-nullus/devops
docs/20_개발가이드/  @cloud-nullus/maintainers
```

### 3.7 로컬 정리 가이드

`docs/20_개발가이드/Nullus_PR_커밋_컨벤션.md` §7 끝에 1줄 추가:

```bash
# 원격 삭제 반영 + 로컬 stale 브랜치 정리
git fetch --prune
git branch -vv | awk '/: gone]/{print $1}' | xargs -r git branch -D
```

---

## 4. 현 origin 브랜치 정리 리스트 (Option 3)

기준일: **2026-05-11** / 총 25개 (origin/main, origin/HEAD 제외)

### 4.1 머지됨 → 즉시 삭제 가능 (21개)

> 모두 `origin/main`에 머지 완료. 자동 삭제 워크플로우 도입 후 일괄 정리하거나, 도입 전 수동 삭제.

| # | 브랜치 | 마지막 커밋 | 작성자 |
|---|---|---|---|
| 1 | `feat/openbao-token-rotation-implementation` | 2026-05-11 | devlos0322 |
| 2 | `feat/f8-compatibility-matrix` ⚠️ 컨벤션 외 | 2026-04-20 | qmin |
| 3 | `fix/o11y/deploy-runtime-readiness-retries` | 2026-04-19 | devlos0322 |
| 4 | `fix/o11y/stack-lifecycle-history-gateway` | 2026-04-19 | devlos |
| 5 | `fix/o11y/install-stability-gateway-runner` | 2026-04-18 | devlos0322 |
| 6 | `fix/o11y/remove-sample-seed-data-detail` | 2026-04-18 | devlos0322 |
| 7 | `fix/cicd/detail-tabs-review` | 2026-04-02 | qmin |
| 8 | `fix/observability/real-monitoring-data` ⚠️ 모듈명 `observability` (v2에서 `o11y`로 통일) | 2026-04-02 | qmin |
| 9 | `feat/stack/history-log-persistence-ux` | 2026-03-31 | devlos0322 |
| 10 | `chore/cleanup-deleted-and-untracked-files` | 2026-03-30 | qmin |
| 11 | `docs/architecture-v0.2-claude` | 2026-03-30 | qmin |
| 12 | `fix/shared/runbook-kind-unbound-variable` | 2026-03-30 | devlos0322 |
| 13 | `feat/cicd/ux-improvements` | 2026-03-29 | KyuMin Jeong |
| 14 | `feat/auth/local-oidc-test-env` | 2026-03-29 | miyoung |
| 15 | `feat/cicd/deploy-progress-ui` | 2026-03-29 | KyuMin Jeong |
| 16 | `feat/cicd/real-k8s-deploy` | 2026-03-28 | KyuMin Jeong |
| 17 | `feat/stack/monitoring-patch-phase1` | 2026-03-28 | devlos0322 |
| 18 | `test/uat-role-scenarios` | 2026-03-22 | KyuMin Jeong |
| 19 | `feat/phase-2` ⚠️ 컨벤션 외 | 2026-03-22 | KyuMin Jeong |
| 20 | `docs/implementation-guides` | 2026-03-22 | KyuMin Jeong |
| 21 | `docs/product/user-role-scenarios` | 2026-03-22 | KyuMin Jeong |

### 4.2 미머지 → 작성자 확인 필요 (2개)

| # | 브랜치 | 마지막 커밋 | 작성자 | 권고 |
|---|---|---|---|---|
| 1 | `fix/cicd/infinite-render-loop` | 2026-03-29 (43일) | KyuMin Jeong | PR 상태 확인 후 (a) 재개해서 머지 (b) close 후 브랜치 삭제 선택 |
| 2 | `fix/phase2-bugfix-and-seed-data` | 2026-03-24 (48일) | KyuMin Jeong | 동일. `feat/phase-2`(머지됨)와 관련 작업으로 보임 — 잔여 이슈가 다른 브랜치에 흡수되었는지 확인 |

### 4.3 로컬 전용 (1개)

| 브랜치 | 상태 | 권고 |
|---|---|---|
| `phase1` (로컬 only) | origin 미존재 | 사용 여부 확인 후 `git branch -D phase1` |

### 4.4 일괄 삭제 스크립트 (자동화 도입 전 수동 정리용)

```bash
# DRY-RUN: 삭제 대상 확인만
for b in \
  feat/openbao-token-rotation-implementation \
  feat/f8-compatibility-matrix \
  fix/o11y/deploy-runtime-readiness-retries \
  fix/o11y/stack-lifecycle-history-gateway \
  fix/o11y/install-stability-gateway-runner \
  fix/o11y/remove-sample-seed-data-detail \
  fix/cicd/detail-tabs-review \
  fix/observability/real-monitoring-data \
  feat/stack/history-log-persistence-ux \
  chore/cleanup-deleted-and-untracked-files \
  docs/architecture-v0.2-claude \
  fix/shared/runbook-kind-unbound-variable \
  feat/cicd/ux-improvements \
  feat/auth/local-oidc-test-env \
  feat/cicd/deploy-progress-ui \
  feat/cicd/real-k8s-deploy \
  feat/stack/monitoring-patch-phase1 \
  test/uat-role-scenarios \
  feat/phase-2 \
  docs/implementation-guides \
  docs/product/user-role-scenarios
do
  echo "would delete: $b"
done

# 실제 실행 (검토 후 echo 제거)
# for b in ... ; do git push origin --delete "$b"; done
```

---

## 5. 도입 순서

1. **PR 1 — 문서 정비**: 본 문서 추가 + v1 컨벤션을 v2 로 개정 (§2.1~2.6 반영, AI 운영 지침 §2.5 와 사람 책임 §2.6 포함)
2. **PR 2 — 기본 자동화**: `.github/workflows/branch-name-lint.yml`, `stale.yml`, `cleanup-stale-branches.yml` 추가 + 저장소 설정(§3.1) 변경
3. **PR 3 — AI 작업자 자동화**: `.github/workflows/pr-meta-check.yml`, `.github/CODEOWNERS` 추가, `ai-authored` 라벨 생성, `.gitignore` 에 `.omc/`·`.claude/state/`·`MEMORY.md`·`*.plan.md` 등록
4. **PR 4 — 기존 브랜치 정리**: §4.1 일괄 삭제, §4.2 작성자에게 작업 처리 의뢰
5. **운영 안착 후 회고**: 1개월 후 브랜치 평균 수명·origin 브랜치 수·AI PR dispatcher 표기율 측정

---

## 6. 측정 지표

### 6.1 브랜치 위생 (전체)

| 지표 | v1 운영(2026-05) | v2 목표(도입 3개월 후) |
|---|---|---|
| origin 활성 브랜치 수 | 25 | ≤ 8 (≈ active contributor 수) |
| 머지 후 origin 잔존 브랜치 수 | 21 | 0 |
| 컨벤션 외 브랜치 비율 | 12% (3/25) | 0% (lint 차단) |
| 모듈명 표기 불일치 | 1건 (`observability`/`o11y`) | 0건 |
| PR 평균 lifetime (open → merge) | 미측정 | ≤ 5 영업일 |

### 6.2 AI 작업자 (§2.5 적용 후)

| 지표 | 목표 |
|---|---|
| AI 작성 PR 중 `Dispatcher:` 표기율 | 100% (없으면 CI fail) |
| AI 작성 PR 중 `ai-authored` 라벨 부착률 | 100% (workflow 자동) |
| AI 작성 PR 의 사람 approve 비율 | 100% (auto-merge 차단) |
| `auth`/`admin` 모듈에 AI 단독 머지 발생 | 0건 (CODEOWNERS 차단) |
| 메타데이터(`.omc/`, `MEMORY.md` 등) commit | 0건 (`.gitignore` 차단) |
| Scope creep 으로 인한 PR 분리 재요청 | 분기당 ≤ 2건 |

### 6.3 사람 책임 영역 (§2.6 적용 후 / 병목 감시)

AI 가 다수 작업자인 환경에서 사람이 병목이 되지 않는지 측정합니다.

| 지표 | 목표 | 대응 |
|---|---|---|
| Dispatcher review lead time (AI push → 사람 첫 코멘트 또는 approve) | ≤ 48 시간 | 초과 시 stale 자동 라벨 |
| AI PR 평균 changed lines | ≤ 500 라인 | 초과 시 분리 요청 |
| Dispatcher 1인당 동시 진행 AI PR 수 | ≤ 5건 | 초과 시 다른 dispatcher 위임 |
| 사람 reviewer 가 도메인·아키텍처 결함 발견 비율 | ≥ 20% | 너무 낮으면 review 형식화 의심 |
| 보안 모듈 (`auth`/`admin`/`billing`) PR 의 maintainer approve 누락 | 0건 | CODEOWNERS 위반 알림 |
| 본 문서·v1 컨벤션·workflow 변경 PR 의 maintainer approve 누락 | 0건 | CODEOWNERS 위반 알림 |
| 60일 미머지 issue 가 7일 내 결정되지 않은 비율 | ≤ 10% | maintainer 백로그 알림 |
