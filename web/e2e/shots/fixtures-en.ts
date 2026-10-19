// 영어 문서용 캡처에서 fixtures.ts 의 한국어 데이터를 바꿔 넣는 표.
//
// fixtures.ts 는 한국어 문서 그림 기준으로 적혀 있다(템플릿 설명 · 채널 이름 ·
// 경보 요약). 영어 화면에 그대로 내보내면 영어 UI 안에 한국어 데이터가 섞여
// 찍힌다. 데이터를 두 벌 두지 않고 문자열 단위로 바꾼다 — fixtures.ts 는
// 그대로 한 곳에서 고친다.
//
// fixtures.ts 에 한국어 문자열을 더하면 여기에도 더한다. 빠뜨리면
// localizeFixture 가 그 문자열을 들고 실패한다(영어 그림에 한국어가 남지 않게).

const EN: Record<string, string> = {
  // 스택 템플릿 (STACK_TEMPLATES)
  'GitLab CI 로 빌드하고 Argo CD 로 배포한다. SonarQube 정적 분석과 Trivy 이미지 스캔이 파이프라인에 붙는다.':
    'Builds with GitLab CI and deploys with Argo CD. SonarQube static analysis and Trivy image scanning run in the pipeline.',
  '보안 게이트가 필요한 조직': 'Teams that need security gates',
  'GitLab CI 와 Harbor 레지스트리를 분리해 GitOps 패턴을 강화한 구성.':
    'Separates GitLab CI from the Harbor registry for a stricter GitOps setup.',
  'GitOps 중심 조직': 'GitOps-first teams',
  '8Gi 노드 하나에 들어가는 경량 구성. 로컬 검증과 소규모 팀에 쓴다.':
    'Lightweight setup that fits on a single 8Gi node. For local testing and small teams.',
  '로컬 검증 · 소규모 팀': 'Local testing · small teams',
  'GitHub · GitHub Actions · GHCR 을 외부 서비스로 쓰고 클러스터에는 Argo CD 와 모니터링만 설치한다.':
    'Uses GitHub, GitHub Actions and GHCR as external services; installs only Argo CD and monitoring in the cluster.',
  'GitHub 사용 조직': 'Teams on GitHub',

  // CI/CD 템플릿 (CICD_TEMPLATES)
  'React 프론트엔드': 'React frontend',
  'Vite 빌드 → 이미지 → Argo CD 배포. 시크릿 탐지와 이미지 스캔이 붙는다.':
    'Vite build → image → Argo CD deploy. Includes secret detection and image scanning.',
  '빌드 설정을 직접 채우는 빈 템플릿.': 'Blank template where you fill in the build settings yourself.',
  '배치 작업': 'Batch job',
  'CronJob 으로 배포되는 배치 파이프라인.': 'Batch pipeline deployed as a CronJob.',

  // 배포 (DEPLOYMENTS)
  'image-scan: CRITICAL 1건 (CVE-2021-44906)': 'image-scan: 1 CRITICAL (CVE-2021-44906)',

  // 골든패스 (GOLDEN_PATHS)
  'GitLab CE 기반 단일 플랫폼. 소스코드 관리, CI/CD, 컨테이너 레지스트리를 GitLab에서 통합 제공합니다.':
    'Single platform on GitLab CE. Source control, CI/CD and the container registry all come from GitLab.',
  '중견기업, 단일 플랫폼 선호': 'Mid-size companies that prefer a single platform',
  'GitLab CI와 Harbor 레지스트리를 분리하여 GitOps 패턴을 강화한 구성입니다.':
    'Separates GitLab CI from the Harbor registry for a stricter GitOps setup.',
  'GitHub·GitHub Actions·GHCR 을 외부 서비스로 사용하고, 클러스터 내에는 Argo CD + 모니터링만 설치합니다.':
    'Uses GitHub, GitHub Actions and GHCR as external services; installs only Argo CD + monitoring in the cluster.',

  // 조직 · 멤버 (MEMBERS · ORGANIZATION)
  이소연: 'Soyeon Lee',
  'Nullus 팀': 'Nullus Team',

  // 스택 경보 (STACK_ALERTS)
  'Pod nullus-app/order-api-6f8d9 가 5분 동안 재시작을 반복하고 있다':
    'Pod nullus-app/order-api-6f8d9 has been restarting repeatedly for 5 minutes',
  'PVC gitlab-postgresql 의 남은 용량이 12% 다': 'PVC gitlab-postgresql has 12% free space left',
  'jenkins 수집 대상이 응답하지 않는다': 'The jenkins scrape target is not responding',

  // 알림 채널 · 이벤트 (NOTIFICATION_CHANNELS · NOTIFICATION_EVENTS)
  '플랫폼 운영 Slack': 'Platform ops Slack',
  '당직 Telegram': 'On-call Telegram',
  '백업 실패': 'Backup failed',
  '백업 부분 성공': 'Backup partially succeeded',
  '백업 성공': 'Backup succeeded',
  '경보 발생': 'Alert fired',
  '경보 해제': 'Alert resolved',

  // 앱 템플릿 (APP_TEMPLATES)
  '샘플 프론트엔드': 'Sample frontend',
  'Vite + React 샘플 앱': 'Vite + React sample app',
  '샘플 백엔드': 'Sample backend',
  'Go + Echo 샘플 API': 'Go + Echo sample API',

  // 알려진 이슈 (KNOWN_ISSUES)
  '스택 이력의 롤백이 400 을 반환한다': 'Rollback from stack history returns 400',
  '호환성 매트릭스 생성·수정·삭제가 404 다': 'Creating, editing or deleting a compatibility matrix returns 404',

  // 스택 이력 (STACK_HISTORY)
  'SonarQube 추가': 'Added SonarQube',
  'Harbor 리소스 상향 (1Gi → 2Gi)': 'Raised Harbor resources (1Gi → 2Gi)',
  '초기 배포 완료': 'Initial deployment complete',
  '템플릿에서 스택 생성': 'Created stack from template',

  // 토큰 소스 (TOKEN_SOURCES)
  'GitLab 프로젝트 토큰': 'GitLab project token',
  'Harbor 로봇 계정': 'Harbor robot account',
  'Argo CD 저장소 자격증명': 'Argo CD repository credential',
}

const HANGUL = /[가-힣]/

/** 캡처 스펙의 `wait` 같은 개별 문자열. 표에 없으면 그대로 둔다(영문 값). */
export function localizeText(text: string): string {
  return EN[text] ?? text
}

/**
 * 고정 응답을 영어로 바꾼다. 표에 없는 한국어 문자열을 만나면 던진다 —
 * 영어 문서 그림에 한국어 데이터가 조용히 남지 않게.
 */
export function localizeFixture<T>(value: T): T {
  if (typeof value === 'string') {
    const translated = EN[value]
    if (translated !== undefined) return translated as T
    if (HANGUL.test(value)) {
      throw new Error(`fixtures-en.ts 에 영어 문자열이 없습니다: ${JSON.stringify(value)}`)
    }
    return value
  }
  if (Array.isArray(value)) return value.map((item) => localizeFixture(item)) as T
  if (value !== null && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value).map(([key, item]) => [key, localizeFixture(item)]),
    ) as T
  }
  return value
}
