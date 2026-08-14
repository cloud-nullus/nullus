#!/usr/bin/env python3
"""CHANGELOG.md 를 검사한다.

두 가지 실제 사고에서 나왔다.

  누락  #114(GitLab 저장소 프로비저닝과 Argo CD 연동)가 CHANGELOG 를 전혀 건드리지
        않고 머지되어, 릴리즈를 자를 때 기능 본체가 통째로 빠져 있었다. 뒤늦게
        커밋 36건을 손으로 대조해 17건을 백필해야 했다.

  중복  릴리즈를 자른 뒤 main 을 되머지하면서, 이미 [0.4.0-alpha] 로 옮긴 항목이
        [Unreleased] 에도 그대로 남았다. 릴리즈 노트에 같은 내용이 두 번 나간다.

둘 다 사람이 눈으로 읽어서는 잘 걸러지지 않는다. 항목 하나가 길고, 파일이 길고,
머지 충돌 해결 화면에서는 더 안 보인다. 그래서 기계가 본다.

로컬에서도 그대로 돌아간다:

    python3 scripts/check_changelog.py                 # 구조만 검사
    python3 scripts/check_changelog.py --changed -     # stdin 의 변경 파일 목록까지
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

CHANGELOG = Path("CHANGELOG.md")

SECTION_RE = re.compile(r"^##\s+\[([^\]]+)\]")
SUBSECTION_RE = re.compile(r"^###\s+(.+?)\s*$")
ENTRY_PREFIX = "- "

# CHANGELOG 항목이 필요 없는 변경. 사용자가 보는 동작이 그대로인 것들이다.
EXEMPT_PATTERNS = (
    re.compile(r"^docs/"),
    re.compile(r"^\.github/ISSUE_TEMPLATE/"),
    re.compile(r"^e2e/"),
    re.compile(r"_test\.go$"),
    re.compile(r"\.test\.tsx?$"),
    re.compile(r"\.spec\.tsx?$"),
    re.compile(r"^CHANGELOG\.md$"),
    re.compile(r"^\.gitignore$"),
    re.compile(r"^LICENSE$"),
    re.compile(r"(^|/)README\.md$"),
)


def parse_sections(text: str) -> dict[str, list[str]]:
    """버전 헤딩별 항목 목록. 헤딩 순서를 유지한다."""
    sections: dict[str, list[str]] = {}
    current: str | None = None
    for line in text.splitlines():
        matched = SECTION_RE.match(line)
        if matched:
            current = matched.group(1)
            sections.setdefault(current, [])
        elif current is not None and line.startswith(ENTRY_PREFIX):
            sections[current].append(line.strip())
    return sections


def check_duplicate_subsections(text: str) -> list[str]:
    """한 릴리즈 안에서 같은 소제목이 두 번 나오는지 본다.

    릴리즈를 자른 뒤 main 을 되머지하면 같은 소제목이 두 벌로 남는다. 항목
    중복(check_structure)과 달리 내용이 겹치지 않아 그 검사에 걸리지 않는데,
    실제로 [0.4.0-alpha] 가 Changed·Fixed 를 각각 두 블록으로 들고 있었다.

    사람이 읽어서는 잘 걸러지지 않는다 — 섹션이 길면 스크롤 밖이라 두 번째
    블록이 보이지 않는다. 릴리즈 노트를 자를 때 한쪽 블록이 통째로 빠진다.
    """
    errors: list[str] = []
    current: str | None = None
    seen: dict[str, int] = {}

    for line in text.splitlines():
        matched = SECTION_RE.match(line)
        if matched:
            current = matched.group(1)
            seen = {}
            continue
        sub = SUBSECTION_RE.match(line)
        if sub and current is not None:
            name = sub.group(1)
            seen[name] = seen.get(name, 0) + 1
            if seen[name] == 2:
                errors.append(
                    f"`[{current}]` 에 `### {name}` 이 두 번 나옵니다. 소제목당 한 "
                    f"블록으로 합치십시오 — 나뉘어 있으면 릴리즈 노트를 자를 때 한쪽이 "
                    f"통째로 빠집니다."
                )
    return errors


def check_structure(text: str) -> list[str]:
    """[Unreleased] 가 있고, 이미 릴리즈된 항목과 겹치지 않는지 본다."""
    errors: list[str] = []
    sections = parse_sections(text)

    if "Unreleased" not in sections:
        errors.append(
            "`## [Unreleased]` 섹션이 없습니다. 다음 릴리즈에 들어갈 항목이 갈 곳이 "
            "없으면 릴리즈를 자를 때 무엇이 새로 들어갔는지 알 수 없습니다."
        )
        return errors

    released = {
        entry: name
        for name, entries in sections.items()
        if name != "Unreleased"
        for entry in entries
    }
    for entry in sections["Unreleased"]:
        if entry in released:
            errors.append(
                f"`[Unreleased]` 의 항목이 이미 릴리즈된 `[{released[entry]}]` 에도 "
                f"있습니다. 릴리즈를 자른 뒤 main 을 되머지하면 생기는 중복입니다 — "
                f"`[Unreleased]` 쪽을 지우십시오.\n"
                f"    {entry[:100]}…"
            )
    return errors


def unquote(path: str) -> str:
    """git 이 감싼 따옴표를 벗긴다.

    경로에 한글이 들어가면 git 은 `"docs/20_\\352\\260\\234..."` 처럼 따옴표로 감싸
    출력한다. 그대로 두면 `^docs/` 같은 면제 패턴이 앞의 따옴표에 걸려 빗나가고,
    문서만 고친 PR 이 CHANGELOG 를 요구받는다. 이 저장소는 문서 경로가 전부
    한글이라 반드시 걸린다.
    """
    if len(path) >= 2 and path.startswith('"') and path.endswith('"'):
        return path[1:-1]
    return path


def needs_entry(changed: list[str]) -> list[str]:
    """CHANGELOG 항목이 필요한 변경 파일만 남긴다."""
    return [
        path
        for path in changed
        if path and not any(pattern.search(unquote(path)) for pattern in EXEMPT_PATTERNS)
    ]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--changed",
        metavar="FILE",
        help="변경된 파일 목록 (한 줄에 하나, '-' 면 stdin). 주면 항목 누락도 검사한다.",
    )
    parser.add_argument(
        "--skip-entry-check",
        action="store_true",
        help="항목 누락 검사를 건너뛴다 (no-changelog 라벨이 붙은 PR).",
    )
    args = parser.parse_args()

    if not CHANGELOG.exists():
        print("CHANGELOG.md 가 없습니다.", file=sys.stderr)
        return 1

    text = CHANGELOG.read_text(encoding="utf-8")
    errors = check_structure(text)
    errors += check_duplicate_subsections(text)

    if args.changed and not args.skip_entry_check:
        source = sys.stdin if args.changed == "-" else open(args.changed, encoding="utf-8")
        with source as handle:
            changed = [line.strip() for line in handle]
        product_changes = needs_entry(changed)
        if product_changes and "CHANGELOG.md" not in changed:
            listed = "\n".join(f"    {p}" for p in product_changes[:10])
            more = f"\n    … 외 {len(product_changes) - 10}건" if len(product_changes) > 10 else ""
            errors.append(
                "동작이 바뀌는 파일을 고쳤는데 `CHANGELOG.md` 를 건드리지 않았습니다. "
                "`## [Unreleased]` 아래에 항목을 추가하십시오.\n"
                f"{listed}{more}\n"
                "    (정말 필요 없다면 PR 에 `no-changelog` 라벨을 붙이십시오.)"
            )

    if errors:
        print("## ❌ CHANGELOG 체크 실패\n", file=sys.stderr)
        for index, error in enumerate(errors, start=1):
            print(f"{index}. {error}\n", file=sys.stderr)
        return 1

    print("CHANGELOG 체크 통과")
    return 0


if __name__ == "__main__":
    sys.exit(main())
