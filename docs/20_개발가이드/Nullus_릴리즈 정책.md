# Nullus 릴리즈 정책

Nullus Platform(`cloud-nullus/nullus`)의 GitHub Release · CHANGELOG · 버전(SemVer) 관리 규칙을 정의합니다.

- 초안: 2026-07-25 / 개정: 2026-07-26 (v2) / 2026-07-28 (v2.1)
- 상태: **적용 개시** — §13의 선행 과제 1·3·7이 끝나 `v0.3.0-alpha`부터 이 문서대로 릴리즈합니다. 남은 과제 2·4·5·6·8은 §13에 그대로 두고, 그중 §9.0의 품질 게이트(과제 2)는 `ci.yml` 재활성화 전까지 **릴리즈 담당의 로컬 검증 + PR 첨부**로 대체합니다.
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
 ├── v0.3.0-alpha Release  ── CHANGELOG.md [0.3.0-alpha] 섹션과 1:1 대응   ← 최초 발행 예정
 └── commit (Unreleased)
```

> `0.1.0-alpha`(2026-03-15)·`0.2.0-alpha`(2026-03-28)는 **CHANGELOG에 섹션만 존재하고 git 태그·GitHub Release는 발행된 적이 없습니다.** 소급 태깅은 해당 시점 커밋을 특정하기 어렵고 이미지도 남아 있지 않으므로 하지 않으며, 다음 릴리즈부터 1:1 대응을 시작합니다.

CHANGELOG.md 하단의 compare 링크는 **실제로 존재하는 태그만** 가리켜야 합니다. 발행되지 않은 태그를 가리키는 링크는 404가 되므로 두지 않습니다.

```markdown
[unreleased]: https://github.com/cloud-nullus/nullus/compare/v0.3.0-alpha...HEAD
[0.3.0-alpha]: https://github.com/cloud-nullus/nullus/releases/tag/v0.3.0-alpha
```

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

> 라벨 검사를 CI로 강제할지는 §13-5의 후속 과제로 둡니다. 체크박스는 리뷰어가 보는 장치일 뿐 차단 장치가 아닙니다.

---

## 5. GitHub 자동 생성 기능

Release 생성 화면의 **"Generate release notes"** 버튼은 머지된 PR·기여자·커밋 목록을 자동으로 나열해 줍니다.

Nullus는 이를 **보조 참고용**으로만 사용합니다. Release Note 본문은 사람이 큐레이션한 `CHANGELOG.md`의 해당 버전 섹션을 그대로 붙여넣는 것을 기본으로 하고, 자동 생성 내용은 "Full Changelog" 링크 형태로 하단에 덧붙이는 정도로 제한합니다.

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

---

## 7. 태그 → CD 파이프라인 연동

### 7.1 이미지 (현행)

`v*` 태그를 push하면 `.github/workflows/cd.yml`이 두 이미지를 빌드해 ghcr에 푸시합니다.

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

> 현재 ghcr에 존재하는 이미지 태그는 `main`과 커밋 short SHA뿐입니다. 버전 태그는 첫 릴리즈 이후에 생깁니다.

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

릴리즈 태그에서 두 값을 함께 주입하는 §7.2 경로에서는 둘이 같아집니다. 위 규칙은 **저장소에 커밋되는 `Chart.yaml` 값**과, 차트만 따로 패치 게시할 때 적용합니다.

---

## 9. 릴리즈 프로세스 (정식 릴리즈)

### 9.0 사전 게이트 (v2 신설)

v1에는 품질 확인 단계가 없어, **테스트가 깨진 상태에서도 태그를 찍을 수 있었습니다.** 아래를 통과하지 못하면 태그를 만들지 않습니다.

1. `ci.yml`(backend/frontend/E2E)이 대상 커밋에서 **성공**했을 것.
   > 현재 `ci.yml`은 `disabled_manually` 상태이고 마지막 실행이 2026-03-15 실패입니다. §13-2를 먼저 끝내야 이 항목이 의미를 가집니다. 그때까지는 릴리즈 담당이 `make build` / `make test` / `web` 타입체크·유닛테스트를 로컬에서 돌리고 **결과를 릴리즈 PR 본문에 붙입니다.**
2. `main` ruleset의 필수 상태 체크가 모두 green일 것 (§13-2 완료 후).
3. 알려진 실패가 남아 있다면 CHANGELOG에 `Known Issues` 항목으로 명시하고, 릴리즈 담당이 승인 근거를 PR에 남길 것.

### 9.1 절차

1. `CHANGELOG.md`의 `## [Unreleased]` 내용을 검토·정리한다.
2. §6의 SemVer 기준으로 버전 번호를 결정한다.
3. `## [Unreleased]` → `## [X.Y.Z] - YYYY-MM-DD`로 헤더를 바꾸고, 그 위에 새 빈 `## [Unreleased]` 섹션을 추가한다.
4. 파일 하단 compare 링크를 갱신한다 (실재하는 태그만 — §3).
5. `deploy/helm/nullus/Chart.yaml`의 `version`/`appVersion`을 §8.2에 따라 동기화한다.
6. 위 변경을 `docs: CHANGELOG vX.Y.Z 릴리즈 준비` 커밋으로 PR 생성 → 리뷰 → `main` 머지. §9.0의 검증 근거를 PR 본문에 첨부한다.
7. `main`에서 태그 생성 및 push:
   ```bash
   git switch main && git pull
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```
8. `cd.yml` 실행을 Actions 탭에서 확인한다 — 이미지 2종 + 차트(§7.2) 푸시 성공.
9. GitHub Release를 생성한다 — 제목 `vX.Y.Z`, 본문은 `CHANGELOG.md`의 해당 섹션(§5).
10. GA(`1.0.0`) 이전 버전은 Release 생성 시 **"Set as a pre-release"**를 반드시 체크한다.
11. 설치 경로를 한 번 검증한다: `helm install`(OCI) → Pod Running → 로그인까지. 이미지 경로 오류는 설치 시점에만 드러납니다(#78).

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
3. 긴급도가 높으면 §9.1의 3~10단계만 압축해 바로 PATCH 태그를 올린다.

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

- 브랜치/PR 규칙은 `Nullus_PR_커밋_컨벤션.md`(v2)를 따른다.
  > **주의**: 루트 `CLAUDE.md`는 `feat/<module>/<description>` 형식에 `refactor/`를 포함한 3종으로, `Nullus_PR_커밋_컨벤션.md`는 `feat`/`fix`/`chore` 3종에 `feat/stack-tools-wizard` 형식으로 서로 다르게 규정하고 있습니다. 실제 브랜치에는 `docs/`·`test/` 타입도 존재합니다. 어느 쪽으로 단일화할지는 이 문서 범위 밖이며 §13-6으로 넘깁니다.
- `cd.yml` 트리거에 남아 있는 `phase1` 브랜치는 legacy이며 제거 대상입니다(§13-3).

---

## 11. 버전 로드맵 (참고)

| 버전 | 상태 | 주요 내용 |
|---|---|---|
| `0.1.0-alpha` | CHANGELOG 기록만 존재 (2026-03-15, **태그 미발행**) | Org/Cluster 등록, Stack 5단계 Wizard, CI/CD Pipeline 템플릿, 모니터링/알림, Keycloak OIDC 인증 기반 |
| `0.2.0-alpha` | CHANGELOG 기록만 존재 (2026-03-28, **태그 미발행**) | Stack Install Wizard 5단계 완성, Helm Orchestrator 다중 Phase DAG, OSS Resource Defaults |
| `0.3.0-alpha` | **릴리즈 준비 완료 — 최초로 실제 태그를 발행할 대상** (태그 미push) | Stack Continue 배포·Pod Watch WebSocket, Compatibility Matrix Admin CRUD, OpenBao 연동, 에어갭 클린설치 전 과정 자동화, 카카오클라우드 air-gap 배포 자산, SBOM 자동 생성, 스택 설정 export/import, OIDC 설치 옵션화 |
| `0.9.0-beta` | 예정 | 오픈 베타 — 외부 사용자 테스트 가능 수준 |
| `1.0.0` | 예정 | 정식 출시(GA) — Domain 100% / UseCase 핵심 시나리오 커버리지 확보, `release/X.Y` 브랜치 도입 재검토(§9.4) |

**버전 산정 근거**: `0.2.0-alpha` 이후 누적된 변경은 신규 기능 다수(에어갭 설치 자동화, export/import, SBOM, OIDC 옵션화 등)와 버그 수정이 섞여 있고, REST 응답 필드 제거·비가역 마이그레이션·`values.yaml` 필수 키 삭제 등 §6.1의 Breaking Change 항목에는 해당하지 않습니다. 따라서 §6의 "가장 상위 자릿수 기준으로 한 번만 올린다"에 따라 **MINOR 상향 → `0.3.0-alpha`**입니다.

---

## 12. FAQ

**Q1. `Unreleased` 갱신은 PR 작성자 의무인가요?**
A. 예. 같은 PR 안에서 처리합니다. 사용자 영향이 없는 변경은 `no-changelog` 라벨로 면제합니다(§4.2).

**Q2. `phase1` 브랜치로 push하면 CD가 도는데 써도 되나요?**
A. 아니요. legacy이며 제거 대상입니다(§10.4). 릴리즈는 항상 `main` + 태그 기준입니다.

**Q3. 코드에 현재 버전을 하드코딩해야 하나요?**
A. 아니요. git 태그를 기준으로 관리합니다(§8). 애플리케이션이 자기 버전을 알아야 한다면 빌드 시 `ldflags`로 주입합니다.
> 현재 `Makefile`·`Dockerfile`·워크플로 어디에도 `ldflags` 주입이 없습니다. 필요해지면 그때 도입하며, 그전까지 앱은 자기 버전을 알지 못합니다.

**Q4. MAJOR를 올려야 할지 애매합니다.**
A. §6.1 기준표를 먼저 확인하고, 애매하면 리뷰어와 PR에서 논의해 결정합니다.

**Q5. 왜 `0.1.0-alpha`/`0.2.0-alpha`를 지금이라도 소급 태깅하지 않나요?**
A. 해당 시점 커밋을 특정하기 어렵고 이미지도 남아 있지 않아, 태그만 만들면 §1의 "태그로 롤백 가능"이라는 서술이 다시 거짓이 됩니다. 실체 없는 태그를 만들지 않는 편이 낫습니다(§3).

---

## 13. 선행 과제 (이 정책을 실행하기 위해 먼저 끝내야 할 것)

릴리즈 산출물에 직접 걸리는 1·3·7이 끝나 `v0.3.0-alpha`부터 정책을 적용합니다. 남은 항목은 릴리즈를 막지는 않지만, **§9.0의 자동 품질 게이트와 §10의 강제 장치가 아직 없다는 뜻**이므로 그때까지는 릴리즈 담당의 수동 검증에 의존합니다.

| # | 과제 | 관련 절 | 비고 |
|---|---|---|---|
| 1 | ~~CHANGELOG `Unreleased` 소급 정리~~ | §2, §4.2 | **완료 (2026-07-26)** — PR #54 이후 37건을 검토해 사용자 영향이 있는 26건을 24개 항목으로 `0.3.0-alpha` 섹션에 편입. 문서·테스트·CI 전용 11건(#69·#72·#74·#77·#80·#81·#84·#89·#94·#95·#96)은 §4.2의 `no-changelog` 기준으로 제외 |
| 2 | `ci.yml` 재활성화 + 현재 실패 2종 수정 후 main ruleset 필수 체크 등록 | §9.0 | 2026-07-28 재측정: `e2e` 2건(`TestScenario4_CICDPipelineFlow`, `TestUAT2_Jieun_Developer`), `vitest` 38건(9파일) — 모두 기존 결함. `tsc` 3건(`cicd-list-page.tsx`)은 해소되어 현재 통과 |
| 3 | ~~차트 이미지 기본값 교정 + `values-dev.yaml` 분리 + 관련 문서 동기화~~ | §8.1 | **완료 (2026-07-28, #100)** — `repository`를 `ghcr.io/cloud-nullus/nullus/nullus-*`로, `tag`를 `""`로. `values-dev.yaml` 신설, `docs/agent-reference.md`·`airgap/helm/README.md` 동기화. `phase1` 트리거 제거는 §10.4에 미완으로 남김 |
| 4 | 저장소 설정: `allowed_merge_methods: ["squash"]`, `delete_branch_on_merge: true`, 릴리즈 PR 승인 1인 이상 | §9.3, §10.2 | 설정 변경 |
| 5 | tag ruleset 신설 (`refs/tags/v*`) | §10.3 | 설정 변경 |
| 6 | 브랜치 명명 규칙을 `CLAUDE.md`와 `Nullus_PR_커밋_컨벤션.md` 중 한쪽으로 단일화 | §10.4 | 이 문서 범위 밖 |
| 7 | ~~`cd.yml`에 `publish-chart` 잡 추가~~ | §7.2 | **완료 (2026-07-28, #100)** — `v*` 태그에서만 도는 잡으로 추가. 실제 게시 성공 여부는 v0.3.0-alpha 태그 push 시 확인 |
| 8 | `cd.yml`의 `phase1` 브랜치 트리거 제거 | §10.4 | 과제 3에서 분리 |
