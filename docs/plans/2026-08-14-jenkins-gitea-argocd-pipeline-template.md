# Jenkins + Gitea + Argo CD 파이프라인 템플릿 설계안

**작성일**: 2026-08-14
**상태**: 리뷰 대기 (구현 착수 전)
**범위**: 스택 설치 엔진 · CI/CD 프로비저닝 · 프론트엔드 전 계층
**목표 산출물**: `gitea-jenkins-argocd-v1` 스택 템플릿이 설치되고, 그 스택에서 파이프라인을 만들면 Gitea 리포 + Jenkins job + Argo CD Application 이 실제로 생성되는 상태

---

## 1. 현재 상태 — 무엇이 있고 무엇이 없나

### 1.1 이미 있는 것

| 항목 | 위치 | 상태 |
|---|---|---|
| Argo CD 설치 | `internal/stack/adapter/helm/helm_step_metadata.go:172` | ✅ 완전 동작. `config.pipeline.cd_tool.Enabled` 만 보므로 **CD 쪽은 추가 작업 없음** |
| Argo CD Application 생성 | `internal/cicd/adapter/argocd/application.go:52` | ✅ SCM 무관하게 `repoURL` 만 받으면 동작 |
| Argo CD 리포 자격증명 Secret | `internal/cicd/adapter/argocd/repository_secret.go:31` | ✅ username/password 방식이라 Gitea 에 그대로 사용 가능 |
| Gitea·Jenkins 리소스 기본값 | `db/migrations/000030_seed_missing_stack_resource_defaults.up.sql:19,24` | ✅ 이미 시드됨 (gitea 1~2 vCPU, jenkins 2~4 vCPU) |
| Gitea·Jenkins 마법사 선택지 | `web/src/features/stack/utils/install-constants.ts:19,37` / `template-config.ts:43,51` | ✅ 선택 가능 |
| Gitea·Jenkins 차트 메타 (프론트) | `install-constants.ts:187,195` | ⚠️ 값은 있으나 백엔드와 무관한 표시용 |
| 아이콘 | `web/public/tool-icons/{gitea,jenkins}.svg` | ✅ |
| i18n 키 | `web/src/i18n/{ko,en}.json:471,507` | ✅ |

### 1.2 없는 것 — 그리고 코드가 그 사실을 명시하고 있음

Gitea·Jenkins 는 **마법사에서 고를 수는 있지만 배포해도 아무것도 설치되지 않는다.**
이는 우연한 누락이 아니라 저장소가 명시적으로 선언한 상태다:

- `web/src/features/home/utils/support-tools.ts:9` — *"Jenkins·Flux·Thanos 따위 — 고를 수는 있지만 배포해도 아무것도 설치되지 않는다"*
- `web/src/features/home/utils/support-tools.test.ts:34-44` `NOT_INSTALLABLE` 이 `gitea: '설치 단계 없음'`, `jenkins: '설치 단계 없음'` 을 들고 있다.
  → **이 테스트가 게이트다.** 마법사 선택지는 "메인 지원 카드로 승격" 또는 "배선 없음 선언" 중 하나를 반드시 골라야 통과한다. 이번 작업은 전자로 옮기는 일이다.
- `internal/stack/usecase/validate_compatibility_test.go:60` 은 Jenkins 를 *호환 조합 없음* 의 대표 예시로 쓰고 있다.

Go 프로덕션 코드에서 `jenkins` / `gitea` 문자열 히트는 **0건**이다 (테스트 픽스처 제외).

### 1.3 누락 지점 정밀 목록

| # | 계층 | 파일 | 현재 동작 |
|---|---|---|---|
| A | 차트 스펙 | `helm/helm_step_metadata.go:61-258` | `installing_gitea` / `installing_jenkins` case 없음 |
| B | 차트 스펙 (DB) | `db/migrations/000056_stack_helm_step_configs.up.sql:29-41` | 13개 스텝만 시드. DB 값이 Go 기본값보다 **우선**한다 (`helm_step_metadata.go:22-27`) |
| C | 스텝 DAG | `usecase/install_stack.go:56-81` | Gitea/Jenkins 스텝 노드 없음 |
| D | 스텝 활성화 술어 | `helm/orchestrator.go:438,477` | `isGitLabSourceRepositorySelection` / `isGitLabCISelection` 만 존재 |
| E | **러너 하드 게이트** | `helm/orchestrator.go:171-176` | `gitlab-runner` 릴리스가 없으면 health check 가 스택 완료를 **차단**한다 |
| F | 워크로드 등록 | `domain/tool_workload.go:71` | `source_repository` 파드 접두사가 `"gitlab"` 하드코딩. **`ci_platform` 슬롯 자체가 없음** |
| G | SCM 플랫폼 매핑 | `cicd/adapter/provisioning/bundle_factory.go:258-266` | `platformFor()` 가 gitlab/github 만 인식, Gitea 는 `""` → 프로비저닝 거부 |
| H | 파이프라인 파일 렌더러 | `cicd/adapter/scaffold/renderer.go:33-34,126-132` | `.gitlab-ci.yml` / GitHub Actions 만. Jenkinsfile 없음 |
| I | 템플릿 시드 | `repository/memory_template.go:72` + 마이그레이션 | 7개 템플릿 모두 GitLab/GitHub 기반 |
| J | 호환성 매트릭스 | `repository/memory_compatibility.go` | Jenkins 조합 없음 |

---

## 2. 목표 / 비목표

### 목표
1. `gitea-jenkins-argocd-v1` 스택 템플릿으로 설치하면 Gitea·Jenkins·Argo CD·Harbor·MinIO·Prometheus·Grafana 가 실제로 클러스터에 뜬다.
2. 설치된 스택의 OSS 목록·모니터링 화면에 Gitea 와 Jenkins 가 나타난다.
3. 그 스택에서 파이프라인을 만들면 Gitea 리포가 생성되고, `Jenkinsfile` + `Dockerfile` + `deploy/*.yaml` 이 스캐폴딩되고, Jenkins 에 job 이 등록되고, Argo CD Application 이 생성된다.
4. 커밋 → Jenkins 빌드 → 이미지 push → 매니페스트 태그 갱신 커밋 → Argo CD 동기화 흐름이 닫힌다.

