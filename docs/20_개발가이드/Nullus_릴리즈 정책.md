# Nullus 릴리즈 정책

Nullus Platform(`cloud-nullus/nullus`)의 GitHub Release · CHANGELOG · 버전(SemVer) 관리 규칙을 정의합니다.

- 초안: 2026-07-25 / 개정: 2026-07-26 (v2) / 2026-07-28 (v2.1) / 2026-08-07 (v2.2) / 2026-08-31 (v2.3)
- 상태: **적용 중** — 태그 3건(`v0.3.0-alpha`·`v0.4.0-alpha`·`v0.4.1`)을 이 문서 기준으로 발행했습니다. 남은 과제는 §13에 있습니다.
- v2.3 개정 사유: `cd.yml`에 `create-release` 잡이 들어오면서 **§9.1의 9·10단계(Release 수동 생성·prerelease 체크)가 자동화**됐는데, 절차는 사람이 하는 것으로 남아 있었습니다. 그대로 따르면 자동 생성본과 충돌합니다. 실제 릴리즈에서 어긋난 지점(`v0.4.1`의 compare 링크·`Chart.yaml` 누락, 접미사를 떼면서 생긴 `0.4` 롤링 태그)도 함께 반영합니다.
- v2.2 개정 사유: `v0.3.0-alpha`를 실제로 릴리즈하고 실 클러스터에 배포해 보니, 문서대로 밟았는데도 산출물이 설치되지 않았습니다. 원인을 §7.1(패키지 가시성)과 §9.1의 실 클러스터 검증(v2.3 이후 D-9)에 반영했습니다.
- 적용 대상: `cloud-nullus/nullus` (구 `cloud-nullus/draft`, 2026-05-25경 리네임)
- 관련 문서: `Nullus_PR_커밋_컨벤션.md`, `Nullus_브랜치_관리_개선안.md`, `Nullus_CICD 흐름.md`, `CLAUDE.md`

---

## 0. 개정 요약 (v1 → v2)

v1은 "이렇게 운영 중"이라고 서술했으나, 실제로는 **태그·Release가 하나도 없고 CHANGELOG도 2026-05-17 이후 정지**한 상태였습니다. v2는 서술을 실제 상태에 맞추고, 비어 있던 결정을 채웁니다.

| # | 항목 | v1 (초안) | v2 (개정안) | 근거 |
|---|---|---|---|---|
| 1 | 저장소·이미지 경로 | `cloud-nullus/draft`, `ghcr.io/cloud-nullus/draft/nullus-*` | `cloud-nullus/nullus`, `ghcr.io/cloud-nullus/**nullus**/nullus-*` | 2026-05-25경 리네임, 경로 교정은 #78(06-09 작성 / 07-25 머지). `github.repository`가 2세그먼트라 이미지 경로에 `nullus`가 두 번 들어감 |
| 2 | 릴리즈 이력 표기 | `0.1.0-alpha`·`0.2.0-alpha` **완료** | **기록만 존재(태그 미발행)** | `git tag`·`gh release list` 모두 0건 |
| 3 | CHANGELOG | "현재 운영 방식을 문서화" | **재개 선언** + 누락분 소급 정리 완료 + PR 템플릿 체크박스로 강제 (§4.2) | PR #54(2026-05-23) 이후 37건 미반영이었음 |
| 4 | Helm 차트 배포 | 산출물로 지목만 하고 게시 경로 없음 | **ghcr OCI push**를 `cd.yml`에 추가 (§7.2) | clone 없이는 차트 입수 불가였음 |
| 5 | 차트 이미지 기본값 | `ghcr.io/cloud-nullus/nullus-api:0.1.0-alpha` (로컬 빌드 플레이스홀더) | **배포 가능한 실제 경로**로 변경, `tag`는 비워 `Chart.AppVersion` 폴백. 로컬용은 `values-dev.yaml`로 분리 (§8.1) | 현 기본값은 어떤 실재 이미지도 가리키지 않음 |
| 6 | 버전 동기화 위치 | 3곳 (태그/CHANGELOG/Chart.yaml) | **동일 3곳 + values.yaml `tag` 제거로 동기화 지점 축소** | `deployment.yaml`이 `tag \| default .Chart.AppVersion` 폴백을 이미 지원 |
| 7 | 릴리즈 품질 게이트 | 없음 | **`ci.yml` 재활성화 → main ruleset 필수 체크 등록** (§9.0) | `ci.yml`이 `disabled_manually`, 마지막 실행 2026-03-15 실패 |
| 8 | 머지 전략 | "Squash 기본"이라 서술 (미강제) | **Squash 단일화 + ruleset `allowed_merge_methods` 제한** (§10.2) | 실제로는 전부 merge commit, ruleset은 3종 모두 허용 |
| 9 | 태그 보호 | "main에서만 생성" (미강제) | **tag ruleset 신설** (§10.3) | `target: tag` ruleset 0건 — 아무 브랜치에서나 태그 push 가능 |
| 10 | 롤백 | 목적으로만 언급 | **절차 신설** (§9.2), DB 마이그레이션 가역성과 연결 | §6.1이 비가역 마이그레이션을 MAJOR 기준으로 삼고 있었음 |
| 11 | 차트 version/appVersion | 이름만 언급 | **증가 규칙 명시** (§8.2) | 차트만 바뀐 경우 규칙 부재 |
| 12 | 책임 주체 | 없음 | **릴리즈 담당·승인 게이트 명시** (§9.3) | `required_approving_review_count: 0` |
| 13 | 릴리즈 브랜치 | 없음 | **GA 시점 재검토로 명시** (§9.4) | 0.x 구간에서는 불필요하나 공백을 남기지 않기 위함 |

유지한 것: §6.1 Breaking Change 판단 기준, §7.1 프리릴리즈 롤링 태그 지적, §4.1 언어 규칙 — v1에서 가장 잘 작성된 부분이며 그대로 가져갑니다.

---

## 1. GitHub Release를 관리하는 이유

Nullus는 릴리즈마다 아래 3가지 산출물이 함께 나갑니다.

| 산출물 | 경로 |
|---|---|
| API 이미지 | `ghcr.io/cloud-nullus/nullus/nullus-api` |
| Web 이미지 | `ghcr.io/cloud-nullus/nullus/nullus-web` |
| Helm 차트 | `oci://ghcr.io/cloud-nullus/charts/nullus` (§7.2로 신설) |

> **세 산출물은 모두 ghcr 패키지 가시성이 `public`이어야 합니다.** 저장소가 public이어도 패키지는 **기본 private으로 생성**되며, private이면 위 경로는 익명 pull이 되지 않아 사실상 산출물이 아닙니다. 전환은 REST API로 불가능하고 `https://github.com/orgs/cloud-nullus/packages` 아래 **패키지별 Settings → Danger Zone**에서만 됩니다 — 저장소 Settings의 "Make … private"과 혼동하면 저장소 자체를 비공개로 만들게 되니 주의하십시오. 확인은 §9.1 D-9에서 합니다.
>
> 경로에 `nullus`가 두 번 들어가는 것은 오타가 아닙니다. `cd.yml`이 `ghcr.io/${{ github.repository }}/nullus-api`를 쓰고 `github.repository`가 `cloud-nullus/nullus`이기 때문입니다. 리네임 이전 경로(`ghcr.io/cloud-nullus/draft/*`)의 패키지도 ghcr에 남아 있으나 **더 이상 갱신되지 않으므로 참조 금지**입니다 — 이 혼동이 실제 로그인 장애를 일으킨 적이 있습니다(#78).

사용자는 "어떤 버전을 클러스터에 설치했는지"를 정확히 알아야 지원·롤백·업그레이드가 가능하므로, 태그 기반 Release 관리가 특히 중요합니다.

| 목적 | Nullus 맥락 |
|---|---|
| 공식 배포 버전 관리 | 어떤 커밋이 `nullus-api`/`nullus-web` 이미지로 실제 빌드·배포되었는지 명확화 |
| 롤백 | 문제가 생기면 이전 태그의 이미지 + 차트로 되돌린다 (절차는 §9.2) |
| 다운로드/배포 | 소스, ghcr 이미지, Helm 차트를 태그 기준으로 함께 배포 |
| 자동 배포(CD) | `v*` 태그 push 시 `.github/workflows/cd.yml`이 이미지·차트를 자동 빌드·푸시 |
| 변경 이력 공유 | Stack/CI-CD/Admin/Auth/O11y 등 모듈별 변경사항을 사용자·운영팀에 전달 |

---

## 2. CHANGELOG를 관리하는 이유

Nullus는 루트에 [`CHANGELOG.md`](../../CHANGELOG.md)를 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) 형식으로 두고 있습니다.

