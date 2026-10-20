# Nullus 서드파티 의존성 취약점 스캔 설계 (SCA — Trivy)

> 대상 이슈: [cloud-nullus/nullus-plan#79](https://github.com/cloud-nullus/nullus-plan/issues/79) — [Level 3] EPIC: 앱 의존성 취약점 스캔(SCA) 도입 판정
> 축: **A축 — 파이프라인 보안** (대상: 사용자가 Nullus 로 만든 앱의 **의존 라이브러리**)
> 작성: 2026-10-12 · 저장소 확인 기준 `6a1e138`

---

## 0. 결정 요약

| # | 결정 | 한 줄 근거 |
|---|---|---|
| 1 | **판정: 별도 단계를 둔다** — 스택 설치의 Security 에 "서드파티 의존성 취약점(SCA)" 을 **선택 항목**(미선택 · Trivy)으로 추가하고, 고르면 파이프라인에 `dependency-scan` 단계가 생긴다 | 이미지 스캔과 겹치지만 대체하지 못한다 — 빌드 **전**에 소스(락파일)를 보므로 피드백이 빠르고, 이미지에 들어가지 않는 의존성(멀티 스테이지 빌드의 빌드 단계·정적 산출물로 끝나는 프런트엔드)도 본다. 이미지 스캐너 없이도 쓸 수 있다 |
| 2 | **도구는 Trivy `fs`** — 이미지 스캔과 같은 Trivy, 같은 **스택 Trivy 서버**(client/server). Dependency-Track · Grype 는 도입하지 않는다 | 설치 자산이 늘지 않는다 — 취약점 DB 는 서버에만 있고 CI 잡은 얇은 client 다. Dependency-Track 은 SBOM 축적·이력에 낫지만 서버·DB·리소스 산정이 하나 더 늘고, Grype 는 DB 를 잡마다 받거나 별도 서버가 필요하다 |
| 3 | **의존성 스캔만 골라도 Trivy 서버가 선다.** 이미지 스캐너와 둘 다 고르면 서버는 하나다 | `installing_trivy` 의 설치 조건이 "이미지 스캐너 **또는** SCA" 다(`domain.HasTrivyServer`). 설치 목록·자원 계획·모니터링 목록도 같은 판단을 본다 |
| 4 | **빌드 앞에 선다** — GitLab 은 stage 순서, GitHub 은 `build.needs`, Jenkins 는 stage 순서. 모든 브랜치에서 돈다 | 소스만 보므로 빌드를 기다릴 이유가 없고, 취약한 의존성으로 이미지를 만들어 올리는 것부터 막는다 |
| 5 | **차단 기준은 이미지 스캔과 같은 정책** — 차단 심각도(기본 CRITICAL) · 수정판 없는 것 제외 · 스캐너 장애 시 동작. 같은 CI 변수(`NULLUS_SCAN_SEVERITY` 등)를 읽는다 | 한 스택에 두 기준이 있으면 같은 취약점이 한쪽에서는 막히고 다른 쪽에서는 지나간다. 정책 화면을 하나 더 만들 이유도 없다 |
| 6 | **예외는 저장소의 `.trivyignore`** | Trivy 가 작업 디렉터리에서 스스로 읽는다. 이미지 스캔과 같은 관리 방식이다 |
| 7 | **라이선스 검사는 하지 않는다**(`--scanners vuln`) | 라이선스 판정은 조직 정책(허용 목록)이 먼저 있어야 의미가 있다. 결정이 서면 같은 단계에 `license` 스캐너를 더한다(§6) |
| 8 | **에어갭은 이미 있는 경로** — 취약점 DB 는 스택 Trivy 서버가 내부 OCI 미러에서 받는다(#76 §7). Java DB 미러(`TRIVY_JAVA_DB_REPOSITORY`)도 이미지 스캔과 같이 넘긴다 | 새 반입 경로가 없다. Trivy CLI 이미지는 이미 번들에 있다 |

---

## 1. 저장소 확인 결과 (2026-10-12)

| 확인 항목 | 결과 | 근거 |
|---|---|---|
| SCA 설정 | 없음 | — |
| 이미지 스캔 구조 | 스택 Trivy 서버(server 모드, DB 는 서버) + CI 잡은 client(`trivy image --server`) | 설계 `Nullus_컨테이너_이미지_스캔_설계.md` §2.3 |
| 스캐폴딩 앱 유형과 락파일 | `web`(React — `package.json` 만, **락파일 없음**) · `backend`·`batch`(Dockerfile 만). **스캐폴딩 자체는 락파일을 만들지 않는다** | `internal/cicd/adapter/scaffold/react_app.go` |
| Trivy `fs` 가 보는 락파일 | npm(`package-lock.json`) · yarn(`yarn.lock`) · pnpm(`pnpm-lock.yaml`) · Go(`go.mod`/`go.sum`) · Maven(`pom.xml`) · Gradle(`gradle.lockfile`) · Python(`requirements.txt`·`Pipfile.lock`·`poetry.lock`·`uv.lock`) · Rust(`Cargo.lock`) · PHP(`composer.lock`) · .NET(`packages.lock.json`) · Ruby(`Gemfile.lock`) 등 | Trivy 문서 "Language and OS coverage" |
| `package.json` 만 있는 저장소 | **검사할 의존성이 없다** — Trivy `fs` 는 락파일로 의존성을 푼다 | 실측(§5) |

---

## 2. 이미지 스캔과의 경계

| | 이미지 스캔 (#76) | 의존성 스캔 (이 설계) |
|---|---|---|
| 시점 | 빌드 **후** — 레지스트리에서 이미지를 다시 받는다 | 빌드 **전** — 체크아웃한 저장소 |
| 대상 | OS 패키지 + 이미지에 들어간 앱 의존성 | 저장소 락파일의 앱 의존성 |
| 겹침 | **이미지에 그대로 들어간 앱 의존성은 둘 다 잡는다** | |
| 이것만 잡는 것 | 베이스 이미지의 OS 패키지 | 이미지에 남지 않는 의존성(빌드 단계 전용·정적 산출물), 그리고 **빌드하기 전에 빨리 실패** |
| 서버 | 스택 Trivy 서버 | **같은 서버** |
| 정책 | 스택 스캔 정책 | **같은 정책** |

둘 다 고르면 같은 취약점이 두 단계에 보일 수 있다 — 의도된 중복이다. 의존성 스캔이 먼저 막으므로 실무에서는 이미지 스캔에 같은 건이 다시 걸리지 않는다.

---

## 3. 스택 설치

- 설치 화면 Security 탭에 **서드파티 의존성 취약점 (SCA)** — 선택지 `미선택`(기본) · `Trivy`. 템플릿 편집기에도 같은 칸(`sca`)이 있다.
- 백엔드 `config.security.sca`(`ToolSelection`). `version: external` 은 스택 안에 서버를 세우지 않는다.
- `installing_trivy` 는 `HasImageScanner || HasSCA` 일 때 선다. 자원 계획은 이미지 스캐너 슬롯(`security.imageScanner`)에 서버 하나의 크기로 실린다 — 설치 화면의 설치 목록에도 한 줄(`Security > Image Scanner` 또는 `Security > Dependency Scan (Trivy server)`)로 보인다.
- 런북: `./scripts/runbook_local.sh stack-up --template=gitlab-argocd-v1 --security=sca`.

## 4. 파이프라인 단계

`scaffold.dependencyScanScriptLines` — CI 3종 공통.

```
trivy fs --server $NULLUS_TRIVY_SERVER --scanners vuln [--ignore-unfixed] --skip-dirs node_modules --skip-dirs .git \
         --format json --output dependency-scan-report.json .      # 리포트 — 실패하면 스캐너 장애 정책
echo "의존성 스캔 대상(락파일): N 개"                                 # 0 이면 "락파일을 커밋하세요"
trivy fs --server ... --severity ${NULLUS_SCAN_SEVERITY:-CRITICAL} [--ignore-unfixed] --exit-code 1 --quiet .   # 게이트
```

| CI | 자리 | 서버 주소 | 스캐너 |
|---|---|---|---|
| GitLab CI | `stages` 의 `secret-scan` 다음 · `build` 앞, `needs: []` | 전역 변수 `NULLUS_TRIVY_SERVER` | `$NULLUS_TRIVY_IMAGE`, `entrypoint: [""]` |
| GitHub Actions | 잡 `dependency-scan`, `build.needs: [secret-scan, dependency-scan]` | 잡 env | `vars.NULLUS_TRIVY_IMAGE` 또는 기본 |
| Jenkins | `stage('DependencyScan')` 를 Build 앞에 | stage environment | 이미지 스캔과 같은 `scanner` 컨테이너(ConfigMap 정책 envFrom) |

단계 이름은 `PipelineStageNames` 가 `SecretScan · DependencyScan · Build · … · Deploy` 순으로 준다. 정책 푸시 대상(`readsScanPolicy`)에 `DependencyScan` 과 `SecretScan`(#80 에서 빠져 있던 것)을 더했다 — 빠지면 정책을 바꿔도 그 단계는 기본값으로 돈다.

## 5. 실측 (2026-10-12, kind `nullus-platform`, GitLab + Argo CD, **SCA 만** 선택)

기존 스택·파이프라인·앱을 모두 지운 뒤 새로 설치했다. 스택 `gitlab-argocd-v1` + `--security=sca`, 앱은 스캐폴딩한 `backend`(`nullus/demo-api`).

| 확인 | 결과 |
|---|---|
| 스택 설치 | 이미지 스캐너·SAST 를 고르지 않고 SCA 만 골랐다. `installing_trivy` 가 돌아 `trivy` 릴리스와 `trivy-0` 파드가 섰고, SonarQube 는 없다 |
| 파이프라인 생성 | 단계 `SecretScan · DependencyScan · Build · Deploy`. `.gitlab-ci.yml` 에 `dependency-scan` 잡(`needs: []`, `$NULLUS_TRIVY_IMAGE`)과 전역 `NULLUS_TRIVY_SERVER=http://trivy.nullus-gitlab-sca.svc.cluster.local:4954` |
| 정책 푸시 | 이미지 스캔·SAST 가 없는 이 파이프라인에도 `NULLUS_SCAN_SEVERITY=CRITICAL` · `NULLUS_SCAN_IGNORE_UNFIXED=true` · `NULLUS_SCAN_ON_UNREACHABLE=block` 이 프로젝트 변수로 실렸다(`readsScanPolicy`) |
| 실행 1 — 스캐폴딩 그대로(락파일 없음) | `dependency-scan` 통과 6초, 로그 "의존성 스캔 대상(락파일): 0 개 … 락파일을 커밋하세요" → build 28초 → deploy 성공 |
| 실행 2 — `tools/package-lock.json`(minimist 1.2.5) 커밋 | `dependency-scan` **실패** 7초 — `CVE-2021-44906` CRITICAL(수정판 1.2.6). **build · deploy 는 skipped**. 리포트 산출물 `dependency-scan-report.json` 에 같은 CVE. 플랫폼 실행 기록 `#3` 이 dependency-scan 실패 · build/deploy 건너뜀으로 남았다 |
| 실행 3 — minimist 1.2.6 으로 올린 커밋 | 대상 1 개 · 취약점 0 → build 26초 → deploy → Argo CD Synced/Healthy, 파드 1/1 Running(이미지 태그 `43cbbd83`) |

리허설이 드러낸 결함 세 가지는 고쳤다.

| 결함 | 원인 | 조치 |
|---|---|---|
| 락파일이 없으면 `dependency-scan` 이 죽는다 | 개수 줄의 `grep` 이 0 건에서 1 로 끝나고, GitLab Runner 의 `pipefail` 이 잡을 끝냈다. 테스트는 `sh -c "set -eu"` 로 돌아 보지 못했다 | `\|\| true` 로 받았다. 스캔 스크립트 테스트 5종을 모두 `bash -c "set -euo pipefail"` 로 돌린다 — 다른 단계에는 같은 결함이 없었다 |
| 새 앱의 첫 파이프라인이 `secret-scan` 에서 막힌다(#80) | 스캐폴딩한 `.gitleaks.toml` 의 `[allowlist]` 에 검사 항목이 없어 gitleaks v8.30.1 이 설정을 거부했다 | allowlist 블록을 주석 예시로 바꿨다. CHANGELOG Fixed 참조 |
| 기존 스택을 지운 뒤 새 스택이 `installing_prometheus` 에서 실패한다 | 삭제가 Prometheus CRD 릴리스를 먼저 지워 ServiceMonitor 를 담은 릴리스의 uninstall 이 멈췄다 | 별도 PR(nullus#317) |

## 6. 이 카드에서 닫지 않는 것

| 항목 | 이유 | 재검토 시점 |
|---|---|---|
| 결과 저장·실행 기록 화면(건수·CVE 목록) | 단계 상태와 리포트 산출물로 먼저 선다. 저장 모양은 이미지 스캔(`image_scan_results`)과 같은 틀로 붙일 수 있다 → **#65 에서 `dependency_scan_results`(000106)로 저장한다**(`Nullus_Security_Dashboard_설계.md` §8 G1) | #65 Security Dashboard |
| 라이선스 검사 | 조직의 허용 라이선스 정책이 먼저다 | 정책 결정 후 — `--scanners vuln,license` 와 `--license-*` 옵션 |
| SBOM 축적·이력(Dependency-Track) | 서버 하나가 더 는다. 공급망 증적(#77)은 릴리즈 이미지 단위로 이미 SBOM 을 남긴다 | 사용자 앱 SBOM 요구가 생길 때 |
| 스캐폴딩이 락파일을 만들도록 | 템플릿 앱은 시작점이고 의존성은 사용자가 정한다. 대신 0 개일 때 로그가 말한다 | 샘플 앱 개편 시 |
| 조직 공통 예외 | 저장소 `.trivyignore` 로만 | #76 §11 과 같은 보류 |

---

## 참고

- 형제 설계: [컨테이너 이미지 스캔 설계](./Nullus_컨테이너_이미지_스캔_설계.md)(서버 구조·정책 푸시·에어갭 DB), [시크릿 탐지 설계](./Nullus_시크릿_탐지_설계.md)(빌드 앞 단계)
- 코드: `internal/stack/domain/sca.go`, `internal/stack/adapter/helm/orchestrator.go`(`installing_trivy`), `internal/cicd/adapter/scaffold/dependency_scan.go`, `internal/cicd/adapter/repository/postgres_stack_reader.go`, `internal/cicd/usecase/scan_policy_service.go`, 웹 `features/stack`(Security 탭)
