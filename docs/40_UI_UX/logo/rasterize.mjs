/**
 * export/*.svg 를 배경 투명 PNG 로 굽는다 — emit.mjs 다음에 돌린다.
 *
 *   node docs/40_UI_UX/logo/rasterize.mjs
 *
 * 크기는 SVG 의 width/height 를 그대로 쓴다(긴 변 2048px). 브라우저로 굽는 이유는
 * 이 마크가 실제로 그려지는 곳이 브라우저이기 때문이다 — 렌더러마다 마스크 처리가
 * 미묘하게 달라지면 끊긴 교차가 PNG 에서만 어긋난다.
 *
 * PNG 에는 원본 SVG 의 sha256 을 tEXt 청크로 남긴다. SVG 만 다시 굽고 PNG 를 잊으면
 * web/src/components/brand/nullus-mark.test.tsx 가 이 값으로 잡는다.
 */
import { createHash } from 'node:crypto'
import { readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { crc32 } from 'node:zlib'

// Playwright 는 web/ 의 devDependency 다(scripts/make-favicon.mjs 와 같은 방식).
const { chromium } = createRequire(new URL('../../../web/package.json', import.meta.url))('@playwright/test')
const dir = new URL('./export/', import.meta.url)

/** IHDR 바로 뒤에 tEXt 청크를 끼운다. PNG 시그니처 8B + IHDR 청크 25B = 33B. */
function withText(png, key, value) {
  const data = Buffer.from(`${key}\0${value}`, 'latin1')
  const type = Buffer.from('tEXt', 'latin1')
  const len = Buffer.alloc(4)
  len.writeUInt32BE(data.length)
  const crc = Buffer.alloc(4)
  crc.writeUInt32BE(crc32(Buffer.concat([type, data])))
  return Buffer.concat([png.subarray(0, 33), len, type, data, crc, png.subarray(33)])
}

const browser = await chromium.launch()
for (const name of readdirSync(dir).filter((n) => n.endsWith('.svg')).sort()) {
  const svg = readFileSync(new URL(name, dir), 'utf8')
  const width = Number(svg.match(/width="(\d+)"/)[1])
  const height = Number(svg.match(/height="(\d+)"/)[1])
  const page = await browser.newPage({ viewport: { width, height }, deviceScaleFactor: 1 })
  // 배경을 투명하게 두어야 어떤 바탕에 얹어도 마크만 보인다.
  await page.setContent(`<style>html,body{margin:0;background:transparent}svg{display:block}</style>${svg}`)
  const png = await page.screenshot({ omitBackground: true })
  await page.close()

  const hash = createHash('sha256').update(svg).digest('hex')
  const out = name.replace(/\.svg$/, '.png')
  writeFileSync(new URL(out, dir), withText(png, 'nullus-source-sha256', hash))
  console.log(`${out} ${width}x${height}`)
}
await browser.close()