> **경위**: CHANGELOG를 마지막으로 갱신한 것은 PR #54(2026-05-23 머지, 커밋 `66bf2c0`)였고, 이후 머지된 PR 37건이 `Unreleased`에 반영되지 않은 채 두 달간 방치되었습니다. 따라서 이 절은 "현행 방식의 문서화"가 아니라 **운영 재개 선언**입니다. 누락분은 2026-07-26 소급 정리해 `0.3.0-alpha` 섹션으로 편입했으며(§13-1), 재발 방지 장치는 §4.2에 둡니다.

| 이해관계자 | 이점 |
|---|---|
| 사용자(운영자) | 업그레이드할 가치가 있는지, Breaking Change가 있는지 판단 |
| QA | 이번 릴리즈에서 무엇을 회귀 테스트해야 하는지 확인 |
| 개발자 | 어떤 버전에 어떤 모듈 변경이 들어갔는지 추적 |
| 운영/SRE | 장애 발생 시 어느 버전부터 문제가 생겼는지 원인 분석 |

---

## 3. Release와 CHANGELOG의 관계

Release마다 CHANGELOG의 해당 버전 섹션을 그대로 Release Note 본문으로 사용합니다.

```
main
 ├── commit
 ├── commit
 ├── v0.3.0-alpha Release  ── CHANGELOG.md [0.3.0-alpha] 섹션과 1:1 대응   ← 2026-07-27 발행
 └── commit (Unreleased)
```

> `0.1.0-alpha`(2026-03-15)·`0.2.0-alpha`(2026-03-28)는 **CHANGELOG에 섹션만 존재하고 git 태그·GitHub Release는 발행된 적이 없습니다.** 소급 태깅은 해당 시점 커밋을 특정하기 어렵고 이미지도 남아 있지 않으므로 하지 않습니다. 1:1 대응은 `0.3.0-alpha`부터 시작됐습니다.

CHANGELOG.md 하단의 compare 링크는 **실제로 존재하는 태그만** 가리켜야 합니다. 발행되지 않은 태그를 가리키는 링크는 404가 되므로 두지 않습니다.

```markdown
[unreleased]: https://github.com/cloud-nullus/nullus/compare/v0.4.1...HEAD
[0.4.1]: https://github.com/cloud-nullus/nullus/compare/v0.4.0-alpha...v0.4.1
[0.4.0-alpha]: https://github.com/cloud-nullus/nullus/compare/v0.3.0-alpha...v0.4.0-alpha
[0.3.0-alpha]: https://github.com/cloud-nullus/nullus/releases/tag/v0.3.0-alpha
```

> **compare 링크는 두 군데에 따로 있습니다 (v2.3).** 위의 `CHANGELOG.md` 하단 링크는 **사람이** 릴리즈
> 절차에서 갱신하고(§9.1 A-4), GitHub Release 본문의 `[<직전 태그> 이후 변경된 코드]` 링크는
> `create-release`가 `git describe`로 직전 태그를 찾아 **자동으로** 붙입니다. 자동 쪽이 붙는다고 해서
> `CHANGELOG.md` 쪽이 갱신되지는 않습니다 — `v0.4.1`에서 실제로 빠졌습니다(§13-12).

---

## 4. CHANGELOG 작성 규칙

Keep a Changelog 표준 카테고리 중 Nullus가 실제로 사용하는 것은 다음과 같습니다.

- `Added` — 신규 기능/엔드포인트/페이지
- `Changed` — 기존 동작·UI·API 응답 변경
- `Fixed` — 버그 수정
- `Deprecated` / `Removed` — 필요 시 사용
- `Security` — **보안 민감 모듈(`auth`, `admin`) 변경**은 반드시 이 카테고리로 별도 기재

### 4.1 작성 언어

**CHANGELOG 엔트리와 GitHub Release 설명은 한글로 작성한다.** 카테고리 헤더(`Added`/`Changed`/`Fixed`/`Security` 등 Keep a Changelog 표준 키워드)와 API 경로·함수명·파일 경로 같은 코드 식별자만 원문(영문)을 그대로 쓰고, 나머지 설명 문장은 한글로 작성한다.

- PR 제목·커밋 메시지는 기존 컨벤션(`Nullus_PR_커밋_컨벤션.md`)을 그대로 따른다 — 이 규칙은 CHANGELOG/Release 설명에만 적용된다.
- GitHub "Generate release notes" 자동 생성 결과는 PR 제목을 영문 그대로 나열하므로, §5에 따라 한글 CHANGELOG 섹션을 우선하고 자동 생성 목록은 보조 링크로만 첨부한다.

```markdown
## [0.3.0-alpha] - 2026-08-XX

### Added
- **Compatibility Matrix CRUD Admin UI**: `admin` 모듈에 매트릭스 생성/수정/삭제 API 및 화면 추가

### Security
- OpenBao 토큰 조회 API에 조직 단위 권한 검증 강화
```

### 4.2 작성 시점과 강제 수단 (v2 신설)

v1은 CHANGELOG 갱신을 "PR 작성자의 의무"로만 규정했고 강제 수단이 없어 **38개 PR 동안 지켜지지 않았습니다.** v2는 다음을 둡니다.

1. PR 본문의 `## Changes`를 작성할 때 **같은 PR에서** `CHANGELOG.md`의 `## [Unreleased]`에 사용자 관점 문장으로 옮겨 적는다. 머지 후로 미루지 않는다.
2. `.github/pull_request_template.md`에 체크박스를 추가한다.
   ```markdown
   - [ ] CHANGELOG.md `Unreleased` 갱신 (해당 없으면 `no-changelog` 라벨)
   ```
3. 사용자 영향이 없는 변경(내부 리팩터링, 테스트, 문서, CI 설정)은 `no-changelog` 라벨로 면제한다.
4. 커밋 타입(`feat`/`fix`/`refactor`/...)과 CHANGELOG 카테고리(`Added`/`Changed`/`Fixed`/...)는 1:1로 매핑되지 않는다 — 사용자 관점에서 "무엇이 바뀌었는가"를 기준으로 분류한다.

> **CI 강제 완료 (2026-08-09)**: 체크박스는 리뷰어가 보는 장치일 뿐 차단 장치가 아니어서, 실제로 #114가 CHANGELOG 없이 머지됐습니다. `Lint Review` 워크플로의 `📝 CHANGELOG Check`(`scripts/check_changelog.py`)가 이제 차단합니다 — 동작이 바뀌는 파일을 고치고 `CHANGELOG.md`를 건드리지 않으면 실패하고, `no-changelog` 라벨로 면제됩니다. 문서·테스트 전용 변경은 라벨 없이도 자동 면제됩니다. 같은 검사가 릴리즈 절단 후 되머지에서 생기는 `[Unreleased]`↔릴리즈 섹션 중복도 잡습니다(실제 발생). 작성 규칙은 `Nullus_PR_커밋_컨벤션.md` §4.

