// 화면 정보 인벤토리 추출기 — UI 개편의 "정보 유실 0" 계약을 기계적으로 고정한다.
//
// 각 화면 파일의 TypeScript AST를 훑어서 사용자에게 보이는 정보만 뽑는다:
//   - i18n 키와 인라인 폴백 문자열
//   - 표 컬럼 정의(accessorKey / header / field)
//   - 라벨성 prop (label, placeholder, title, aria-label, emptyMessage ...)
//   - JSX 텍스트 노드
//   - data-testid
//
// 스타일(색, 클래스, 여백)은 의도적으로 무시한다 — 그건 바뀌어야 하는 것이고,
// 여기서 고정하려는 건 "무엇이 보이는가" 뿐이다.
//
// 사용법:
//   node scripts/extract-ui-inventory.mjs            # snapshot 갱신 + 마크다운 생성
//   node scripts/extract-ui-inventory.mjs --check     # snapshot 과 비교, 유실 시 exit 1

import ts from 'typescript'
import { readFileSync, writeFileSync, readdirSync, statSync, existsSync } from 'node:fs'
import { join, relative, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const WEB_ROOT = join(dirname(fileURLToPath(import.meta.url)), '..')
const SRC = join(WEB_ROOT, 'src')
const SNAPSHOT = join(WEB_ROOT, 'e2e', 'inventory', 'ui-inventory.json')
const MARKDOWN = join(
  WEB_ROOT,
  '..',
  'docs',
  '40_UI_UX',
  '화면_정보_인벤토리.md',
)

/** 라벨성 값을 담는 prop / 객체 키 — 여기 붙은 문자열은 사용자에게 보인다. */
const LABEL_KEYS = new Set([
  'label',
  'header',
  'headerName',
  'title',
  'placeholder',
  'aria-label',
  'ariaLabel',
  'emptyMessage',
  'description',
  'confirmLabel',
  'cancelLabel',
  'message',
  'accessorKey',
  'field',
  'id',
  'name',
])

function walkFiles(dir, out = []) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) {
      walkFiles(full, out)
    } else if (/\.tsx$/.test(entry) && !/\.test\.tsx$/.test(entry)) {
      out.push(full)
    }
  }
  return out
}

function literalText(node) {
  if (!node) return null
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
    return node.text
  }
  return null
}

function extract(filePath) {
  const source = readFileSync(filePath, 'utf8')
  const sf = ts.createSourceFile(filePath, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX)

  const i18nKeys = new Set()
  const fallbacks = new Set()
  const labels = new Set()
  const columns = new Set()
  const jsxText = new Set()
  const testIds = new Set()

  function visit(node) {
    // t('key', 'fallback')
    if (
      ts.isCallExpression(node) &&
      ((ts.isIdentifier(node.expression) && node.expression.text === 't') ||
        (ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === 't'))
    ) {
      const key = literalText(node.arguments[0])
      if (key) i18nKeys.add(key)
      const fallback = literalText(node.arguments[1])
      if (fallback) fallbacks.add(fallback)
    }

    // { header: 'Pipeline', accessorKey: 'status' } — 표 컬럼 정의
    if (ts.isPropertyAssignment(node) && node.name) {
      const key = ts.isIdentifier(node.name)
        ? node.name.text
        : ts.isStringLiteral(node.name)
          ? node.name.text
          : null
      const value = literalText(node.initializer)
      if (key && value) {
        if (key === 'accessorKey' || key === 'field') columns.add(value)
        else if (LABEL_KEYS.has(key)) labels.add(value)
      }
    }

    // <X label="..." placeholder="..." data-testid="..." />
    if (ts.isJsxAttribute(node) && node.name && node.initializer) {
      const attr = node.name.getText(sf)
      const value = ts.isStringLiteral(node.initializer)
        ? node.initializer.text
        : ts.isJsxExpression(node.initializer)
          ? literalText(node.initializer.expression)
          : null
      if (value) {
        if (attr === 'data-testid') testIds.add(value)
        else if (LABEL_KEYS.has(attr)) labels.add(value)
      }
    }

    // JSX 사이의 맨 텍스트
    if (ts.isJsxText(node)) {
      const text = node.text.trim()
      if (text && /[A-Za-z가-힣0-9]/.test(text)) jsxText.add(text)
    }

    ts.forEachChild(node, visit)
  }

  visit(sf)

  const sorted = (set) => [...set].sort()
  return {
    file: relative(WEB_ROOT, filePath).replace(/\\/g, '/'),
    i18nKeys: sorted(i18nKeys),
    fallbacks: sorted(fallbacks),
    labels: sorted(labels),
    columns: sorted(columns),
    jsxText: sorted(jsxText),
    testIds: sorted(testIds),
  }
}

