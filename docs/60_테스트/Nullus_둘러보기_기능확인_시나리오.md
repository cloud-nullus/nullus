# Nullus 둘러보기 기반 기능 확인 시나리오

> 제품 투어(둘러보기, PR #147 · `web/src/features/tour/tour-steps.ts`)가 안내하는 여정을 그대로 기능 검증 시나리오로 변환한 기록지다.
> 환경: 로컬 — web `:5174` · API `:8091` · PostgreSQL `:5433` · kind 듀얼(`nullus-platform` 3노드 / `nullus-develop` 2노드, v1.35.1) · Mock Auth(admin@nullus.dev)

작성일: 2026-08-17 · 기록: 결과 칸에 `✅ 통과 / ❌ 실패(증상) / ⏳ 대기 / ➖ 부분` 로 기입

---

## 시나리오 표

| # | 시나리오 (투어 구간) | 계획 (절차 → 기대 결과) | 결과 |
|---|---------------------|------------------------|------|
| S0 | **접속·로그인** (전제) | `:5174` 접속 → `admin@nullus.dev / admin123` 로그인 → 홈 진입 | ✅ 통과 (스모크에서 검증 — 로그인·홈 렌더 정상) |
| S1 | **홈·둘러보기 시작** (welcome·quickStart) | 홈에서 히어로 CTA·Quick Start 카드 노출 확인 → "둘러보기" 시작 → 오버레이가 첫 스텝을 하이라이트 | ✅ 통과 (2026-08-17 Playwright) — 헤더 🎓 버튼으로 투어 시작, "1/30 Welcome to Nullus" 오버레이 + CTA 하이라이트 + End/Back/Next 컨트롤 확인 |
| S2 | **클러스터 등록** (registerCluster·clusterForm) | Admin→Cluster Management → Register Cluster 다이얼로그 열림 → kind kubeconfig 붙여넣기 등록 → Verify → `Connected` | ✅ 통과 — API 등록·Verify(platform 3노드·develop 2노드, v1.35.1) + **UI 확인**: 목록 2건 `Connected` 배지, 상세 패널에 타입/가용 리소스/Verify 버튼 정상 |
| S3 | **템플릿 선택** (pickTemplate·templateDetail·useBaseTemplate) | Stack Template → **Lite 템플릿**(Gitea+Jenkins+Argo CD Lite) 상세 열기 → "이 템플릿 사용" → 설치 마법사 진입 | ✅ 통과 (Playwright) — 상세 다이얼로그 열림 → "이 템플릿 사용" 클릭 → `/stack/install` 진입 |
| S4 | **설치 마법사 7탭** (installAuthentication~DryRun) | authentication → artifacts → pipeline → monitoring → storage → resources → **dry-run** 탭 순회: 각 탭 열림·입력 유지·클러스터 선택지에 kind 표시 → Dry Run 결과 렌더 | ✅ 통과 (Playwright) — **7탭 전부 전환 성공**, 클러스터 선택지에 `kind-nullus-platform` 노출 확인 |
| S5 | **스택 배포** (deployStack) | Deploy 클릭 → 배포 시작 → 진행률·실시간 로그(WS) 표시 → 완료 상태 | ✅ 파이프라인 검증 (headed) — `installing→completed`, 기반 6릴리스(cert-manager·envoy-gw·ESO·openbao·postgresql·metrics) Running, 이상 파드 0. **단 [발견 F1]로 empty 구성 배포됨** → 도구 포함 재배포는 `lite-e2e-v2`(stk_8b349a88bb28)로 **진행 중(2026-08-17 중단 시점)** |
| S6 | **스택 확인** (stackList·Workloads·gatewayPfCopy·hostsCopy·stackMonitoring) | Stack List 에 스택 표시 → 상세 workloads 탭 파드 목록 → Info 탭 gateway 포트포워드/hosts 복사 버튼 동작 → 스택 monitoring 탭 | ✅ 통과 (headed) — 행 표시(Running·클러스터)·탭 전환, **복사 2종 클립보드 실검증**(포트포워드 스크립트 `KUBE_CONTEXT=kind-nullus-platform`·hosts 엔트리). workloads "0/0 no pods" 는 도구 없는 스택과 정합. **재검 완료(08-17 Playwright)**: lite-e2e-v2 workloads 탭 **9/9 pods ready** — argocd 7·gitea·jenkins 파드 전부 Running·재시작 0 표시 확인 |
| S7 | **CI/CD 셀프서비스** (cicdBasicInfo~Create·pipelineList) | CI/CD→Developer Deploy 6단계 폼(기본정보→checkout→build→test→security→생성) 순회 입력 → 생성 → CI/CD List 에 파이프라인 표시 | ✅ **최종 통과** — ① UI: Execute → `POST /pipelines` 201 → active, deploy 202, 실패 배포 `failed` 정직 기록(1차, [F6][F7] 로 빌드 실패) ② **마지막 마일 완주**(API 경로): Gitea 시드(`gitea_admin/spring-sample`) + clone URL 을 PF(`localhost:3100`)로 지정 → `pip_53735947bf4e` → **clone→docker build→kind load(nullus-develop)→apply 전 구간 성공, `dep_14d781f7f8c1 success`, 파드 `e2e-direct` 1/1 Running, 앱 HTTP 응답 실확인** |
| S8 | **관측** (observe·alertRules) | Monitoring Dashboard 에서 클러스터/스택 필터에 kind 클러스터 노출·선택 시 패널 렌더 → Alert Rules 신규 작성 폼 열림·저장 | ✅ **완전 통과** — 필터에 kind 클러스터 노출 + **Alert Rule 실제 저장**(`e2e-cpu-alert` 생성·목록 표시 확인, 08-17 재개 세션) |
| S9 | **마무리** (finish) | 투어 종료 → Quick Start 로 복귀(시작 자리 = 끝 자리) | ✅ 통과 (2026-08-17 Playwright) — **30 스텝 전체 Next 순회**(전 스텝 카드 렌더 확인), 30/30 finish 가 홈 `/` 의 Quick Start 를 하이라이트 → Finish 클릭 시 오버레이 종료·홈 유지·Quick Start 노출(**시작 자리 = 끝 자리 확인**) |
| R1 | **반응형 회귀** (부가 — 투어 외) | `npm run responsive:audit` 27건(9화면×3뷰포트) 오버플로우·사이드바 검사 | ✅ 통과 — **27/27 이슈 0** (모바일 aside 48px, #150 fix 확인) |

---

## 환경 전제·주의 (이번 기동에서 실측된 것)

- 포트: 8090(Rancher mux)·5173(타 프로젝트) 점유 → **API 8091·web 5174** 사용, vite proxy 임시 수정 상태(커밋 금지)
- kind on Rancher VM: `fs.inotify.max_user_instances` 기본 128 로는 노드 부팅 실패 → **1024 상향 필요**(VM 재시작 시 원복)
- kind 생성 격동 시 docker 데몬 재시작으로 **compose 인프라가 조용히 내려갈 수 있음** → 시나리오 전 `/health` 확인 습관
- 스택 배포는 **Lite 템플릿 한정** (VM 4 vCPU — GitLab All-in-One 등 불가)

## 발견 사항 (2026-08-17 실행분 — 이슈 등록 후보)

| # | 발견 | 근거 | 성격 |
|---|------|------|------|
| **F1** | Lite 템플릿 상세의 "이 템플릿 사용"(`use-base-template`) 버튼으로 만든 스택이 **`empty-template-v1`** 로 생성됨 — 도구 0개 배포 | 스택 헤더 실측 (S5 1차) | UX 오해 유발 또는 템플릿 전달 버그 — **확인 필요** |
| **F2** | Add Tools 의 **"Confirm & Deploy"가 배포를 트리거하지 않음** — PATCH(설정 저장)만 하고 **"Tool addition deployment has started" 토스트를 띄움**. completed 스택은 `POST /deploy` 도 상태전이 거부(`completed→validating`) | `stack-add-tools-page.tsx:331 handleAddTools`(deploy 호출 없음), API 실측 400 | **버그 후보 (High)** — "실행되지 않은 일을 했다고 말함" (000070 교훈과 동일 패턴) |
| **F3** | API 로 스택 생성 시 **`golden_path_id`는 도구 목록을 펼치지 않음** — 설치 대상은 `config.artifacts/pipeline` 섹션의 `ToolSelection(enabled+name)`이 결정. 웹 마법사가 클라이언트에서 이를 조립함 | `helm/manifest-builders.go:118-145`, 빈 config 생성 실측(도구 미설치 completed) | 문서화 필요 — Deploy Preview 결정 카드(#34)의 "클라 조립" 구조와 정합 |
| F4 | `POST /stacks/:id/config` 는 **전체 교체형** — 부분 갱신인 줄 알고 쓰면 기존 섹션 유실 | 실측(본 테스트 중 config 손상 경험) | API 주의사항 문서화 |
| F5 | kind 격동 중 docker 데몬 재시작 시 **compose 인프라가 조용히 내려감** + kind 는 `fs.inotify.max_user_instances=128` 기본값으로 생성 실패(1024 상향 필요, VM 재시작 시 원복) | 실측 | 테스트 스위트 preflight 항목 |
| **F6** | **레지스트리 없는 스택에서 CI 번들 생성 불가** — Lite 구성(container_registry disabled)으로 파이프라인 실행 시 "이미지 레지스트리를 결정할 수 없습니다 (tool=\"\"): 외부 저장소 접두사를 지정하세요" 반복 경고, Gitea repo·Jenkins job 프로비저닝도 미진행 | API 로그 실측 (pip_48e52f5af935) | Lite 템플릿 ↔ CI 경로 궁합 — 문서화 또는 가드 필요 |
| **F7** | **직접 배포 빌드는 API 호스트에서 `git clone https://gitea.<stack>.internal/...`** — 게이트웨이 도메인 해석(hosts 엔트리 + 게이트웨이 포트포워드, S6 복사 버튼의 그 절차)과 샘플 리포(`root/spring-sample`) 존재가 전제. 로컬 미구성 시 "Could not resolve host" 로 빌드 실패 | API 빌드 로그 실측 | 로컬 전제 문서화 (수행 가이드 §2 연계) — **nullus#158 등록**(nullus-plan#30 파생, 舊 plan#52 이관) |
| **F8** | Deploy Configuration 의 Review Manifest 로드 실패 메시지가 **원인 불문 "경로와 파일명을 확인하세요"** — manifest fetch 는 브라우저에서 직접 실행되는데, `catch` 가 오류 원인을 버려 DNS 미해석(F7 계열)·CORS·404 를 전부 경로 문제로 안내 → 오진 유도 | `developer-deploy-page.tsx:524-527`, 실측 스크린샷 (dep_fdb72face327 세션) | **버그 후보** — F2와 같은 '오해 유발 메시지' 계열 — **nullus#158 등록** |
| **F9** | **Deploy YAML 리로드는 레지스트리 없는 스택(Lite)에서 구조적으로 동작 불가** — `package_registry` 부재 시 base 가 clone URL 로 폴백되어 최종 URL 이 `…/spring-sample.git/root/spring-sample/deploy/…`(리포 경로 중복·Gitea raw 형식 아님)로 **항상 404**. 부가로 endpoint `https://` 하드코딩 vs envoy svc 80/TCP 만 노출(스킴 불일치), Gitea CORS 헤더 부재(추정). 환경 전제(F7 계열: hosts·게이트웨이 PF·샘플 리포)를 다 갖춰도 실패하는 제품 결함 | `developer-deploy-page.tsx:496-514`·`stack_handler.go:275`·`kubectl get svc`·Gitea API 실측 — 상세: [DeployYAML 로드실패 원인분석](Nullus_DeployYAML_로드실패_원인분석.md) | **버그 후보 (High)** — F8(메시지)과 별개의 기능 결함, 이슈 등록 시 nullus#158 연계 |

## 최종 상태 (2026-08-17 완료 시점)

- **`lite-e2e-v2`(stk_8b349a88bb28)**: completed — **도구 3종(gitea·jenkins·argocd) 전부 Running**, 릴리스 9개 deployed, workloads 9/9 pods ready
- **`pip_48e52f5af935`(e2e-sample-app)**: active — 배포 1건 `failed` (F6·F7 요인, 이력·로그 화면 정상 동작)
- **마지막 마일 완주 (08-17 재개 세션)**: `pip_53735947bf4e` → `dep_14d781f7f8c1` **success** — 파드 `e2e-direct` 1/1 Running(develop 클러스터), 앱 HTTP 응답 실확인. **sudo·hosts·레지스트리 전부 불필요했던 우회 경로**: ① Gitea PF(`localhost:3100`)를 clone URL 로 직접 사용 ② `gitea_admin/spring-sample` 시드(nginx 8080 리슨 — backend 템플릿 포트) ③ 레지스트리는 애초에 불필요 — **`IMAGE_REGISTRY_URL` 이 비면 빌더가 `kind load docker-image` 로 적재**(`docker/builder.go`, `kind-` 접두사도 서버가 제거). F6 경고는 CI 기록용 WARN 일 뿐 직접 배포 비차단으로 판명
- 환경: API 8091(+`NULLUS_GITEA_URL=:3100`·`NULLUS_JENKINS_URL=:8480` override) · web 5174(proxy 임시, **커밋 금지**) · 인프라 4종 · kind 듀얼 · PF 2개(gitea 3100·jenkins 8480)

## 이후 활용

- 이 표의 통과 시나리오가 곧 **기능 회귀 테스트 스위트**(nullus-plan#53 "테스트 관리 1차" EPIC)의 자동화 대상 목록이 된다 — S0·R1 은 이미 스크립트화됨(responsive-audit), S2 는 API 호출로 자동화 가능, S3~S8 이 Playwright 시나리오 후보.
