# Nullus 빠른 시작

노트북 한 대에서 **설치 → 실행 → 첫 결과 확인**까지 가는 가장 짧은 길이다. 플랫폼을 띄우고, 로컬 kind 클러스터에 GitLab + Argo CD 스택을 깔고, 샘플 템플릿으로 파이프라인을 돌려 앱이 배포되는 것까지 본다.

- **기준**: [로컬 E2E 가이드](https://cloud-nullus.github.io/nullus/guides/local-e2e.html) 1~9단계를 줄였다. 그 가이드는 2026-10-28 에 macOS 에서 1단계부터 다시 돌려 영상으로 남겼다. Linux 도 명령은 같다
- **걸리는 시간**: 1시간 안팎. 대부분 스택 설치를 기다리는 시간이고, 처음에는 이미지를 받는 만큼 더 걸린다
- **모든 명령은 저장소 루트의 한 터미널에서 차례로 실행한다** — 2단계에서 정한 환경 변수를 뒤 단계가 물려받는다

## 1. 설치

### 준비물

| 도구 | 버전 |
|------|------|
| Docker (Docker Desktop 또는 OrbStack) | 데몬이 떠 있어야 한다 |
| Go | 1.26.1 이상 (`go.mod`) |
| Node.js | 22.12 이상 (Vite 8) |
| helm | 3.x 또는 4.x — 없으면 `up` 이 멈춘다 |
| kind · kubectl | 최신 (kind 노드 이미지는 `kindest/node:v1.35.1`) |
| jq · curl · python3 · lsof | 스크립트가 쓴다 |

macOS 는 Homebrew 로 한 번에 깐다(Docker 는 OrbStack 대신 Docker Desktop 이어도 된다). Linux 는 각 배포처의 바이너리를 받는다.

```bash
brew install go node kind kubectl helm jq
brew install --cask orbstack
```

**Docker 에 CPU 8개 이상, 메모리 20GB 이상을 준다.** 이 가이드가 까는 `gitlab-argocd-v1` 템플릿의 계획값이 10 vCPU · 20Gi 이고, 4 vCPU 에서는 GitLab 스택이 뜨지 않은 기록이 있다. 플랫폼만 띄우는 데는 4GB 남짓이면 된다.

API 는 로그인 셸(`bash -l`)로 뜬다. `go` · `helm` · `kubectl` 이 `~/.zprofile`(또는 `~/.bash_profile`)의 PATH 에서도 보여야 한다.

### 저장소 받기

```bash
git clone https://github.com/cloud-nullus/nullus.git
cd nullus
./scripts/runbook_local.sh preflight
```

**됐으면** 도구 버전과 `docker daemon: running` 뒤에 `preflight OK` 가 찍힌다.

## 2. 실행

플랫폼(PostgreSQL · Redis · 오브젝트 스토리지 · Keycloak · API · 웹)과 kind 클러스터 두 개를 띄우고, 클러스터를 Nullus 에 등록한다.

```bash
export NULLUS_LOCAL_DOMAIN=nullus.local
./scripts/runbook_local.sh up --kind
./scripts/setup-local-domain.sh --apply-hosts
./scripts/runbook_local.sh smoke
```

- `NULLUS_LOCAL_DOMAIN` 은 `up` 보다 먼저 정한다. 빠지면 3단계 스택 설치가 멈춘다
- `up --kind` 는 처음에 이미지와 의존성을 받느라 몇 분 걸린다. 끝에 `Nullus Local Environment Ready` 표가 찍힌다
- `setup-local-domain.sh --apply-hosts` 는 `keycloak.nullus.local` 을 kind 의 CoreDNS 와 호스트 `/etc/hosts` 에 넣는다. sudo 비밀번호를 묻는다
- 이 스크립트가 "내부 CA … 를 찾지 못했습니다", "게이트웨이 서비스를 찾지 못했습니다" 를 찍는 것은 정상이다 — 아직 스택이 없다. 마지막 "다음 단계" 안내의 `up` 은 이미 했다

**됐으면** `smoke: 16 passed, 0 failed` 가 찍힌다.

| 주소 | 무엇 |
|------|------|
| http://localhost:5173 | 웹 |
| http://localhost:8090/health | API — `"db":"connected"` 면 정상 |
| http://localhost:8180 | Keycloak 관리 콘솔 (admin / admin) |

## 3. 첫 결과 확인

### 3-1. 로그인하고 클러스터 확인

http://localhost:5173 을 열면 Keycloak 로그인으로 넘어간다. **admin@nullus.io / nullus123!** 로 들어간다. 화면은 영어가 기본이고, 오른쪽 위에서 한국어로 바꿀 수 있다.

왼쪽 메뉴 **Admin › Cluster Management** 를 연다.

**됐으면** `kind-nullus-platform`(pipeline)과 `kind-nullus-develop`(target)이 **Connected** 로 보인다. 터미널로도 같은 것을 본다.

```bash
curl -s localhost:8090/api/v1/admin/clusters | jq '.items[] | {name, types, connection_status}'
```

둘이 없으면 `./scripts/register-kind-clusters.sh` 로 등록한다.

### 3-2. 템플릿으로 스택 설치

템플릿 `gitlab-argocd-v1`(GitLab CE · GitLab Runner · GitLab Registry · 오브젝트 스토리지 · Argo CD · Prometheus · Grafana)을 `kind-nullus-platform` 에 깐다. E2E 가이드가 쓰는 `gitlab-argocd-sonarqube-v1` 에서 SonarQube 를 뺀, 가이드가 "가볍게 하려면" 으로 안내하는 구성이다.

```bash
./scripts/runbook_local.sh stack-up --template=gitlab-argocd-v1 --wait
```

이 명령은 화면의 설치와 같은 백엔드 API 를 부른다. 설치가 끝나면 kind 노드에 스택 레지스트리 주소와 내부 CA 를 넣고, GitLab 포트포워드(기본 :8181)를 띄운 뒤 API 를 다시 띄운다. 진행은 다른 터미널에서 본다: `kubectl --context kind-nullus-platform -n nullus-gitlab-argocd get pods -w`

**됐으면** `stack created: stk_…` → `state: completed` → `설치 완료` → `kind 노드 배선 완료` 가 차례로 찍히고, `NULLUS_GITLAB_URL=nullus-gitlab-argocd=http://127.0.0.1:8181` 로 API 가 다시 뜬다. E2E 가이드 실측은 12분 남짓(740초)이고, 느린 기계에서는 40분을 넘긴다. 웹의 **DevSecOps Stack › Stack List** 에 `nullus-gitlab-argocd` 가 **Completed** 로 보인다.

파드가 실제로 떠 있는지도 본다. 0 이면 클러스터에 깔지 못한 채 완료로 끝난 것이다 — `./scripts/runbook_local.sh logs api` 에서 원인을 본다.

```bash
kubectl --context kind-nullus-platform -n nullus-gitlab-argocd get pods --no-headers | wc -l
```

### 3-3. 샘플 템플릿으로 파이프라인 만들고 실행

1. 왼쪽 메뉴 **CI/CD › CI/CD Template** 에서 **Nullus Sample App — Backend** 카드의 **Use Base Template** 을 누른다
2. **Pipeline Setup** 화면에서 세 칸만 정한다. 나머지는 템플릿 값이 채워져 있다

   | 칸 | 값 |
   |----|-----|
   | App Name | `demo-api` |
   | Select Stack | `nullus-gitlab-argocd` — Cluster 는 스택의 클러스터로 고정된다 |
   | Namespace | **New Namespace** 를 고르고 `demo` |

3. 맨 아래 **Review Manifest** 에서 만들어질 매니페스트를 보고 **Execute** 를 누른다

Nullus 가 GitLab 에 프로젝트 `nullus/demo-api` 를 만들고 `.gitlab-ci.yml` · `Dockerfile` · `deploy/` 매니페스트를 첫 커밋으로 올린다. 그 커밋이 GitLab 파이프라인(`secret-scan → build → deploy`)을 시작하고, Argo CD Application `demo-api` 가 그 매니페스트를 배포한다.

**됐으면** 파이프라인 실행 로그 화면으로 넘어간다.

### 3-4. 결과 보기

5~10분 뒤에 Argo CD Application, 앱 Deployment, Nullus 가 GitLab 에서 가져온 실행 기록을 차례로 본다.

```bash
kubectl --context kind-nullus-platform -n nullus-gitlab-argocd get applications.argoproj.io demo-api
kubectl --context kind-nullus-platform -n demo get deploy,pods -o wide
PIPELINE_ID=$(curl -s localhost:8090/api/v1/cicd/pipelines | jq -r '.items[] | select(.name=="demo-api") | .id')
curl -s "localhost:8090/api/v1/cicd/deployments?pipeline_id=$PIPELINE_ID" | jq '.items[] | {version, status}'
```

**됐으면**

- Application `demo-api` 가 `Synced` · `Healthy` 다
- 파드가 `Running` 이고 이미지가 `registry.nullus.local/nullus/demo-api:<커밋 SHA>` 다. 처음 몇 분은 `:bootstrap` 태그로 `ImagePullBackOff` 가 보이는데 실패가 아니다 — 아래 "문제가 생기면" 마지막 줄
- 실행 기록의 `status` 가 `success` 다. 웹의 **CI/CD › CI/CD List** → `demo-api` → **History** 탭에도 같은 실행이 보인다

앱 응답까지 본다. 포트포워딩을 다른 터미널에서 띄우고,

```bash
kubectl --context kind-nullus-platform -n demo port-forward svc/demo-api 8080:8080
```

원래 터미널에서 부른다. `<title>Welcome to nginx!</title>` 이 나오면 된다.

```bash
curl -s localhost:8080 | grep '<title>'
```

샘플 백엔드 템플릿은 지금 nginx 자리표시자 이미지를 만든다. 저장소의 `Dockerfile` 을 고쳐 커밋하면 같은 길(GitLab CI → 레지스트리 → Argo CD)로 다시 배포된다. GitLab · Argo CD 화면을 브라우저로 열려면 게이트웨이를 열고 내부 CA 를 신뢰시킨다 — [E2E 가이드 5단계](https://cloud-nullus.github.io/nullus/guides/local-e2e.html#s5).

## 문제가 생기면

- **`port … in use` 또는 `Bind for 0.0.0.0:… failed`** — 플랫폼은 호스트 포트 5173 · 8090 · 5433 · 6380 · 8180 · 9000 · 9001 을 쓴다. `lsof -iTCP:<포트> -sTCP:LISTEN` 으로 주인을 찾아 끄고, 이전에 띄운 Nullus 면 `./scripts/runbook_local.sh down` 뒤 다시 `up --kind`
- **로그인 화면이 `keycloak.nullus.local` 을 찾지 못한다** — `/etc/hosts` 에 그 이름이 없다. `grep keycloak.nullus.local /etc/hosts` 로 보고, 없으면 `./scripts/setup-local-domain.sh --apply-hosts` 를 다시 돌린다
- **스택 설치가 끝나지 않는다** — `installing_minio` 에서 멈추면 `NULLUS_LOCAL_DOMAIN` 없이 `up` 한 것이다. 도구가 파드 안에서 닿지 못하는 `localhost:8180` 을 Keycloak 주소로 받았으므로 `./scripts/runbook_local.sh purge` 뒤 2단계부터 다시 한다. 파드가 `Pending` 에 머물면 Docker 자원이 모자라다(1단계). `--wait` 는 60분에서 끊긴다 — 그 뒤는 `./scripts/runbook_local.sh stack-status <stk_…> --wait` 로 이어 기다린다. 원인은 `./scripts/runbook_local.sh logs api` 에서 본다
- **파이프라인을 만들 때 GitLab 에 닿지 못하거나 "Pipeline created, but setup is incomplete" 가 뜬다** — GitLab 포트포워드가 죽었거나 API 가 그 주소 없이 떠 있다. `./scripts/runbook_local.sh status | grep gitlab` 에 `gitlab[nullus-gitlab-argocd]: forwarded on :…` 줄이 없거나 `not answering` 이면 `NULLUS_LOCAL_DOMAIN` 을 export 한 셸에서 `./scripts/runbook_local.sh gitlab-forward` 뒤 `./scripts/runbook_local.sh refresh`. 경고가 떠도 GitLab 파이프라인은 이미 시작됐을 수 있으니 **CI/CD List** 를 먼저 본다
- **앱 파드가 `ImagePullBackOff` 다** — 이미지 태그가 `:bootstrap` 이면 정상이다. 첫 커밋의 자리표시자 태그라, deploy 잡이 실제 태그를 되커밋하고 Argo CD 가 그 커밋을 가져오면(기본 3분 주기) 바뀐다. 바로 가져오게 하려면 `kubectl --context kind-nullus-platform -n nullus-gitlab-argocd annotate applications.argoproj.io demo-api argocd.argoproj.io/refresh=hard --overwrite`. 커밋 SHA 태그인데도 그러면 kind 노드 배선이 빠진 것이다 — `./scripts/runbook_local.sh wire`

더 많은 증상은 [E2E 가이드 부록 A](https://cloud-nullus.github.io/nullus/guides/local-e2e.html#ax-a)에 있다.

## 정리

파이프라인 → 스택 → 컨테이너·볼륨 → kind 클러스터 순서로 지운다.

```bash
./scripts/runbook_local.sh purge
```

`/etc/hosts` 에 들어간 `# nullus local domain (setup-local-domain.sh)` 표식과 그 아래 한 줄은 직접 지운다. 게이트웨이를 열었다면 `./scripts/port-forward-gateway.sh --down` 으로 라우터 컨테이너도 내린다.

## 더 보기

- [문서 사이트](https://cloud-nullus.github.io/nullus/) — 화면별 사용법, 설정, 권한 표
- [로컬 E2E 가이드](https://cloud-nullus.github.io/nullus/guides/local-e2e.html) — 이 문서 다음 단계: 보안 게이트(SonarQube · Trivy), 백업 → 삭제 → 복구, OSS 업그레이드, AI 어시스턴트
- [README](./README.md) — 동작 방식, 아키텍처, Helm 으로 클러스터에 배포하기