### 비목표 (이번 범위 밖)
- Tekton, Flux, Spinnaker
- Gitea Actions (Gitea 내장 CI) — Jenkins 를 CI 로 쓰므로 불필요
- 외부(클러스터 밖) Gitea/Jenkins 연동 — 스택 내부 설치만
- 기존 GitLab 경로의 동작 변경 (회귀 금지)

---

## 3. 핵심 설계 결정

### 3.1 기존 슬롯의 "또 다른 선택지" 로 추가한다

새 축을 만들지 않는다. `StackConfig` 스키마는 그대로 두고 술어만 늘린다.

```
config.artifacts.source_repository.name = "Gitea"    → installing_gitea
config.pipeline.ci_platform.name        = "Jenkins"  → installing_jenkins
config.pipeline.cd_tool.enabled         = true       → installing_argocd  (기존 그대로)
```

`isGitLabSourceRepositorySelection()` 옆에 `isGiteaSourceRepositorySelection()` 을 두고, `isGitLabCISelection()` 옆에 `isJenkinsCISelection()` 을 둔다. 기존 GitLab 술어는 손대지 않는다 — 회귀 위험이 가장 낮은 형태다.

### 3.2 Jenkins 는 GitLab CI 와 트리거 모델이 근본적으로 다르다 — 이것이 이번 작업의 최대 난점

| | GitLab CI | Jenkins |
|---|---|---|
| 파이프라인 정의 발견 | `.gitlab-ci.yml` 을 푸시하면 **자동 감지** | job 이 **먼저 존재해야** 함 |
| 트리거 | 내장 | webhook 또는 SCM 폴링 필요 |
| 실행기 | `gitlab-runner` 차트 (등록 토큰 필요) | Jenkins 가 Kubernetes plugin 으로 agent pod 을 직접 띄움 |
| 자격증명 주입 | 프로젝트 CI 변수 API | Jenkins Credentials API 또는 JCasC |

즉 GitHub 을 추가할 때처럼 "렌더러에 case 하나 추가" 로 끝나지 않는다.
**`ProvisionAppProject` 이후에 Jenkins job 을 생성하는 단계가 새로 필요하다.**

