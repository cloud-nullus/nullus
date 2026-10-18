---
hide:
  - navigation
---

# Nullus 문서

검증된 DevSecOps 스택을 **노코드로 Kubernetes에 설치하는** 오픈소스 플랫폼입니다.
템플릿을 고르고 설치 위자드를 채우면 GitLab·Argo CD·Prometheus·Grafana 같은 도구가
한 클러스터 안에서 서로 연동된 상태로 올라옵니다.

<div class="nullus-cards" markdown>

[**시작하기**<span>준비물부터 로컬에서 띄우는 법까지. 처음 오셨다면 여기부터.</span>](getting-started/index.md)

[**기능 한눈에 보기**<span>화면 19개가 각각 무엇을 하고 누가 쓸 수 있는지 한 표에 모았습니다.</span>](features/index.md)

[**역할별 사용법**<span>Admin·DevOps·Developer 가 각자 무엇을 하는지.</span>](roles/index.md)

[**처음부터 끝까지 (E2E)**<span>빈 노트북에서 스택 설치·파이프라인·백업·복구까지 한 번에.</span>](getting-started/e2e.md)

</div>

![스택 템플릿 화면 — 검증된 도구 조합을 카드로 고른다](assets/screens/stack-templates-light.png#only-light)
![스택 템플릿 화면 — 검증된 도구 조합을 카드로 고른다](assets/screens/stack-templates-dark.png#only-dark)

## 무엇을 해결하나

여러 프로젝트가 동시에 돌아가면 프로젝트마다 Kubernetes 클러스터에 파이프라인을
반복 설치해야 하고, 팀마다 절차가 달라 표준화가 어려워집니다. GitLab·Argo CD·
Prometheus·Grafana 같은 조합은 버전 호환성과 의존 관계를 함께 따져야 해서, "어떤
조합이 실제로 안전하게 도는지" 판단에 시간이 많이 듭니다.

Nullus 는 그 조합을 **골든패스 템플릿**으로 미리 묶어 두고, 설치와 연동 절차를
**UI 워크플로우 하나**로 통일하고, 배포 진행률과 로그를 **한 화면**에서 보게 합니다.

## 핵심 흐름

```mermaid
graph LR
  A[클러스터 등록] --> B[스택 템플릿 선택]
  B --> C[설치 위자드]
  C --> D[배포 진행·로그]
  D --> E[CI/CD 파이프라인]
  E --> F[모니터링·알림]
```

| 단계 | 화면 | 문서 |
|------|------|------|
| ① 클러스터를 등록한다 | `/admin/clusters` | [클러스터 관리](features/admin/clusters.md) |
| ② 템플릿을 고른다 | `/stack/templates` | [스택 템플릿](features/stack/templates.md) |
| ③ 설치 위자드를 채운다 | `/stack/install` | [스택 설치](features/stack/install.md) |
| ④ 배포를 지켜본다 | `/stack/deploy/:id` | [스택 설치](features/stack/install.md) |
| ⑤ 파이프라인을 만든다 | `/cicd/create` | [CI/CD 목록](features/cicd/list.md) |
| ⑥ 상태를 본다 | `/observability/monitoring` | [모니터링 대시보드](features/observability/monitoring.md) |

## 출처

화면 경로·권한·버튼·API 는 저장소의 코드에서 뽑았습니다 — 화면 목록은
`web/src/app/routes.tsx` 와 `web/src/components/layout/nav-model.tsx`, 화면 이름은
`web/src/i18n/ko.json`, API 경로는 Go 라우터 등록부입니다. 어느 페이지에서나 우측
상단 연필 아이콘으로 그 문서의 원본을 열 수 있습니다.

틀린 곳을 찾으면 [이슈로 알려 주세요](https://github.com/cloud-nullus/nullus/issues).

## 구현 범위

스택 템플릿은 11개이고 세 조합을 지원합니다 — **GitLab**(All-in-One · Argo CD ·
Harbor · Nexus · SonarQube · OTel), **GitHub + Argo CD**(저장소·Actions·GHCR 을
외부 서비스로 쓰고 클러스터에는 Argo CD 와 모니터링만 설치), **Gitea + Jenkins +
Argo CD**(Lite 포함). 가장 많이 검증된 경로는 GitLab 이고, CI/CD 영역은 안정화를
계속 진행하고 있습니다.

아직 동작하지 않거나 화면에 노출되지 않은 기능은 각 페이지에 적어 두었습니다.