/** 화면(page) 과 컴포넌트를 나눠 담는다. */
function build() {
  const files = walkFiles(SRC).sort()
  const entries = files.map(extract).filter(
    (e) =>
      e.i18nKeys.length ||
      e.fallbacks.length ||
      e.labels.length ||
      e.columns.length ||
      e.jsxText.length ||
      e.testIds.length,
  )
  return { entries }
}

function countAll(inv) {
  return inv.entries.reduce(
    (acc, e) => {
      acc.i18nKeys += e.i18nKeys.length
      acc.fallbacks += e.fallbacks.length
      acc.labels += e.labels.length
      acc.columns += e.columns.length
      acc.jsxText += e.jsxText.length
      acc.testIds += e.testIds.length
      return acc
    },
    { i18nKeys: 0, fallbacks: 0, labels: 0, columns: 0, jsxText: 0, testIds: 0 },
  )
}

function toMarkdown(inv) {
  const total = countAll(inv)
  const pages = inv.entries.filter((e) => e.file.includes('/pages/'))
  const components = inv.entries.filter((e) => !e.file.includes('/pages/'))

  const lines = []
  lines.push('# 화면 정보 인벤토리 (자동 생성)')
  lines.push('')
  lines.push('> **이 파일은 손으로 고치지 않는다.** `node scripts/extract-ui-inventory.mjs` 가 생성한다.')
  lines.push('>')
  lines.push('> UI 전면 개편(`docs/40_UI_UX/Nullus_UIUX_전면개편_기획안.md`)의 "정보 유실 0" 계약이다.')
  lines.push('> 개편 커밋마다 `node scripts/extract-ui-inventory.mjs --check` 를 돌려 아래 정보가')
  lines.push('> 사라지지 않았는지 확인한다. 스타일(색·클래스·여백)은 의도적으로 추적하지 않는다.')
  lines.push('')
  lines.push('## 총계')
  lines.push('')
  lines.push('| 항목 | 개수 |')
  lines.push('|------|------|')
  lines.push(`| 추적 대상 파일 | ${inv.entries.length} |`)
  lines.push(`| 화면(pages) | ${pages.length} |`)
  lines.push(`| 공용 컴포넌트 | ${components.length} |`)
  lines.push(`| i18n 키 | ${total.i18nKeys} |`)
  lines.push(`| 인라인 폴백 문자열 | ${total.fallbacks} |`)
  lines.push(`| 라벨성 문자열 | ${total.labels} |`)
  lines.push(`| 표 컬럼 필드 | ${total.columns} |`)
  lines.push(`| JSX 텍스트 | ${total.jsxText} |`)
  lines.push(`| data-testid | ${total.testIds} |`)
  lines.push('')

  const section = (title, list) => {
    lines.push(`## ${title}`)
    lines.push('')
    for (const e of list) {
      lines.push(`### \`${e.file}\``)
      lines.push('')
      const row = (label, values) => {
        if (!values.length) return
        lines.push(`- **${label}** (${values.length}): ${values.map((v) => `\`${v}\``).join(', ')}`)
      }
      row('표 컬럼', e.columns)
      row('i18n 키', e.i18nKeys)
      row('표시 문자열(폴백)', e.fallbacks)
      row('라벨', e.labels)
      row('JSX 텍스트', e.jsxText)
      row('testid', e.testIds)
      lines.push('')
    }
  }

  section('화면 (pages)', pages)
  section('공용 컴포넌트', components)

  return lines.join('\n')
}

/**
 * 이전 스냅샷 대비 사라진 항목만 보고한다(추가는 허용).
 *
 * 판정 기준은 "이 사용자에게 보이던 문자열이 소스 어딘가에 아직 있는가" 다.
 * 그래서 파일 경계와 **필드 종류를 모두 넘나들며** 찾는다:
 *   - 파일 경계: 컴포넌트 추출로 다른 파일로 옮겨가도 정보는 남아 있다
 *   - 필드 경계: 하드코딩 라벨이 t() 의 인라인 폴백으로 옮겨가는 것은
 *     리팩터링이지 유실이 아니다 (labels → fallbacks 이동)
 * 단, i18n 키와 표 컬럼 필드명은 종류가 다른 식별자이므로 자기 종류 안에서만 찾는다.
 */
const TEXT_FIELDS = ['fallbacks', 'labels', 'jsxText']
const ID_FIELDS = ['i18nKeys', 'columns', 'testIds']

/**
 * i18n 리소스 파일의 모든 문자열 값.
 *
 * 하드코딩된 한글을 `t()` 로 빼면 그 문자열은 TSX 에서 사라지고 ko.json 으로
 * 옮겨간다 — 화면에는 그대로 나오므로 유실이 아니다. 추출기가 TSX 만 훑으면
 * 그걸 유실로 오판하니, 리소스 파일도 "아직 있는 곳"에 포함한다.
 */
function i18nStrings() {
  const values = new Set()
  const collect = (node) => {
    for (const value of Object.values(node)) {
      if (value !== null && typeof value === 'object') collect(value)
      else if (typeof value === 'string') values.add(value)
    }
  }
  for (const locale of ['en', 'ko']) {
    const path = join(SRC, 'i18n', `${locale}.json`)
    if (existsSync(path)) collect(JSON.parse(readFileSync(path, 'utf8')))
  }
  return values
}

function check(prev, next) {
  const union = (fields) => new Set(next.entries.flatMap((e) => fields.flatMap((f) => e[f])))
  const textAnywhere = new Set([...union(TEXT_FIELDS), ...i18nStrings()])
  const idAnywhere = Object.fromEntries(ID_FIELDS.map((f) => [f, union([f])]))

  const losses = []
  for (const before of prev.entries) {
    for (const field of [...TEXT_FIELDS, ...ID_FIELDS]) {
      const stillThere = TEXT_FIELDS.includes(field) ? textAnywhere : idAnywhere[field]
      for (const value of before[field]) {
        if (!stillThere.has(value)) losses.push({ file: before.file, field, value })
      }
    }
  }
  return losses
}

const isCheck = process.argv.includes('--check')
const next = build()

if (isCheck) {
  if (!existsSync(SNAPSHOT)) {
    console.error(`스냅샷이 없다: ${SNAPSHOT}\n먼저 --check 없이 실행해 기준선을 만든다.`)
    process.exit(1)
  }
  const prev = JSON.parse(readFileSync(SNAPSHOT, 'utf8'))
  const losses = check(prev, next)
  const total = countAll(next)
  if (losses.length === 0) {
    console.log(
      `정보 유실 없음 — 파일 ${next.entries.length}개 / 컬럼 ${total.columns} / i18n ${total.i18nKeys} / 문자열 ${total.fallbacks + total.labels + total.jsxText}`,
    )
    process.exit(0)
  }
  console.error(`정보 유실 ${losses.length}건 발견:\n`)
  const byFile = new Map()
  for (const l of losses) {
    if (!byFile.has(l.file)) byFile.set(l.file, [])
    byFile.get(l.file).push(l)
  }
  for (const [file, list] of byFile) {
    console.error(`  ${file}`)
    for (const l of list) console.error(`    - [${l.field}] ${JSON.stringify(l.value)}`)
  }
  console.error(
    '\n의도한 제거라면 기획안 §8.1 절차대로 승인을 받고 스냅샷을 갱신한다(--check 없이 재실행).',
  )
  process.exit(1)
}

writeFileSync(SNAPSHOT, `${JSON.stringify(next, null, 2)}\n`)
writeFileSync(MARKDOWN, `${toMarkdown(next)}\n`)
const total = countAll(next)
console.log(`스냅샷 갱신: ${relative(WEB_ROOT, SNAPSHOT)}`)
console.log(`마크다운 갱신: ${relative(WEB_ROOT, MARKDOWN)}`)
console.log(
  `파일 ${next.entries.length} / 컬럼 ${total.columns} / i18n ${total.i18nKeys} / 문자열 ${total.fallbacks + total.labels + total.jsxText} / testid ${total.testIds}`,
)
