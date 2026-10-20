# Nullus 시크릿 탐지 설계 (Secret Detection — gitleaks)

> 대상 이슈: [cloud-nullus/nullus-plan#80](https://github.com/cloud-nullus/nullus-plan/issues/80) — [Level 3] EPIC: Secret Detection — 저장소 시크릿 유출 탐지
> 축: **A축 — 파이프라인 보안** (대상: 사용자 앱 저장소의 **코드·이력에 섞여 들어간 시크릿**). #68(Nullus 가 보관·주입하는 시크릿)과 별개다.
> 작성: 2026-10-12 · 저장소 확인 기준 `850402f`

---

## 0. 결정 요약

| # | 결정 | 한 줄 근거 |
|---|---|---|
| 1 | **탐지 시점은 병행** — 개발자 로컬 pre-commit 훅 + CI 단계. SCM pre-receive 훅은 **제외** | 훅은 유출 **전에** 잡지만 강제할 수 없고, CI 는 강제하지만 이미 푸시된 **뒤**다. 둘이 서로의 빈칸을 채운다. pre-receive 는 GitLab 은 Ultimate 전용(push rules secret detection), Gitea 는 훅 API 가 없어 두 계열에 고르게 둘 수 없다 |
| 2 | **도구는 gitleaks** | MIT · 단일 바이너리 · 공식 이미지가 amd64/arm64 를 다 낸다 · 규칙(TOML)·allowlist·baseline 이 있다. trufflehog 는 AGPL-3.0 이고 "검증(verification)" 이 네트워크로 나가 에어갭과 맞지 않는다 |
| 3 | **범위는 신규 커밋만** — 이번 푸시의 `이전 커밋..HEAD`. 기준 커밋이 없으면(첫 푸시·수동 실행·얕은 클론) **작업 트리** | 매 실행마다 전체 이력을 훑으면 저장소가 커질수록 느려지고, 이미 지난 이력의 유출은 이 게이트가 막을 수 있는 것이 아니다. 전체 이력은 온보딩 때 한 번 하는 일이다(§4) |
| 4 | **GitLab·Gitea(Jenkins)·GitHub 모두 스캐폴딩 렌더러 한 곳에서** — `secret-scan` 단계를 세 CI 정의에 같은 스크립트로 넣는다 | SCM 이 아니라 CI 로 분기하는 기존 구조(renderer.go)를 그대로 쓴다. Gitea 스택은 Jenkinsfile, GitLab 은 `.gitlab-ci.yml`, GitHub 은 Actions |
| 5 | **빌드 앞에 선다.** GitLab 은 stage 순서, GitHub 은 `build.needs`, Jenkins 는 stage 선언 순서 | 시크릿이 든 소스로 이미지를 만들어 레지스트리에 올리는 것부터 막아야 한다. SAST 는 빌드와 나란히 돌지만 이 단계는 빌드가 기다린다 |
| 6 | **모든 파이프라인에 단계가 있다.** 끄는 것은 정책(`secret_detection: off`)이지 렌더링이 아니다 | 스택에 설치하는 것이 없다 — 이미지 스캔(Trivy 서버)·SAST(SonarQube)처럼 "없는 스택" 이 생기지 않는다. 옵션화는 #64 가 정한 자리(스택 스캔 정책)에 값으로 들어간다 |
| 7 | **정책 `secret_detection` = block(기본) · warn · off** — 스택 스캔 정책(`image_scan_policies`)의 열 하나. CI 변수 `NULLUS_SECRET_DETECTION` 으로 푸시 | 이미지 스캔·SAST 와 같은 길(설계 #76 결정 9). 스캔 실패(이미지·git 장애)는 `NULLUS_SCAN_ON_UNREACHABLE` 을 따른다 — 경고 정책을 스캔 실패에까지 적용하지 않는다 |
| 8 | **오탐은 저장소 안에서 다룬다** — `.gitleaks.toml`(allowlist, 기본 규칙 확장) + `.gitleaksignore`(Fingerprint 한 줄). 둘 다 처음 스캐폴딩할 때 비어 있는 채로 넣는다 | 파일이 있어야 "어디에 적는지" 를 찾지 않는다. 기본 allowlist 는 비어 있다 — 배포 매니페스트 경로를 미리 풀어 두면 그 안의 진짜 Secret 이 가려진다 |
| 9 | **탐지 후 절차는 메시지에 박는다** — "커밋을 지워도 이미 유출입니다. 즉시 폐기·회전." 리포트(`gitleaks-report.json`, 비밀값은 가려짐)는 걸려도 산출물로 남긴다 | CI 단계는 사후 통보다. 회전 안내가 없는 탐지는 "지우면 되지" 로 끝난다. 알림 채널은 #63 이 생기면 얹는다 |

---

## 1. 저장소 확인 결과 (2026-10-12)

| 확인 항목 | 결과 | 근거 |
|---|---|---|
| 시크릿 탐지 설정 | **없음** | `grep -ril gitleaks\|trufflehog` 0건 |
| 스캐폴딩 선택 단계 | `StageOptions{ImageScan, SAST}` — 스택 구성이 정한다 | `internal/cicd/adapter/scaffold/renderer.go` |
| 정책 저장·푸시 | `image_scan_policies` 한 행/스택 → CI 변수(GitLab 프로젝트 변수 · GitHub Actions 변수 · Jenkins ConfigMap `nullus-scan-policy`) | `internal/cicd/port/scan_policy_vars.go`, `adapter/kube/scan_policy_configmap.go` |
| CI 3종의 "이전 커밋" | GitLab `CI_COMMIT_BEFORE_SHA`(첫 푸시는 `0000…`) · GitHub `github.event.before` · Jenkins `GIT_PREVIOUS_SUCCESSFUL_COMMIT`(첫 빌드는 null) | 각 CI 문서 |
| 에어갭 이미지 등록 경로 | `shareddomain.RuntimeImages()` ↔ `airgap/scripts/00-generate-images.sh` 의 `RUNTIME_IMAGES` ↔ `images.txt` — 셋이 어긋나면 테스트가 막는다 | `runtime_images_test.go` |
| gitleaks v8.30.1 실측 | `git --log-opts A..B` 로 범위 스캔, `dir` 로 작업 트리, `--exit-code 2` 로 탐지(2)와 오류(1)를 가름, `--redact` 로 리포트 비밀값 가림, 이미지에 git 2.49 포함, 컨테이너 안 uid 0 | 로컬 docker 실행(§6) |

---

## 2. 탐지 시점 — 왜 병행인가

| 시점 | 막는가 | 강제 | 구현 |
|---|---|---|---|
| 커밋 전 — pre-commit 훅 | **유출 전** | ❌ 개발자가 켜야 돈다 | 스캐폴딩이 `.pre-commit-config.yaml`(gitleaks 훅, CI 와 같은 버전)을 넣고 README 가 `pre-commit install` 을 안내 |
| 푸시 시 — SCM pre-receive | 유출 전 | ✅ | **제외.** GitLab 은 Ultimate 전용, Gitea 는 서버 훅 API 가 없다. GitHub 은 push protection(GHAS) — 조직이 켤 수 있으면 켜도 이 설계와 충돌하지 않는다 |
| CI 단계 | **이미 푸시된 뒤** | ✅ | `secret-scan` 단계 — 이번 푸시의 커밋만. 탐지 시 차단 + 회전 안내 |

병행의 뜻: 훅을 켠 개발자는 유출 전에 잡히고, 훅을 켜지 않은 커밋은 CI 가 잡는다. **CI 가 잡은 것은 이미 유출이다** — 그래서 메시지의 첫 문장이 "커밋을 지워도 이미 유출입니다" 다.

---

## 3. 단계 설계

### 3.1 스크립트 (CI 3종 공통, `scaffold.secretScanScriptLines`)

```
git config --global --add safe.directory '*'          # 컨테이너 잡의 uid 가 작업 디렉터리 소유자와 다르다
[정책 off] → 빈 리포트 남기고 exit 0
base = NULLUS_SCAN_BASE (0000… 이면 비움)
base 가 있고 클론에 있으면  gitleaks git --log-opts "$base..HEAD" .     # 신규 커밋만
아니면                       gitleaks dir .                               # 작업 트리
rc 0 → 통과
rc 2 → 탐지: 폐기·회전 안내 → warn 이면 exit 0, 아니면 exit 1
그 밖 → 스캔 실패: NULLUS_SCAN_ON_UNREACHABLE=allow 면 exit 0, 아니면 exit 1
```

공통 옵션 `--redact --no-banner --exit-code 2 --report-format json --report-path gitleaks-report.json`. 스크립트에 역슬래시가 없다(Jenkins `sh '''…'''` 는 Groovy 문자열).

### 3.2 CI 별 자리

| CI | 단계 | 범위 시작점 | 특이점 |
|---|---|---|---|
| GitLab CI | `stages` 맨 앞 `secret-scan`, `needs: []`, `GIT_DEPTH: "0"`, 모든 브랜치 | `$CI_COMMIT_BEFORE_SHA` | 이미지는 전역 변수 `NULLUS_GITLEAKS_IMAGE`(프로젝트 변수가 앞선다), `entrypoint: [""]` |
| GitHub Actions | 잡 `secret-scan`, `build.needs: secret-scan`, `fetch-depth: 0` | `${{ github.event.before }}` | 컨테이너 `${{ vars.NULLUS_GITLEAKS_IMAGE \|\| 기본 }}`, `--user root`(checkout 이 쓰려면) |
| Jenkins | 파드 컨테이너 `gitleaks`(ConfigMap `nullus-scan-policy` envFrom), `stage('SecretScan')` 을 Build 앞에 | `${env.GIT_PREVIOUS_SUCCESSFUL_COMMIT ?: ''}` | `deploy/**` 만 바뀐 되커밋은 건너뛴다(Build 와 같은 이유) |

SAST 가 기본 브랜치에서만 도는 것과 달리 **모든 브랜치**에서 돈다 — 기능 브랜치의 유출도 유출이다.

### 3.3 단계 이름과 실행 기록

`PipelineStageNames` 가 `["SecretScan", "Build", …, "Deploy"]` 를 준다. 새로 만들거나 정의를 갱신한 파이프라인은 이 목록을 저장하고, 실행 기록 화면은 저장된 단계로 그린다(`stage-states.ts` 의 키 정규화가 `secret-scan`/`SecretScan` 을 같은 단계로 본다). 옛 파이프라인은 저장된 2·3단계 목록 그대로다 — 정의를 갱신하면 단계가 붙는다.

---

## 4. 범위 — 신규 커밋만, 그리고 전체 이력은 언제

| 상황 | 범위 | 왜 |
|---|---|---|
| 보통의 푸시 | `이전 커밋..HEAD` | 이번에 들어온 것만 보면 된다. 빠르고, 걸리면 "이번 푸시가 넣었다" 가 분명하다 |
| 첫 푸시 · 브랜치 생성(`0000…`) · 수동 실행 · 얕은 클론 | 작업 트리 | 범위를 만들 기준이 없다. 현재 파일에 든 시크릿은 다 잡는다 — 이력에만 남은 것은 못 본다 |
| **온보딩** — 기존 저장소를 Nullus 파이프라인에 처음 붙일 때 | (후속) 전체 이력 1회 | 이미 지난 유출을 찾는 일은 게이트가 아니라 **점검**이다. `gitleaks git .`(범위 없음)을 한 번 돌려 결과를 회전 목록으로 쓴다. 자동화는 후속(§7) |

---

## 5. 정책 · 오탐 · 탐지 후

**정책.** `image_scan_policies.secret_detection`(000095) — `block`(기본) · `warn` · `off`. `PUT /stacks/:id/image-scan-policy` 의 선택 필드 `secret_detection`; 모르는 옛 클라이언트가 보내지 않으면 저장된 값을 유지한다(SAST 와 같다). 푸시되는 변수: `NULLUS_SECRET_DETECTION`, `NULLUS_GITLEAKS_IMAGE`(이미지도 정책과 함께 민다 — 플랫폼이 이미지를 올리면 정의를 다시 쓰지 않아도 따라온다).

**오탐.** 저장소의 `.gitleaks.toml`(`[extend] useDefault = true` + `[allowlist]`)과 `.gitleaksignore`(리포트의 `Fingerprint` 한 줄). 둘은 처음 스캐폴딩할 때만 넣고 **정의 갱신은 덮어쓰지 않는다** — 사용자가 다듬는 파일이다. 조직 공통 allowlist 는 두지 않는다(#76 §11 과 같은 보류).

**탐지 후.** 단계 로그에 "커밋을 지워도 이미 유출 — 즉시 폐기·회전" + 리포트 위치 + 오탐 처리 경로. 리포트는 산출물로 남고(비밀값 가림) 실행 기록은 단계 상태(실패/성공)를 보여 준다. 알림(#63)·결과 저장·대시보드(#65)는 후속.

---

## 6. 실측 (2026-10-12, 로컬 docker, `ghcr.io/gitleaks/gitleaks:v8.30.1`)

| 확인 | 결과 |
|---|---|
| GitHub PAT 형태의 값을 넣은 커밋 범위 `base..HEAD` 스캔 | exit **2**, 리포트에 규칙 ID(`github-pat`)와 **가려진** 값 |
| 그 앞 커밋까지의 범위(`base..clean`) | exit **0** |
| 작업 트리(`dir`) | exit **2** |
| git 저장소가 아닌 빈 경로에 `gitleaks git` | **"no leaks found", exit 0** — gitleaks 가 오류로 보지 않는다. 스크립트가 `git cat-file -e` 로 기준 커밋을 먼저 확인하는 이유다(없으면 `dir`). 장애 경로(exit 1)는 이미지 pull 실패·잘못된 인자에서 난다 |
| `safe.directory` 없이 다른 uid 의 저장소 | 경고 없이 동작(이미지 안 uid 0) — 그래도 넣어 둔다. GitLab Kubernetes executor 는 비루트로 바꿔 둘 수 있다 |
| AWS 문서의 예시 키(`AKIAIOSFODNN7EXAMPLE`) | **0건** — 기본 규칙의 allowlist 가 예시 키를 뺀다. 테스트 데이터로 쓰지 말 것 |

---

## 7. 이 카드에서 닫지 않는 것

| 항목 | 이유 | 재검토 시점 |
|---|---|---|
| 탐지 결과 저장·화면(실행별 건수·규칙·파일) | 단계 상태와 산출물로 먼저 선다. 저장 모양은 이미지 스캔(`image_scan_results`)·SAST(`sast_results`)와 같은 틀 → **#65 에서 `secret_scan_results`(000107)로 저장한다**(`Nullus_Security_Dashboard_설계.md` §8 G2) | #65 Security Dashboard |
| 탐지 알림 | 채널이 없다 | #63 Alert 채널 |
| 온보딩 전체 이력 스캔 자동화 | 게이트가 아니라 점검. 파이프라인 생성 시 1회 Job 으로 돌리고 결과를 사용자에게 돌려줄 자리(결과 저장)가 먼저 필요하다 | 결과 저장 뒤 |
| SCM push protection 연계(GitHub GHAS · GitLab Ultimate) | 조직 라이선스에 달려 있다. 켤 수 있는 조직은 켜도 이 설계와 겹치지 않는다 | — |
| 훅 설치 여부 확인·강제 | 로컬 환경은 플랫폼이 볼 수 없다. CI 단계가 그 빈칸을 메운다 | — |

---

## 참고

- 형제 설계: [컨테이너 이미지 스캔 설계](./Nullus_컨테이너_이미지_스캔_설계.md)(정책 푸시 구조 — 결정 9), SAST(#64, 렌더러 `sast.go`)
- 코드: `internal/cicd/adapter/scaffold/secret_scan.go` · `renderer.go` · `jenkins_renderer.go`, `internal/cicd/domain/scan_policy.go`, `internal/cicd/port/scan_policy_vars.go`, `db/migrations/000095`, `internal/shared/domain/runtime_images.go`
- gitleaks: https://github.com/gitleaks/gitleaks (MIT) — `git`/`dir` 명령, `--exit-code`, `.gitleaks.toml`, `.gitleaksignore`