**결정: Multibranch Pipeline job + Gitea webhook** 을 쓴다.
- 대안인 Job DSL seed job 은 플러그인 의존이 하나 더 늘고, JCasC 단독은 리포마다 job 을 만들 수 없다.
- Multibranch 는 `Jenkinsfile` 을 리포에서 스스로 찾으므로 스캐폴딩 결과와 자연스럽게 맞물린다.
- Gitea 는 Jenkins `gitea-plugin` 으로 organization/multibranch 소스를 지원한다. **플러그인 가용성·버전은 구현 착수 전 확인 필요 (미해결 #1).**

### 3.3 deploy 단계는 기존 GitOps 패턴을 그대로 따른다

`renderer.go:271-286` 의 GitLab deploy job 과 동일하게, Jenkins 도 **배포하지 않는다**.
`deploy/deployment.yaml` 의 이미지 태그를 `sed` 로 바꿔 되커밋하고, Argo CD 가 그 커밋을 동기화한다.
`cicd-golden-path.md` 가 선택한 "Git + Argo CD" 방식을 유지한다.

### 3.4 CI 자격증명은 OpenBao → ESO → K8s Secret 평면에 얹는다

Gitea 에는 GitLab 의 프로젝트 CI 변수 같은 저장소가 없다. 후보는 셋이었다 —
(a) Jenkins Credentials, (b) Gitea Actions secrets API 재활용, (c) K8s Secret.
**(c) 를 택한다.** 스택이 이미 이 평면을 갖고 있고, 그것도 *선택이 아니라 항상 켜지는* 필수 경로이기 때문이다 (`orchestrator.go:456-467`).

```
Nullus 생성 → OpenBao write → ExternalSecret → K8s Secret → 소비자가 참조
```

`managedSecrets()` (`helm/secret-provisioning.go:77`) 는 선언형 목록이라 항목을 더하면
`externalSecretManifest()` (`:215`) 가 ExternalSecret 을 자동 생성한다.
`RestartRequired` 로 회전 후 재시작 전략까지 이미 모델링되어 있다.

Jenkins Credentials 를 1차 저장소로 쓰지 않는 이유: 자격증명 사본이 하나 더 생기고
회전 경로가 둘로 갈린다. OpenBao 가 단일 출처라는 원칙이 깨진다.

#### 3.4.1 자격증명은 두 부류이고, 발급 시점이 다르다

**이 구분이 이번 설계의 핵심이다.** `managedSecrets()` 는 `provisioning_secrets`(phase A) 에서
도는데, 이는 Gitea·Jenkins·Harbor 가 **설치되기 전**이다.

| 부류 | 예시 | 발급 시점 | 담당 경로 |
|---|---|---|---|
| **A. 사전 생성 가능** | Gitea admin 비밀번호, Jenkins admin 비밀번호 | 설치 전 랜덤 생성 | `managedSecrets()` 에 항목 추가 (기존 `gitlab-initial-root-password` 와 동형) |
| **B. provider 발급** | Gitea 액세스 토큰, Harbor robot 계정, git push-back 토큰 | 해당 OSS 가 뜬 **이후** | `managedSecrets()` **불가**. `internal/admin/rotation/` 재발급기 + 파이프라인 프로비저닝 시점 |

**파이프라인이 실제로 쓰는 자격증명은 대부분 B 다.** `ImageTarget.UsernameVar/PasswordVar` 와
`DeployTokenVar` 가 모두 여기 해당한다. 따라서 L4-2 의 `PipelineConfigurator` 구현은
"Gitea API 호출" 도 "no-op" 도 아니고 **OpenBao 기록 + ExternalSecret 보장**이 된다.

`internal/admin/rotation/` 에 `Reissuer` 인터페이스와 `RouterReissuer` 가 이미 있으므로
(`github_reissuer.go` 가 참고 구현), Gitea 재발급기를 같은 형태로 등록한다.

#### 3.4.2 소비자가 둘이고, 주입 방식이 다르다

| 소비자 | 무엇에 쓰나 | 주입 방식 |
|---|---|---|
| **Jenkins agent pod** | 이미지 build/push, 매니페스트 태그 되커밋 | K8s Secret → `secretKeyRef` 로 env var. Jenkinsfile 이 `$HARBOR_USERNAME` 식으로 읽는다. **Jenkins Credentials 불필요** |
| **Jenkins controller** | Gitea org multibranch 스캔, webhook 인증 | ⚠️ env var 로 안 된다. Jenkins 가 실제 credential 객체를 요구한다 |

controller 쪽이 유일한 예외다. 해법: ESO 가 만든 Secret 을 controller 파드에 마운트하고
**JCasC 의 `${VAR}` 보간**으로 credential 객체를 선언한다 (Jenkins 차트의
`controller.additionalExistingSecrets` + `controller.JCasC`). OpenBao 는 여전히 단일 출처이고,
Jenkins Credentials 는 파생 사본일 뿐이다.
**JCasC 보간이 기대대로 동작하는지는 실측이 필요하다 (미해결 #1-b).**

#### 3.4.3 Secret 범위는 파이프라인 단위로 한다

`PipelineConfigurator.SetProjectVariable(ctx, projectID, v)` 의 의미를 보존하려면
Secret 도 프로젝트 단위여야 한다. 이름은 `nullus-ci-<app>` 으로 두어
Argo CD 리포 Secret 의 기존 규약 `nullus-repo-<app>` (`argocd/repository_secret.go:12`) 과 맞춘다.

이렇게 하면 파이프라인 삭제 시 자격증명도 함께 지울 수 있고 (`delete_pipeline.go` 확장),
한 파이프라인의 자격증명 유출이 다른 파이프라인으로 번지지 않는다.

네임스페이스는 스택 네임스페이스다 — ESO `SecretStore` 가 namespace-scoped 이고
(`external-secrets.go:62`), Jenkins agent 파드도 같은 네임스페이스에서 뜬다.
앱이 배포되는 `destNamespace` 의 imagePullSecret 은 별개 경로(`kube.RenderImagePullSecret`)라
충돌하지 않는다.

### 3.5 러너 게이트를 CI 플랫폼별로 분기한다

`orchestrator.go:171-176` 은 `gitlab-runner` 릴리스 부재를 하드 실패로 만든다.
Jenkins 스택에서는 `installing_runner` 가 애초에 비활성이므로 health check 대상에 들어가지 않아야 한다.
**이 게이트를 그대로 두면 Jenkins 스택은 설치가 끝나도 절대 `completed` 가 되지 않고, `bundle_factory.go:76-79` 가 파이프라인 생성을 거부한다.** 놓치기 쉬운 함정이므로 별도 테스트로 못 박는다.

---

## 4. 계층별 변경 설계

### L1. 스택 설치 엔진

**L1-1. 차트 스펙 추가** — `internal/stack/adapter/helm/helm_step_metadata.go`

```go
case "installing_gitea":
    return ChartSpec{
        ReleaseName: domain.GiteaReleaseName,   // 신설. Service 이름 = 릴리스명
        ChartName:   "gitea",
        RepoURL:     "https://dl.gitea.com/charts/",   // ⚠️ 미해결 #2
        Version:     domain.GiteaChartVersion,          // 신설
        Values:      DefaultValues("installing_gitea"),
        Wait:        false,
    }, true
case "installing_jenkins":
    return ChartSpec{
        ReleaseName: domain.JenkinsReleaseName,
        ChartName:   "jenkins",
        RepoURL:     "https://charts.jenkins.io",
        Version:     domain.JenkinsChartVersion,
        Values:      DefaultValues("installing_jenkins"),
        Wait:        false,
    }, true
```

버전 상수는 `internal/stack/domain/connection.go:103-110` (기존 `HarborChartVersion` 등과 같은 자리)에 둔다.
`TestChartVersionsMatchCompatibilityMatrix` 가 이 값과 호환성 매트릭스를 대조하므로 양쪽을 함께 넣는다.

**L1-2. DB 스텝 설정 마이그레이션** — `db/migrations/000067_helm_step_config_gitea_jenkins.up.sql`

`stack_helm_step_configs` 에 두 행을 넣는다. `helm_step_metadata.go:22-27` 이 DB 값을 우선하므로 **Go 기본값만 넣으면 DB 가 있는 환경에서는 조회 실패로 기본값 폴백에 의존하게 된다** — 명시적으로 시드한다.
`sort_order` 는 Gitea 를 `installing_gitlab` 과 같은 자리(8), Jenkins 를 `installing_runner` 자리(10) 근처에 배치한다.

**L1-3. 스텝 DAG** — `internal/stack/usecase/install_stack.go`

```go
{name: "installing_gitea",   phase: "B", duration: time.Second, deps: []string{"provisioning_sso"}},
{name: "installing_jenkins", phase: "B", duration: time.Second, deps: []string{"provisioning_sso"}},
```

Jenkins 는 소스 저장소에 의존하지 않는다 — job 등록은 설치가 아니라 파이프라인 생성 시점이다.
(GitLab 러너가 `installing_gitlab` 에 의존하는 것과 다르다.)

**L1-4. 활성화 술어** — `internal/stack/adapter/helm/orchestrator.go`

`stepConfigFieldPath` 에:
```go
"installing_gitea":   "config.artifacts.source_repository",
"installing_jenkins": "config.pipeline.ci_platform",
```

`stepConfigEnabled` 에:
```go
"installing_gitea":   func(cfg domain.StackConfig) bool { return isGiteaSourceRepositorySelection(cfg.Artifacts.SourceRepository) },
"installing_jenkins": func(cfg domain.StackConfig) bool { return isJenkinsCISelection(cfg.Pipeline.CIPlatform) },
```

신규 술어는 기존 `isGitLabSourceRepositorySelection:524-536` 과 같은 형태(`Enabled` 확인 → `"external"` 제외 → 이름 정규화)로 쓴다.

**L1-5. 러너 게이트 분기** — `orchestrator.go:171-176`
`stepInstallingRunner` health check 예외를 "GitLab CI 를 고른 스택에서만" 적용되도록 좁힌다.

**L1-6. Gitea/Jenkins values** — `internal/stack/adapter/helm/values.go` + `helm-values.go`

- **Gitea**: 내장 SQLite 대신 스택 PostgreSQL 사용(`gitea.config.database`), 오브젝트 스토리지는 MinIO, `gitea.admin` 계정을 `provisioning_secrets` 가 만든 Secret 에서 참조. `service.http.port` 고정.
- **Jenkins**: `controller.installPlugins` 에 `kubernetes`, `workflow-aggregator`, `git`, `gitea`, `configuration-as-code` 지정. `persistence.storageClass` 를 선택된 StorageClass 로. `controller.jenkinsUrl` 을 access domain 기반으로.
  §3.4.2 에 따라 `controller.additionalExistingSecrets` 로 ESO Secret 을 마운트하고
  `controller.JCasC.configScripts` 에서 `${VAR}` 보간으로 Gitea credential 을 선언한다.
- **OIDC**: `provisioning_sso` 가 OpenBao 로 클라이언트를 만든다. Gitea 는 `oauth2_client`, Jenkins 는 `oic-auth` 플러그인. `domain/release_values.go:39-64` 의 보호 키 목록에도 등록한다.

**L1-8. 사전 생성 자격증명** — `helm/secret-provisioning.go:77` `managedSecrets()`

§3.4.1 의 A 부류를 추가한다. 기존 `ProvisionedGitLabRootSecret` 항목과 같은 형태:

```go
{
    TargetSecret:    ProvisionedGiteaAdminSecret,
    Consumer:        "Gitea",
    RestartRequired: true,
    Entries: []SecretEntry{
        {PathSuffix: "artifacts/gitea/admin-username", TargetKey: "username", Fixed: GiteaAdminUser},
        {PathSuffix: "artifacts/gitea/admin-password", TargetKey: "password"},
    },
},
{
    TargetSecret:    ProvisionedJenkinsAdminSecret,
    Consumer:        "Jenkins",
    RestartRequired: true,
    Entries: []SecretEntry{
        {PathSuffix: "cicd/jenkins/admin-username", TargetKey: "jenkins-admin-user", Fixed: JenkinsAdminUser},
        {PathSuffix: "cicd/jenkins/admin-password", TargetKey: "jenkins-admin-password"},
    },
},
```

키 이름은 각 차트가 `existingSecret` 으로 요구하는 이름과 정확히 일치해야 한다
(Jenkins 차트는 `jenkins-admin-user`/`jenkins-admin-password` 를 읽는다 — 확인 필요).
Secret 이름 상수는 다른 항목과 같이 `domain` 에 둔다.

⚠️ B 부류(Gitea 액세스 토큰, Harbor robot)는 **여기 넣으면 안 된다.** 이 스텝은 phase A 라
Gitea·Harbor 가 아직 존재하지 않는다.

**L1-7. 게이트웨이 라우트** — `helm/manifest-builders.go:144-149`
`gitea.<domain>`, `jenkins.<domain>` 라우트를 추가한다. Harbor 가 UI/레지스트리 두 호스트를 만드는 패턴을 참고.

### L2. 워크로드 등록 (모니터링 노출)

**`internal/stack/domain/tool_workload.go`** — 여기에 없으면 파드가 멀쩡히 떠도 어느 화면에도 안 나온다.

1. **`source_repository` 접두사 하드코딩 제거** (`:71`)
   현재 `"gitlab"` 고정이라 Gitea 를 고르면 `gitlab-*` 파드를 찾다가 "0 파드 warning" 으로 남는다.
   `logStoreNamePrefixes` / `registryNamePrefixes` 와 같은 **선택 기반 동적 접두사 함수** `sourceRepositoryNamePrefixes(sel)` 를 신설한다.
2. **`ci_platform` 슬롯 신설** — 현재 후보 목록에 CI 플랫폼 자체가 없다.
   GitLab CI 는 `gitlab-runner-*`, Jenkins 는 `jenkins-*` 접두사.
   GitLab 스택에서 소스 저장소 슬롯과 이중 계상되지 않도록 주의 (Nexus 가 두 슬롯 겸할 때 쓰는 `slices.Equal` 가드 패턴 재사용).
3. `canonicalToolNameByKey:21` 에 `ci_platform` 기본 이름 추가.

### L3. 호환성 매트릭스 + 템플릿 시드

**L3-1. 호환성 매트릭스**
`Gitea + Jenkins + Argo CD + Harbor` 조합을 검증된 조합으로 추가한다.
차트 버전은 L1-1 의 `domain` 상수와 반드시 일치해야 한다 (`TestChartVersionsMatchCompatibilityMatrix`).

**L3-2. 스택 템플릿** — ⚠️ **세 곳을 동기화해야 한다**

1. `db/migrations/000068_seed_gitea_jenkins_template.up/down.sql` → `golden_path_templates`
2. `internal/stack/adapter/repository/memory_template.go:72` `goldenPathTemplates()`
   → `seed_migration_contract_test.go` 가 둘의 일치를 강제한다
3. `internal/cicd/adapter/repository/memory_cicd_template.go:151` `cicdGoldenPaths()`
   → 스택 템플릿 ID 를 재선언하는 **중복 레지스트리**. 기존 3건이 이미 버전 드리프트 상태다 (미해결 #4).

템플릿 내용:

```json
{
  "id": "gitea-jenkins-argocd-v1",
  "name": "Gitea + Jenkins + Argo CD",
  "tools": [
    {"category":"source_repository","name":"Gitea","helm_version":"<GiteaChartVersion>","app_version":"..."},
    {"category":"ci_platform","name":"Jenkins","helm_version":"<JenkinsChartVersion>","app_version":"..."},
    {"category":"cd_tool","name":"Argo CD","helm_version":"<ArgoCDChartVersion>","app_version":"..."},
    {"category":"container_registry","name":"Harbor","helm_version":"<HarborChartVersion>","app_version":"..."},
    {"category":"storage_backend","name":"MinIO", ...},
    {"category":"monitoring_collection","name":"Prometheus", ...},
    {"category":"monitoring_visualization","name":"Grafana", ...}
  ],
  "recommended_use_case": "기존 Jenkins 운영 조직, 경량 Git 서버 선호",
  "min_resources": "10 vCPU / 20Gi RAM / 120Gi Storage"
}
```

⚠️ `estimated_install_time` 은 나노초를 `INTEGER` 컬럼에 넣는 기존 버그가 있다 (90분 = 5.4e12 > int32). 새 템플릿도 같은 경로를 타므로 **미해결 #5** 로 분리한다.

**L3-3. CI/CD 파이프라인 템플릿** — `pipeline_templates` 에 Jenkins 용 stage 조합 행 추가 (`["Build","Test","ImageBuild","Deploy"]`).

### L4. CI/CD 프로비저닝 — Gitea SCM 어댑터

**L4-1. 플랫폼 상수** — `internal/cicd/port/scm_connection.go:11-16`
```go
SCMPlatformGitea SCMPlatform = "gitea"
```

**L4-2. Gitea 클라이언트** — 신규 `internal/cicd/adapter/gitea/`

Gitea API 는 GitHub API 와 형태가 매우 유사하므로 `adapter/github/client.go` 를 참고 구현으로 쓴다.

| 파일 | 구현할 인터페이스 | 비고 |
|---|---|---|
| `client.go` | `port.SCMProvisioner` | `EnsureGroup`→org, `EnsureProject`→repo, `CommitFiles`, `DeleteProject`. 기본 URL `http://<release>-http.<ns>.svc:3000` |
| `pipeline_config.go` | `port.PipelineConfigurator` | **§3.4 결정에 따라 K8s Secret 경로.** `SetProjectVariable` → OpenBao 기록 + `nullus-ci-<app>` ExternalSecret 보장. `CreateProjectAccessToken` → Gitea 토큰 API |
| `token_issuer.go` | `port.SCMTokenIssuer` | `gitea admin user generate-access-token` 을 `kubectl exec` 로 실행. GitLab 의 Rails 콘솔 방식(`gitlab/token_issuer.go:152-163`)과 같은 구조 |

**L4-2b. Gitea 재발급기** — 신규 `internal/admin/rotation/gitea_reissuer.go`

§3.4.1 의 B 부류(액세스 토큰)를 회전 스케줄러에 태운다.
`github_reissuer.go` 를 참고 구현으로 쓰고 `RouterReissuer.Register("gitea", ...)` 로 등록한다.

**L4-2c. ExternalSecret 헬퍼 재사용 여부 결정 필요**

`externalSecretManifest()` (`helm/secret-provisioning.go:215`) 는 현재 `internal/stack` 안에 있다.
`internal/cicd` 에서 쓰려면 **모듈 간 직접 import 금지** 규약에 걸린다.
→ `internal/shared/` 로 올리거나, `cicd` 쪽에 별도 렌더러를 두어야 한다. **미해결 #8.**

**L4-3. 번들 팩토리** — `bundle_factory.go`
- `platformFor()` 에 `"gitea"` 케이스 추가
- `giteaBundle()` 신설 (`gitLabBundle():94` 와 같은 형태 — 클러스터 내부 서비스 DNS, 토큰 재발급 재시도 포함)
- `registry.ResolverFor` 는 Harbor 를 이미 지원하므로 그대로 사용

### L5. Jenkinsfile 스캐폴딩 + Jenkins job 등록

**L5-1. 렌더러** — `internal/cicd/adapter/scaffold/renderer.go`

```go
JenkinsfilePath = "Jenkinsfile"

func renderPipelineFor(platform port.SCMPlatform, ci CIPlatform, app string, target *port.ImageTarget) (path, content string)
```

⚠️ **현재 시그니처는 `platform` 하나로 분기한다.** Gitea + Jenkins 는 "SCM 은 Gitea, CI 는 Jenkins" 라 **SCM 과 CI 축이 분리되어야 한다.** 이 시그니처 변경이 L5 의 핵심이며, 기존 GitLab/GitHub 경로가 회귀하지 않도록 계약 테스트를 먼저 쓴다.

Jenkinsfile 내용(선언적 파이프라인, Kubernetes agent):
```groovy
pipeline {
  agent { kubernetes { ... }}       // dind 또는 kaniko
  environment { IMAGE_REPOSITORY = '...'; REGISTRY_HOST = '...'; IMAGE_TAG = "${env.GIT_COMMIT.take(8)}" }
  stages {
    stage('Build')  { steps { /* docker login / build / push */ }}
    stage('Deploy') { steps { /* sed 태그 치환 → commit → push (기존 GitLab deploy job 과 동일 패턴) */ }}
  }
}
```
`when { branch 'main' }` 으로 기본 브랜치 한정. 되커밋 메시지에 `[skip ci]` 상당 처리 필요 (미해결 #6).

**L5-2. Jenkins job 등록** — 신규 `internal/cicd/adapter/jenkins/`

새 포트가 필요하다. 기존 `SCMProvisioner`/`PipelineConfigurator` 어디에도 "CI 서버에 job 을 만든다" 개념이 없다.

```go
// internal/cicd/port/ci_server.go (신규)
type CIJobProvisioner interface {
    EnsureJob(ctx context.Context, spec CIJobSpec) (*CIJob, error)
    DeleteJob(ctx context.Context, jobID string) error
}
```

`port.SCMBundle` 에 `CIJobs CIJobProvisioner` 필드를 더하고, `ProvisionAppProject` 에서 스캐폴딩 커밋 직후 호출한다.
GitLab/GitHub 번들은 이 필드를 `nil` 로 두고, 호출부는 nil 이면 건너뛴다 — **기존 경로 무영향.**

**L5-3. Gitea → Jenkins webhook**
Gitea 리포에 Jenkins multibranch 스캔을 트리거하는 webhook 을 건다. `gitea/client.go` 에 `EnsureWebhook` 추가.

### L6. Argo CD 연동

**변경 거의 없음.** `RenderRepositorySecret` 은 username/password 를 받으므로 Gitea 액세스 토큰을 그대로 넣으면 된다.
확인할 지점: `provision_pipeline_repository.go:130-137` `pullSecretUsername()` 이 GitHub owner / `nullus-argocd-read` 로 분기한다 — Gitea + Harbor 조합에서 어느 쪽을 쓸지 결정 필요.

### L7. 프론트엔드

대부분 이미 있다. 남은 일:

1. `web/src/features/home/utils/support-tools.ts` — Gitea·Jenkins 카드 추가
2. `web/src/features/home/utils/support-tools.test.ts` — `NOT_INSTALLABLE` 에서 두 항목 제거 (**이 테스트가 게이트**)
3. `stack-config-store.ts:34,43` 의 차트/앱 버전을 백엔드 `domain` 상수와 일치시킨다
4. `install-constants.ts:187` Gitea repoUrl 검증 (미해결 #2)
5. `template-config.ts:80-101` `TEMPLATE_DESCRIPTION_I18N` 에 `gitea-jenkins-argocd-v1` 설명 추가
6. i18n 키를 `en.json`/`ko.json` 양쪽에 등록 — `t()` 인라인 fallback 은 테스트에서 동작하지 않는다
7. `tool-logo.ts` 매핑 확인 (아이콘 파일은 이미 존재)

---

## 4.5 실측 결과 (2026-08-14)

빈 kind 클러스터(`kind-nullus-platform`)를 확보한 뒤 차트를 직접 조회해 확인했다.

| 항목 | 결과 |
|---|---|
| **Gitea 차트 저장소** | `dl.gitea.io/charts` 와 `dl.gitea.com/charts` **둘 다 동작하며 내용이 동일**하다. 백엔드는 공식 도메인인 `https://dl.gitea.com/charts` 를 쓴다 |
| **Gitea 차트 버전** | `12.7.0` (app `1.27.0`). 프론트 기재값 `10.4.0`/`1.22.2` 는 크게 뒤처져 있어 갱신 필요 |
| **Gitea admin 키** | `gitea.admin.existingSecret` 이 읽는 키는 **`username`, `password`** (`templates/gitea/deployment.yaml:272-283`) |
| **Jenkins 차트 버전** | `5.9.54` (app `2.568.2`) |
| **Jenkins admin 키** | `controller.admin.userKey = jenkins-admin-user`, `passwordKey = jenkins-admin-password` |
| **JCasC 시크릿 보간** | ✅ 동작 확인. `controller.additionalExistingSecrets: [{name, keyName}]` → JCasC 에서 `${name-keyName}` 으로 참조 (이름과 키를 `-` 로 연결). `controller.existingSecret` 을 쓰면 접두사 없이 `${keyName}` |
| **Jenkins gitea 플러그인** | ✅ 활발히 유지보수 중 (plugin id `gitea`, 최신 `282.v31f586d2cb_30`, 7일 전 릴리스). multibranch / organization folder SCM 소스 제공 |

### ⚠️ 실측에서 나온 새 제약 — Jenkins 최소 버전

**gitea 플러그인은 Jenkins `2.528.3` 이상을 요구한다.**

- 프론트가 광고하는 `2.452.3` (`stack-config-store.ts:43`) 으로는 **플러그인을 설치할 수 없다.**
- 차트 `5.9.54` 의 app version `2.568.2` 는 요건을 만족한다 → **이 조합으로 고정한다.**
- 즉 Jenkins 차트 버전은 자유롭게 내릴 수 없다. 호환성 매트릭스와 템플릿 시드에 이 하한을 반영하고,
  `TestJenkinsChartVersion_SatisfiesGiteaPluginMinimum` 으로 못 박는다.

---

## 4.6 구현 현황 (2026-08-14)

7개 PR 모두 커밋됨. Go 전체 빌드·vet·테스트 통과, 프론트 751 테스트 + tsc 통과.

| PR | 브랜치 | 상태 |
|---|---|---|
| 1 | `feat/stack/gitea-install-step` | ✅ |
| 2 | `feat/stack/jenkins-install-step` | ✅ |
| 3 | `feat/stack/gitea-jenkins-template` | ✅ |
| 4 | `feat/cicd/gitea-scm-adapter` | ✅ |
| 5 | `feat/cicd/jenkinsfile-scaffold` | ✅ |
| 6 | `feat/cicd/jenkins-job-provisioning` | ✅ |
| 7 | `feat/ui/gitea-jenkins-support` | ✅ |

### ⚠️ 아직 닫히지 않은 구간 — 이대로는 파이프라인이 끝까지 돌지 않는다

설치(스택 → Gitea·Jenkins·Argo CD 기동)와 프로비저닝 코드 경로는 완성됐지만,
**런타임 배선과 자격증명 평면이 남아 있어 실제 빌드는 아직 돌지 않는다.**

| # | 남은 일 | 없으면 무슨 일이 생기나 |
|---|---|---|
| R1 | **`cmd/api/main.go` 배선** — `BundleFactory.WithGitea(...)` / `WithJenkins(...)` 호출 | Gitea 스택에서 파이프라인 생성이 *"Gitea 연동이 배선되지 않아 stack ... 를 프로비저닝할 수 없습니다"* 로 거부된다. 가장 먼저 해야 할 일 |
| R2 | **`nullus-ci-<app>` Secret 생성** (§3.4.3) — OpenBao 기록 + ExternalSecret 렌더 | Jenkinsfile 의 `envFrom.secretRef` 가 없는 Secret 을 가리켜 agent 파드가 기동하지 못한다. 미해결 #8(모듈 경계) 결정이 선행돼야 한다 |
| R3 | **Gitea 용 `PipelineConfigurator`** | 지금은 `bundle.Pipeline` 이 nil 이라 `configureGitLabPipeline` 으로 떨어져 GitLab 문구의 경고와 함께 누락 변수만 보고한다(패닉은 없음). R2 와 같은 작업이다 |
| R4 | **JCasC `nullus-gitea` credential** — Jenkins values 에 `additionalExistingSecrets` + `configScripts` | multibranch job 이 private Gitea 리포를 스캔하지 못해 브랜치를 하나도 찾지 못한다. `giteaCredentialID` 상수가 이 이름을 기대하고 있다 |
| R5 | **`internal/admin/rotation/gitea_reissuer.go`** | 액세스 토큰이 만료되면 자동 회전되지 않는다. 초기 동작에는 지장 없음 |

R1 → R4 → R2/R3 순서가 자연스럽다. R1 만 해도 리포 생성·스캐폴딩·Argo CD
Application 까지는 동작하고, R4 까지 하면 job 이 브랜치를 찾는다. R2/R3 이
끝나야 빌드가 레지스트리에 push 할 수 있다.

---

## 5. 미해결 질문 — 구현 착수 전 확인 필요

| # | 질문 | 왜 중요한가 | 확인 방법 |
|---|---|---|---|
| 1 | ~~Jenkins `gitea-plugin` 유지보수 상태~~ | **해소됨 → §4.5.** 활발히 유지보수 중, multibranch 지원. 단 Jenkins ≥ 2.528.3 제약이 새로 생김 | — |
| 1-b | ~~JCasC 보간 가능 여부~~ | **해소됨 → §4.5.** `additionalExistingSecrets` → `${name-keyName}` 로 동작 | — |
| 2 | ~~Gitea 차트 저장소 URL~~ | **해소됨 → §4.5.** 양쪽 다 동작·동일. 백엔드는 `dl.gitea.com` 사용 | — |
| 3 | ~~Gitea 에 CI 변수 저장소가 없다~~ | **해소됨 → §3.4.** OpenBao → ESO → K8s Secret 평면을 쓴다. 남은 파생 질문은 #1-b 와 #8 | — |
| 4 | `CICDGoldenPath` 중복 레지스트리를 이번에 정리할 것인가 | 기존 3건이 이미 버전 드리프트 상태 (GitLab 8.7.2 vs 9.5.1, Argo CD 7.7.2 vs 6.8.0). 새 템플릿을 여기 또 넣으면 부채가 늘어난다 | 별도 리팩터 PR 로 분리 권장 |
| 5 | `estimated_install_time` int32 오버플로우 | 90분 = 5.4e12 로 `INTEGER` 범위 초과. 기존 템플릿도 영향 | 별도 fix PR |
| 6 | Jenkins 되커밋 루프 차단 | GitLab 은 `[skip ci]` 로 끊는다. Jenkins multibranch 는 이를 자동 인식하지 않음 | `[ci skip]` + branch 필터 또는 `changeset` 조건 |
| 7 | 한 클러스터 = 스택 하나 제약과의 상호작용 | OpenBao ClusterRoleBinding 소유권 때문에 두 번째 스택 설치는 의도적으로 차단된다. Gitea 스택 검증 시 기존 GitLab 스택을 지워야 함 | 검증 절차에 반영 |
| 8 | `externalSecretManifest()` 를 `internal/cicd` 에서 어떻게 쓸 것인가 | 현재 `internal/stack` 안에 있어 **모듈 간 직접 import 금지** 규약에 걸린다 | `internal/shared/` 승격 vs cicd 전용 렌더러 — 리뷰에서 결정 |
| 9 | ~~차트 admin `existingSecret` 키 이름~~ | **해소됨 → §4.5.** Jenkins `jenkins-admin-user`/`jenkins-admin-password`, Gitea `username`/`password` | — |

---

## 6. 구현 순서 — PR 분할

각 PR 은 독립적으로 머지 가능하고, 앞 PR 없이는 뒤 PR 이 의미 없도록 배치했다.
브랜치·커밋 규약은 `docs/20_개발가이드/Nullus_PR_커밋_컨벤션.md` (v3) 를 따른다.

| PR | 브랜치 | 내용 | 검증 기준 |
|---|---|---|---|
| 1 | `feat/stack/gitea-install-step` | L1-1~L1-7 중 Gitea 부분 + **L1-8 Gitea admin 시크릿** + L2 소스 저장소 접두사 동적화 | Gitea 를 고른 스택이 설치 완료되고 파드가 뜨고 OSS 목록에 나온다 |
| 2 | `feat/stack/jenkins-install-step` | L1 중 Jenkins 부분 + **L1-8 Jenkins admin 시크릿** + L1-5 러너 게이트 분기 + L2 `ci_platform` 슬롯 신설 | Jenkins 스택이 `completed` 에 도달한다 (게이트 회귀 테스트 포함) |
| 3 | `feat/stack/gitea-jenkins-template` | L3 전체 (호환성 매트릭스 + 3곳 템플릿 시드) | 템플릿 카드가 뜨고, 선택 시 설치 계획이 실제 스텝과 일치한다 |
| 4 | `feat/cicd/gitea-scm-adapter` | L4 전체 (L4-2b 재발급기, L4-2c 헬퍼 위치 결정 포함) | Gitea 스택에서 리포 생성 + 파일 커밋이 되고, CI 자격증명이 OpenBao→ESO→`nullus-ci-<app>` 로 흐른다 |
| 5 | `feat/cicd/jenkinsfile-scaffold` | L5-1 (렌더러 SCM/CI 축 분리) | GitLab/GitHub 경로 무회귀 + Jenkinsfile 생성 |
| 6 | `feat/cicd/jenkins-job-provisioning` | L5-2, L5-3, L6 | 커밋 → 빌드 → push → 태그 갱신 → Argo CD 동기화 전 구간 |
| 7 | `feat/ui/gitea-jenkins-support` | L7 전체 | `NOT_INSTALLABLE` 에서 제거해도 테스트 통과 |

---

## 7. 테스트 계획

프로젝트 규약대로 **테스트를 먼저 쓰고 실패를 확인한 뒤** 구현한다.

### Domain (단위, 100%)
- `TestInstalledToolWorkloads_Gitea_UsesGiteaPrefix` — 접두사 하드코딩 회귀 방지
- `TestInstalledToolWorkloads_Jenkins_RegistersCIPlatformSlot`
- `TestInstalledToolWorkloads_GitLab_NoDoubleCount` — 소스/CI 슬롯 이중 계상 방지
- `TestChartVersionsMatchCompatibilityMatrix` — 기존 테스트가 신규 상수까지 덮는지 확인

### UseCase (단위 + 모킹)
- `TestInstallStack_GiteaSelected_SchedulesGiteaStep`
- `TestInstallStack_JenkinsSelected_SkipsGitLabRunner`
- `TestInstallStack_JenkinsStack_CompletesWithoutRunnerRelease` — **L1-5 게이트 회귀 방지, 가장 중요**
- `TestValidateCompatibility_GiteaJenkinsArgoCD_Compatible` — 현재 Jenkins 를 *비호환* 예시로 쓰는 `validate_compatibility_test.go:60` 을 함께 갱신

### Orchestrator (계약)
- `TestDefaultChartSpecForStep_GiteaJenkins` — 차트 스펙 존재 + 버전이 domain 상수와 일치
- `TestSeedMigrationContract` — 기존 가드가 새 템플릿까지 덮는지

### 시크릿 평면 (§3.4)
- `TestManagedSecrets_IncludesGiteaAndJenkinsAdmin` — A 부류 등록 확인
- `TestManagedSecrets_ExcludesProviderIssuedTokens` — **B 부류가 phase A 로 새어 들어오지 않는지.** 가장 틀리기 쉬운 지점
- `TestExternalSecretManifest_GiteaAdmin_MatchesChartKeys` — 차트가 요구하는 키 이름과 `TargetKey` 일치 (미해결 #9 를 테스트로 고정)
- `TestGiteaPipelineConfigurator_SetProjectVariable_WritesOpenBaoAndExternalSecret`
- `TestGiteaPipelineConfigurator_SecretScopedPerPipeline` — `nullus-ci-<app>` 네이밍, 파이프라인 간 격리
- `TestGiteaReissuer_Reissue_RotatesAccessToken`

### CI/CD 어댑터
- `TestGiteaClient_EnsureProject_Idempotent` (httptest)
- `TestBundleFactory_GiteaStack_ReturnsGiteaBundle`
- `TestBundleFactory_UnknownSCM_StillRefuses` — `platformFor` 확장이 폴백을 열지 않았는지
- `TestRenderPipelineFor_GitLab_Unchanged` / `_GitHub_Unchanged` — **L5-1 시그니처 변경 회귀 방지**
- `TestRenderPipelineFor_GiteaJenkins_EmitsJenkinsfile`

### 프론트엔드
- `support-tools.test.ts` — `NOT_INSTALLABLE` 제거 후 통과
- 템플릿 카드 렌더링 (`stack-template-page.test.tsx`)

### E2E (Playwright)
- 템플릿 선택 → 설치 마법사 → 설치 완료 → OSS 목록에 Gitea·Jenkins 노출
- 파이프라인 생성 → Gitea 리포 + Jenkins job + Argo CD Application 존재 확인

---

## 8. 리스크

| 리스크 | 영향 | 완화 |
|---|---|---|
| Jenkins gitea-plugin 이 기대대로 동작하지 않음 | L5-2 재설계 | 미해결 #1 을 PR 1 착수 전에 실측으로 해소 |
| `renderPipelineFor` 시그니처 변경이 GitLab/GitHub 회귀 유발 | 기존 사용자 파이프라인 파손 | 변경 전에 두 플랫폼 골든 파일 테스트를 먼저 고정 |
| 러너 게이트 분기 누락 | Jenkins 스택이 영원히 `completed` 안 됨 → 파이프라인 생성 불가 | 전용 회귀 테스트 (PR 2) |
| 세 템플릿 레지스트리 동기화 실패 | 화면과 실제 설치가 어긋남 | `seed_migration_contract_test.go` 활용 + 미해결 #4 를 별도 PR 로 |
| 검증 시 기존 GitLab 스택과 충돌 | 두 번째 스택 설치가 의도적으로 차단됨 | 검증 환경에서 기존 스택 제거 후 진행 |
| B 부류 자격증명을 `managedSecrets()` 에 잘못 넣음 | phase A 에는 Gitea/Harbor 가 없어 프로비저닝이 실패하거나 빈 값이 굳는다 | §3.4.1 구분을 테스트로 고정 (`TestManagedSecrets_ExcludesProviderIssuedTokens`) |
| 차트 `existingSecret` 키 이름 불일치 | 파드가 FailedMount 로 영원히 안 뜬다 — 원인이 멀리 떨어진 실패 | 미해결 #9 를 PR 1·2 착수 전 실측 |
| JCasC 보간 실패 | Jenkins 가 Gitea 를 스캔하지 못해 job 이 안 돈다 | 미해결 #1-b 실측. 실패 시 Credentials API 직접 호출로 선회 |

---

## 9. 참고

| 자료 | 경로 |
|---|---|
| 템플릿 추가 절차 | `docs/20_개발가이드/Nullus_템플릿_추가_가이드.md` |
| CD 방식 선택 근거 | `cicd-golden-path.md` |
| PR·커밋 규약 | `docs/20_개발가이드/Nullus_PR_커밋_컨벤션.md` (v3) |
| 설치 스텝 카탈로그 | `internal/stack/adapter/helm/helm_step_metadata.go` |
| 스텝 DAG | `internal/stack/usecase/install_stack.go:29-82` |
| 워크로드 단일 관문 | `internal/stack/domain/tool_workload.go:39` |
| SCM 번들 조립 | `internal/cicd/adapter/provisioning/bundle_factory.go` |
| 파이프라인 파일 렌더러 | `internal/cicd/adapter/scaffold/renderer.go` |
