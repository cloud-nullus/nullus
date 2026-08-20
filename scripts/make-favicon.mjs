// knot.svg 를 favicon.ico 로 굽는다.
//
//   node scripts/make-favicon.mjs
//
// knot.svg 를 고쳤을 때만 다시 돌리면 된다. 결과물(.ico)은 저장소에 커밋한다 —
// 아이콘 하나 때문에 브라우저(Playwright)를 CI 의 필수 의존성으로 만들 이유가 없다.
//
// 왜 .ico 인가: 부모 테마의 템플릿이 <link rel="icon" href=".../img/favicon.ico">
// 를 박아 넣고 우리는 템플릿을 복사하지 않는다. 링크가 가리키는 그 자리에 파일을
// 두는 것이 유일하게 자바스크립트 없이 듣는 방법이다.
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
// Playwright 는 web/ 의 devDependency 다. 이 스크립트는 저장소 도구라 scripts/ 에
// 두고, 모듈은 web/node_modules 에서 찾는다.
const { chromium } = createRequire(path.join(here, '../web/package.json'))('@playwright/test')
const theme = path.join(here, '../deploy/helm/nullus/files/keycloak-theme/nullus/login/resources')
const svg = fs.readFileSync(path.join(theme, 'knot.svg'), 'utf8')
const out = path.join(theme, 'img/favicon.ico')
const SIZES = [16, 32, 48]

const browser = await chromium.launch()
const pngs = []
for (const size of SIZES) {
  const page = await browser.newPage({ viewport: { width: size, height: size } })
  // 배경을 투명하게 두어야 탭 배경색과 상관없이 마크만 보인다.
  await page.setContent(
    `<style>html,body{margin:0;background:transparent}svg{display:block;width:${size}px;height:${size}px}</style>` + svg)
  pngs.push(await page.screenshot({ omitBackground: true }))
  await page.close()
}
await browser.close()

// ICO 컨테이너. 각 항목이 PNG 를 그대로 품는다 (Vista 이후 규격).
const head = Buffer.alloc(6)
head.writeUInt16LE(0, 0)            // reserved
head.writeUInt16LE(1, 2)            // type: 아이콘
head.writeUInt16LE(SIZES.length, 4)
let offset = 6 + 16 * SIZES.length
const dir = SIZES.map((size, i) => {
  const e = Buffer.alloc(16)
  e.writeUInt8(size === 256 ? 0 : size, 0)   // 폭 (256 은 0 으로 적는 규격)
  e.writeUInt8(size === 256 ? 0 : size, 1)   // 높이
  e.writeUInt8(0, 2)                          // 팔레트 색 수 (PNG 는 0)
  e.writeUInt8(0, 3)                          // reserved
  e.writeUInt16LE(1, 4)                       // 색 평면
  e.writeUInt16LE(32, 6)                      // 픽셀당 비트
  e.writeUInt32LE(pngs[i].length, 8)
  e.writeUInt32LE(offset, 12)
  offset += pngs[i].length
  return e
})
fs.mkdirSync(path.dirname(out), { recursive: true })
fs.writeFileSync(out, Buffer.concat([head, ...dir, ...pngs]))
console.log(`wrote ${path.relative(process.cwd(), out)} (${SIZES.join('/')}px, ${fs.statSync(out).size}B)`)