---

## 5. Release 본문은 누가 쓰는가 (v2.3 개정)

**Release 본문은 사람이 쓰지 않습니다.** `cd.yml`의 `create-release` 잡이 `CHANGELOG.md`에서
`## [X.Y.Z]` 섹션을 찾아 본문으로 싣고, 그 앞에 설치 명령·산출물 경로·문서 링크를 붙입니다.

```
릴리즈 본문 = [prerelease 경고] + 설치 + 산출물 표 + 문서 링크 + 직전 태그 compare
              + '## 변경 내역' + CHANGELOG 의 [X.Y.Z] 섹션 원문
```

GitHub Release 화면의 **"Generate release notes"**(머지된 PR·기여자 목록 자동 나열)는 이제
**폴백 경로**입니다 — `create-release`가 CHANGELOG에서 해당 버전 섹션을 **찾지 못했을 때만**
`generate_release_notes: true`로 넘어갑니다. 사람이 누르는 버튼이 아닙니다.

> **폴백은 실패가 아니라 조용한 열화입니다.** 섹션이 없어도 워크플로는 성공하고, PR 제목이
> 영문 그대로 나열된 Release가 남습니다(§4.1 언어 규칙 위반). 그래서 §9.1은 CHANGELOG 절단을
> **태그 push보다 앞**에 둡니다.

---

## 6. 버전 번호 규칙 (SemVer)

