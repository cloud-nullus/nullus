import type { Role } from '../../types'

/**
 * 제품 둘러보기(투어)의 한 걸음.
 *
 * 대상은 CSS 선택자로 적는다. 화면 코드가 클래스 이름을 바꿔도 투어가 조용히
 * 깨지지 않도록 `data-tour` 속성을 붙여 두고 그것을 가리킨다 — 클래스는 스타일의
 * 것이고 이 속성은 계약이다.
 */
export interface TourStep {
  id: string
  /** 이 걸음을 보여 줄 화면. 다르면 투어가 먼저 그리로 옮긴다. */
  route: string
  /** 강조할 요소. 못 찾으면 화면 가운데에 설명만 띄운다. */
  target?: string
  /**
   * 이 걸음에 들어올 때 눌러 둘 요소. 여럿이면 적은 순서대로 누른다.
   *
   * 팝업 안이나 다른 탭에 있는 것을 강조하려면 먼저 그것을 열어야 한다. 두 번
   * 걸쳐 열리는 자리도 있다 — 스택 목록은 행을 골라야 상세가 열리고, 그 안에서야
   * 탭을 누를 수 있다. 사용자가 강조된 버튼을 직접 눌러 왔다면 이미 열려 있으므로,
   * 대상이 이미 보일 때는 누르지 않는다 — 두 번 눌러 팝업이 닫히면 안 된다.
   */
  activate?: string | string[]
  /**
   * 강조를 함께 감쌀 두 번째 요소.
   *
   * 탭처럼 "누르는 곳" 과 "보이는 곳" 이 갈린 걸음에 쓴다 — 탭 버튼만 강조하면
   * 그 탭에서 무엇을 고르는지가 화면에서 잘려 나간다.
   */
  union?: string
  /** 이 역할에게만 보여 준다. 비우면 전원. */
  roles?: Role[]
  /** 설명 상자를 대상의 어느 쪽에 붙일지. 자리가 없으면 반대쪽으로 뒤집는다. */
  placement?: 'top' | 'bottom' | 'left' | 'right'
}

/**
 * 투어가 훑는 순서. 플랫폼을 처음 쓰는 사람이 실제로 밟는 길과 같다.
 *
 * 걸음을 잘게 나눈 이유는 이 제품의 어려움이 "어느 메뉴에 있는가" 가 아니라
 * "무엇을 어떤 순서로 정해야 하는가" 에 있기 때문이다. 마법사 탭 하나하나,
 * 배포 뒤 연결 정보 버튼 하나하나가 각자 결정을 요구한다.
 *
 * 관리자·devops 전용 화면은 roles 로 걸러 낸다. ProtectedRoute 가 되돌려 보내는
 * 화면으로 투어가 안내하면 그 걸음은 빈 화면을 강조하게 된다.
 */
const LITE_TEMPLATE = '[data-tour-template="gitea-jenkins-argocd-lite-v1"]'
/**
 * 열려 있는 앱 팝업.
 *
 * role="dialog" 를 쓰지 않는다 — 투어 설명 카드도 그 역할을 갖고 있어, 팝업을
 * 가리키는 걸음이 자기 자신을 강조하고 화면이 요동쳤다. Modal 이 붙이는
 * data-modal 은 앱의 팝업만 가리킨다.
 */
const DIALOG = '[data-modal]'
const INSTALLER: Role[] = ['admin', 'devops']

/** 마법사 탭 한 칸. 라벨은 번역되므로 id 로 집는다. */
const tab = (id: string) => `[data-tab="${id}"]`
/** 설치 마법사에서 탭과 함께 감쌀 본문. */
const INSTALL_PANEL = '[data-tour="install-panel"]'
/** 스택 상세에서 탭과 함께 감쌀 본문. */
const STACK_PANEL = '[data-tour="stack-detail-panel"]'
/**
 * 스택 상세를 여는 첫 걸음.
 *
 * 목록에서 행을 고르기 전에는 상세 탭이 화면에 없다. 그 상태로 탭을 가리키면
 * 강조할 것이 없어 설명만 덩그러니 뜬다.
 */
const STACK_ROW = '[data-tour="stack-list"] tbody tr'
/**
 * 개발자 배포 화면에서 그 단계가 실제로 채우는 영역.
 *
 * 강조는 이쪽이고 누르는 곳은 단계 표시줄이다 — 표시줄 칩만 강조하면 무엇을
 * 입력하는 단계인지 보이지 않고, 둘을 합치면 화면을 통째로 덮는다.
 */
const cicdSection = (id: string) => `#pipeline-step-${id}`
/** 개발자 배포 화면의 단계 표시줄 한 칸. */
const cicdStep = (n: number) => `[data-tour-cicd-step="${n}"]`

