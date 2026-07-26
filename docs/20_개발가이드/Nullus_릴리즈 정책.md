# Nullus 릴리즈 정책

Nullus Platform(`cloud-nullus/draft`)의 GitHub Release · CHANGELOG · 버전(SemVer) 관리 규칙을 정의합니다.

- 작성일: 2026-07-25
- 적용 대상: `cloud-nullus/draft`
- 관련 문서: `Nullus_PR_커밋_컨벤션.md`, `Nullus_브랜치_관리_개선안.md`, `Nullus_CICD 흐름.md`, `CLAUDE.md`

---

## 1. GitHub Release를 관리하는 이유

Nullus는 릴리즈마다 아래 3가지 산출물이 함께 나갑니다.

- `ghcr.io/cloud-nullus/draft/nullus-api` 컨테이너 이미지
- `ghcr.io/cloud-nullus/draft/nullus-web` 컨테이너 이미지
- `deploy/helm/nullus` Helm 차트 (사용자가 실제로 클러스터에 설치하는 단위)

사용자는 "어떤 버전을 클러스터에 설치했는지"를 정확히 알아야 지원·롤백·업그레이드가 가능하므로, 태그 기반 Release 관리가 특히 중요합니다.

| 목적 | Nullus 맥락 |
|---|---|
| 공식 배포 버전 관리 | 어떤 커밋이 `nullus-api`/`nullus-web` 이미지로 실제 빌드·배포되었는지 명확화 |
| 롤백 | 문제가 생기면 이전 태그의 이미지 + Helm 차트로 즉시 되돌릴 수 있음 |
| 다운로드/배포 | 소스, `ghcr.io` 이미지, Helm 차트를 태그 기준으로 함께 배포 |
| 자동 배포(CD) | `v*` 태그 push 시 `.github/workflows/cd.yml`이 두 이미지를 자동 빌드·푸시 |
| 변경 이력 공유 | Stack/CI-CD/Admin/Auth/O11y 등 모듈별 변경사항을 사용자·운영팀에 전달 |

---

## 2. CHANGELOG를 관리하는 이유

Nullus는 이미 루트에 [`CHANGELOG.md`](../../CHANGELOG.md)를 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) 형식으로 운영 중입니다. 새로 도입하는 규칙이 아니라 **현재 운영 방식을 문서화**하는 것이 이 절의 목적입니다.

```markdown
## [Unreleased]

### Added
- Stack Continue 배포 (`POST /api/v1/stacks/:id/continue`): ...
- Pod Watch WebSocket (`GET /ws/deployments/:id/pods`): ...

### Changed
- 배포 로그 페이지 UI 개선: ...

### Fixed
- Sizing Profile 드롭다운 즉시 반영 버그 수정

## [0.2.0-alpha] - 2026-03-28
...

## [0.1.0-alpha] - 2026-03-15
...
```

| 이해관계자 | 이점 |
|---|---|
| 사용자(운영자) | 업그레이드할 가치가 있는지, Breaking Change가 있는지 판단 |
| QA | 이번 릴리즈에서 무엇을 회귀 테스트해야 하는지 확인 |
| 개발자 | 어떤 버전에 어떤 모듈 변경이 들어갔는지 추적 |
| 운영/SRE | 장애 발생 시 어느 버전부터 문제가 생겼는지 원인 분석 |

---

## 3. Release와 CHANGELOG의 관계

Nullus는 Release마다 CHANGELOG의 해당 버전 섹션을 그대로 Release Note 본문으로 사용합니다.

```
main
 ├── commit
 ├── commit
 ├── v0.1.0-alpha Release  ── CHANGELOG.md [0.1.0-alpha] 섹션과 1:1 대응
 ├── commit
 ├── commit
 ├── v0.2.0-alpha Release  ── CHANGELOG.md [0.2.0-alpha] 섹션과 1:1 대응
 └── commit (Unreleased)
```

CHANGELOG.md 하단에는 버전 간 diff를 볼 수 있는 compare 링크를 유지합니다.

```markdown
[unreleased]: https://github.com/cloud-nullus/draft/compare/v0.2.0-alpha...HEAD
[0.2.0-alpha]: https://github.com/cloud-nullus/draft/compare/v0.1.0-alpha...v0.2.0-alpha
[0.1.0-alpha]: https://github.com/cloud-nullus/draft/releases/tag/v0.1.0-alpha
```

---

## 4. CHANGELOG 작성 규칙

Keep a Changelog 표준 카테고리 중 Nullus가 실제로 사용하는 것은 다음과 같습니다.

- `Added` — 신규 기능/엔드포인트/페이지
- `Changed` — 기존 동작·UI·API 응답 변경
- `Fixed` — 버그 수정
- `Deprecated` / `Removed` — 필요 시 사용 (아직 사용 사례 적음)
- `Security` — **보안 민감 모듈(`auth`, `admin`) 변경**은 반드시 이 카테고리로 별도 기재

### 4.1 작성 언어

