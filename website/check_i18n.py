#!/usr/bin/env python3
"""문서 사이트의 한국어판과 영어판이 짝을 이루는지 본다.

한국어(`X.md`)가 원본이고 영어판은 옆의 `X.en.md` 다. i18n 플러그인은 영어판이
없는 페이지를 한국어로 대신 보여 주므로(fallback) 빌드만으로는 빠진 번역이
드러나지 않는다. 이 검사가 그 빈칸을 막는다.

  1. 페이지마다 짝이 있다 — 한국어만 있거나 영어만 있으면 실패
  2. 두 판의 제목 구조(단계의 순서)와 그림 수가 같다 — 한쪽에만 절을 더하거나
     그림을 넣으면 실패. 번역이 낡았다는 가장 싼 신호다
  3. nav 의 한국어 제목마다 영어 번역(nav_translations)이 있다
  4. 화면 캡처마다 영어판(`*.en.png`)이 있다

실행: python3 website/check_i18n.py
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent
DOCS = ROOT / "docs"
CONFIG = ROOT / "mkdocs.yml"
SCREENS = DOCS / "assets" / "screens"

FENCE = re.compile(r"^\s*(```|~~~)")
HEADING = re.compile(r"^(#{1,6})\s")
IMAGE = re.compile(r"!\[[^\]]*\]\(")
HANGUL = re.compile(r"[가-힣]")


def outline(path: Path) -> tuple[list[int], int]:
    """제목 단계의 순서와 그림 수. 코드 블록 안의 `#` 주석은 제목이 아니다."""
    levels: list[int] = []
    images = 0
    in_fence = False
    for line in path.read_text(encoding="utf-8").splitlines():
        if FENCE.match(line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        m = HEADING.match(line)
        if m:
            levels.append(len(m.group(1)))
        images += len(IMAGE.findall(line))
    return levels, images


def check_pages() -> list[str]:
    errors: list[str] = []
    for page in sorted(DOCS.rglob("*.md")):
        rel = page.relative_to(DOCS)
        if page.name.endswith(".en.md"):
            source = page.with_name(page.name[: -len(".en.md")] + ".md")
            if not source.exists():
                errors.append(f"{rel}: 한국어 원본({source.relative_to(DOCS)})이 없습니다")
            continue

        english = page.with_name(page.stem + ".en.md")
        if not english.exists():
            errors.append(f"{rel}: 영어판({english.relative_to(DOCS)})이 없습니다")
            continue

        ko_levels, ko_images = outline(page)
        en_levels, en_images = outline(english)
        if ko_levels != en_levels:
            errors.append(
                f"{rel}: 제목 구조가 영어판과 다릅니다 "
                f"(한국어 {len(ko_levels)}개, 영어 {len(en_levels)}개) — 한쪽만 고쳤는지 보세요"
            )
        if ko_images != en_images:
            errors.append(f"{rel}: 그림 수가 영어판과 다릅니다 (한국어 {ko_images}, 영어 {en_images})")
    return errors


def nav_titles(config: str) -> set[str]:
    """`nav:` 블록의 제목들. `- 제목: 경로` 와 `- 제목:`(절) 둘 다."""
    titles: set[str] = set()
    in_nav = False
    for line in config.splitlines():
        if line.startswith("nav:"):
            in_nav = True
            continue
        if in_nav and line and not line.startswith(" "):
            break
        if in_nav:
            m = re.match(r"^\s*-\s+(.+?):(\s|$)", line)
            if m:
                titles.add(m.group(1).strip())
    return titles


def nav_translations(config: str) -> set[str]:
    keys: set[str] = set()
    lines = config.splitlines()
    for i, line in enumerate(lines):
        if line.strip() != "nav_translations:":
            continue
        indent = len(line) - len(line.lstrip())
        for item in lines[i + 1 :]:
            if item.strip().startswith("#"):
                continue
            if not item.strip() or len(item) - len(item.lstrip()) <= indent:
                break
            keys.add(item.split(":", 1)[0].strip())
    return keys


def check_nav() -> list[str]:
    config = CONFIG.read_text(encoding="utf-8")
    # 처음부터 영어인 제목(API · CLI · DevOps …)은 바꿀 것이 없다.
    titles = {t for t in nav_titles(config) if HANGUL.search(t)}
    translated = nav_translations(config)
    errors = [f"mkdocs.yml: nav 제목 '{t}' 의 영어 번역이 nav_translations 에 없습니다" for t in sorted(titles - translated)]
    errors += [f"mkdocs.yml: nav_translations 의 '{t}' 는 nav 에 없는 제목입니다" for t in sorted(translated - titles)]
    return errors


def check_screens() -> list[str]:
    errors: list[str] = []
    for shot in sorted(SCREENS.glob("*.png")):
        if shot.name.endswith(".en.png"):
            continue
        english = shot.with_name(shot.stem + ".en.png")
        if not english.exists():
            errors.append(
                f"assets/screens/{shot.name}: 영어 캡처({english.name})가 없습니다 — "
                "web 에서 npm run shots:docs 로 두 언어를 함께 찍습니다"
            )
    return errors


def main() -> int:
    errors = check_pages() + check_nav() + check_screens()
    for error in errors:
        print(f"[i18n] {error}", file=sys.stderr)
    if errors:
        print(f"[i18n] {len(errors)}건 — 한국어와 영어를 함께 고쳐야 합니다", file=sys.stderr)
        return 1
    pages = len([p for p in DOCS.rglob("*.md") if not p.name.endswith(".en.md")])
    print(f"[i18n] 페이지 {pages}개 · 캡처 모두 한국어/영어 짝이 맞습니다")
    return 0


if __name__ == "__main__":
    sys.exit(main())