export const TOUR_STEPS: TourStep[] = [
  // ── 시작 ────────────────────────────────────────────────
  { id: 'welcome', route: '/', target: '[data-tour="hero-cta"]', placement: 'bottom' },
  { id: 'quickStart', route: '/', target: '[data-tour="quick-start"]', placement: 'bottom' },

  // ── 1. 클러스터 등록 ────────────────────────────────────
  { id: 'registerCluster', route: '/admin/clusters', target: '[data-tour="register-cluster"]', roles: ['admin'], placement: 'bottom' },
  { id: 'clusterForm', route: '/admin/clusters', activate: '[data-tour="register-cluster"]', target: DIALOG, roles: ['admin'], placement: 'left' },

  // ── 2. 템플릿 고르기 ────────────────────────────────────
  { id: 'pickTemplate', route: '/stack/templates', target: LITE_TEMPLATE, placement: 'right' },
  { id: 'templateDetail', route: '/stack/templates', activate: `${LITE_TEMPLATE} [data-tour="template-detail"]`, target: DIALOG, placement: 'left' },
  { id: 'useBaseTemplate', route: '/stack/templates', activate: `${LITE_TEMPLATE} [data-tour="template-detail"]`, target: '[data-tour="use-base-template"]', placement: 'top' },

  // ── 3. 설치 마법사: 탭을 한 단계씩 ──────────────────────
  { id: 'installAuthentication', route: '/stack/install', activate: tab('authentication'), target: tab('authentication'), union: INSTALL_PANEL, roles: INSTALLER, placement: 'bottom' },
  { id: 'installArtifacts', route: '/stack/install', activate: tab('artifacts'), target: tab('artifacts'), union: INSTALL_PANEL, roles: INSTALLER, placement: 'bottom' },
  { id: 'installPipeline', route: '/stack/install', activate: tab('pipeline'), target: tab('pipeline'), union: INSTALL_PANEL, roles: INSTALLER, placement: 'bottom' },
  { id: 'installObservability', route: '/stack/install', activate: tab('monitoring'), target: tab('monitoring'), union: INSTALL_PANEL, roles: INSTALLER, placement: 'bottom' },
  { id: 'installStorage', route: '/stack/install', activate: tab('storage'), target: tab('storage'), union: INSTALL_PANEL, roles: INSTALLER, placement: 'bottom' },
  { id: 'installResources', route: '/stack/install', activate: tab('resources'), target: tab('resources'), union: INSTALL_PANEL, roles: INSTALLER, placement: 'left' },
  { id: 'installDryRun', route: '/stack/install', activate: tab('dry-run'), target: tab('dry-run'), union: INSTALL_PANEL, roles: INSTALLER, placement: 'bottom' },
  { id: 'deployStack', route: '/stack/install', target: '[data-tour="deploy-stack"]', roles: INSTALLER, placement: 'top' },

  // ── 4. 설치된 스택 보기 ─────────────────────────────────
  { id: 'stackList', route: '/stack/list', target: '[data-tour="stack-list"]', placement: 'top' },
  { id: 'stackWorkloads', route: '/stack/list', activate: [STACK_ROW, tab('workloads')], target: tab('workloads'), union: STACK_PANEL, placement: 'top' },
  { id: 'gatewayPfCopy', route: '/stack/list', activate: [STACK_ROW, tab('info')], target: '[data-tour="gateway-pf-copy"]', placement: 'top' },
  { id: 'hostsCopy', route: '/stack/list', activate: [STACK_ROW, tab('info')], target: '[data-tour="hosts-copy"]', placement: 'top' },
  { id: 'stackMonitoring', route: '/stack/list', activate: [STACK_ROW, tab('monitoring')], target: tab('monitoring'), union: STACK_PANEL, placement: 'top' },

  // ── 5. CI/CD 파이프라인 만들기 ──────────────────────────
  { id: 'cicdBasicInfo', route: '/cicd/developer-deploy', activate: cicdStep(1), target: cicdSection('basic-info'), placement: 'bottom' },
  { id: 'cicdCheckout', route: '/cicd/developer-deploy', activate: cicdStep(2), target: cicdSection('configuration'), placement: 'bottom' },
  { id: 'cicdBuild', route: '/cicd/developer-deploy', activate: cicdStep(3), target: cicdSection('configuration'), placement: 'bottom' },
  { id: 'cicdTest', route: '/cicd/developer-deploy', activate: cicdStep(4), target: cicdStep(4), placement: 'bottom' },
  { id: 'cicdSecurity', route: '/cicd/developer-deploy', activate: cicdStep(5), target: cicdStep(5), placement: 'bottom' },
  { id: 'cicdCreate', route: '/cicd/developer-deploy', activate: cicdStep(6), target: cicdSection('deploy'), placement: 'bottom' },
  { id: 'pipelineList', route: '/cicd/list', target: '[data-tour="pipeline-list"]', placement: 'top' },

  // ── 6. 돌아가는 것 지켜보기 ─────────────────────────────
  { id: 'observe', route: '/observability/monitoring', target: '[data-tour="monitoring"]', placement: 'bottom' },
  { id: 'alertRules', route: '/observability/alerts', target: '[data-tour="alert-rule-new"]', roles: INSTALLER, placement: 'bottom' },

  // ── 마무리 ──────────────────────────────────────────────
  //
  // 설명으로 끝내지 않고 처음 눌러야 할 버튼으로 돌려보낸다. 투어가 끝나는
  // 자리가 곧 시작하는 자리여야 한다.
  { id: 'finish', route: '/', target: '[data-tour="quick-start"]', placement: 'bottom' },
]

/** 역할이 볼 수 있는 걸음만 남긴다. */
export function stepsForRole(role: Role): TourStep[] {
  return TOUR_STEPS.filter((step) => !step.roles || step.roles.includes(role))
}
