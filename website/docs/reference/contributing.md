# 기여하기

전체 규칙은 저장소의
[CONTRIBUTING.md](https://github.com/cloud-nullus/nullus/blob/main/CONTRIBUTING.md)
와 [CLAUDE.md](https://github.com/cloud-nullus/nullus/blob/main/CLAUDE.md) 에
있습니다. 여기는 자주 걸리는 것만 모았습니다.

## 브랜치

```
feat/<module>/<설명>     기능 추가
fix/<module>/<설명>      버그 수정
chore/<module>/<설명>    그 외 (문서·테스트·리팩터링·CI·설정)
```

브랜치 타입은 **이 셋뿐**입니다. `refactor`·`test`·`docs`·`perf`·`ci` 는
**커밋 타입**으로만 씁니다.

`<module>` 은 `stack` · `cicd` · `admin` · `auth` · `o11y` · `product` · `ui` ·
`shared` · `infra` · `cli` 중 하나입니다.

!!! note "`csp` 모듈은 없습니다"
    클라우드·배포 인프라는 **`infra`** 를 씁니다.

## 커밋

```
<type>(<module>): <설명>

feat(stack): 스택 설치 워크플로우 구현
fix(cicd): 파이프라인 배포 롤백 시 PVC 보존
docs(infra): 로컬 E2E 가이드 교정
```

## PR

CI 의 **PR Convention Check** 가 제목과 본문을 강제합니다.

- 제목: `<type>(<module>): <설명>` — 설명 1~72자
- 본문 필수: `## Summary` · `## Changes` · `## Testing` 섹션과 내용이 있는
  `Scope:` 줄

!!! warning "`gh pr create --body` 는 템플릿을 적용하지 않습니다"
    본문을 직접 쓸 때도 위 네 요소를 반드시 넣어야 합니다.

!!! note "제목·본문만 고쳤다면 PR 을 닫고 다시 열어야 재검사됩니다"
    워크플로 트리거가 `opened`·`reopened` 뿐입니다.
    ```bash
    gh pr close <번호> && gh pr reopen <번호>
    ```

## CHANGELOG

**CHANGELOG Check** 가 변경에 대응하는 항목을 요구합니다. 면제되는 경로는
`docs/` · `e2e/` · `_test.go` · `.test.tsx` · `.spec.tsx` ·
`.github/ISSUE_TEMPLATE/` · `CHANGELOG.md` · `.gitignore` · `LICENSE` ·
`README.md` 입니다.

!!! note "`website/` 는 면제가 아닙니다"
    이 문서 사이트를 고칠 때는 CHANGELOG 항목이 필요합니다.

## 테스트

| 하려는 것 | 명령 |
|-----------|------|
| Go 단위 테스트 | `make test` (태그 없는 것만) |
| DB 통합 테스트 | `make test-integration` — testcontainers 가 Postgres 를 직접 띄우므로 **`make dev` 가 필요 없습니다** |
| kind E2E | `make test-golden-path` (60분) |
| Go 린트 | `make lint` |
| 프런트엔드 단위 테스트 | `cd web && npx vitest run` |
| 프런트엔드 린트 | `cd web && npm run lint` |

!!! warning "DB 가 없으면 E2E 테스트는 실패가 아니라 건너뜁니다"
    `go test ./e2e/` 의 DB 연동 테스트는 태그가 없어 평소 테스트에 섞여 돌고,
    `localhost:5433` 에 붙지 못하면 **`t.Skip`** 으로 빠집니다. 결과는 `ok` 라
    통과처럼 보입니다. `make dev` 로 인프라를 띄우거나, 인프라 없이 돌리려면
    testcontainers 를 쓰는 `make test-integration` 을 쓰세요.

## 커버리지

!!! note "게이트와 목표가 다릅니다"
    CI 가 막는 선은 **49%** 이고(`scripts/check-coverage.sh`, `./internal/...` 만
    계산), 70% 는 v1 GA 목표입니다.

## 아키텍처 규칙

- **Modular Monolith** — 모듈 간 통신은 공개 인터페이스로만. 다른 모듈의
  internal 패키지를 직접 import 하지 않습니다
- **Clean Architecture** — 의존성은 항상 안쪽(도메인)을 향합니다.
  `domain` 은 프레임워크를 import 하지 않습니다
- **DDD** — 모듈이 바운디드 컨텍스트입니다. 테이블도 모듈이 소유하고, 다른
  모듈의 테이블을 직접 조회하지 않습니다
- **TDD** — 새 기능은 테스트부터, 버그는 재현 테스트부터

## 이 사이트를 고치려면

```bash
pip install -r website/requirements.txt
./website/stage.sh                  # docs/ 의 단독 HTML 가이드를 가져옵니다
python3 website/check_i18n.py       # 한국어·영어판 짝을 봅니다
cd website && mkdocs serve          # http://127.0.0.1:8000 (영어판은 /en/)
```

**페이지는 한국어·영어 두 벌입니다.** 한국어 `X.md` 가 원본이고 영어판 `X.en.md` 가
같은 자리에 있습니다. 한국어를 고치면 같은 PR 에서 영어판도 고칩니다.

- 영어판의 화면 문구는 영어 UI 의 실제 문구(`web/src/i18n/en.json`)를 씁니다
- 페이지 링크는 한국어 파일명(`list.md`)으로 겁니다 — 영어판에서는 플러그인이 영어 페이지로
  바꿉니다. 제목 앵커는 영어 제목 기준입니다
- nav 에 제목을 더하면 `website/mkdocs.yml` 의 `nav_translations` 에 영어 제목도 더합니다
- 화면 캡처는 `web` 에서 `npm run shots:docs` 로 찍으면 `*.png`(한국어)와 `*.en.png`(영어)가
  함께 나옵니다. 본문의 그림 경로는 두 판 모두 `*.png` 그대로 둡니다

`main` 에 들어가면 GitHub Actions 가 자동으로 배포합니다. PR 에서는 빌드만
합니다 — `mkdocs build --strict` 라서 **nav 에 적은 문서가 없거나 내부 링크·앵커가
깨지면 CI 가 실패합니다.** 영어판이 빠졌거나 한쪽에만 절·그림을 더한 페이지도
`check_i18n.py` 가 막습니다.
