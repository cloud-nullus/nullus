import {
  ARTIFACTS_OPTIONS,
  AUTHENTICATION_OPTIONS,
  LOGGING_OPTIONS,
  MONITORING_OPTIONS,
  PIPELINE_OPTIONS,
  type ToolOption,
} from './install-constants'

/**
 * 도구 이름·설명 문구를 찾는 자리.
 *
 * 문구 키가 `stackAddTools.tools.<id>` 하나뿐이라 **한 도구가 역할마다 다른
 * 것을 뜻하는 경우를 표현하지 못했다.** GitLab 은 소스 저장소이면서 패키지
 * 레지스트리인데 둘 다 id 가 `gitlab` 이라, 설치 마법사와 템플릿 편집기 양쪽에서
 * "GitLab Package Registry" 밑에 "GitLab 소스 코드 관리" 가 붙어 있었다.
 *
 * 그래서 키를 **(슬롯, id) 쌍**으로 넓힌다. 슬롯별 키가 있으면 그것을 쓰고,
 * 없으면 종전의 id 키로 떨어진다 — 실제로 갈라지는 것은 `gitlab` 하나뿐이라
 * 나머지 27개 도구의 번역을 옮겨 적을 이유가 없다.
 */

/**
 * 슬롯 이름 → 그 슬롯이 고를 수 있는 도구.
 *
 * 옵션 그룹의 키가 곧 슬롯 이름이다(`ARTIFACTS_OPTIONS.packageRegistry` 등).
 * 그룹을 가로질러도 이름이 겹치지 않으므로 하나로 합쳐 둔다.
 */
export const TOOL_OPTIONS_BY_SLOT: Record<string, ToolOption[]> = {
  ...ARTIFACTS_OPTIONS,
  ...PIPELINE_OPTIONS,
  ...MONITORING_OPTIONS,
  ...LOGGING_OPTIONS,
  authentication: AUTHENTICATION_OPTIONS,
}

/**
 * `t()` 에 넘길 키 목록. i18next 는 배열을 받으면 **먼저 존재하는 키**를 쓴다.
 *
 * 슬롯을 모르면(옛 호출부) id 키만 돌려주므로 동작이 예전과 같다.
 */
export function toolCopyKeys(
  slot: string | undefined,
  toolId: string,
  field: 'label' | 'description',
): string[] {
  const generic = `stackAddTools.tools.${toolId}.${field}`
  return slot ? [`stackAddTools.slots.${slot}.${toolId}.${field}`, generic] : [generic]
}

/** 슬롯 안에서 도구를 찾는다. 슬롯을 좁혔으므로 id 가 겹쳐도 헷갈리지 않는다. */
export function findSlotOption(slot: string | undefined, predicate: (option: ToolOption) => boolean) {
  if (!slot) return undefined
  return TOOL_OPTIONS_BY_SLOT[slot]?.find(predicate)
}
