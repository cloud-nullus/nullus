import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SupportToolsMarquee } from './support-tools-marquee'

describe('SupportToolsMarquee', () => {
  // GitHub·GitHub Actions 는 클러스터 밖 서비스라 설치하지 않고 연동만 한다. 띠 전체를
  // "설치하는 오픈소스" 라고 부르면 그 둘에 대해 사실이 아니다.
  it('describes the strip as tools Nullus installs or connects to', () => {
    render(<SupportToolsMarquee />)

    expect(screen.getByText(/installs into your cluster or connects to/)).toBeInTheDocument()
    expect(screen.queryByText(/^Open source/)).toBeNull()
  })

  // 제품 이름을 글자로 적는 것은 사실 표시다. 제휴·보증 관계가 아니라는 고지를 함께 둔다.
  it('states that product names belong to their owners and imply no affiliation', () => {
    render(<SupportToolsMarquee />)

    expect(screen.getByText(/trademarks of their respective owners/)).toBeInTheDocument()
    expect(screen.getByText(/not affiliated with or endorsed by/)).toBeInTheDocument()
  })

  // 설치되는 것은 MinIO 포크 SILO 다. MinIO 상표 정책은 수정판에 MinIO 이름을 붙이거나
  // 다른 단어와 묶는 것을 금지한다 — 카드에 MinIO 를 적지 않는다.
  it('lists the forked object storage as SILO, never under the MinIO name', () => {
    render(<SupportToolsMarquee />)

    expect(screen.getAllByText('SILO').length).toBeGreaterThan(0)
    expect(screen.queryAllByText(/MinIO/)).toEqual([])
  })
})
