# Nullus CLI 컨셉 (설계 초안)

- **EPIC**: [cloud-nullus/nullus-plan#42](https://github.com/cloud-nullus/nullus-plan/issues/42) — CLI 컨셉 문서화
- **상태**: 컨셉 확정용 초안 · **구현 보류**(이 문서는 범위·명령 체계만 고정한다)
- **작성일**: 2026-08-15

> 이 문서의 목적은 `nullus` CLI가 **맡을 역할과 대표 명령 체계를 고정**하는 것이다. 실제 구현은 하지 않는다(보류 사유는 §6). 명령 후보는 확정 스펙이 아니라 방향성 초안이다.

---

## 1. 배경 — As-Is (현재 CLI 상태, 코드 확인)

현재 **통합 `nullus` CLI는 없다.** 목적특화 도구 3종만 존재한다:

| 도구 | 형태 | 명령 | 빌드/배포 | 용도 |
|------|------|------|-----------|------|
| `cmd/nullus-bootstrap` | Go | `issue` / `revoke` | Makefile `build` → `bin/` · 런타임 이미지 미포함 | air-gap 무인설치용 Keycloak 토큰 발급/폐기 (`airgap/scripts/29-*`가 사용) |
| `cmd/token-source-sync` | Go | env 구동(서브커맨드 없음) | Makefile 빌드 대상 아님 | 토큰소스 동기화 배치 |
| `scripts/runbook_local.sh` | sh | `up/down/status/info/smoke/logs/all/refresh/preflight/kind-up/kind-down` (+`--auth`/`--seed`/`--kind`) | dev 전용 | 로컬 개발환경 라이프사이클 |

> 참고: `nullus-bootstrap issue`는 대상 realm(`nullus`)이 **이미 존재해야** 동작한다 — 그 realm은 `runbook_local.sh up --auth=keycloak`(내부 `setup-keycloak.sh`)가 만들고, `make dev-up`은 raw Keycloak만 띄운다(realm 없음). 실제 로컬 검증은 항상 존재하는 `master` realm으로 `issue`→JWT 발급, `revoke`→폐기까지 확인했다.

- 제품 기능(cluster/stack/cicd/observability)은 전부 `/api/v1/*` REST + 웹 UI 전용 → **CLI로는 미존재.**
- 전용 CLI 프레임워크 없음(`cobra`는 indirect 의존만). 도입 시 `spf13/cobra`가 자연스러운 선택.

**무엇이 그린필드인가 (혼동 방지)**: 위 3종은 **존재한다.** 이 중 2종은 로컬 실행으로 동작까지 확인했다 — `nullus-bootstrap`의 `issue`(→JWT 발급)·`revoke`, 그리고 `runbook_local.sh`(서브커맨드 실행). `token-source-sync`만 OpenBao 전제라 정적 확인에 그친다. **부재한 것은 오직 (1) 통합 `nullus` 진입점(단일 바이너리)과 (2) 제품 기능 명령**(`nullus cluster/stack/pipeline/obs ...`)이다 — 이 층위만 그린필드이고, 제품 기능은 `/api/v1/*` REST + 웹 UI로만 존재한다. 즉 "조각 3종은 있으나, 그것들을 묶고 제품 기능까지 얹은 `nullus` 우산은 없다." 재활용 가능한 조각은 (a) 인증(`nullus-bootstrap`), (b) 로컬 개발(`runbook_local.sh`) 둘이다.

---

## 2. 목표 사용자 (① 목표 사용자 정의)

CLI는 웹 UI(노코드)를 **대체하지 않는다.** 웹이 못 채우는 자리를 메운다.

| 사용자 | 웹 UI로 부족한 점 | CLI가 주는 것 |
|--------|-------------------|---------------|
| **DevOps/플랫폼 엔지니어** | 클릭 반복, 스크립트화 불가 | 스택/파이프라인 배포를 `nullus stack deploy -f config.yaml`로 코드화 |
| **CI/CD 파이프라인(무인)** | 브라우저 로그인 필요 | 서비스 계정 토큰(`bootstrap`)으로 헤드리스 실행 |
| **Air-gap 설치자** | UI 접근 전에 부트스트랩 필요 | 이미 `nullus-bootstrap`이 담당 → CLI로 흡수 |
| **파워유저/디버깅** | 화면당 정보 파편화 | `nullus stack status`, `logs -f` 로 빠른 조회 |

**핵심 원칙**: 웹은 "탐색·설정 마법사(노코드)", CLI는 "반복·자동화·헤드리스". 두 표면은 **같은 API를 공유**한다(§5).

---

## 3. 명령 체계 (② 명령 후보 정리)

`noun verb` 구조. API 그룹(`admin`/`stacks`/`cicd`/`observability`)과 RBAC(admin/devops/developer)을 그대로 반영한다.

### 3.1 1급 — automation 핵심 (CLI가 존재할 이유)

```
nullus login                         # OIDC 로그인 → 토큰 캐시
nullus auth bootstrap issue|revoke   # 무인/air-gap용 토큰 (기존 nullus-bootstrap 흡수)

nullus cluster register -f kube.yaml # 클러스터 등록 (kubeconfig)
nullus cluster verify <id>           # 연결 검증
nullus cluster ls | get <id> | rm <id>

nullus stack deploy -f stack.yaml    # 스택 생성+배포 (비대화식)
nullus stack status <id>             # 배포 상태
nullus stack logs <id> -f            # 실시간 로그 (WS 래핑)
nullus stack rollback <id> | retry <id> | rm <id>
nullus stack ls

nullus pipeline deploy <id>          # CI/CD 파이프라인 배포
nullus app deploy -f app.yaml        # Developer self-service 앱 배포
```

### 3.2 2급 — 선택 (있으면 좋은 것)

```
nullus stack template ls | get <id>        # Golden Path 카탈로그
nullus stack config get|set|preview <id>   # 릴리스 values 편집
nullus compat ls                           # 호환성 매트릭스
nullus token-source ls | rotate | reveal   # 토큰소스 운영
nullus alert ls | create | rm              # 알림 규칙
nullus dev up|down|smoke|kind-up           # 로컬 개발 (runbook_local.sh 흡수)
```

### 3.3 CLI 제외 — 웹 UI 유지

| 기능 | 이유 |
|------|------|
| 5단계 설치 마법사 / `draft` / `estimate` | 대화형 탐색이 본질 — CLI는 완성된 `-f config.yaml`을 받는다 |
| 조직/멤버/초대 관리 | 관리 화면 UX가 적합, 자동화 수요 낮음 |
| 감사 로그 / known-issues 열람 | 조회 위주, 웹 대시보드로 충분 |

---

## 4. 대표 워크플로 (예시)

```bash
# 무인 CI 파이프라인에서 스택 배포
export NULLUS_TOKEN=$(nullus auth bootstrap issue)     # air-gap/CI용 토큰
nullus cluster verify prod-01
nullus stack deploy -f gitlab-argocd.yaml --cluster prod-01
nullus stack status $STACK_ID --wait                   # 완료까지 대기 (exit code로 성패)
```

---

## 5. API 재사용 방식 (③ API 재사용 방식 정리)

- **CLI = `/api/v1/*` REST의 얇은 클라이언트.** 별도 백엔드/로직 없음 — 웹이 쓰는 바로 그 엔드포인트(약 118개, 4모듈)를 호출한다.
- **인증 2경로**:
  - 사람: `nullus login`(OIDC) → 토큰을 `~/.nullus/`에 캐시.
  - 무인: `nullus auth bootstrap issue`(기존 `nullus-bootstrap`) → 서비스 계정 토큰. air-gap·CI용.
  - dev 모드(`auth.mode=session`)에서는 인증 미들웨어가 꺼져 있어 로컬은 토큰 없이도 동작.
- **RBAC이 명령 가시성을 규정**: admin/devops/developer 롤이 API 그룹 접근을 이미 나누므로, CLI는 이를 그대로 노출한다(권한 없는 명령은 403).
- **실시간 로그**: `/ws/deployments/:id/logs`(WebSocket)를 `nullus stack logs -f`로 래핑.
- **출력**: 기본 사람이 읽는 표, `-o json`으로 스크립트 친화 출력(자동화 전제).

---

## 6. 구현을 보류하는 이유 (④ 구현 보류 사유 문서화)

1. **웹 UI가 1차 UX**다. 제품의 핵심 가치가 "노코드 설치 마법사"라 CLI는 후순위.
2. **두 표면 동시 유지비**. API가 아직 진화 중이다 — 특히 **CI/CD 파이프라인 기능**(제품 기능분해도의 `F5`=파이프라인 템플릿·Manifest Generator, `F6`=파이프라인 배포/이력·Manifest Applier)이 README "기능 구현 현황"상 여전히 **"안정화 진행 중"** 상태다(나머지 F0~F12는 "구현됨"). 이 상태에서 CLI를 붙이면 API가 바뀔 때마다 웹·CLI 양쪽을 함께 고쳐야 한다 → **API 안정화가 선행 조건**.
3. **자동화 수요는 이미 부분 충족**. air-gap/CI의 급한 곳은 `nullus-bootstrap` + `airgap/scripts/*`가 메우고 있어, 통합 CLI의 긴급도가 낮다.
4. **컨셉 고정이 먼저**. 명령 체계를 정하지 않고 구현하면 표면이 굳어 되돌리기 어렵다 — 이 문서로 방향만 고정한다.

### 승격(구현 착수) 트리거 — 2차 판단 기준
- API v1이 안정화되어 변경 빈도가 낮아질 때 — 구체적으로 위 CI/CD 파이프라인 기능(`F5`·`F6`)이 README 기능 현황에서 "안정화 진행 중"→"구현됨"으로 전환되는 시점.
- 외부 사용자/파트너가 헤드리스 자동화를 요구할 때.
- air-gap 무인설치 시나리오가 `nullus-bootstrap` 범위를 넘어 스택/파이프라인 전체로 확장될 때.

---

## 7. OSS CLI 통합 래퍼 아이디어 (메모)

`nullus`를 kubectl/helm/argocd/gitlab CLI의 **우산**으로 두는 구상(스케치 수준):

```
nullus k get pods            # 등록된 클러스터 컨텍스트로 kubectl 위임
nullus argo app list         # 스택이 설치한 Argo CD로 위임(SSO 토큰 재사용)
```

- 가치: 사용자가 스택별 OSS 콘솔/CLI 자격증명을 따로 챙기지 않아도 됨 — 플랫폼이 이미 OSS 앱 단일 SSO·토큰을 관리한다(OSS SSO 자동로그인, `cloud-nullus/nullus` PR #98).
- 리스크: 각 OSS CLI 버전·인증 방식을 따라가야 해 유지비가 큼 → **아이디어로만 기록, 범위 밖.**

**TUI 표면 (같은 취급의 아이디어, 2026-08-23 추가)**: k9s/lazygit 류의 터미널 전체 화면 표면. 배포 여러 개를 한 화면에서 지켜보는 용도다. 자동화(CLI)도 탐색(웹)도 대체하지 못하는 제3의 표면이라 v1 범위 밖 — 터미널 상주 운영자가 실사용자로 확인되면 `nullus tui` 서브커맨드로 검토한다. API·인증은 `pkg/nullusclient` 재사용이므로 추가 비용은 화면 계층뿐이다.

---

## 8. 열린 질문 (→ 순차 확정 중, 2026-08-23)

- 배포 형태: 단일 정적 바이너리(cobra) vs 이미지 동봉? — [CLI+MCP 구현 백로그](../plans/2026-08-22-cli-mcp-구현-백로그.md) 0-4·R-1에서 확정 예정
- 기존 조각 흡수 범위: **결정** — `runbook_local.sh`는 편입하지 않고 스크립트로 존치 ([ADR-0001](../adr/0001-cli-구현을-위한-논의.md))
- 설정/토큰 저장 위치와 형식: **결정** — `~/.nullus/config` + 파일 권한 0600, `NULLUS_*` env 우선 ([Automation 계약](./Nullus_CLI_Automation_계약.md) §5, 구현은 백로그 S-2)
- `-o json`/`--wait`/exit code 등 automation 계약: **결정** — [Nullus CLI Automation 계약](./Nullus_CLI_Automation_계약.md)으로 확정

---

## 참고
- EPIC: [nullus-plan#42](https://github.com/cloud-nullus/nullus-plan/issues/42) (본 문서의 As-Is·명령 골격은 #42 코멘트에서 제안됨)
- API 표면: `cmd/api/main.go`(라우트 그룹), 각 `internal/*/adapter/handler`
- 기존 도구: `cmd/nullus-bootstrap`, `cmd/token-source-sync`, `scripts/runbook_local.sh`
- 인증/RBAC: `internal/auth/`, [Nullus OIDC Provider 가이드](../20_개발가이드/Nullus_OIDC_Provider_가이드.md)
