# API 레벨 권한 설계 (nullus-plan#43)

> EPIC: [cloud-nullus/nullus-plan#43](https://github.com/cloud-nullus/nullus-plan/issues/43) — API 레벨 권한 설계.
> 본 문서는 설계 카드다 — 산출물은 이 문서뿐이고, 구현은 적용 순서(§5)에 따라 별도 작업으로 분리한다.

작성일: 2026-08-17

---

## 요약 (비관계자용 — 이것만 읽어도 논의 가능)

**무엇이 문제인가.**
Nullus의 API 권한은 지금 **모듈(구역) 단위**로만 걸려 있다 — "admin 구역은 Admin만, stacks 구역은 Admin/DevOps, cicd 구역은 전원". 구역 안에 들어오면 그 안의 **모든 행위(조회·생성·삭제·민감 작업)가 똑같이 허용**된다. 문 하나만 잠그고 방 안의 서랍은 다 열어둔 구조다.

**실제로 어긋난 곳이 있다.**
화면(웹)은 역할에 따라 메뉴를 숨기지만, API는 그만큼 세밀하지 않다. 예를 들어 **알림 규칙 관리 화면은 Developer에게 숨겨져 있는데, API로는 Developer가 알림 규칙을 만들고 지울 수 있다.** 숨긴 것과 막은 것은 다르다 — 화면은 안내일 뿐이고 보호는 API가 해야 한다.

**무엇을 설계했나.**
- 권한의 축을 "구역"에서 **"리소스 × 행위 × 역할"** 로 세분한 정책표(§4)를 만들었다.
- 새 권한 시스템을 도입하지 않는다 — **이미 있는 역할 검사 장치(`RequireRole`)를 구역이 아니라 라우트(행위) 단위로 거는 것**만으로 정책표 대부분이 구현된다.
- 적용은 위험(권한 과다) 해소부터 4단계로 나눴다(§5). 1단계는 코드 몇 줄 수준이다.

**원칙 한 줄.** "화면이 숨긴 것은 API도 막는다" — 그리고 그 기준을 화면·API가 각자 들고 있지 말고 **정책표 하나**로 모은다.

---

## 상세 (근거와 코드 위치)

### 1. As-Is — 구역(그룹) 단위 권한

`cmd/api/main.go:416-419`, 검사 장치는 `RequireRole`(`internal/auth/adapter/middleware/auth_middleware.go:44`) — 컨텍스트의 사용자 역할이 허용 목록에 있는지 보는 단순 멤버십 검사다(역할 계층 없음, admin도 명시돼야 통과).

| API 그룹 | 현재 가드 | 그룹 안에 함께 있는 것 |
|---|---|---|
| `/admin/*` | `RequireRole("admin")` | 조직·클러스터·멤버 관리 + **토큰소스 reveal/rotate/approve** + 감사로그 조회 |
| `/stacks/*` | `RequireRole("admin","devops")` | 스택 CRUD·배포 + **템플릿·호환성 매트릭스·리소스 기본값 CRUD**(플랫폼 정책) |
| `/cicd/*` | `RequireRole("admin","devops","developer")` | 조회·앱 배포 + **템플릿·golden-path·파이프라인 CRUD/삭제** |
| `/observability/*` | **없음 (인증만)** | 대시보드 조회 + **알림 규칙 생성/수정/삭제** |

전제(범위 밖): development 모드는 인증 미들웨어 자체가 꺼져 있고, session 모드는 클라이언트 헤더를 신뢰하는 알파 구조라 운영 기본은 이미 OIDC로 전환됐다(CHANGELOG). 본 설계는 **인증 이후의 인가(authorization)** 만 다룬다 — 인증(로그인·SSO, #38)과는 층이 다르다.

### 2. 현재 권한과 기능 불일치 목록 (세부 태스크 ①)

프론트 라우트 가드(`web/src/components/layout/nav-model.tsx`의 roles)·README 역할 체계와 API 실제 허용을 대조했다:

| # | 불일치 | 화면 기대 | API 실제 | 위험 |
|---|---|---|---|---|
| 1 | **알림 규칙 CRUD** | admin·devops만 (nav `alertRules`) | **developer도 가능** — observability 그룹 무가드 | 권한 과다 (High) |
| 2 | **CI/CD 템플릿·golden-path CRUD** | admin·devops만 (nav `cicdTemplate`·`cicdGoldenPath`) | **developer도 생성·수정·삭제 가능** | 권한 과다 (High) |
| 3 | **파이프라인 생성·삭제** | README: Developer는 "파이프라인 배포"만 | developer가 `POST/DELETE /pipelines` 가능 | 권한 과다 (Med) |
| 4 | **토큰소스 민감 액션** | 별도 구분 없음 | `reveal`(시크릿 평문 노출)·`rotate`·`approve` 가 조회성 admin API와 **같은 등급** | 미분화 (Med) — 이슈 본문의 "토큰·승인·조회 세분화" 대상 |
| 5 | **플랫폼 정책성 리소스** | 버전 관리 화면은 admin 전용(`/admin/stack-versions`) | 호환성 매트릭스·리소스 기본값 CRUD는 devops도 가능(stacks 그룹) | 검토 필요 (Low) — 의도일 수 있음 |

공통 패턴: **화면은 숨기는데 API는 열려 있다.** 숨김(UX)과 보호(인가)가 다른 곳에서 각자 관리되기 때문이다.

**다섯 건의 성격은 셋으로 갈린다** — 조치의 종류가 다르므로 구분해 둔다:

| 성격 | 해당 | 의미 | 조치 종류 |
|---|---|---|---|
| **API 인가 갭** (진짜 불일치) | #1·#2·#3 | 화면이 숨겨도 토큰만 있으면 **API 직접 호출로 우회 가능** — 잘못된 역할이 통과한다 | 라우트 가드(`RequireRole`) 추가 |
| **API 설계 미분화** | #4 | 접근 자체는 admin 으로 이미 막혀 있어 뚫린 곳은 없다. 같은 admin 안에서 민감 행위(reveal 등)가 조회와 동급 취급되는 것이 문제 | 가드가 아니라 **감사 기록 강제** |
| **API 정책 재검토** | #5 | 화면·API 가 현재 **일치**한다(devops 허용). "플랫폼 정책을 devops 가 고쳐도 되는가"라는 정책 질문 | 팀 결정 후 상향 or 유지 |

프론트엔드는 다섯 건 모두에서 **고칠 것이 없다** — 이미 올바른 정책을 기대하고 있고, 작업 위치는 전부 서버(라우트 등록부·핸들러)다.

### 3. 보호가 필요한 액션 분류 (세부 태스크 ②)

행위를 다섯 등급으로 나눈다 — 정책표(§4)의 열이 된다:

| 등급 | 예 | 성격 |
|---|---|---|
| 조회 (read) | GET 전반, 대시보드, 이력 | 넓게 허용 |
| 실행 (execute) | stack deploy/retry/rollback, app deploy, cluster verify | 역할별 핵심 업무 |
| 쓰기 (write) | 스택·파이프라인·알림 규칙 생성/수정 | 담당 역할로 제한 |
| 삭제 (delete) | 스택·파이프라인·템플릿 삭제 | 쓰기보다 좁게 |
| **민감 (sensitive)** | 토큰 reveal/rotate/approve, kubeconfig 등록, 멤버 역할 변경 | 최소 인원 + 감사 필수 |

### 4. RBAC 정책표 (세부 태스크 ③)

리소스 × 행위 × 역할. ✅ 허용 / ❌ 거부 / 굵게 = 현재와 달라지는 칸:

| 리소스 | 행위 | admin | devops | developer |
|---|---|:---:|:---:|:---:|
| 대시보드·이력·조회 전반 | read | ✅ | ✅ | ✅ |
| 알림 규칙 | write/delete | ✅ | ✅ | **❌** |
| CI/CD 앱 배포 (self-service) | execute | ✅ | ✅ | ✅ |
| CI/CD 파이프라인 | execute(deploy) | ✅ | ✅ | ✅ |
| CI/CD 파이프라인 | write/delete | ✅ | ✅ | **❌** |
| CI/CD 템플릿·golden-path | write/delete | ✅ | ✅ | **❌** |
| 스택 | read/execute/write/delete | ✅ | ✅ | ❌ (현행 유지) |
| 스택 템플릿·호환성·리소스 기본값 | write/delete | ✅ | ✅→**검토** | ❌ |
| 조직·클러스터·멤버 | 전 행위 | ✅ | ❌ | ❌ (현행 유지) |
| 토큰소스 | read(목록·이벤트) | ✅ | ❌ | ❌ |
| 토큰소스 | **sensitive**(reveal/rotate/approve/re-auth) | ✅ *+감사 필수* | ❌ | ❌ |

- developer의 정체성은 **"배포하고 관찰하는 사람"** — 조회·자기 앱 배포는 넓게, 공용 자원(템플릿·규칙·파이프라인 정의)의 변경은 닫는다.
- 토큰소스 sensitive는 역할을 새로 만들지 않고 **admin 유지 + 감사 로그 필수**로 시작한다(값이 아니라 행위·키만 기록 — 기존 감사 원칙 재사용). 승인 2단계(4-eyes)는 필요가 확인되면 후속.

### 5. 적용 순서 제안 (세부 태스크 ④)

새 프레임워크 없이 기존 `RequireRole`을 **라우트 단위로 내리는 것**이 전부다. 위험 큰 것부터:

| 단계 | 내용 | 변경 규모 |
|---|---|---|
| **1** | observability 무가드 해소 — 그룹에 전 롤 가드 + 알림 규칙 write/delete 라우트에 `RequireRole("admin","devops")` | 라우트 등록부 몇 줄 (불일치 #1) |
| **2** | cicd 쓰기 분리 — 템플릿·golden-path·파이프라인 CRUD에 admin/devops 가드, 조회·deploy-app은 전 롤 유지 | 라우트 등록부 (불일치 #2·#3) |
| **3** | 토큰소스 sensitive 감사 강제 — reveal/rotate/approve 에 감사 기록 필수화 검증 | 핸들러 점검 (불일치 #4) |
| **4** | (장기) **정책표 중앙화** — main.go 에 흩어진 가드 나열을 "라우트→정책" 테이블 하나로 모으고, **§4 표와 코드가 일치하는지 테스트로 고정** (도구 버전표가 화면·서버 각자 소유로 9/11 갈라졌던 교훈의 재적용) | 리팩터링 + 테스트 |

1·2단계만으로 발견된 권한 과다(High)가 전부 닫힌다. 프론트는 변경 불필요 — 이미 같은 정책을 기대하고 있다.

## 부록 — 불일치별 대상 라우트와 조치 (구현 참고용)

구현 시 이 표를 그대로 체크리스트로 쓴다. 모든 조치는 기존 `authmw.RequireRole(...)`을 해당 라우트(또는 서브그룹)에 거는 것이다. 등록 위치는 각 모듈 핸들러의 라우트 등록부와 `cmd/api/main.go`의 그룹 정의다.

### 불일치 #1 — observability (High)

| 라우트 | 현재 | 조치 |
|---|---|---|
| `POST /observability/alert-rules` | 인증만 | `RequireRole("admin","devops")` |
| `PATCH /observability/alert-rules/:id` | 인증만 | `RequireRole("admin","devops")` |
| `DELETE /observability/alert-rules/:id` | 인증만 | `RequireRole("admin","devops")` |
| `GET /observability/*` (dashboard·alert-rules·alert-history·deployed-apps) | 인증만 | `RequireRole("admin","devops","developer")` — 그룹 기본 가드로 |

### 불일치 #2 — cicd 템플릿·golden-path (High)

| 라우트 | 현재 | 조치 |
|---|---|---|
| `POST /cicd/templates` · `PUT /cicd/templates/:id` · `DELETE /cicd/templates/:id` | 전 롤 | `RequireRole("admin","devops")` |
| `POST /cicd/golden-paths` · `PUT /cicd/golden-paths/:id` · `DELETE /cicd/golden-paths/:id` | 전 롤 | `RequireRole("admin","devops")` |

### 불일치 #3 — cicd 파이프라인 (Med)

| 라우트 | 현재 | 조치 |
|---|---|---|
| `POST /cicd/pipelines` · `DELETE /cicd/pipelines/:id` | 전 롤 | `RequireRole("admin","devops")` |
| `POST /cicd/pipelines/:id/deploy` | 전 롤 | **유지** (README: developer 는 배포 가능) |
| `POST /cicd/deploy-app` | 전 롤 | **유지** (developer self-service 의 존재 이유) |
| `GET /cicd/*` (templates·pipelines·deployments·history 등) | 전 롤 | **유지** |

### 불일치 #4 — 토큰소스 민감 액션 (Med)

| 라우트 | 현재 | 조치 |
|---|---|---|
| `POST /admin/token-sources/:id/reveal` | admin | admin 유지 + **감사 기록 필수 검증** (값이 아니라 행위·키만 기록) |
| `POST /admin/token-sources/:id/rotate` · `/approve` · `/re-auth` · `/pause` · `/resume` | admin | 상동 |

### 불일치 #5 — stacks 플랫폼 정책성 리소스 (Low, 팀 검토 후)

| 라우트 | 현재 | 조치(제안) |
|---|---|---|
| `POST /stacks/resource-defaults` | admin·devops | admin 상향 검토 |
| `POST/PUT/DELETE /stacks/templates(/:id)` | admin·devops | 현행 유지 가능성 높음 (화면도 devops 허용) — 팀 확인 |
| `POST/PUT/DELETE /stacks/compatibility/matrices(/:id)` | admin·devops | admin 상향 검토 (버전 정책은 admin 화면 소관) |

### 구현 시 공통 주의

- `RequireRole` 은 **계층이 없다** — admin 도 목록에 명시해야 통과한다(`CanAccess` 는 단순 멤버십).
- 가드는 `authMW`·`userRateLimit` **뒤에** 건다 — 403 요청도 사용량에 잡히게 하는 기존 순서 유지(`main.go:414` 주석).
- 단계 1·2 반영 후 **정책표(§4)와 라우트 가드의 일치를 테스트로 고정**한다 — 표만 고치고 코드를 빠뜨리는(또는 반대) drift 방지.
- development 모드는 인증 미들웨어가 꺼져 있으므로 로컬 검증은 `--auth=keycloak` 기동 + OIDC 계정으로 403 을 실측한다.

## 참고 (코드 위치)

- 그룹 가드: `cmd/api/main.go:416-419` · 검사 장치: `internal/auth/adapter/middleware/auth_middleware.go:44`(RequireRole), `internal/admin/domain/user.go`(CanAccess — 단순 멤버십)
- 프론트 기대 정책: `web/src/components/layout/nav-model.tsx`(roles 배열), `README.md` §역할 체계
- 토큰소스 액션: `internal/admin/adapter/handler/token_source_handler.go` (reveal/rotate/approve/pause/re-auth)
- 감사 원칙(값 대신 키만 기록): CHANGELOG — 설정 변경 감사(F040) 항목
- 인증 층(범위 밖, 층 구분): 자동 SSO 설계(#38), OIDC Provider 가이드
