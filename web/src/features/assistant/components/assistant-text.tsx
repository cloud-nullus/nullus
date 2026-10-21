import type { ReactNode } from 'react'

// 어시스턴트 답변의 마크다운 일부 — 제목 · 목록 · 표 · 굵게 · 인라인 코드 · 코드 블록 — 를 그린다.
// 모델은 지시와 무관하게 마크다운으로 답하는 일이 많아, 그대로 두면 **·#### 기호가 화면에 남는다.
// HTML 은 만들지 않고 React 요소로만 그린다 — 답변에 섞인 태그는 글자로 보인다.

type Block =
  | { kind: 'p'; text: string }
  | { kind: 'h'; text: string }
  | { kind: 'ul' | 'ol'; items: string[] }
  | { kind: 'pre'; text: string }
  | { kind: 'table'; head: string[]; rows: string[][] }
  | { kind: 'gap' }

const HEADING = /^#{1,6}\s+(.*)$/
const BULLET = /^\s*[-*•]\s+(.*)$/
const NUMBERED = /^\s*\d+[.)]\s+(.*)$/
const FENCE = /^\s*```/
const TABLE_ROW = /^\s*\|.*\|\s*$/
const TABLE_SEPARATOR = /^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?\s*$/

function tableCells(line: string): string[] {
  return line.trim().replace(/^\|/, '').replace(/\|$/, '').split('|').map((cell) => cell.trim())
}

function parseBlocks(text: string): Block[] {
  const blocks: Block[] = []
  const lines = text.replace(/\r\n/g, '\n').split('\n')
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (FENCE.test(line)) {
      const body: string[] = []
      for (i++; i < lines.length && !FENCE.test(lines[i]); i++) body.push(lines[i])
      blocks.push({ kind: 'pre', text: body.join('\n') })
      continue
    }
    // 표: '|' 로 시작하고 끝나는 줄이 이어진다. 둘째 줄이 구분선(|---|)이면 첫 줄이 머리다.
    if (TABLE_ROW.test(line)) {
      const rows: string[] = []
      for (; i < lines.length && TABLE_ROW.test(lines[i]); i++) rows.push(lines[i])
      i--
      const hasHead = rows.length > 1 && TABLE_SEPARATOR.test(rows[1])
      const body = rows.filter((row, n) => !(hasHead && n === 1) && !TABLE_SEPARATOR.test(row))
      blocks.push({
        kind: 'table',
        head: hasHead ? tableCells(body[0]) : [],
        rows: (hasHead ? body.slice(1) : body).map(tableCells),
      })
      continue
    }
    const heading = HEADING.exec(line)
    if (heading) {
      blocks.push({ kind: 'h', text: heading[1] })
      continue
    }
    const bullet = BULLET.exec(line)
    const numbered = bullet ? null : NUMBERED.exec(line)
    if (bullet || numbered) {
      const kind = bullet ? 'ul' : 'ol'
      const item = (bullet ?? numbered)![1]
      const last = blocks[blocks.length - 1]
      if (last && last.kind === kind) last.items.push(item)
      else blocks.push({ kind, items: [item] })
      continue
    }
    if (line.trim() === '') {
      if (blocks.length > 0 && blocks[blocks.length - 1].kind !== 'gap') blocks.push({ kind: 'gap' })
      continue
    }
    blocks.push({ kind: 'p', text: line })
  }
  return blocks
}

// 굵게(**…**)와 인라인 코드(`…`)만 본다. 별 하나(*…*)는 목록 기호와 헷갈려 그대로 둔다.
function renderInline(text: string): ReactNode[] {
  return text.split(/(\*\*[^*]+\*\*|`[^`]+`)/g).map((part, i) => {
    if (part.startsWith('**') && part.endsWith('**') && part.length > 4) return <strong key={i}>{part.slice(2, -2)}</strong>
    if (part.startsWith('`') && part.endsWith('`') && part.length > 2) {
      return (
        <code key={i} className="rounded bg-[color-mix(in_srgb,_var(--color-text-primary)_10%,_transparent)] px-1 text-[12px]">
          {part.slice(1, -1)}
        </code>
      )
    }
    return part
  })
}

export function AssistantText({ text }: { text: string }) {
  return (
    <div className="flex flex-col gap-1">
      {parseBlocks(text).map((block, i) => {
        switch (block.kind) {
          case 'h':
            return (
              <p key={i} className="m-0 mt-1 font-semibold">
                {renderInline(block.text)}
              </p>
            )
          case 'ul':
          case 'ol': {
            const List = block.kind
            return (
              <List key={i} className={`m-0 pl-5 ${block.kind === 'ul' ? 'list-disc' : 'list-decimal'}`}>
                {block.items.map((item, j) => (
                  <li key={j}>{renderInline(item)}</li>
                ))}
              </List>
            )
          }
          case 'pre':
            return (
              <pre key={i} className="m-0 overflow-x-auto whitespace-pre rounded bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)] p-2 text-[12px]">
                {block.text}
              </pre>
            )
          case 'table':
            return (
              <div key={i} className="overflow-x-auto">
                <table className="w-full border-collapse text-[12px]">
                  {block.head.length > 0 && (
                    <thead>
                      <tr>
                        {block.head.map((cell, j) => (
                          <th key={j} className="border-b border-[var(--color-border-default)] px-2 py-1 text-left font-semibold">
                            {renderInline(cell)}
                          </th>
                        ))}
                      </tr>
                    </thead>
                  )}
                  <tbody>
                    {block.rows.map((row, r) => (
                      <tr key={r}>
                        {row.map((cell, j) => (
                          <td key={j} className="border-b border-[var(--color-border-default)] px-2 py-1 align-top">
                            {renderInline(cell)}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )
          case 'gap':
            return <div key={i} className="h-1" aria-hidden />
          default:
            return (
              <p key={i} className="m-0">
                {renderInline(block.text)}
              </p>
            )
        }
      })}
    </div>
  )
}