**CHANGELOG 엔트리와 GitHub Release 설명(본문)은 한글로 작성한다.** 카테고리 헤더(`Added`/`Changed`/`Fixed`/`Security` 등 Keep a Changelog 표준 키워드)와 API 경로·함수명·파일 경로 같은 코드 식별자만 원문(영문)을 그대로 쓰고, 나머지 설명 문장은 한글로 작성한다.

- PR 제목·커밋 메시지는 기존 컨벤션(`Nullus_PR_커밋_컨벤션.md`)을 그대로 따른다 — 이 규칙은 CHANGELOG/Release 설명에만 적용된다.
- GitHub의 "Generate release notes" 자동 생성 결과는 PR 제목을 영문 그대로 나열하므로, §5에 따라 최종 Release 본문에 쓸 때는 한글 CHANGELOG 섹션을 우선하고 자동 생성 목록은 보조 링크로만 첨부한다.

```markdown
## [0.3.0-alpha] - 2026-08-XX

### Added
- **Compatibility Matrix CRUD Admin UI**: `admin` 모듈에 매트릭스 생성/수정/삭제 API 및 화면 추가

### Security
- OpenBao 토큰 조회 API에 조직 단위 권한 검증 강화
```

**엔트리 작성 흐름 (PR 컨벤션 v2와 연계):**

1. PR 본문의 `## Changes` 섹션에 변경 내용을 작성한다 (`Nullus_PR_커밋_컨벤션.md` §3.3).
2. 머지 시 PR 작성자(또는 dispatcher)가 동일 내용을 `CHANGELOG.md`의 `## [Unreleased]`에 옮겨 적는다.
3. 커밋 타입(`feat`/`fix`/`refactor`/...)과 CHANGELOG 카테고리(`Added`/`Changed`/`Fixed`/...)는 1:1로 매핑되지 않는다 — 사용자 관점에서 "무엇이 바뀌었는가"를 기준으로 분류한다.

---

## 5. GitHub 자동 생성 기능

GitHub Release 생성 화면의 **"Generate release notes"** 버튼은 머지된 PR·기여자·커밋 목록을 자동으로 나열해 줍니다.

Nullus는 이를 **보조 참고용**으로만 사용합니다. Release Note 본문은 사람이 큐레이션한 `CHANGELOG.md`의 해당 버전 섹션을 그대로 붙여넣는 것을 기본으로 하고, 자동 생성 내용은 "Full Changelog" 링크 형태로 하단에 덧붙이는 정도로 제한합니다. PR 컨벤션 v2가 `## Summary`/`## Changes`/`## Testing` 구조를 강제하므로, 자동 생성 노트의 품질 자체도 이전보다 높아졌지만 여전히 최종 편집 없이 그대로 게시하지 않습니다.

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
- 되돌릴 수 없는(non-reversible) DB 마이그레이션
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

`v*` 태그를 push하면 `.github/workflows/cd.yml`이 자동으로 두 이미지를 빌드해 `ghcr.io`에 푸시합니다.

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

**따라서 alpha/beta/rc 단계에서는 `0.3` 같은 rolling 태그가 만들어지지 않으므로, 배포 시 반드시 patch까지 명시된 전체 버전 태그(`0.3.0-alpha` 등)를 사용해야 합니다.** `0.3`/`latest` 같은 rolling 태그에 의존한 배포는 정식 버전(GA 이후)에서만 안전합니다.

---

## 8. 버전이 존재하는 위치 — 동기화 체크리스트

버전 문자열은 저장소 내 3곳에 각각 존재하며, 릴리즈 시 **모두 함께** 갱신해야 합니다. 코드에는 버전을 하드코딩하지 않고 git 태그를 단일 진실 공급원(source of truth)으로 삼습니다.

| 위치 | 파일 | 비고 |
|---|---|---|
| Git 태그 | - | `vX.Y.Z[-PRERELEASE]`, CD 트리거 기준 |
| CHANGELOG 헤더 | `CHANGELOG.md` | `## [X.Y.Z] - YYYY-MM-DD` |
| Helm 차트 | `deploy/helm/nullus/Chart.yaml` | `version`(차트 버전) / `appVersion`(Nullus 앱 버전) |

> **현재 드리프트 주의**: 이 문서 작성 시점 기준 `deploy/helm/nullus/Chart.yaml`은 `version: 0.1.0` / `appVersion: "0.1.0-alpha"`로 고정되어 있지만, `CHANGELOG.md`는 이미 `0.2.0-alpha`를 지나 `Unreleased` 상태입니다. 다음 릴리즈 태깅 시 Chart.yaml 동기화를 반드시 §9 체크리스트에 포함시켜야 합니다.

---

## 9. 릴리즈 프로세스 (정식 릴리즈)