Nullus는 [Semantic Versioning](https://semver.org/spec/v2.0.0.html)을 따릅니다.

```
MAJOR . MINOR . PATCH [-PRERELEASE]
  1  .   4   .   2      예: 1.4.2
```

| 자릿수 | 위치 | 이름 | 증가 조건 | 증가 시 하위 자릿수 | Nullus 예시 |
|---|---|---|---|---|---|
| 1번째 | `X`.0.0 | MAJOR | 기존 버전과 호환되지 않는 Breaking Change 도입 | MINOR, PATCH 모두 `0`으로 초기화 | REST API 응답 스키마 변경, DB 마이그레이션 비가역 변경, Helm `values.yaml` 필수 키 삭제 |
| 2번째 | 0.`X`.0 | MINOR | 기존 호환성을 유지하며 새 기능 추가 | PATCH만 `0`으로 초기화 | 신규 모듈 기능(`stack`/`cicd`/`o11y` 등) 추가, 신규 엔드포인트 추가 |
| 3번째 | 0.0.`X` | PATCH | 호환성에 영향 없는 버그 수정·성능 개선 | 없음 | Sizing Profile 캐시 무효화 누락 수정 등 |

**버전 변경 예시**

| 변경 전 | 변경 사유 | 변경 후 |
|---|---|---|
| `1.2.3` | 버그 수정만 반영 | `1.2.4` (PATCH만 +1) |
| `1.2.4` | 호환 유지되는 신규 기능 추가 | `1.3.0` (MINOR +1, PATCH는 0으로 리셋) |
| `1.3.0` | Breaking Change 도입 | `2.0.0` (MAJOR +1, MINOR·PATCH 모두 0으로 리셋) |

같은 릴리즈에 버그 수정과 신규 기능이 함께 포함되면, **가장 상위 자릿수 기준(MINOR > PATCH)**으로 한 번만 올린다 — PATCH를 먼저 올리고 다시 MINOR를 올리지 않는다.

### 6.1 Breaking Change 판단 기준 (Nullus 기준)

아래 중 하나라도 해당하면 MAJOR 상향 대상입니다.

- REST API 응답 필드 제거/타입 변경 (필드 추가는 해당 없음)
- 되돌릴 수 없는(non-reversible) DB 마이그레이션 → §9.2 롤백 절차와 직접 연결됩니다
- `deploy/helm/nullus/values.yaml`의 필수 키 삭제·이름 변경
- 모듈 간 공개 인터페이스(`port/`) 시그니처 변경으로 인해 의존 모듈이 함께 수정되어야 하는 경우
- Compatibility Matrix의 검증 시맨틱 변경으로 기존 verified 조합이 fail로 바뀌는 경우

### 6.2 개발 단계 버전(0.x.x)과 프리릴리즈 접미사

v1.0.0(GA) 이전에는 `0.x.x`를 사용하며, 상태에 따라 접미사를 붙입니다.

```
0.1.0-alpha   내부 개발 단계, 기능이 자주 변경됨
0.2.0-alpha
...
0.9.0-beta    외부 테스트 가능
1.0.0-rc.1    정식 출시 직전 후보
1.0.0         정식 출시(GA)
1.0.1         GA 이후 패치
```

`0.x.x` 구간에서는 SemVer 상 MINOR 상향도 Breaking Change를 포함할 수 있다는 점(SemVer §4)을 감안해, Breaking Change가 있으면 CHANGELOG `Added`/`Changed` 항목 앞에 `**BREAKING**` 표기를 남깁니다.

**접미사를 뗄 수 있습니다 — 다만 이미지 태그가 달라집니다 (v2.3 신설).**

`v0.4.1`은 접미사 없이 발행했습니다. 그래도 GitHub Release는 pre-release로 표시됩니다 —
`create-release`는 접미사뿐 아니라 **MAJOR가 `0`이면 prerelease로 판정**하기 때문입니다.
prerelease 표시는 접미사에 의존하지 않으니, 접미사를 떼도 GA로 오인될 위험은 없습니다.

바뀌는 것은 **이미지 롤링 태그**입니다.

| 태그 | 생성되는 이미지 태그 | GitHub Release |
|---|---|---|
| `v0.4.0-alpha` | `0.4.0-alpha` | Pre-release |
| `v0.4.1` | `0.4.1` **+ `0.4` (롤링)** | Pre-release |

`docker/metadata-action`이 프리릴리즈에는 `{{major}}.{{minor}}` 패턴을 만들지 않기 때문입니다(§7.1).
ghcr에 `0.4` 태그가 실재하는 것이 그 결과이고, **접미사를 뗀 첫 릴리즈에서 예고 없이 생겼습니다.**
롤링 태그는 다음 패치에서 말없이 다른 이미지를 가리키므로, `0.x`에서 접미사를 떼기로 했다면
§7.1의 "배포에는 patch까지 명시된 전체 버전 태그를 쓴다"를 함께 지켜야 합니다.

---

## 7. 태그 → CD 파이프라인 연동

### 7.1 이미지 (현행)

`v*` 태그를 push하면 `.github/workflows/cd.yml`이 **세 이미지**를 빌드해 ghcr에 푸시합니다.

```yaml
on:
  push:
    tags:
      - 'v*'
    branches:
      - main
```

`docker/metadata-action`이 다음 규칙으로 이미지 태그를 생성합니다.

| 패턴 | 예: 태그 `v0.3.0` | 예: 태그 `v0.3.0-alpha` |
|---|---|---|
| `{{version}}` | `0.3.0` | `0.3.0-alpha` |
| `{{major}}.{{minor}}` | `0.3` (rolling) | 생성 안 됨 — 프리릴리즈는 major.minor rolling 태그 대상 아님 |
| `type=ref,event=branch` | `main` (main push 시) | 동일 |
| `type=sha` | 커밋 short SHA | 동일 |

**따라서 alpha/beta/rc 단계에서는 `0.3` 같은 rolling 태그가 만들어지지 않으므로, 배포 시 반드시 patch까지 명시된 전체 버전 태그(`0.3.0-alpha` 등)를 사용해야 합니다.** `0.3`/`latest` 같은 rolling 태그에 의존한 배포는 GA 이후에만 안전합니다.

> **접미사 없는 `0.x` 태그는 이 규칙의 예외가 아니라 대상입니다 (v2.3).** `v0.4.1`이 프리릴리즈가
> 아니라서 `0.4` 롤링 태그가 만들어졌습니다. 2026-08-31 기준 ghcr `nullus-api`의 버전 태그는
> `0.3.0-alpha`·`0.4.0-alpha`·`0.4.1`·**`0.4`** 입니다. 배포 매니페스트가 `0.4`를 가리키면
> 다음 패치에서 말없이 다른 이미지로 바뀝니다 — §6.2를 함께 보십시오.

**세 번째 이미지는 릴리즈 버전을 따르지 않습니다.** `build-and-push-jenkins`는 스택이 설치하는
Jenkins 커스텀 이미지를 빌드하는데, 태그를 릴리즈 버전이 아니라
`deploy/images/jenkins/Dockerfile`의 `ARG JENKINS_VERSION` 값에서 뽑습니다.

| 이미지 | 태그 기준 | 태그 push 전용? |
|---|---|---|
| `nullus-api` | 릴리즈 태그 (`docker/metadata-action`) | 아니오 — `main` push 시에도 `main`·SHA 태그로 빌드 |
| `nullus-web` | 릴리즈 태그 (`docker/metadata-action`) | 아니오 — 위와 같음 |
| `nullus-jenkins` | `Dockerfile`의 `JENKINS_VERSION` | 아니오 — 릴리즈와 무관하게 갱신 |

따라서 `nullus-jenkins`는 릴리즈 산출물 표(§1)에 넣지 않고, 버전 동기화 대상(§8)에서도 제외합니다.
Jenkins를 올리는 것은 릴리즈가 아니라 그 `ARG` 값을 바꾸는 PR입니다.

### 7.2 Helm 차트 게시 (v2 신설)

v1은 차트를 산출물로 지목했지만 게시 경로가 없어, 사용자가 저장소를 clone하지 않으면 차트를 받을 수 없었습니다. **이미지와 동일한 ghcr·동일한 인증을 쓰는 OCI 레지스트리 방식**을 채택합니다 (별도 gh-pages 운영 불필요, Helm 3.8+ 정식 지원).

`cd.yml`에 태그 push 시에만 도는 잡을 추가합니다.

```yaml
  publish-chart:
    name: Package & Push Helm Chart
    if: startsWith(github.ref, 'refs/tags/v')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: azure/setup-helm@v4
      - name: Log in to ghcr
        run: helm registry login ghcr.io -u ${{ github.actor }} --password ${{ secrets.GITHUB_TOKEN }}
      - name: Package & push
        run: |
          VERSION="${GITHUB_REF_NAME#v}"
          helm dependency update deploy/helm/nullus
          helm package deploy/helm/nullus --version "$VERSION" --app-version "$VERSION"
          helm push "nullus-${VERSION}.tgz" oci://ghcr.io/cloud-nullus/charts
```

설치 측 사용법:

```bash
helm install nullus oci://ghcr.io/cloud-nullus/charts/nullus --version 0.3.0-alpha
```

> `--version`/`--app-version`을 태그에서 주입하므로, `Chart.yaml`에 커밋된 값은 개발 중 기본값 역할만 합니다. 그래도 §8의 동기화 대상에서 빼지 않습니다 — 저장소를 직접 clone해 설치하는 경로가 남아 있기 때문입니다.

### 7.3 태그를 밀면 자동으로 일어나는 일 (v2.3 신설)

v2.2까지 이 문서는 이미지와 차트 게시만 자동으로 서술하고, Release 생성과 배포는 §9.1에서
사람이 하는 것으로 두었습니다. 실제로는 아래 5종이 태그 push 하나로 모두 돕니다.

| 잡 | 하는 일 | 조건 | 선행 |
|---|---|---|---|
| `build-and-push-api` | API 이미지 빌드·푸시 (amd64/arm64) | 태그·`main`·`phase1` | — |
| `build-and-push-web` | Web 이미지 빌드·푸시 (amd64/arm64) | 태그·`main`·`phase1` | — |
| `build-and-push-jenkins` | Jenkins 이미지 (§7.1 — 릴리즈 버전 아님) | 태그·`main`·`phase1` | — |
| `publish-chart` | 차트 package·push, `--version`/`--app-version`을 **태그에서 주입** | **태그만** | — |
| `create-release` | GitHub Release 생성, 본문·prerelease 자동 판정 (§5) | **태그만** | api·web·chart |
| `deploy-zadara` | Zadara Cloud 배포 (`environment: zadara`) | 태그·`main`·수동 | api·web |

읽는 방법 세 가지.

1. **`create-release`는 `publish-chart`를 기다립니다.** 차트 게시가 실패하면 Release는 생기지
   않습니다 — 태그만 남고 Release가 없다면 여기부터 봅니다.
2. **`deploy-zadara`는 `main` 머지에서도 돕니다.** 릴리즈 태그는 이 배포의 유일한 경로가 아니며,
   `environment: zadara`에 승인 게이트를 걸어 두면 태그 런도 그 게이트에서 멈춥니다.
3. **같은 커밋에 브랜치 push와 태그 push가 잇따르면** `concurrency` 설정이 앞선 브랜치 런을
   접고 태그 런을 남깁니다(태그 런이 하는 일이 상위 집합이라서). 릴리즈 PR을 머지하고 곧바로
   태그를 밀면 반드시 일어나는 일이며, 접힌 런은 실패가 아닙니다.

---

## 8. 버전이 존재하는 위치 — 동기화 체크리스트

버전 문자열의 단일 진실 공급원(source of truth)은 **git 태그**이며, 코드에는 하드코딩하지 않습니다.

| 위치 | 파일 | 비고 |
|---|---|---|
| Git 태그 | - | `vX.Y.Z[-PRERELEASE]`, CD 트리거 기준 |
| CHANGELOG 헤더 | `CHANGELOG.md` | `## [X.Y.Z] - YYYY-MM-DD` |
| Helm 차트 | `deploy/helm/nullus/Chart.yaml` | `version`(차트 버전) / `appVersion`(Nullus 앱 버전) |

> **드리프트 해소 (2026-07-26)**: `Chart.yaml`이 `version: 0.1.0` / `appVersion: "0.1.0-alpha"`로 CHANGELOG와 어긋나 있던 것을 `0.3.0` / `"0.3.0-alpha"`로 동기화했습니다.
>
> **폴백 복구 (2026-07-28, §13-3 완료)**: 위 변경만으로는 렌더 결과가 바뀌지 않았습니다. `values.yaml`이 `tag: "0.1.0-alpha"`를 고정하고 있어 `helm template` 결과가 여전히 `...:0.1.0-alpha`였기 때문입니다. §8.1대로 `tag`를 비워 `Chart.AppVersion` 폴백을 복구했고, 이제 렌더 결과는 `ghcr.io/cloud-nullus/nullus/nullus-api:0.3.0-alpha`입니다. **이것이 v1에서 `values.yaml`을 동기화 대상에서 빠뜨린 결과가 실제로 드러난 사례입니다.**

### 8.1 차트 이미지 기본값 (v2 신설)

v1은 `values.yaml`을 동기화 대상에서 빠뜨렸고, 그 결과 현재 기본값이 **어떤 실재 이미지도 가리키지 않습니다.**

```yaml
# 현재 (배포 불가 — 로컬 빌드 후 kind load 해야만 동작)
api.image.repository: ghcr.io/cloud-nullus/nullus-api    # 세그먼트 부족: 실재 경로는 .../nullus/nullus-api
api.image.tag: "0.1.0-alpha"                             # ghcr에 존재하지 않는 태그
```

v2에서는 **차트 기본값을 배포 가능한 값으로 되돌리고, `tag`는 비웁니다.**

```yaml
api:
  image:
    repository: ghcr.io/cloud-nullus/nullus/nullus-api
    tag: ""          # 비우면 deployment.yaml 이 .Chart.AppVersion 으로 폴백
web:
  image:
    repository: ghcr.io/cloud-nullus/nullus/nullus-web
    tag: ""
```

`deploy/helm/nullus/templates/deployment.yaml`은 이미 `{{ .Values.api.image.tag | default .Chart.AppVersion }}` 폴백을 쓰고 있으므로, **`tag`를 비우면 릴리즈 시 동기화할 지점이 `Chart.yaml` 한 곳으로 줄어듭니다.** v1의 3곳 동기화 부담이 실질적으로 완화됩니다.

로컬 kind 개발용 값(로컬 레지스트리 경로, `pullPolicy: Never`, `auth.mode: oidc` 등)은 차트 기본값을 덮어쓰지 말고 **`deploy/helm/nullus/values-dev.yaml`로 분리**해 `-f`로 넘깁니다. 기본값에 로컬 환경 값을 커밋하는 사고가 실제로 있었습니다.

> `docs/agent-reference.md`의 kind 로컬 빌드 안내와 `airgap/helm/README.md`의 기본값 표도 이 변경에 맞춰 함께 갱신해야 합니다(§13-3).

### 8.2 `version` vs `appVersion` 증가 규칙 (v2 신설)

| 변경 내용 | `appVersion` | `version` |
|---|---|---|
| 애플리케이션 코드 변경 (이미지 재빌드 필요) | 릴리즈 버전으로 상향 | 함께 상향 |
| 차트 템플릿·기본값만 변경 (이미지 동일) | 유지 | PATCH 상향 |
| 차트 의존성(postgresql 등) 버전 변경 | 유지 | MINOR 상향 |

릴리즈 태그에서 두 값을 함께 주입하는 §7.2 경로에서는 둘이 같아집니다. 위 규칙은 **저장소에 커밋되는 `Chart.yaml` 값**에 적용합니다.

> **차트 전용 패치는 태그로 게시하지 않습니다 (v2.3 — §13-11 결론).** `publish-chart`는 태그 하나를
> `--version`과 `--app-version`에 **모두** 넣습니다. 차트 템플릿만 고치고 `v0.4.2` 태그를 밀면
> `appVersion: 0.4.2`인 차트가 나오는데 그 버전의 이미지는 존재하지 않아, 설치하면 `ImagePullBackOff`가
> 됩니다. 위 표의 "차트만 변경 → `version`만 상향"은 **저장소 clone 설치 경로에만** 유효합니다.
>
> 차트만 고쳐야 한다면 둘 중 하나입니다 — (a) 다음 애플리케이션 릴리즈에 얹어 함께 나간다,
> (b) 급하면 `Chart.yaml`의 `version`만 올려 머지하고 clone 설치 경로로 안내한다. 태그는 밀지 않습니다.

---

## 9. 릴리즈 프로세스 (정식 릴리즈)

### 9.0 사전 게이트 (v2 신설 / v2.3 개정)

아래를 통과하지 못하면 태그를 만들지 않습니다.

1. `ci.yml`(backend/frontend/E2E)이 대상 커밋에서 **성공**했을 것.
2. `Lint Review`의 `📋 PR Convention Check`·`📝 CHANGELOG Check`가 릴리즈 PR에서 통과했을 것.

> **`ci.yml`은 2026-08-31 기준 `active`입니다** — v2.2까지 적혀 있던 "`disabled_manually`라
> 릴리즈 담당이 로컬에서 돌리고 결과를 PR 본문에 붙인다"는 대체 절차는 더 이상 필요 없습니다.
> PR을 열면 `backend`·`frontend` 잡이 자동으로 돕니다.
>
> **다만 강제되지는 않습니다.** `main` ruleset의 필수 상태 체크는 여전히 **0건**이라
> (`required_status_checks` 규칙 없음), 잡이 빨간 상태에서도 머지와 태그가 가능합니다.
> 이 게이트는 아직 **릴리즈 담당이 눈으로 확인하는 규칙**입니다 — 설정으로 올리는 것은 §13-2.

### 9.1 절차 (v2.3 전면 개정)

릴리즈 한 번은 **사람 6단계 → 태그 push → 자동 6잡(§7.3) → 사람 검증**입니다.

v2.2까지의 절차는 11단계를 모두 사람이 하는 것으로 적혀 있었으나, `create-release`가 들어오면서
9·10단계(Release 생성, prerelease 체크)가 자동화됐습니다. **그대로 따라 사람이 Release를 따로
만들면 자동 생성본과 충돌합니다.** 아래는 그 경계를 다시 그은 것입니다.

#### A. 태그 전 — 사람이 한다

1. `CHANGELOG.md`의 `## [Unreleased]` 내용을 검토·정리한다.
2. §6의 SemVer 기준으로 버전 번호를 결정한다. `0.x`에서 접미사를 뗄지는 §6.2를 먼저 읽는다
   (롤링 태그가 생긴다).
3. `## [Unreleased]` → `## [X.Y.Z] - YYYY-MM-DD`로 헤더를 바꾸고, 그 위에 새 빈 `## [Unreleased]`
   섹션을 추가한다.
   > **이 단계를 건너뛰고 태그를 밀면 Release 본문이 조용히 열화됩니다.** `create-release`는
   > `CHANGELOG.md`에서 `## [X.Y.Z]` 섹션을 정규식으로 찾고, 못 찾으면 실패하는 대신
   > `generate_release_notes`로 폴백합니다(§5). 워크플로는 초록불이고, 영문 PR 제목만 나열된
   > Release가 남습니다.
4. 파일 하단 compare 링크를 갱신한다 — `[unreleased]`의 기준 태그를 이번 버전으로 바꾸고,
   `[X.Y.Z]` 줄을 새로 추가한다 (실재하는 태그만 — §3).
5. `deploy/helm/nullus/Chart.yaml`의 `version`/`appVersion`을 이번 릴리즈 버전으로 맞춘다 (§8.2).
6. 위 변경을 `docs: CHANGELOG vX.Y.Z 릴리즈 준비` 커밋으로 PR 생성 → 리뷰 → `main` 머지.
   §9.0의 게이트가 초록인지 확인한다.

> **4·5는 빠져도 아무 것도 실패하지 않습니다.** `v0.4.1`에서 실제로 둘 다 누락됐고, 이미지·차트·
> Release·배포가 전부 성공했습니다. 자동화가 검사하지 않는 두 단계라서, 여기서 놓치면 다음
> 릴리즈 담당이 잘못된 기준값을 그대로 물려받습니다(§13-12). **D-10에서 되짚습니다.**

#### B. 태그 push — 사람이 한다

7. `main`에서 태그를 만들어 push한다. 태그는 `main`에서만 만든다 (§10.1).
   ```bash
   git switch main && git pull
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```

#### C. 태그 push 이후 — `cd.yml`이 한다 (사람은 보기만)

8. Actions 탭에서 §7.3의 잡들이 초록인지 확인한다. **사람이 손으로 할 일은 없다** — 이미지 3종,
   차트 게시, GitHub Release 생성(본문 = CHANGELOG 섹션, prerelease 자동 판정), Zadara 배포까지
   자동이다.
   - Release가 생기지 않았다면 `publish-chart` 실패를 먼저 본다 (`create-release`의 선행 잡).
   - Release 본문이 영문 PR 목록이라면 A-3을 건너뛴 것이다. **CHANGELOG를 고쳐 머지한 뒤
     Release 본문을 수동으로 교체한다** — 태그를 지우고 다시 밀지 않는다 (§10.3).

#### D. 릴리즈 후 검증 — 사람이 한다

9. 설치 경로를 검증한다 — **실 Kubernetes 클러스터에서, 인증 없이, 차트 기본값으로** 수행한다.
   로컬 `helm template`은 이 단계를 대체하지 못한다.

   ```bash
   # (a) 산출물 3종이 익명 접근 가능한가 (§7.1)
   gh api /orgs/cloud-nullus/packages/container/nullus%2Fnullus-api --jq .visibility   # public
   gh api /orgs/cloud-nullus/packages/container/nullus%2Fnullus-web --jq .visibility   # public
   gh api /orgs/cloud-nullus/packages/container/charts%2Fnullus     --jq .visibility   # public
   helm registry logout ghcr.io 2>/dev/null
   helm pull oci://ghcr.io/cloud-nullus/charts/nullus --version X.Y.Z

   # (b) 클러스터가 실제로 받아들이는가 — 서버 사이드 렌더 후 설치
   helm upgrade --install nullus oci://ghcr.io/cloud-nullus/charts/nullus --version X.Y.Z \
     --namespace nullus --create-namespace --dry-run=server
   ```

   이어서 실제 설치 → 전 Pod Running → 로그인까지 확인한다.

   > 이 단계를 로컬 렌더로 대신하면 놓치는 유형: 이미지 경로 오류(#78), 존재하지 않는 이미지 태그,
   > 필수 마운트된 시크릿 부재로 인한 `FailedMount`, 패키지 가시성. **v0.3.0-alpha에서 실제로 4종
   > 모두 발생했습니다** (§13-9, §13-10).

10. **A-4·A-5가 실제로 반영됐는지 되짚는다.** 자동화가 검사하지 않는 두 지점이다.

    ```bash
    grep -n "^\[X.Y.Z\]:" CHANGELOG.md                    # compare 링크가 있는가
    grep -nE "^(version|appVersion):" deploy/helm/nullus/Chart.yaml   # 릴리즈 버전과 같은가
    ```

    빠졌으면 **후속 PR로 채운다.** 태그를 다시 밀지 않는다 — 두 값 모두 게시된 산출물에 영향을
    주지 않고(차트는 태그에서 주입, §7.2), 저장소 기준값만 어긋난 상태이기 때문이다.

#### 요약 체크리스트

| # | 단계 | 주체 | 빠뜨리면 |
|---|---|---|---|
| A-3 | CHANGELOG 절단 | 사람 | Release 본문이 영문 자동 생성으로 폴백 (조용함) |
| A-4 | compare 링크 | 사람 | CHANGELOG 하단이 옛 태그를 가리킴 (조용함) |
| A-5 | `Chart.yaml` 동기화 | 사람 | clone 설치 경로가 옛 버전을 씀 (조용함) |
| B-7 | `main`에서 태그 push | 사람 | — |
| C-8 | 자동 6잡 확인 | 사람 | 산출물 누락을 릴리즈 후에 발견 |
| D-9 | 실 클러스터 설치 검증 | 사람 | 설치되지 않는 릴리즈 발행 (v0.3.0-alpha 사례) |
| D-10 | A-4·A-5 되짚기 | 사람 | 다음 담당이 잘못된 기준값을 물려받음 (v0.4.1 사례) |

### 9.2 롤백 절차 (v2 신설)

**1단계 — 애플리케이션만 되돌리는 경우 (기본)**

```bash
helm rollback nullus            # 직전 리비전
# 또는 특정 버전 재설치
helm upgrade nullus oci://ghcr.io/cloud-nullus/charts/nullus --version <이전버전>
```

**2단계 — DB 마이그레이션이 포함된 경우**

- 마이그레이션이 **가역**이면 `make migrate-down`으로 되돌린 뒤 1단계를 수행한다.
- **비가역**이면 롤백할 수 없다. §6.1에 따라 이런 변경은 MAJOR이며, **릴리즈 PR에 비가역 여부와 복구 방법(백업 복원 등)을 반드시 명시**한다. 명시가 없으면 리뷰어는 승인하지 않는다.

**3단계 — 이미지가 사라진 경우**: ghcr의 이전 버전 태그는 삭제하지 않는다. 정리가 필요하면 SHA 태그만 대상으로 하고 버전 태그는 보존한다.

### 9.3 책임 주체와 승인 (v2 신설)

- **릴리즈 담당**: 릴리즈 PR 작성자가 그 릴리즈의 담당이 되며, §9.0 게이트 확인과 §9.1 전 단계를 수행한다.
- **승인**: 릴리즈 PR은 담당자 외 **1명 이상의 승인**을 받는다. 현재 ruleset은 `required_approving_review_count: 0`이라 강제되지 않으므로, 최소한 릴리즈 PR에 대해서는 운영 규칙으로 지킨다(§13-4에서 설정으로 승격).
- **태그 push**: 릴리즈 담당이 수행한다. §10.3의 tag ruleset 도입 후에는 권한이 제한된다.

### 9.4 패치 릴리즈(핫픽스)

1. `fix/<module>-<desc>` 브랜치에서 수정 + 실패 재현 테스트 추가.
2. PR 리뷰·머지 후 `CHANGELOG.md`의 `Unreleased > Fixed`에 항목을 남긴다.
3. 긴급도가 높으면 §9.1의 A-3~B-7만 압축해 바로 PATCH 태그를 올린다. **D-9·D-10은 압축 대상이
   아니다** — 급한 릴리즈일수록 설치 검증과 되짚기를 건너뛰기 쉽고, `v0.4.1`이 그렇게 나왔다.

> **릴리즈 브랜치**: 이미 다음 MINOR가 `main`에 머지된 뒤 이전 버전에 핫픽스가 필요한 경우, 위 절차로는 처리할 수 없습니다. `0.x` 구간에서는 사용자에게 최신 버전 업그레이드를 안내하는 것으로 갈음하고, **GA(`1.0.0`) 시점에 `release/X.Y` 브랜치 도입 여부를 재검토**합니다.

---

## 10. 브랜치·머지 전략과의 관계

### 10.1 태그 생성 위치

릴리즈 태그는 **`main` 브랜치에서만** 생성한다.

### 10.2 머지 전략 (v2에서 결정)

**Squash and Merge로 단일화한다.** CHANGELOG와 Release Note가 PR 단위로 작성되므로 커밋 이력도 PR 단위로 유지하는 편이 추적에 유리합니다. v1이 "Squash 기본"이라 서술했으나 실제로는 전부 merge commit이었고 ruleset은 3종을 모두 허용하고 있었으므로, **설정으로 강제**합니다.

- `main` ruleset의 `allowed_merge_methods`를 `["squash"]`로 제한
- 머지 후 브랜치 자동 삭제(`delete_branch_on_merge`) 활성화 — 현재 `false`라 머지된 원격 브랜치가 누적되고 있습니다

### 10.3 태그 보호 (v2 신설)

현재 `target: tag` ruleset이 0건이라 **아무 브랜치에서나 임의의 태그를 push할 수 있고, 태그 삭제·강제 이동도 가능합니다.** §10.1을 실효화하려면 다음이 필요합니다.

- `refs/tags/v*` 대상 ruleset 생성: `deletion`, `non_fast_forward` 금지
- 생성 권한을 릴리즈 담당(또는 팀)으로 제한

### 10.4 기타

- 브랜치/PR 규칙은 `Nullus_PR_커밋_컨벤션.md`(v3)를 따른다.
  > **해소 (2026-08-09, §13-6 완료)**: `CLAUDE.md`(중첩형 `feat/<module>/<description>` + `refactor` 포함 3종)와 `Nullus_PR_커밋_컨벤션.md` v2(평면형 `feat/stack-tools-wizard` + `feat`/`fix`/`chore` 3종)가 어긋나 있던 것을 **중첩형 + `feat`/`fix`/`chore` 3종**으로 단일화했습니다. 실제 브랜치가 두 형식으로 34:33 갈려 있었으나 최근 것은 전부 중첩형이라 그쪽을 정본으로 삼았습니다. 기존 평면형 브랜치는 그대로 두고 새로 만드는 것만 따릅니다. 실제 브랜치에 남아 있는 `docs/`·`test/` 타입도 legacy 로 두고 신규 생성만 막습니다.
- `cd.yml` 트리거에 남아 있는 `phase1` 브랜치는 legacy이며 제거 대상입니다(§13-3).

---

## 11. 버전 로드맵 (v2.3 갱신)

| 버전 | 상태 | 주요 내용 |
|---|---|---|
| `0.1.0-alpha` | CHANGELOG 기록만 존재 (2026-03-15, **태그 미발행**) | Org/Cluster 등록, Stack 5단계 Wizard, CI/CD Pipeline 템플릿, 모니터링/알림, Keycloak OIDC 인증 기반 |
| `0.2.0-alpha` | CHANGELOG 기록만 존재 (2026-03-28, **태그 미발행**) | Stack Install Wizard 5단계 완성, Helm Orchestrator 다중 Phase DAG, OSS Resource Defaults |
| `0.3.0-alpha` | **발행 완료 (2026-07-27)** — 최초 태그·Release | Stack Continue 배포·Pod Watch WebSocket, Compatibility Matrix Admin CRUD, OpenBao 연동, 에어갭 클린설치 전 과정 자동화, 카카오클라우드 air-gap 배포 자산, SBOM 자동 생성, 스택 설정 export/import, OIDC 설치 옵션화 |
| `0.4.0-alpha` | **발행 완료 (2026-08-09)** | CHANGELOG·PR 컨벤션 CI 강제, Zadara Cloud PoC 배포 자산·운영 스크립트, `create-release`로 태그 push 릴리즈 자동화, 차트 SPA 런타임 설정 값, 브랜치 규칙 v3 단일화 |
| `0.4.1` | **발행 완료 (2026-08-09)** — 접미사를 뗀 첫 릴리즈 | 릴리즈 파이프라인(멀티아키 이미지·차트 게시·Release 본문 렌더링·Zadara 배포) 전 구간 검증. 부수 효과로 `0.4` 롤링 태그가 생겼다(§6.2) |
| `0.9.0-beta` | 예정 | 오픈 베타 — 외부 사용자 테스트 가능 수준 |
| `1.0.0` | 예정 | 정식 출시(GA) — Domain 100% / UseCase 핵심 시나리오 커버리지 확보, `release/X.Y` 브랜치 도입 재검토(§9.4) |

**`0.3.0-alpha` 산정 근거 (2026-07-26 기록)**: `0.2.0-alpha` 이후 누적된 변경은 신규 기능 다수(에어갭 설치 자동화, export/import, SBOM, OIDC 옵션화 등)와 버그 수정이 섞여 있고, REST 응답 필드 제거·비가역 마이그레이션·`values.yaml` 필수 키 삭제 등 §6.1의 Breaking Change 항목에는 해당하지 않습니다. 따라서 §6의 "가장 상위 자릿수 기준으로 한 번만 올린다"에 따라 **MINOR 상향 → `0.3.0-alpha`**입니다.

---

## 12. FAQ

**Q1. `Unreleased` 갱신은 PR 작성자 의무인가요?**
A. 예. 같은 PR 안에서 처리합니다. 사용자 영향이 없는 변경은 `no-changelog` 라벨로 면제합니다(§4.2).

**Q2. `phase1` 브랜치로 push하면 CD가 도는데 써도 되나요?**
A. 아니요. legacy이며 제거 대상입니다(§10.4). 릴리즈는 항상 `main` + 태그 기준입니다.

**Q3. 코드에 현재 버전을 하드코딩해야 하나요?**
A. 아니요. git 태그를 기준으로 관리합니다(§8). 애플리케이션이 자기 버전을 알아야 한다면 빌드 시 `ldflags`로 주입합니다.
> **규칙과 코드가 어긋나 있습니다 (v2.3 확인).** `ldflags` 주입은 여전히 `Makefile`·`Dockerfile`·워크플로 어디에도 없는데, `cmd/api/main.go`의 `/health` 핸들러가 `"version": "0.1.0-alpha"`를 **하드코딩**해 응답합니다. 태그 3건을 발행하는 동안 이 값은 한 번도 바뀌지 않았습니다.
>
> v2.2까지는 "앱이 자기 버전을 모른다"로 끝나는 문제였지만, 이제 **틀린 버전을 단언합니다.** `pkg/nullusclient`의 서버 버전 스큐 검사가 이 필드를 읽으므로(`MinServerVersion = "0.1.0-alpha"`), 어느 버전을 배포하든 스큐 판정이 같은 값을 보고 통과합니다 — 검사가 있으나 아무것도 걸러내지 못합니다. 처리는 §13-13.

**Q4. MAJOR를 올려야 할지 애매합니다.**
A. §6.1 기준표를 먼저 확인하고, 애매하면 리뷰어와 PR에서 논의해 결정합니다.

**Q5. 왜 `0.1.0-alpha`/`0.2.0-alpha`를 지금이라도 소급 태깅하지 않나요?**
A. 해당 시점 커밋을 특정하기 어렵고 이미지도 남아 있지 않아, 태그만 만들면 §1의 "태그로 롤백 가능"이라는 서술이 다시 거짓이 됩니다. 실체 없는 태그를 만들지 않는 편이 낫습니다(§3).

---

## 13. 선행 과제 (이 정책을 실행하기 위해 먼저 끝내야 할 것)

릴리즈 산출물에 직접 걸리는 1·3·7이 끝나 `v0.3.0-alpha`부터 정책을 적용합니다. 남은 항목은 릴리즈를 막지는 않지만, **§9.0의 자동 품질 게이트와 §10의 강제 장치가 아직 없다는 뜻**이므로 그때까지는 릴리즈 담당의 수동 검증에 의존합니다.

**2026-08-31 재측정 (v2.3)**: 과제 11은 §8.2에 규칙을 명시해 닫았고, 과제 2는 절반만 진척됐습니다(`ci.yml`은 `active`이나 필수 체크 미등록). 4·5·8은 그대로 열려 있습니다 — 저장소·ruleset 설정은 이 저장소를 소유한 계정에서만 바꿀 수 있어 문서 PR로는 닫히지 않습니다. `v0.4.1`에서 드러난 것 두 가지를 12·13으로 추가합니다.

과제 9~11은 **`v0.3.0-alpha`를 실제로 릴리즈하고 실 클러스터에 배포해 보고서 드러난 것**입니다. 셋 다 "정책 문서에 쓰인 절차를 그대로 밟았는데도 산출물이 쓸 수 없는 상태로 나온" 사례입니다. 9·10은 v2.2에서 §7.1·§9.1에 반영해 닫았고, 11은 v2.3에서 **`cd.yml`을 고치는 대신 규칙을 코드에 맞춰** 닫았습니다(§8.2 — 차트 전용 패치를 태그로 게시하지 않는다).

과제 12~13은 **`v0.4.1`을 발행하고 나서 드러난 것**입니다. 둘 다 "자동화가 검사하지 않아 조용히 지나간" 유형입니다.

| # | 과제 | 관련 절 | 비고 |
|---|---|---|---|
| 1 | ~~CHANGELOG `Unreleased` 소급 정리~~ | §2, §4.2 | **완료 (2026-07-26)** — PR #54 이후 37건을 검토해 사용자 영향이 있는 26건을 24개 항목으로 `0.3.0-alpha` 섹션에 편입. 문서·테스트·CI 전용 11건(#69·#72·#74·#77·#80·#81·#84·#89·#94·#95·#96)은 §4.2의 `no-changelog` 기준으로 제외 |
| 2 | ~~`ci.yml` 재활성화~~ + main ruleset 필수 체크 등록 | §9.0 | **절반 완료 (2026-08-31 확인)** — `ci.yml`은 `active`이고 PR마다 `backend`·`frontend`가 돈다. 그러나 `main` ruleset의 `required_status_checks`가 여전히 **0건**이라 빨간 상태로도 머지된다. 게이트를 설정으로 올리는 일이 남아 있다. 이전 측정 기록 — 2026-07-28 재측정: `e2e` 2건(`TestScenario4_CICDPipelineFlow`, `TestUAT2_Jieun_Developer`), `vitest` 38건(9파일) — 모두 기존 결함. `tsc` 3건(`cicd-list-page.tsx`)은 해소되어 현재 통과 |
| 3 | ~~차트 이미지 기본값 교정 + `values-dev.yaml` 분리 + 관련 문서 동기화~~ | §8.1 | **완료 (2026-07-28, #100)** — `repository`를 `ghcr.io/cloud-nullus/nullus/nullus-*`로, `tag`를 `""`로. `values-dev.yaml` 신설, `docs/agent-reference.md`·`airgap/helm/README.md` 동기화. `phase1` 트리거 제거는 §10.4에 미완으로 남김 |
| 4 | 저장소 설정: `allowed_merge_methods: ["squash"]`, ~~`delete_branch_on_merge: true`~~, 릴리즈 PR 승인 1인 이상 | §9.3, §10.2 | **부분 완료 (2026-08-31 확인)** — `delete_branch_on_merge`는 `true`로 반영됐다. `allowed_merge_methods`는 아직 `["merge","squash","rebase"]` 3종 허용이고 `required_approving_review_count`는 `0`이다 |
| 5 | tag ruleset 신설 (`refs/tags/v*`) | §10.3 | **미착수 (2026-08-31 확인)** — `target: tag` ruleset 여전히 0건. 태그 3건을 발행한 지금은 §9.1 C-8의 "태그를 지우고 다시 밀지 않는다"가 규칙일 뿐 강제되지 않는다 |
| 6 | ~~브랜치 명명 규칙을 `CLAUDE.md`와 `Nullus_PR_커밋_컨벤션.md` 중 한쪽으로 단일화~~ | §10.4 | **완료 (2026-08-09, 컨벤션 v3)** — 중첩형 `<type>/<module>/<desc>` + `feat`/`fix`/`chore` 3종으로 단일화. 두 문서와 `CLAUDE.md`를 모두 갱신. 실제 브랜치가 평면형 34 : 중첩형 33으로 갈려 있었으나 최근 것이 전부 중첩형이라 그쪽을 정본으로 삼음 |
| 7 | ~~`cd.yml`에 `publish-chart` 잡 추가~~ | §7.2 | **완료 (2026-07-28, #100)** — `v*` 태그에서만 도는 잡으로 추가. 실제 게시 성공 여부는 v0.3.0-alpha 태그 push 시 확인 |
| 8 | `cd.yml`의 `phase1` 브랜치 트리거 제거 | §10.4, §7.3 | **미착수 (2026-08-31 확인)** — `cd.yml` `on.push.branches`에 `phase1`이 남아 있어 이미지 3종과 Zadara 배포가 그 브랜치에서도 돈다 |
| 9 | ~~ghcr 패키지 가시성을 릴리즈 산출물 정의(§7.1)와 §9.1 체크리스트에 포함~~ | §7.1, §9.1 | **완료 (2026-08-07, v2.2)** — 저장소가 public이어도 패키지는 기본 private이다. 2026-07-28 확인 시점에 `nullus-api`·`nullus-web`·`charts/nullus` 3건 모두 private이라 §7.1이 광고하는 `helm install oci://…` 익명 설치 경로가 동작하지 않았다. **2026-08-07 public 전환 완료** — 익명 `helm pull`과 클러스터의 pull secret 없는 이미지 pull로 검증했다. 다만 이는 **1회성 수동 조치**이고 새 패키지는 다시 private으로 생성되므로, §9.1에 확인 항목이 남아야 한다. 전환은 REST API로 불가능하고 패키지 설정 UI에서만 된다 (저장소 Settings가 아님 — 혼동 시 저장소 자체를 private으로 만들 위험) |
| 10 | ~~§9.1 step 11(설치 검증)을 **실 클러스터 서버 사이드**로 명시~~ | §9.1 | **완료 (2026-08-07, v2.2)** — step 11을 가시성 확인 + 익명 `helm pull` + `--dry-run=server` + 실제 설치 4단계로 구체화. v0.3.0-alpha 릴리즈 후 Zadara PoC 클러스터에 `--dry-run=server`를 돌려서야 차트 결함 2건(사설 CA 시크릿 필수 마운트, `bitnami/postgresql` 이미지 소멸)이 드러났다. 로컬 `helm template`은 둘 다 통과시킨다 — step 11이 로컬 렌더로 충족된다고 읽히면 같은 유형을 계속 놓친다 |
| 11 | ~~§7.2 `publish-chart`가 `--version`/`--app-version`을 함께 주입하는 문제~~ | §7.2, §8.2 | **완료 (2026-08-31, v2.3)** — 잡을 고치는 대신 규칙을 코드에 맞췄다. `publish-chart`가 태그를 두 값에 모두 넣는 것은 "차트와 앱이 같은 버전으로 함께 나간다"는 전제에서 옳고, 어긋나는 것은 §8.2의 "차트만 바뀌면 `version`만 올린다" 쪽이다. §8.2에 **차트 전용 패치는 태그로 게시하지 않는다**를 명시했다 — 태그로 게시하면 존재하지 않는 `appVersion`이 찍혀 `ImagePullBackOff`가 된다 |
| 12 | 릴리즈 절단 시 compare 링크·`Chart.yaml` 누락 | §9.1 A-4·A-5 | **열림** — `v0.4.1` 발행 시 둘 다 빠졌다. `CHANGELOG.md` 하단은 아직 `[unreleased]: …/compare/v0.4.0-alpha...HEAD`이고 `[0.4.1]` 줄이 없으며, `Chart.yaml`은 `version: 0.4.0` / `appVersion: "0.4.0-alpha"`다. **게시된 산출물에는 영향이 없다**(차트 버전은 태그에서 주입) — 저장소 기준값만 어긋난 상태라 후속 PR로 채우면 된다. 절차 쪽 방어는 v2.3에서 §9.1 D-10으로 넣었고, 자동 검사로 올리려면 `check_changelog.py`에 "릴리즈 섹션이 있으면 같은 버전의 compare 링크도 있어야 한다"를 더하는 방법이 있다 |
| 13 | `/health`가 버전을 하드코딩한다 | §8, §12 Q3 | **열림** — `cmd/api/main.go`의 `/health`가 `"version": "0.1.0-alpha"`를 고정 응답한다. §8의 "코드에 하드코딩하지 않는다"에 정면으로 어긋나고, `pkg/nullusclient`의 버전 스큐 검사가 이 필드를 읽으므로 **검사가 어떤 스큐도 잡지 못한다**. `ldflags`로 태그를 주입하고(§12 Q3) `MinServerVersion`을 실제 하한으로 올리는 코드 변경이 필요하다 |