1. `CHANGELOG.md`의 `## [Unreleased]` 내용을 검토·정리한다 (중복/구식 항목 정리).
2. §6의 SemVer 기준으로 버전 번호를 결정한다.
3. `## [Unreleased]` → `## [X.Y.Z] - YYYY-MM-DD`로 헤더를 바꾸고, 그 위에 새 빈 `## [Unreleased]` 섹션을 추가한다.
4. 파일 하단 compare 링크를 갱신한다 (`[unreleased]`, `[X.Y.Z]` 링크 추가/수정).
5. `deploy/helm/nullus/Chart.yaml`의 `version`/`appVersion`을 동기화한다.
6. 위 변경을 `docs: CHANGELOG vX.Y.Z 릴리즈 준비` 커밋으로 PR 생성 → 리뷰 → `main` 머지.
7. `main`에서 태그 생성 및 push:
   ```bash
   git switch main && git pull
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```
8. `cd.yml`이 자동 실행되어 `nullus-api`/`nullus-web` 이미지가 `ghcr.io`에 푸시되는지 Actions 탭에서 확인한다.
9. GitHub Release를 생성한다 — 제목 `vX.Y.Z`, 본문은 `CHANGELOG.md`의 해당 섹션을 붙여넣는다(§5).
10. GA(`1.0.0`) 이전 버전(`-alpha`/`-beta`/`-rc` 포함)은 Release 생성 시 **"Set as a pre-release"** 체크박스를 반드시 사용한다.

### 9.1 패치 릴리즈(핫픽스)

1. `fix/<module>-<desc>` 브랜치에서 수정 + 실패 재현 테스트 추가 (`Nullus_PR_커밋_컨벤션.md` §8.2).
2. PR 리뷰·머지(Squash) 후 `CHANGELOG.md`의 `Unreleased > Fixed`에 항목을 남긴다.
3. 긴급도가 높으면 §9의 3~9단계만 압축해 바로 PATCH 태그를 올린다 (MINOR 변경 누적을 기다리지 않는다).

---

## 10. 브랜치 전략과의 관계

- 릴리즈 태그는 **`main` 브랜치에서만** 생성한다.
- 브랜치/PR 규칙은 `Nullus_PR_커밋_컨벤션.md`(v2)를 그대로 따른다 — 브랜치 타입은 `feat`/`fix`/`chore` 3종, 머지는 Squash and Merge 기본.
- `cd.yml`의 트리거에는 `main` 외 `phase1` 브랜치도 남아 있으나, `Nullus_브랜치_관리_개선안.md`에서 정리 대상 legacy 브랜치로 식별된 항목이다. 신규 릴리즈 플로우는 `phase1`을 사용하지 않으며, 워크플로에서 해당 트리거 제거는 별도 `chore(infra)` 작업으로 처리한다.

---

## 11. 버전 로드맵 (참고)

CHANGELOG 기준 현재까지의 실제 이력과, CLAUDE.md의 v1 GA 목표(커버리지 >70%)에 맞춘 이후 계획입니다. 실제 마일스톤 내용은 진행 상황에 따라 조정됩니다.

| 버전 | 상태 | 주요 내용 |
|---|---|---|
| `0.1.0-alpha` | 완료 (2026-03-15) | Org/Cluster 등록, Stack 5단계 Wizard, CI/CD Pipeline 템플릿, 모니터링/알림, Keycloak OIDC 인증 기반 |
| `0.2.0-alpha` | 완료 (2026-03-28) | Stack Install Wizard 5단계 완성, Helm Orchestrator 다중 Phase DAG, OSS Resource Defaults |
| 다음 버전 | 진행 중 (`Unreleased`) | Stack Continue 배포, Pod Watch WebSocket, Compatibility Matrix Admin CRUD, OpenBao 연동 |
| `0.9.0-beta` | 예정 | 오픈 베타 — 외부 사용자 테스트 가능 수준 |
| `1.0.0` | 예정 | 정식 출시(GA) — Domain 100% / UseCase 핵심 시나리오 테스트 커버리지 확보 |

---

## 12. FAQ

**Q1. `Unreleased` 섹션에 항목을 추가하는 게 PR 작성자 의무인가요?**
A. 원칙적으로 그렇습니다. PR 본문 `## Changes`와 CHANGELOG 항목은 같은 내용을 사용자 관점으로 다시 쓴 것이므로, PR 작성 시점에 함께 추가하는 것을 권장합니다.

**Q2. `phase1` 브랜치로 push하면 CD가 도는데, 지금도 써도 되나요?**
A. 아니요. legacy 브랜치이며 정리 대상입니다(§10). 릴리즈는 항상 `main` + 태그 기준으로만 수행합니다.

**Q3. 코드 어딘가에 현재 버전을 하드코딩해야 하나요?**
A. 아니요. Nullus는 버전을 코드에 하드코딩하지 않고 git 태그를 기준으로 관리합니다(§8). 필요하면 빌드 시점에 ldflags 등으로 주입하는 방식을 사용합니다.

**Q4. MAJOR를 올려야 할지 애매합니다.**
A. §6.1의 Breaking Change 기준표를 먼저 확인하고, 애매하면 리뷰어와 PR에서 논의 후 결정합니다(`Nullus_PR_커밋_컨벤션.md` §5 코드 리뷰 기준 참고).
