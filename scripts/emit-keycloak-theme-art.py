#!/usr/bin/env python3
"""Keycloak 로그인 화면 왼쪽의 아이소메트릭 일러스트를 만든다.

  python3 scripts/emit-keycloak-theme-art.py [출력폴더]

장면 (좌 -> 우로 읽는다):
  왼쪽 아래에서 올라오는 컨베이어 위에 크레인이 검증된 OSS 를 하나씩 얹어
  스택을 쌓는다 -> 그 스택이 Nullus 게이트로 들어간다 -> 반대편으로 Nullus
  로고가 붙은 큰 상자(= 완성된 스택)가 나오고, 그 위에 다시 크레인이
  React·Go·Spring 을 얹는다 -> 제품이 되어 오른쪽 아래로 나간다.

정지 그림 두 장과 움직이는 조각들, 그리고 그 조각들의 자리·타이밍을 담은 CSS 를
쓴다. 손으로 고치지 말고 이 파일을 고친 뒤 다시 실행한다
(deploy/helm/keycloak_theme_test.go 가 둘이 어긋나면 실패한다).

왜 이렇게 쪼개는가. 장면이 움직여야 하는데 SVG 안에 애니메이션을 넣으면 WebKit
이 CSS 배경 이미지를 정지 프레임으로 렌더해 멈춘 채로 보인다. 그래서 움직이는
것은 전부 DOM 요소로 두고 CSS 로 돌린다. 대신 조각들이 정지 장면 위 제자리에
앉아야 하므로 좌표를 그림과 같은 계산에서 여기서 한꺼번에 뽑는다.

아이소메트릭 기저: x 는 오른쪽-아래, y 는 왼쪽-아래, z 는 위.
  iso(1,0,0) = ( cos30,  0.5)
  iso(0,1,0) = (-cos30,  0.5)
  iso(0,0,1) = (     0, -1  )
z 가 순수 세로라서 크레인이 내리고 올리는 동작은 세로 이동 하나로 끝난다.

그림에는 검은 윤곽선을 쓰지 않는다. 면끼리 밝기 차로 갈라 모형 사진처럼 보이게
하고, 같은 색 면이 맞닿는 데만 그 색을 어둡게 한 실선을 얇게 넣는다.
"""
import math
import pathlib
import re

COS30 = math.sqrt(3) / 2
UNIT = 25.0
PAD = 16.0

REPO = pathlib.Path(__file__).resolve().parent.parent
ICON_DIR = REPO / "web/public/tool-icons"
MARK_GEOMETRY = REPO / "web/src/components/brand/mark-geometry.generated.ts"
DEFAULT_OUT = "deploy/helm/nullus/files/keycloak-theme/nullus/login/resources"

FONT = "system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', sans-serif"

# ── 색 ────────────────────────────────────────────────────────────────────
# (윗면, +x 면, +y 면). 참고한 모형 이미지처럼 윤곽선 없이 밝기 차로만 가른다.
CARD = ("#d9b382", "#c1976a", "#a87f56")      # 골판지
BELT = ("#2f3a4c", "#3b4759", "#232c3b")      # 벨트 윗면과 옆면
RAIL = ("#9aa4b2", "#828d9c", "#6f7987")      # 컨베이어 프레임
LEG = ("#7b8592", "#69737f", "#59626d")
JAW = ("#57616f", "#49525f", "#3d4550")      # 집게 — 크레인 뼈대보다 한 단 어둡게
PALLET = ("#f2c229", "#d8a81d", "#bd8f15")     # 판넬 — 노란 철판
PAL_RIB = "#a87f10"                            # 철판 위 미끄럼 방지 골
MACH = ("#b6bfca", "#9ea8b4", "#88919d")      # 게이트 몸통
MOUTH = ("#1a212b", "#141a22", "#0f141a")     # 게이트 개구부
SCAN = ("#8f99a7", "#7a8493", "#68717e")

NULLUS_BLUE = "#2f5fe0"
BADGE_GREEN = "#22c07f"

# 들어가는 OSS. 제품이 실제로 설치하는 도구 중 아이콘이 있는 것들이다
# (internal/stack/domain 의 스택 템플릿 ↔ web/public/tool-icons).
# 화면에는 이 중 셋만 실리고, 어느 셋인지는 접속할 때마다 달라진다 —
# 카탈로그가 넓다는 게 제품의 요지라 매번 같은 셋만 보이면 그게 가려진다.
OSS_CATALOG = [
    "gitlab", "gitea", "jenkins", "argo", "harbor", "jfrog", "sonatype", "minio",
    "openbao", "prometheus", "grafana", "elasticsearch", "opensearch", "jaeger",
    "opentelemetry", "victoriametrics",
]
# 자바스크립트가 막힌 환경에서 보일 기본 셋.
OSS_DEFAULT = ["gitlab", "argo", "grafana"]

# 나온 Nullus 상자 위에 쌓이는 애플리케이션 스택.
# 이 셋은 도구 카탈로그(web/public/tool-icons)에 없어서 여기서 직접 그린다.
APPS = [("react", "#2aa5c7"), ("go", "#0090b4"), ("spring", "#5c9c34")]

# ── 배치 (월드 좌표) ──────────────────────────────────────────────────────
# 입구 벨트는 y 축을 따라 (y 가 큰 화면 왼쪽 아래 -> 게이트),
# 출구 벨트는 x 축을 따라 (게이트 -> x 가 큰 화면 오른쪽 아래) 흐른다.
# 둘 다 게이트가 있는 원점에서 만나 참고한 모형 이미지의 꺾인 구도가 된다.
BELT_W = 3.4
HALF = BELT_W / 2
BELT_TOP = 2.4
FRAME_BOT = 1.3

IN_Y_FAR = 24.0
OUT_X_FAR = 26.0

GATE_HALF = 3.2
GATE_Z0, GATE_Z1 = 0.3, 12.2
# 개구부는 통과하는 스택보다 높아야 한다. 낮으면 문보다 큰 짐이 지나간다.
MOUTH_Z0, MOUTH_Z1 = FRAME_BOT, 7.3

# 후크에 매달린 컨테이너가 스택 꼭대기보다 확실히 높아야 한다. 붙으면
# 매달린 것과 얹힌 것이 구별되지 않는다.
# 라인 설비. 경광등은 벨트 가까운 쪽에, 키오스크는 나가는 쪽 앞에 선다.
# (x, y) 는 기둥 밑동. 벨트를 따라 가는 축이 다르므로 좌표 순서도 다르다.
# 경광등. 벨트 양편에 번갈아, 간격도 일부러 고르지 않게 둔다 — 한쪽에 같은
# 간격으로 세우면 설비가 아니라 무늬로 읽힌다.
# (x, y, 색, 깜빡임 지연, 층). 층은 벨트의 어느 편에 섰는지다 — 먼 편("back")에
# 선 것은 벨트보다 뒤에 그려야 한다. 앞 층에 두면 벨트를 덮어 공중에 뜬 것처럼
# 보인다.
BEACON_AMBER = "#ffa62e"
BEACON_GREEN = "#33c76b"
# 벨트 프레임 앞면에 붙는 표시등. 바닥에서 올라오는 기둥으로 두면 기둥이 벨트에
# 가려 알만 공중에 뜬 것처럼 보이고, 건너편에 세워도 마찬가지다. 프레임에 직접
# 붙이면 어디에 달렸는지가 분명하고, 벨트 윗면(2.4) 아래에 있어 그 위를 지나는
# 화물과 화면에서 아예 겹치지 않는다.
#   (pos, 라인, 색, 깜빡임 지연) — 라인 "in" 은 입구(x=+HALF 면), "out" 은 출구
BEACON_Z = 1.75                               # 알 아랫변. 여기보다 높으면 알 윗변이
                                              # 화물 상자 아랫변 위로 올라온다
BEACONS = [
    (21.4, "in", BEACON_AMBER, -0.00),
    (17.2, "in", BEACON_GREEN, -0.90),
    (8.5, "in", BEACON_GREEN, -0.35),
    (13.2, "out", BEACON_AMBER, -1.25),
    (18.0, "out", BEACON_GREEN, -0.60),
    (23.4, "out", BEACON_AMBER, -1.05),
]


def beacon_pos(pos, line):
    """알의 (뒤쪽 위 모서리) 세계 좌표. 프레임 앞면에서 살짝 튀어나온다."""
    out = 0.1
    if line == "in":
        return HALF + out, pos - LAMP / 2, BEACON_Z + LAMP
    return pos - LAMP / 2, HALF + out, BEACON_Z + LAMP
LAMP = 0.36                                   # 표시등 알 — 정육면체 한 변
# 나가는 라인 입구 쪽(게이트에 가까운 자리)에 나란히 선다. 모양을 달리하는
# 이유는 하나다 — 같은 모양 둘을 세우면 서로 다른 일을 하는 설비로 읽히지 않는다.
#   monitor : 기둥 위에 얹힌 가로 모니터
#   console : 바닥에 선 콘솔 (캐비닛 + 키보드 선반 + 작은 화면)
KIOSKS = [(6.8, HALF + 1.5, "monitor"), (11.4, HALF + 1.9, "console")]

# 올라오는 벨트 앞에 앉아 화면을 보며 스택을 짜는 사람. 라인 옆에 사람이 하나
# 있어야 "설비" 가 아니라 "누가 쓰는 도구" 로 읽힌다.
DEV_POS = (HALF + 3.0, 15.6)
# 책상은 벨트를 마주 보게 놓는다 — 긴 변이 y 축, 사람은 +x 쪽에 앉아 -x(벨트)를
# 본다. DESK_LEN 은 긴 변(y), DESK_DEP 은 짧은 변(x)이다.
DESK_LEN, DESK_DEP, DESK_H = 3.6, 1.8, 1.55

MON_POST = 2.0
MON_W, MON_D, MON_H = 3.8, 0.5, 2.2
MON_INSET = 0.2

CON_W, CON_D, CON_H = 2.4, 1.1, 1.7      # 아래 캐비닛
CON_SCR_W, CON_SCR_D, CON_SCR_H = 2.2, 0.55, 1.7
CON_SCR_Z = CON_H + 0.45
CON_INSET = 0.17

CRANE_TOP = 10.4

# 안무의 기준점 (한 크레인 주기를 0~1 로).
# 짐은 HOIST_SHOW 에 후크에 나타나 HOIST_DROP_FROM 부터 내려가고 0.5 에 놓인다.
# nullus.css 의 @keyframes nl-hoist 와 같은 값이어야 한다.
HOIST_DROP_FROM = 0.38
# 쉴 때 후크에서 집게까지 남는 케이블. 0 이면 집게가 지브에 딱 붙어 한 덩어리로
# 보인다 — 실제 크레인처럼 조금 내려와 매달려 있어야 한다.
HOOK_REST = 0.95
# 크레인이 다음 짐을 집는 시점 (한 바퀴를 0~1 로). 받침이 들어오는 때부터 이미
# 물고 있어야 라인이 놀고 있는 것처럼 보이지 않는다. 0 보다 뒤여야 하는 이유는
# 입구 쪽 OSS 를 한 바퀴마다 다시 고르기 때문이다 — 경계에서 그 도구가 화면에
# 하나도 없어야 바뀌는 게 눈에 띄지 않는다.
GRAB_AT = 0.04
# 나오는 컨테이너가 다 드러나는 시점 (nullus.css 의 @keyframes nl-out-base).
OUT_FADE_IN = 0.04
LEG_T = 0.45
JIB_BACK = 3.8            # 마스트는 벨트 뒤(먼 쪽)에 선다

IN_CRANE_YS = [18.0, 12.5, 7.0]     # 올라오면서 위로 차례로 쌓인다
# 나가면서 긴 상자를 오른쪽부터 채운다. 첫 크레인은 컨테이너가 게이트를 완전히
# 빠져나온 뒤에 내려오도록 충분히 오른쪽에 둔다 (아래 단언이 지킨다).
OUT_CRANE_XS = [12.0, 15.5, 19.0]

# 흐르는 조각
# 들어가는 스택과 나오는 스택의 총 높이를 같게 맞춘다 (3*OSS_H = NUL_H+3*APP_H).
# 다르면 같은 것이 게이트를 통과한 것으로 읽히지 않는다.
# 판넬은 싣는 것에 맞춰 두 규격이다 — 나가는 쪽은 긴 상자를 받쳐야 한다.
PAL_H = 0.3
PAL_IN_W, PAL_IN_D = 3.0, 2.6
OSS_W, OSS_D, OSS_H = 2.4, 2.1, 1.2

# 나가는 쪽은 긴 상자 하나에 애플리케이션 셋을 같은 높이로 나란히 얹는다.
# APP_SLOT 은 자리 사이 간격(월드 x). 오른쪽 자리부터 채워진다.
# 자리 간격은 컨테이너 폭 + 집게가 들어갈 틈이어야 한다. 붙여 두면 집게발이
# 옆 컨테이너 자리를 파고들어, 그리는 순서를 어떻게 잡아도 하나가 가려진다.
APP_W, APP_D, APP_H = 1.5, 1.35, 1.1
APP_SLOT = APP_W + 0.45
# 세 자리를 상자 뒤쪽 모서리에 붙인다. 깊이 한가운데에 두면 앞 모서리에 걸친
# 것처럼 보여 얹혔다는 느낌이 나지 않는다. 앞에 남는 턱이 받침대로 읽힌다.
APP_SLOT_Y = -0.37
NUL_W, NUL_D, NUL_H = APP_SLOT * 2 + APP_W + 0.5, 2.45, 2.5
PAL_OUT_W, PAL_OUT_D = NUL_W + 0.25, 2.65

# 게이트 안까지 들어가 사라진다. 끝점을 너무 깊이 두면 사라지는 시점이 한 바퀴의
# 앞쪽으로 당겨져, 왼쪽이 빈 채로 오른쪽만 도는 구간이 길어진다.
IN_START, IN_END = IN_Y_FAR, 1.0
# 사라지고 나타나는 지점은 상자가 게이트 벽면과 겹치지 않는 자리여야 한다.
# 겹치면 상자가 벽을 뚫고 나온 것처럼 보인다 — 상자 절반 깊이만큼 띄운다.
IN_VANISH = GATE_HALF + PAL_IN_D / 2 + 0.3
# 끝점을 벨트 끝보다 앞에 둔다. 넘겨 두면 완성되자마자 화면 밖으로 잘린다.
OUT_START, OUT_END = GATE_HALF + PAL_OUT_W / 2 + 0.3, OUT_X_FAR - 0.5
# 완성 배지. 마지막 크레인이 놓은 직후여야 하고, 붙은 뒤에도 제품이 한동안
# 온전히 보이도록 끝점에서 충분히 앞에 둔다.
OUT_BADGE_X = 21.35

def iso_raw(x, y, z=0.0):
    return ((x - y) * COS30 * UNIT, ((x + y) * 0.5 - z) * UNIT)


def shade_all(colors, f):
    """면 세 벌을 통째로 밝기만 민다. box() 는 세 면 색을 받으므로 shade() 하나를
    그대로 넘기면 터진다 — 그 실수를 막으려고 둔다."""
    return tuple(shade(c, f) for c in colors)


def shade(hex_color, f):
    """면 색을 어둡게(f<1) 또는 밝게(f>1) 민다. 맞닿은 같은 색 면을 가르는 데 쓴다."""
    r, g, b = (int(hex_color[i:i + 2], 16) for i in (1, 3, 5))
    c = lambda v: max(0, min(255, round(v * f)))
    return f"#{c(r):02x}{c(g):02x}{c(b):02x}"


def tint(hex_color, amount):
    """색을 흰색 쪽으로 옅게 민다. amount 는 원래 색을 남기는 비율."""
    r, g, b = (int(hex_color[i:i + 2], 16) for i in (1, 3, 5))
    m = lambda v: round(255 - (255 - v) * amount)
    return f"#{m(r):02x}{m(g):02x}{m(b):02x}"


class Draw:
    """그리면서 화면 좌표의 경계를 같이 잰다. viewBox 를 손으로 적으면 장면을
    조금만 옮겨도 잘리거나 남는다."""

    def __init__(self):
        self.items = []
        self.min_x = self.min_y = float("inf")
        self.max_x = self.max_y = float("-inf")

    def mark(self, px, py):
        self.min_x, self.max_x = min(self.min_x, px), max(self.max_x, px)
        self.min_y, self.max_y = min(self.min_y, py), max(self.max_y, py)

    def iso(self, x, y, z=0.0):
        px, py = iso_raw(x, y, z)
        self.mark(px, py)
        return (px, py)

    def bbox(self):
        return (self.min_x - PAD, self.min_y - PAD,
                self.max_x - self.min_x + PAD * 2,
                self.max_y - self.min_y + PAD * 2)


def poly(points, fill, edge=None, width=1.0):
    pts = " ".join(f"{px:.1f},{py:.1f}" for px, py in points)
    st = f' stroke="{edge}" stroke-width="{width}" stroke-linejoin="round"' if edge else ""
    return f'<polygon points="{pts}" fill="{fill}"{st}/>'


def box(d, x, y, z, w, dp, h, colors, edge=True):
    """월드 좌표계 직육면체. 보이는 세 면만 그린다. edge 는 같은 색끼리 맞닿을 때
    서로 갈라 보이도록 면 색을 어둡게 한 얇은 선이다 — 검은 윤곽선이 아니다."""
    top, right, left = colors
    e = (lambda c: shade(c, 0.86)) if edge else (lambda c: None)
    d.items.append(poly([d.iso(x + w, y, z), d.iso(x + w, y + dp, z),
                         d.iso(x + w, y + dp, z + h), d.iso(x + w, y, z + h)],
                        right, e(right)))
    d.items.append(poly([d.iso(x, y + dp, z), d.iso(x + w, y + dp, z),
                         d.iso(x + w, y + dp, z + h), d.iso(x, y + dp, z + h)],
                        left, e(left)))
    d.items.append(poly([d.iso(x, y, z + h), d.iso(x + w, y, z + h),
                         d.iso(x + w, y + dp, z + h), d.iso(x, y + dp, z + h)],
                        top, e(top)))


def face_top(d, x, y, z, w, dp, fill, edge=None):
    d.items.append(poly([d.iso(x, y, z), d.iso(x + w, y, z),
                         d.iso(x + w, y + dp, z), d.iso(x, y + dp, z)], fill, edge))


def face_xy(d, x, y0, y1, z0, z1, fill, edge=None):
    """x = const 인 세로면 (오른쪽을 향한 면)."""
    d.items.append(poly([d.iso(x, y0, z0), d.iso(x, y1, z0),
                         d.iso(x, y1, z1), d.iso(x, y0, z1)], fill, edge))


def face_yx(d, y, x0, x1, z0, z1, fill, edge=None):
    """y = const 인 세로면 (왼쪽을 향한 면)."""
    d.items.append(poly([d.iso(x0, y, z0), d.iso(x1, y, z0),
                         d.iso(x1, y, z1), d.iso(x0, y, z1)], fill, edge))


def seg(d, p0, p1, color, width=1.4):
    d.mark(*p0)
    d.mark(*p1)
    d.items.append(f'<line x1="{p0[0]:.1f}" y1="{p0[1]:.1f}" x2="{p1[0]:.1f}" '
                   f'y2="{p1[1]:.1f}" stroke="{color}" stroke-width="{width}" '
                   'stroke-linecap="round"/>')


def disc(d, x, y, z, rx, fill):
    """바닥 받침. 아이소메트릭에서 원은 가로:세로 = 1:0.5 인 타원이 된다."""
    px, py = iso_raw(x, y, z)
    d.mark(px - rx, py - rx * 0.5)
    d.mark(px + rx, py + rx * 0.5)
    d.items.append(f'<ellipse cx="{px:.1f}" cy="{py:.1f}" rx="{rx:.1f}" '
                   f'ry="{rx * 0.5:.1f}" fill="{fill}"/>')


# ── 면 위에 그림 얹기 ─────────────────────────────────────────────────────
def _plane_matrix(u_screen, v_screen, origin, iw, ih, size_u, size_v):
    a = u_screen[0] * size_u * UNIT / iw
    b = u_screen[1] * size_u * UNIT / iw
    c = v_screen[0] * size_v * UNIT / ih
    dd = v_screen[1] * size_v * UNIT / ih
    return f'matrix({a:.4f} {b:.4f} {c:.4f} {dd:.4f} {origin[0]:.2f} {origin[1]:.2f})'


def _text_matrix(u_screen, v_screen, origin):
    """면 위에 글자를 눕히는 행렬. 아이콘용과 달리 배율을 곱하지 않는다 —
    font-size 가 이미 SVG 단위라 여기서 또 곱하면 글자가 장면을 덮는다."""
    return (f'matrix({u_screen[0]:.4f} {u_screen[1]:.4f} {v_screen[0]:.4f} '
            f'{v_screen[1]:.4f} {origin[0]:.2f} {origin[1]:.2f})')


def text_on_left(d, x, y, z, size, weight, spacing, fill, label):
    """y = const 인 면(왼쪽을 향한 면)에 글자를 눕힌다. 가로축은 +x, 세로축은 -z."""
    origin = iso_raw(x, y, z)
    m = _text_matrix((COS30, 0.5), (0.0, 1.0), origin)
    # 기울어진 글자의 경계는 대충 잡아 둔다 — 잘리지만 않으면 된다.
    d.mark(origin[0], origin[1] - size)
    d.mark(origin[0] + size * len(label) * 0.75, origin[1] + size * 0.4)
    d.items.append(f'<text transform="{m}" x="0" y="0" font-size="{size}" '
                   f'font-weight="{weight}" letter-spacing="{spacing}" '
                   f'font-family="{FONT}" fill="{fill}">{label}</text>')


def text_on_top(d, cx, cy, z, size, weight, spacing, fill, label):
    """윗면에 글자를 눕힌다. 가로축은 +x, 세로축은 +y — 즉 "아래" 가 화면에서
    왼쪽 아래로 간다. 가운데 맞춤이라 기준점이 글자의 한가운데다."""
    origin = iso_raw(cx, cy, z)
    m = _text_matrix((COS30, 0.5), (-COS30, 0.5), origin)
    half = size * len(label) * 0.42
    for ox, oy in ((-half, -size * 0.6), (half, size * 0.6)):
        d.mark(origin[0] + ox, origin[1] + oy)
    d.items.append(f'<text transform="{m}" x="0" y="0" font-size="{size}" '
                   f'font-weight="{weight}" letter-spacing="{spacing}" '
                   f'text-anchor="middle" font-family="{FONT}" fill="{fill}">{label}</text>')


def art_on_top(d, cx, cy, z, size, body, iw, ih, fill=None):
    """윗면에 눕혀 인쇄한다. 세워 두면 상자에서 떠 보인다."""
    origin = iso_raw(cx - size / 2, cy - size / 2, z)
    m = _plane_matrix((COS30, 0.5), (-COS30, 0.5), origin, iw, ih, size, size)
    f = f' fill="{fill}"' if fill else ""
    d.items.append(f'<g{f} transform="{m}">{body}</g>')


def art_on_left(d, cx, y, z_bottom, size, body, iw, ih, fill=None):
    """y = const 인 면(왼쪽을 향한 면)에 인쇄한다. 가로축은 +x, 세로축은 -z."""
    h = size * ih / iw
    origin = iso_raw(cx - size / 2, y, z_bottom + h)
    m = _plane_matrix((COS30, 0.5), (0.0, 1.0), origin, iw, ih, size, h)
    f = f' fill="{fill}"' if fill else ""
    d.items.append(f'<g{f} transform="{m}">{body}</g>')


def icon_color(slug):
    """아이콘 파일에 박힌 브랜드 색. 색표를 따로 두면 아이콘과 갈라진다."""
    src = (ICON_DIR / f"{slug}.svg").read_text()
    m = re.search(r'fill="(#[0-9A-Fa-f]{6})"', src)
    if not m:
        raise SystemExit(f"{slug}.svg 에 브랜드 색이 없다")
    return m.group(1).lower()


def load_icon(slug):
    """제품이 쓰는 도구 아이콘을 읽어 안쪽 마크업과 viewBox 크기를 돌려준다."""
    src = (ICON_DIR / f"{slug}.svg").read_text()
    m = re.search(r'viewBox="0 0 ([\d.]+) ([\d.]+)"', src)
    if not m:
        raise SystemExit(f"{slug}.svg 에 viewBox 가 없다")
    inner = src[src.index(">") + 1:src.rindex("</svg>")]
    return re.sub(r"<title>.*?</title>", "", inner, flags=re.S).strip(), \
        float(m.group(1)), float(m.group(2))


def _mark_strokes(src, name):
    """기하 파일의 조각 목록을 [(경로, 밝은 바탕 색)] 로 읽는다."""
    block = re.search(name + r"[^=]*= \[(.*?)\n\]", src, re.S)
    if not block:
        raise SystemExit(f"mark-geometry.generated.ts 에서 {name} 을 찾지 못했다")
    return re.findall(r"\['([^']+)', '(#[0-9a-fA-F]{6})', '#[0-9a-fA-F]{6}'\]", block.group(1))


def nullus_mark(colored=True):
    """제품 로고(세잎 매듭).

    colored=True 면 조각별 색을 그대로 살린다 — 로고는 한 가닥이 파랑에서 보라,
    청록으로 흐르는 것이 정체성이라 단색으로 뭉개면 다른 마크가 된다.
    아래 가닥(MARK_SEGMENTS)을 먼저 깔고 위 가닥(MARK_OVERS)을 덮으면 마스크 없이
    엮인 모양이 나온다 — 아래 가닥은 교차점이 이미 비워져 있다."""
    src = MARK_GEOMETRY.read_text()
    stroke = re.search(r"MARK_STROKE = ([\d.]+)", src)
    if not stroke:
        raise SystemExit("mark-geometry.generated.ts 에서 굵기를 찾지 못했다")
    sw = stroke.group(1)

    if not colored:
        path = re.search(r"MARK_WHOLE = '([^']+)'", src)
        if not path:
            raise SystemExit("mark-geometry.generated.ts 에서 로고 경로를 찾지 못했다")
        return (f'<path d="{path.group(1)}" fill="none" stroke="currentColor" '
                f'stroke-width="{sw}" stroke-linecap="round"/>'), 32.0, 32.0

    parts = []
    for name in ("MARK_SEGMENTS", "MARK_OVERS"):
        for d_, color in _mark_strokes(src, name):
            parts.append(f'<path d="{d_}" fill="none" stroke="{color}" '
                         f'stroke-width="{sw}" stroke-linecap="round"/>')
    if not parts:
        raise SystemExit("로고 조각을 하나도 읽지 못했다")
    return "".join(parts), 32.0, 32.0


def app_mark(name):
    """React·Go·Spring 은 도구 카탈로그에 없어 직접 그린다. 24x24 기준."""
    if name == "react":
        rings = "".join(
            f'<ellipse cx="12" cy="12" rx="11" ry="4.2" fill="none" '
            f'stroke="currentColor" stroke-width="1.5" '
            f'transform="rotate({a} 12 12)"/>' for a in (0, 60, 120))
        return rings + '<circle cx="12" cy="12" r="2.3" fill="currentColor"/>', 24.0, 24.0
    if name == "go":
        return ('<text x="12" y="12" font-size="15" font-weight="800" '
                f'font-family="{FONT}" fill="currentColor" text-anchor="middle" '
                'dominant-baseline="central" letter-spacing="-0.5">Go</text>'), 24.0, 24.0
    if name == "spring":
        # 잎 하나. 스프링을 뜻하는 초록 잎사귀다.
        return ('<path d="M12 2.5C5.5 7 4.5 15 12 21.5C19.5 15 18.5 7 12 2.5Z" '
                'fill="currentColor"/>'
                '<path d="M12 5.5V19" stroke="#ffffff" stroke-width="1.1" '
                'stroke-linecap="round" opacity="0.55"/>'), 24.0, 24.0
    raise SystemExit(f"모르는 마크: {name}")


# ── 정지 장면 ─────────────────────────────────────────────────────────────
def belt_legs(d, points, z_top):
    """컨베이어 다리. 받침판 → 기둥 → 프레임에 물리는 머리판.

    프레임 밑이 1.3 밖에 안 돼 기둥만 세우면 원반 위에 꽂힌 그루터기로 보인다.
    위아래를 판으로 받쳐야 벨트를 떠받치는 다리로 읽힌다."""
    for x, y in points:
        disc(d, x, y, 0.0, 15.0, shade(LEG[2], 0.9))
        box(d, x - 0.45, y - 0.45, 0.0, 0.9, 0.9, 0.13, shade_all(LEG, 0.82))
        box(d, x - 0.21, y - 0.21, 0.13, 0.42, 0.42, z_top - 0.31, LEG)
        box(d, x - 0.39, y - 0.39, z_top - 0.18, 0.78, 0.78, 0.18,
            shade_all(LEG, 0.92))


def conveyor_along_y(d, y0, y1, ):
    """입구 벨트. x 는 폭, y 를 따라 뻗는다."""
    box(d, -HALF, y1, FRAME_BOT, BELT_W, y0 - y1, BELT_TOP - FRAME_BOT, RAIL)
    face_top(d, -HALF, y1, BELT_TOP, BELT_W, y0 - y1, BELT[0])


def conveyor_along_x(d, x0, x1):
    """출구 벨트. y 는 폭, x 를 따라 뻗는다."""
    box(d, x0, -HALF, FRAME_BOT, x1 - x0, BELT_W, BELT_TOP - FRAME_BOT, RAIL)
    face_top(d, x0, -HALF, BELT_TOP, x1 - x0, BELT_W, BELT[0])


def scanner(d, x, y, facing_x):
    """벨트 옆의 판독기. 참고 이미지의 라인 설비 느낌을 준다."""
    disc(d, x, y, 0.0, 11.0, shade(LEG[2], 0.9))
    box(d, x - 0.22, y - 0.22, 0.0, 0.44, 0.44, 1.5, LEG)
    box(d, x - 0.55, y - 0.5, 1.5, 1.1, 1.0, 0.85, SCAN)
    # 화면 쪽을 향한 표시창
    if facing_x:
        face_xy(d, x + 0.55, y - 0.34, y + 0.34, 1.72, 2.24, "#2b3440")
    else:
        face_yx(d, y + 0.5, x - 0.34, x + 0.34, 1.72, 2.24, "#2b3440")


def beacon(d, pos, line):
    """벨트 프레임에 붙는 표시등. 깜빡이는 알은 CSS 가 얹으므로 등집만 그린다."""
    dark = (shade(LEG[2], 0.62),) * 3
    pad = LAMP / 2 + 0.11
    if line == "in":
        box(d, HALF - 0.06, pos - pad, BEACON_Z - 0.11, 0.16, pad * 2,
            LAMP + 0.22, dark)
    else:
        box(d, pos - pad, HALF - 0.06, BEACON_Z - 0.11, pad * 2, 0.16,
            LAMP + 0.22, dark)


def kiosk_screen(kind):
    """화면(어두운 면)의 자리와 크기를 월드 좌표로 돌려준다.
    그림과 CSS 가 같은 값을 봐야 화면 층이 모니터 밖으로 삐져나가지 않는다."""
    if kind == "monitor":
        return dict(half_w=MON_W / 2 - MON_INSET, dy=MON_D / 2,
                    z0=MON_POST + MON_INSET, z1=MON_POST + MON_H - MON_INSET)
    return dict(half_w=CON_SCR_W / 2 - CON_INSET, dy=CON_SCR_D / 2,
                z0=CON_SCR_Z + CON_INSET, z1=CON_SCR_Z + CON_SCR_H - CON_INSET)


def kiosk(d, x, y, kind):
    """모니터링 키오스크. 화면 내용(그래프·로그)은 CSS 가 얹는다."""
    if kind == "monitor":
        disc(d, x, y, 0.0, 13.0, shade(LEG[2], 0.9))
        box(d, x - 0.2, y - 0.2, 0.0, 0.4, 0.4, MON_POST, LEG)
        box(d, x - MON_W / 2, y - MON_D / 2, MON_POST, MON_W, MON_D, MON_H, SCAN)
    else:
        box(d, x - CON_W / 2, y - CON_D / 2, 0.0, CON_W, CON_D, CON_H, SCAN)
        # 키보드 선반 — 앞으로 조금 내민다
        box(d, x - CON_W / 2 + 0.1, y + CON_D / 2 - 0.15, CON_H - 0.02,
            CON_W - 0.2, 0.7, 0.14, LEG)
        box(d, x - CON_SCR_W / 2, y - CON_SCR_D / 2, CON_SCR_Z,
            CON_SCR_W, CON_SCR_D, CON_SCR_H, SCAN)
        # 화면과 캐비닛을 잇는 목
        box(d, x - 0.16, y - 0.16, CON_H, 0.32, 0.32, CON_SCR_Z - CON_H, LEG)

    # 화면 자리를 어둡게 파 둔다. CSS 층이 못 붙어도 모니터로는 보인다.
    g = kiosk_screen(kind)
    face_yx(d, y + g["dy"] + 0.01, x - g["half_w"], x + g["half_w"],
            g["z0"], g["z1"], "#141b24")


# 개발자 모니터 화면. 마지막 줄은 CSS 가 한 글자씩 쳐 넣으므로 그림과 CSS 가
# 같은 값을 봐야 한다.
DEV_SCR_HW = 1.06                              # 화면 반폭 (월드 y)
DEV_LINE_H = 0.085
def DEV_LINE_Z(i):
    return DESK_H + 1.46 - i * 0.175
DEV_TYPE_LINE = 5                              # 치고 있는 줄
DEV_TYPE_W = 0.62                              # 다 쳤을 때의 값 길이. 40px 남짓한 화면이라
                                               # 이만큼은 돼야 켜지고 꺼지는 게 보인다.


def dev_type_anchor(x, y):
    """치고 있는 값의 왼쪽 위 모서리. CSS 층이 여기에 붙는다."""
    hx = DESK_DEP / 2
    sx = (x - hx + 0.3) + 0.13
    left = y + DEV_SCR_HW
    col0 = left - 0.20
    ind, kw = 0.16, 0.30
    return sx + 0.02, col0 - ind - kw - 0.08, DEV_LINE_Z(DEV_TYPE_LINE) + DEV_LINE_H


DEV_SKIN = ("#f0c49b", "#dcae86", "#c99a73")
DEV_WEAR = ("#4a78ec", "#3d67dd", "#3357c7")
DEV_HAIR = ("#3a3f4a", "#31353f", "#282c34")


def developer(d, x, y):
    """책상 앞에 앉아 화면을 보며 스택을 짜는 사람. 벨트를 마주 보도록 -x 쪽을
    향해 앉는다. 뒷모습이라 화면을 가리지 않는다 — 앞모습으로 두면 사람이
    모니터를 등지고 앉은 꼴이 된다."""
    hx, hy = DESK_DEP / 2, DESK_LEN / 2
    top = DESK_H - 0.12

    # 책상 — 긴 변이 y 축
    for fy in (y - hy + 0.15, y + hy - 0.3):
        box(d, x - hx + 0.18, fy, 0.0, DESK_DEP - 0.36, 0.15, top, SCAN)
    box(d, x - hx, y - hy, top, DESK_DEP, DESK_LEN, 0.12, RAIL)

    # 모니터 — 책상 안쪽(-x)에 두고 화면은 +x 를 본다
    mx = x - hx + 0.3
    box(d, mx - 0.18, y - 0.2, DESK_H, 0.34, 0.4, 0.34, SCAN)
    box(d, mx - 0.14, y - 1.2, DESK_H + 0.34, 0.26, 2.4, 1.5, SCAN)
    sx = mx + 0.13
    face_xy(d, sx, y - 1.06, y + 1.06, DESK_H + 0.48, DESK_H + 1.7, "#182230")
    # 화면 속 — 스택 매니페스트를 YAML 로 고치는 중. 이 크기에서는 글자가 뭉개지므로
    # 들여쓰기와 키/값 두 색으로만 코드라는 걸 알린다.
    # 이 면은 x 가 일정한 면이라 y 가 커질수록 화면 왼쪽이다. 줄 번호 여백을 왼쪽에
    # 두고 들여쓰기가 오른쪽으로 가려면 열 번호를 y 에서 빼야 한다.
    left = y + DEV_SCR_HW
    face_xy(d, sx + 0.01, left - 0.11, left - 0.06, DESK_H + 0.6, DESK_H + 1.6,
            "#2a3646")
    col0 = left - 0.20
    lines = [(0.00, 0.42, 0.30),      # apiVersion: v1
             (0.00, 0.26, 0.50),      # kind: Deployment
             (0.00, 0.38, 0.00),      # metadata:
             (0.16, 0.28, 0.38),      #   name: nullus
             (0.00, 0.22, 0.00),      # spec:
             (0.16, 0.30, 0.00)]      #   - image: ...  ← 값은 CSS 가 쳐 넣는다
    for i, (ind, kw, vw) in enumerate(lines):
        zz = DEV_LINE_Z(i)
        face_xy(d, sx + 0.01, col0 - ind - kw, col0 - ind, zz, zz + DEV_LINE_H,
                "#7fd4c1")
        if vw:
            v0 = col0 - ind - kw - 0.08
            face_xy(d, sx + 0.01, v0 - vw, v0, zz, zz + DEV_LINE_H, "#c9d6e4")

    # 키보드 — 앞쪽(+y) 손 아래. 뒤쪽에 두면 머리에 가려 안 보인다.
    # 사람보다 먼저 그려야 손이 자판 위에 얹힌다.
    kb0, kb1 = y + 0.08, y + 1.02
    box(d, x + 0.24, kb0, DESK_H, 0.62, kb1 - kb0, 0.07, shade_all(RAIL, 1.06))
    face_top(d, x + 0.28, kb0 + 0.05, DESK_H + 0.07, 0.54, (kb1 - kb0) - 0.1, "#3c4655")
    for i in range(3):
        face_top(d, x + 0.32, kb0 + 0.11 + i * 0.26, DESK_H + 0.072, 0.46, 0.11,
                 "#5b6779")

    # 마우스와 패드 — 뒤쪽(-y) 손 아래. 패드를 어둡게 깔아야 밝은 마우스가
    # 상판과 같은 회색에 묻히지 않는다.
    face_top(d, x + 0.24, y - 1.02, DESK_H + 0.002, 0.64, 0.7, "#3c4655")
    box(d, x + 0.38, y - 0.88, DESK_H, 0.34, 0.36, 0.13, shade_all(RAIL, 1.22))

    # 의자
    px = x + hx + 0.85
    box(d, px - 0.12, y - 0.12, 0.0, 0.24, 0.24, 0.62, LEG)
    box(d, px - 0.5, y - 0.5, 0.62, 1.0, 1.0, 0.14, shade_all(LEG, 0.9))

    # 사람 — 뒷모습. 팔은 상판 높이에 두어야 책상에 가려지지 않는다.
    box(d, px - 0.34, y - 0.44, 0.76, 0.68, 0.88, 0.92, DEV_WEAR)          # 몸통
    for ay in (y - 0.62, y + 0.4):                                          # 팔
        box(d, px - 1.05, ay, top + 0.02, 0.95, 0.22, 0.2, DEV_WEAR)
    for ay in (y - 0.62, y + 0.4):                                          # 손
        box(d, px - 1.28, ay, top + 0.02, 0.23, 0.22, 0.18, DEV_SKIN)
    box(d, px - 0.2, y - 0.2, 1.68, 0.4, 0.4, 0.18, DEV_SKIN)               # 목
    box(d, px - 0.3, y - 0.32, 1.86, 0.6, 0.64, 0.58, DEV_HAIR)            # 머리


def gate_body(d):
    """Nullus 게이트. 스택이 들어가 제품으로 나오는 자리다.
    민무늬 육면체는 밋밋해서, 받침대·모서리 기둥·판 이음매·윗단 테두리로 설비의
    결을 넣는다."""
    g, z0, z1 = GATE_HALF, GATE_Z0, GATE_Z1
    pill, cap = 0.26, 0.5

    # 받침대 — 몸통보다 넓어 바닥에 앉은 느낌을 준다
    box(d, -g - 0.26, -g - 0.26, 0.0, (g + 0.26) * 2, (g + 0.26) * 2, z0 + 0.4, RAIL)
    box(d, -g, -g, z0, g * 2, g * 2, z1 - z0, MACH)

    # 판 이음매 — 보이는 두 면에 가로선 몇 줄
    for t in (0.36, 0.64):
        zz = z0 + (z1 - z0) * t
        seg(d, iso_raw(g + 0.02, -g, zz), iso_raw(g + 0.02, g, zz), shade(MACH[1], 0.9), 1.1)
        seg(d, iso_raw(-g, g + 0.02, zz), iso_raw(g, g + 0.02, zz), shade(MACH[2], 0.9), 1.1)

    # 윗단 테두리
    face_xy(d, g + 0.03, -g, g, z1 - cap, z1, shade(MACH[1], 0.93))
    face_yx(d, g + 0.03, -g, g, z1 - cap, z1, shade(MACH[2], 0.93))

    # 보이는 세 세로 모서리에 기둥
    for cx, cy in ((g - pill, g - pill), (g - pill, -g), (-g, g - pill)):
        box(d, cx, cy, z0, pill, pill, z1 - z0, RAIL)

    # 개구부 문틀 — 한 겹 밝게 파 두면 문으로 읽힌다
    fr = 0.22
    face_yx(d, g + 0.04, -HALF - fr, HALF + fr, MOUTH_Z0, MOUTH_Z1 + fr, shade(MACH[2], 1.07))
    face_xy(d, g + 0.04, -HALF - fr, HALF + fr, MOUTH_Z0, MOUTH_Z1 + fr, shade(MACH[1], 1.07))

    # 개구부 — 들어가는 쪽(+y 면)과 나오는 쪽(+x 면).
    # 통과하는 스택보다 높아야 하다 보니 검은 면이 넓어진다. 참고한 모형처럼
    # 스트립 커튼을 넣어 무게를 덜고 "안으로 들어간다"는 느낌을 준다.
    face_yx(d, g + 0.05, -HALF, HALF, MOUTH_Z0, MOUTH_Z1, MOUTH[2])
    face_xy(d, g + 0.05, -HALF, HALF, MOUTH_Z0, MOUTH_Z1, MOUTH[1])
    # 스트립 커튼은 여닫혀야 하므로 여기 그리지 않는다 — CSS 가 DOM 으로 얹는다
    # (자리는 --nl-curtain-* 로 넘긴다).

    # 나오는 쪽 면의 띠. 글자는 넣지 않는다 — 이 면은 가로축이 +y 라 뒤집혀 읽힌다.
    face_xy(d, g + 0.06, -g, g, MOUTH_Z1 + 0.6, MOUTH_Z1 + 1.3, NULLUS_BLUE)
    # 상태 표시등 줄
    for i in range(4):
        ly = -g + 0.9 + i * 0.6
        face_xy(d, g + 0.07, ly, ly + 0.32, MOUTH_Z1 + 1.62, MOUTH_Z1 + 1.9,
                "#7fd4a8" if i else "#ffc46b")


# 윗면 로고. 마크를 크게 두고 이름을 그 아래에 놓는다 — 가로로 나란히 두면
# 마름모인 윗면에서 폭을 다 써 버려 마크를 키울 수 없다.
# 글자 폭은 브라우저에서 재 둔 값이다. 눈대중으로 두면 모서리를 넘어간다.
BRAND_MARK = 2.5
BRAND_FONT = 17
BRAND_TEXT_W = 3.3      # "NULLUS" @ font-size 17, letter-spacing 1.4
BRAND_TEXT_H = 0.7      # 글자가 기준선 위로 올라가는 높이
# 화면에서 곧장 아래로 내려가려면 x 와 y 를 같이 더해야 한다 —
# iso(1,1,0) = (0, 1) 이라 그 방향만이 순수 세로다. 한쪽만 더하면 대각으로 밀린다.
# 둘 사이는 마크 반지름 + 글자 높이보다 넉넉히 벌려야 겹치지 않는다.
BRAND_MARK_POS = (-1.15, -1.15)
BRAND_TEXT_POS = (1.0, 1.0)


def gate_brand(d):
    """게이트 윗면의 로고. 옆면에 붙이면 들어가는 스택이 그 앞을 지나며 가린다 —
    윗면은 무엇에도 가리지 않는다.

    윗면은 월드에서 정사각형이지만 화면에서는 마름모다. 그래서 자리는 화면이
    아니라 월드 좌표로 잡고, 묶음 전체를 면 한가운데에 놓는다."""
    g, z = GATE_HALF, GATE_Z1
    mx, my = BRAND_MARK_POS
    tx, ty = BRAND_TEXT_POS
    for cx, cy, half_w, half_d in ((mx, my, BRAND_MARK / 2, BRAND_MARK / 2),
                                   (tx, ty, BRAND_TEXT_W / 2, 0.4)):
        assert abs(cx) + half_w < g and abs(cy) + half_d < g, "로고가 윗면을 넘는다"

    # 마크 아랫변과 글자 윗변이 겹치지 않는지 (둘 다 월드 y 로 잰다)
    assert my + BRAND_MARK / 2 < ty - BRAND_TEXT_H, "마크와 이름이 겹친다"

    body, iw, ih = nullus_mark(colored=True)
    art_on_top(d, mx, my, z + 0.01, BRAND_MARK, body, iw, ih)
    text_on_top(d, tx, ty, z + 0.01, BRAND_FONT, 800, 1.4, "#3d4757", "NULLUS")


def build_scene():
    back, front = Draw(), Draw()

    gate_body(back)

    conveyor_along_y(back, IN_Y_FAR, GATE_HALF)
    conveyor_along_x(back, GATE_HALF, OUT_X_FAR)
    belt_legs(back, [(0, IN_Y_FAR - 1.6), (0, IN_Y_FAR * 0.55), (0, GATE_HALF + 1.6)], FRAME_BOT)
    belt_legs(back, [(GATE_HALF + 1.6, 0), (OUT_X_FAR * 0.55, 0), (OUT_X_FAR - 1.6, 0)], FRAME_BOT)

    # 크레인 구조물. 케이블과 매달린 짐은 CSS 가 움직이므로 여기에 없다.
    for cy in IN_CRANE_YS:
        crane_structure(back, -JIB_BACK, cy, along_x=False)
    for cx in OUT_CRANE_XS:
        crane_structure(back, cx, -JIB_BACK, along_x=True)

    gate_brand(front)

    # 맨 앞의 설비 — 흐르는 상자가 이 뒤로 지나가며 깊이가 생긴다.
    scanner(front, HALF + 1.5, IN_Y_FAR * 0.42, facing_x=True)
    for pos, line, _hue, _delay in BEACONS:
        beacon(front, pos, line)
    for kx, ky, kind in KIOSKS:
        kiosk(front, kx, ky, kind)
    developer(front, *DEV_POS)

    # 흐르는 조각이 나가는 끝까지 화면에 남도록 경계에 넣어 둔다. 정지 그림만으로
    # 재면 완성된 제품이 마지막에 잘린다.
    for zz in (0.0, PAL_H + NUL_H + APP_H + 0.9):
        front.mark(*iso_raw(OUT_END + PAL_OUT_W / 2 + 1.4, -NUL_D / 2, BELT_TOP + zz))

    for d in (back, front):
        d.mark(back.min_x, back.min_y)
        d.mark(back.max_x, back.max_y)
        d.mark(front.min_x, front.min_y)
        d.mark(front.max_x, front.max_y)
    return back, front


def crane_structure(d, x, y, along_x):
    """지브 크레인. 마스트는 벨트 뒤에 서고 팔만 벨트 위로 뻗는다.
    갠트리로 두면 앞다리가 화물 앞을 가로질러 그림이 읽히지 않는다."""
    t = LEG_T
    disc(d, x + t / 2, y + t / 2, 0.0, 15.0, shade(LEG[2], 0.9))
    box(d, x, y, 0.0, t, t, CRANE_TOP, RAIL)
    if along_x:
        reach = APP_SLOT_Y - y
        seg(d, iso_raw(x + t / 2, y, CRANE_TOP + 1.2),
            iso_raw(x + t / 2, y + reach - 0.3, CRANE_TOP + 0.4), shade(RAIL[2], 0.8), 1.3)
        box(d, x, y, CRANE_TOP, t, t, 1.2, RAIL)
        box(d, x, y, CRANE_TOP, t, reach + t, 0.42, RAIL)
        box(d, x - 0.14, y + reach - 0.5, CRANE_TOP - 0.4, t + 0.28, 0.9, 0.4, LEG)
    else:
        reach = -x
        seg(d, iso_raw(x, y + t / 2, CRANE_TOP + 1.2),
            iso_raw(x + reach - 0.3, y + t / 2, CRANE_TOP + 0.4), shade(RAIL[2], 0.8), 1.3)
        box(d, x, y, CRANE_TOP, t, t, 1.2, RAIL)
        box(d, x, y, CRANE_TOP, reach + t, t, 0.42, RAIL)
        box(d, x + reach - 0.5, y - 0.14, CRANE_TOP - 0.4, 0.9, t + 0.28, 0.4, LEG)


# ── 움직이는 조각 ─────────────────────────────────────────────────────────
def svg_doc(d, note):
    bx, by, bw, bh = d.bbox()
    return "\n".join([
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{bx:.1f} {by:.1f} {bw:.1f} {bh:.1f}" '
        f'width="{bw:.0f}" height="{bh:.0f}" role="presentation" aria-hidden="true">',
        "<!-- 자동 생성 — scripts/emit-keycloak-theme-art.py",
        f"     {note} -->",
        f'<g font-family="{FONT}">',
        *d.items,
        "</g>",
        "</svg>",
        "",
    ])


def sprite(draw_fn, note):
    """조각 하나를 제 크기에 딱 맞는 SVG 로 뽑고, 기준점(월드 원점)에서 왼쪽 위
    모서리까지의 거리를 함께 돌려준다. CSS 는 그 거리로 조각을 앉힌다."""
    d = Draw()
    draw_fn(d)
    bx, by, bw, bh = d.bbox()
    return {"svg": svg_doc(d, note), "dx": bx, "dy": by, "w": bw, "h": bh}


def container(d, w, dp, h, color, art=None, art_size=0.0):
    """해상 컨테이너. 기준점은 바닥 한가운데다.
    세로 골판과 위아래 테두리로 강판 느낌을 낸다 — 민무늬 육면체는 그냥 상자다."""
    x, y = -w / 2, -dp / 2
    body = (shade(color, 1.14), color, shade(color, 0.80))
    box(d, x, y, 0.0, w, dp, h, body, edge=False)

    rail = 0.13                      # 위아래 테두리(코너 캐스팅) 두께
    rib_x = shade(color, 0.90)
    rib_y = shade(color, 0.72)

    # 세로 골판. 면마다 자기 색보다 한 단 어둡게 그어 강판 주름으로 읽히게 한다.
    n = max(4, int(dp / 0.34))
    for i in range(1, n):
        t = y + dp * i / n
        seg(d, iso_raw(x + w, t, rail), iso_raw(x + w, t, h - rail), rib_x, 1.0)
    n = max(4, int(w / 0.34))
    for i in range(1, n):
        t = x + w * i / n
        seg(d, iso_raw(t, y + dp, rail), iso_raw(t, y + dp, h - rail), rib_y, 1.0)

    # 위아래 테두리
    for z0, z1 in ((0.0, rail), (h - rail, h)):
        face_xy(d, x + w + 0.005, y, y + dp, z0, z1, shade(color, 0.86))
        face_yx(d, y + dp + 0.005, x, x + w, z0, z1, shade(color, 0.68))

    if art is not None:
        # 뚜껑에 옅은 판을 깔고 그 위에 마크 — 강판 색 위에 바로 얹으면 묻힌다.
        face_top(d, x + 0.26, y + 0.26, h + 0.01, w - 0.52, dp - 0.52,
                 tint(color, 0.10), shade(tint(color, 0.10), 0.93))
        b, iw, ih = art
        art_on_top(d, 0, 0, h + 0.02, art_size, b, iw, ih, shade(color, 0.68))


def pallet(d, w, dp):
    """벨트를 타는 노란 철판. 발 위에 상판을 얹은 모양이라야 "싣는 것" 으로 읽힌다.
    나뭇결 대신 가장자리 테두리와 미끄럼 방지 골로 철판임을 알린다."""
    x, y = -w / 2, -dp / 2
    foot = 0.45
    feet = max(3, int(w / 1.8))
    for i in range(feet):
        fx = x + 0.12 + (w - 0.24 - foot) * i / (feet - 1)
        box(d, fx, y + 0.18, 0.0, foot, dp - 0.36, PAL_H * 0.55, shade_all(PALLET, 0.8))
    box(d, x, y, PAL_H * 0.55, w, dp, PAL_H * 0.45, PALLET)
    # 테두리 — 접어 올린 가장자리
    face_top(d, x + 0.14, y + 0.12, PAL_H + 0.001, w - 0.28, dp - 0.24,
             shade(PALLET[0], 1.06))
    # 미끄럼 방지 골
    ribs = max(4, int(w / 0.62))
    for k in range(1, ribs):
        gx = x + 0.14 + (w - 0.28) * k / ribs
        seg(d, iso_raw(gx, y + 0.2, PAL_H + 0.002),
            iso_raw(gx, y + dp - 0.2, PAL_H + 0.002), PAL_RIB, 1.0)


def cicd_container(d):
    """게이트에서 나오는 완성된 스택. 조합된 CI/CD 파이프라인이다."""
    x, y = -NUL_W / 2, -NUL_D / 2
    container(d, NUL_W, NUL_D, NUL_H, NULLUS_BLUE)
    # 앞면(+y) 라벨판 — 골판 위에 바로 글자를 얹으면 읽히지 않는다.
    # 판과 글자 모두 컨테이너 한가운데에 둔다.
    #
    # 여기에는 제품 마크를 넣지 않는다. 바로 옆 게이트가 이미 NULLUS 이고, 한
    # 화면에 같은 마크가 여러 번 나오면 어느 것도 눈에 남지 않는다. 이 컨테이너가
    # 무엇인지는 "CI/CD" 라는 이름이 말한다.
    text_w = 2.05                            # "CI/CD" @ font-size 17 실측
    plate_w = text_w + 0.7
    px = -plate_w / 2
    face_yx(d, y + NUL_D + 0.02, px, px + plate_w, 0.5, NUL_H - 0.5,
            "#f4f6fa", shade("#f4f6fa", 0.9))
    text_on_left(d, px + 0.35, y + NUL_D + 0.03, 1.05, 17, 800, 0.6,
                 "#2b3a5a", "CI/CD")


# 집게. 벌어지는 폭(월드 x). 짐을 놓고 이만큼 벌렸다가 올라가며 다시 오므린다.
GRIP_OPEN = 0.55
# 가로대가 집게발 바깥으로 나온 몫. 가로대는 "다문 길이" 로 그리고 벌어질 때
# CSS 가 늘인다 — 벌어진 길이로 그려 두면 다물었을 때 양쪽이 허공에 남는다.
GRIP_BAR_CAP = 0.20
PAD_HALF = 0.24                               # 집게발이 컨테이너 옆면을 무는 반폭
BAR_THICK = 0.32                              # 가로대 굵기 (축과 직각 방향)
# 집게가 늘어서는 방향. 크레인 팔과 나란해야 한다 — 입구 크레인은 세계 x 를,
# 출구 크레인은 세계 y 를 따라 뻗는다(crane_structure 참고). 화면 가로인
# (1,-1) 대각선에 두면 두 발이 다 보이는 대신 장면에서 그 막대만 축을 벗어나
# 아이소메트릭이 깨진다.
AXIS = {"in": (1, 0), "out": (0, 1)}


def grip_bar_open(w, dp):
    """벌어졌을 때 가로대가 늘어나는 배율.

    가로대는 두 발을 잇는 대각선 위에 있고 그 방향이 화면에서 순수 가로라,
    scaleX 하나면 각도가 흐트러지지 않고 길이만 는다. 배율은 화면 길이의 비인데
    화면 길이가 (x - y) * cos30 이라 cos30 이 약분돼 세계 좌표 비로 계산된다."""
    base = w / 2 + dp / 2 + 0.12 + 2 * GRIP_BAR_CAP
    return (base + GRIP_OPEN * math.sqrt(2)) / base
def _grip_top(dp):
    """가로대 꼭대기가 컨테이너 위로 솟는 높이.

    가로대가 뚜껑을 가로지르지 않으려면, 같은 **화면 x** 에서 가로대가 뚜껑 뒷변
    보다 위에 있어야 한다. 세계 좌표로 비교하면 안 된다 — 가로대는 앞면 쪽(y 가
    큰 쪽)에 있어서 같은 세계 x 라도 화면에서는 오른쪽으로 밀리기 때문이다.
    가로대를 컨테이너 한가운데에 두면 깊이의 절반 남짓이면 된다
    (아래 _grip_clears 가 실제로 검사한다).
    매다는 몫(--nl-hang)에 이만큼을 더해야 조립체 꼭대기가 후크에 맞는다."""
    return dp / 2 + 0.52


def _lid_top_sy(w, dp, h, sx):
    """그 화면 x 에서 뚜껑의 가장 높은(화면상 위쪽) 점. 뚜껑 밖이면 None."""
    top = None
    u = sx / (COS30 * UNIT)                         # iso_raw 는 UNIT 까지 곱해 준다
    lx = u - dp / 2                                 # 뒤쪽 긴 변 y = -dp/2
    if -w / 2 <= lx <= w / 2:
        top = iso_raw(lx, -dp / 2, h)[1]
    ly = -w / 2 - u                                 # 뒤쪽 짧은 변 x = -w/2
    if -dp / 2 <= ly <= dp / 2:
        v = iso_raw(-w / 2, ly, h)[1]
        top = v if top is None else min(top, v)
    return top


def _member_clears(w, dp, h, p0, p1, z):
    """수평 부재 하나가 화면에서 뚜껑을 가로지르지 않는지 같은 화면 x 에서 잰다.

    세계 좌표로 비교하면 안 된다 — 부재는 컨테이너 위에 있어서 같은 세계 좌표라도
    화면에서는 위로 밀려 있기 때문이다. 화면에서 가장 낮은 모서리는 x·y 가 둘 다
    큰 쪽이므로 두께의 절반을 양쪽에 더해 잰다."""
    t = BAR_THICK / 2
    for i in range(41):
        u = i / 40
        px = p0[0] + (p1[0] - p0[0]) * u + t
        py = p0[1] + (p1[1] - p0[1]) * u + t
        pt = iso_raw(px, py, z)
        top = _lid_top_sy(w, dp, h, pt[0])
        if top is not None and pt[1] >= top:
            return False
    return True


def _grip_clears(w, dp, h, axis):
    """집게 틀(가로대 + 양쪽 꺾인 팔)이 뚜껑을 가로지르지 않는지."""
    zb = h + _grip_top(dp) - 0.22
    half = _bar_half(w, dp, axis, False)
    ends = (_along(axis, -half, 0.0), _along(axis, half, 0.0))
    if not _member_clears(w, dp, h, ends[0], ends[1], zb):
        return False
    for side in (-1, 1):
        p0, p1 = _cant_span(w, dp, axis, side)
        if not _member_clears(w, dp, h, p0, p1, zb):
            return False
    return True


def _grip_half(w, dp, axis):
    """축 방향으로 컨테이너의 절반 길이."""
    return w / 2 if axis[0] else dp / 2


def _grip_cross(w, dp, axis):
    """축과 직각 방향으로 컨테이너의 절반 길이 — 끝 가로보가 걸치는 폭."""
    return dp / 2 if axis[0] else w / 2


def _pad_spot(w, dp, axis, side):
    """이 끝이 무는 모서리. 화면에서 컨테이너의 좌·우 끝인 두 모서리
    (+x,-y) 와 (-x,+y) 다 — 거기라야 두 발이 다 실루엣 위로 나온다.

    축 방향 양 끝의 면 한가운데를 물면 뒤쪽 발이 컨테이너에 통째로 가리고,
    네 모서리를 다 물면 앞 모서리로 뻗는 팔이 뚜껑을 가로질러 로고를 덮는다.
    화면에서 위쪽(-y, -x)으로 뻗는 팔만 뚜껑 위를 지나지 않는다."""
    t = side if axis[0] else -side
    return t * w / 2, -t * dp / 2


def _pad_box(h, spot):
    """집게발 상자. 모서리를 한가운데 두고 안팎을 같게, 높이도 옆변 한가운데."""
    ph = h * 0.42
    return spot[0] - PAD_HALF, spot[1] - PAD_HALF, (h - ph) / 2, ph


def _pad_on_corner_mid(w, dp, h, axis, side):
    """집게발의 화면상 한가운데가 무는 모서리의 한가운데와 맞는지.

    실제로 그리는 상자의 화면 경계상자를 재서 확인한다 — 세계 좌표로 맞춰 놓아도
    투영에서 어긋나면 아래쪽이나 한쪽으로 치우쳐 보인다."""
    spot = _pad_spot(w, dp, axis, side)
    px, py, pz, ph = _pad_box(h, spot)
    xs, ys = [], []
    for cx in (px, px + PAD_HALF * 2):
        for cy in (py, py + PAD_HALF * 2):
            for cz in (pz, pz + ph):
                sx_, sy_ = iso_raw(cx, cy, cz)
                xs.append(sx_)
                ys.append(sy_)
    mid = iso_raw(spot[0], spot[1], h / 2)
    if not (abs((min(xs) + max(xs)) / 2 - mid[0]) < 1e-9
            and abs((min(ys) + max(ys)) / 2 - mid[1]) < 1e-9):
        return False
    # 그리고 그 자리가 화면에서 컨테이너의 좌·우 끝이어야 한다. 축 방향 면
    # 한가운데로 옮기면 실루엣 안쪽이라 한쪽 발이 통째로 가린다.
    edge = max(iso_raw(cx, cy, 0)[0] for cx in (-w / 2, w / 2) for cy in (-dp / 2, dp / 2))
    return abs(abs(mid[0]) - edge) < 1e-9


def _bar_half(w, dp, axis, open_):
    """가로대 반길이. 벌어지면 집게발이 나간 만큼 늘어난다."""
    return _grip_half(w, dp, axis) + (GRIP_OPEN if open_ else 0.0) + GRIP_BAR_CAP


def grip_bar_open(w, dp, axis):
    """벌어졌을 때 가로대가 축 방향으로 늘어나는 배율."""
    return _bar_half(w, dp, axis, True) / _bar_half(w, dp, axis, False)


def bar_stretch_matrix(axis, k):
    """가로대의 축 방향만 k 배 늘리는 CSS matrix().

    두 아이소메트릭 축은 화면에서 직각이 아니라(내적 -0.5) 회전+scaleX 로는 못
    만든다. 축 기저 B = [iso(x), iso(y)] 로 옮겨 한 축만 늘리고 되돌린다:
    M = B · diag · B⁻¹. 직각 방향은 그대로라 막대 굵기가 변하지 않는다."""
    c, hh = COS30, 0.5
    b = ((c, -c), (hh, hh))
    det = 2 * c * hh
    binv = ((hh / det, c / det), (-hh / det, c / det))
    dg = (k, 1.0) if axis[0] else (1.0, k)
    m = [[sum(b[r][i] * dg[i] * binv[i][col] for i in (0, 1)) for col in (0, 1)]
         for r in (0, 1)]
    return (f"matrix({m[0][0]:.4f}, {m[1][0]:.4f}, {m[0][1]:.4f}, "
            f"{m[1][1]:.4f}, 0, 0)")


def _along(axis, along, across):
    """축 방향 길이와 직각 방향 길이를 (x, y) 크기로 옮긴다."""
    return (along, across) if axis[0] else (across, along)


def grip_bar(d, w, dp, h, axis):
    """집게를 매단 가로대. 크레인 팔과 같은 축을 따라 뻗는다."""
    gt = _grip_top(dp)
    half = _bar_half(w, dp, axis, False)
    bw, bd = _along(axis, half * 2, BAR_THICK)
    box(d, -bw / 2, -bd / 2, h + gt - 0.22, bw, bd, 0.22, JAW)
    # 케이블이 물리는 자리
    box(d, -0.13, -0.13, h + gt, 0.26, 0.26, 0.18, shade_all(JAW, 1.18))


def _cant_span(w, dp, axis, side):
    """가로대 끝에서 무는 모서리까지 꺾어 나가는 팔. 축과 직각으로만 뻗는다."""
    spot = _pad_spot(w, dp, axis, side)
    e = side * _grip_half(w, dp, axis)
    if axis[0]:
        return (e, min(0.0, spot[1])), (e, max(0.0, spot[1]))
    return (min(0.0, spot[0]), e), (max(0.0, spot[0]), e)


def grip_jaw(d, w, dp, h, axis, side):
    """가로대 한쪽 끝 — 꺾인 팔 + 다리 + 발. side -1 은 축의 뒤쪽이다."""
    gt = _grip_top(dp)
    arm = JAW
    pad = shade_all(JAW, 0.86)
    zb = h + gt - 0.22
    (x0, y0), (x1, y1) = _cant_span(w, dp, axis, side)
    t = BAR_THICK / 2
    box(d, x0 - t, y0 - t, zb, (x1 - x0) + t * 2, (y1 - y0) + t * 2, 0.22, arm)
    spot = _pad_spot(w, dp, axis, side)
    px, py, pz, ph = _pad_box(h, spot)
    # 다리 — 가늘게. 굵으면 바로 뒤 크레인 기둥과 한 덩어리로 보인다.
    a = 0.09
    box(d, spot[0] - a, spot[1] - a, pz + ph, a * 2, a * 2, zb - (pz + ph), arm)
    # 발 — 모서리를 문다
    box(d, px, py, pz, PAD_HALF * 2, PAD_HALF * 2, ph, pad)


def badge(d):
    """완성 표시."""
    px, py = iso_raw(0, 0, 0)
    d.mark(px - 17, py - 17)
    d.mark(px + 17, py + 17)
    d.items.append(f'<circle cx="{px:.1f}" cy="{py:.1f}" r="15" fill="{BADGE_GREEN}"/>')
    d.items.append(f'<path d="M{px - 6.5:.1f} {py:.1f} l4.5 4.5 l8 -8.5" fill="none" '
                   'stroke="#ffffff" stroke-width="2.8" stroke-linecap="round" '
                   'stroke-linejoin="round"/>')


def metric_tile():
    """키오스크 화면에 흐를 꺾은선 한 칸. 끝 높이를 시작과 같게 맞춰야 이어 붙여
    밀 때 이음매가 보이지 않는다."""
    w, h, n = 120.0, 60.0, 24
    ys = []
    for i in range(n + 1):
        t = i / n
        # 되풀이되는 파형. 마지막 점이 첫 점과 같아지도록 주기를 정수로 잡는다.
        v = (math.sin(t * 2 * math.pi) * 0.45
             + math.sin(t * 6 * math.pi) * 0.22
             + math.sin(t * 10 * math.pi) * 0.11)
        ys.append(h * 0.5 - v * h * 0.34)
    pts = " ".join(f"{i / n * w:.2f},{y:.2f}" for i, y in enumerate(ys))
    area = f"0,{h} " + pts + f" {w},{h}"
    grid = "".join(
        f'<line x1="0" y1="{h * k / 4:.1f}" x2="{w}" y2="{h * k / 4:.1f}" '
        f'stroke="#2b3a4d" stroke-width="0.8"/>' for k in range(1, 4))
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w:.0f} {h:.0f}" '
            f'width="{w:.0f}" height="{h:.0f}" role="presentation" aria-hidden="true">'
            "<!-- 자동 생성 — scripts/emit-keycloak-theme-art.py. 키오스크 그래프 한 칸. -->"
            f"{grid}"
            f'<polygon points="{area}" fill="#1d9bd1" opacity="0.22"/>'
            f'<polyline points="{pts}" fill="none" stroke="#4fc3f7" stroke-width="2.2" '
            'stroke-linejoin="round" stroke-linecap="round"/>'
            "</svg>\n")


def build_sprites():
    out = {}
    for slug in OSS_CATALOG:
        art, color = load_icon(slug), icon_color(slug)
        out[f"oss-{slug}"] = sprite(
            (lambda a, c: lambda d: container(d, OSS_W, OSS_D, OSS_H, c, a,
                                              min(OSS_W, OSS_D) * 0.66))(art, color),
            f"벨트에 얹히는 {slug} 컨테이너.")
    for name, color in APPS:
        art = app_mark(name)
        out[f"app-{name}"] = sprite(
            (lambda a, c: lambda d: container(d, APP_W, APP_D, APP_H, c, a,
                                              min(APP_W, APP_D) * 0.62))(art, color),
            f"CI/CD 스택 위에 얹히는 {name} 컨테이너.")
    out["pallet-in"] = sprite(lambda d: pallet(d, PAL_IN_W, PAL_IN_D), "올라오는 쪽 판넬.")
    out["pallet-out"] = sprite(lambda d: pallet(d, PAL_OUT_W, PAL_OUT_D),
                               "나가는 쪽 판넬 — 긴 상자를 받친다.")
    out["cicd-container"] = sprite(cicd_container, "게이트에서 나오는 완성된 CI/CD 스택.")
    for tag, (w, dp, h) in (("in", (OSS_W, OSS_D, OSS_H)),
                            ("out", (APP_W, APP_D, APP_H))):
        ax = AXIS[tag]
        out[f"grip-bar-{tag}"] = sprite(
            (lambda ww, dd, hh, aa: lambda d: grip_bar(d, ww, dd, hh, aa))(w, dp, h, ax),
            f"{tag} 크레인 집게의 가로대.")
        for side, nm in ((-1, "l"), (1, "r")):
            out[f"grip-jaw-{nm}-{tag}"] = sprite(
                (lambda ww, dd, hh, aa, sd: lambda d: grip_jaw(d, ww, dd, hh, aa, sd))(
                    w, dp, h, ax, side),
                f"{tag} 크레인 집게발 ({nm}).")
    out["badge"] = sprite(badge, "완성 표시.")
    return out


# ── CSS 로 넘기는 좌표와 타이밍 ───────────────────────────────────────────
def generated_css(back, sprites):
    bx, by, bw, bh = back.bbox()
    sx = lambda px: px - bx
    sy = lambda py: py - by
    fx = lambda px: f"{sx(px) / bw * 100:.4f}%"
    fy = lambda py: f"{sy(py) / bh * 100:.4f}%"

    L = ["/* 자동 생성 — scripts/emit-keycloak-theme-art.py. 손으로 고치지 않는다. */",
         "/*",
         " * 정지 장면 위에 움직이는 조각을 앉히는 좌표와, 크레인이 짐을 놓는 순간의",
         " * 타이밍. 그림과 같은 계산에서 나온 값이라 장면을 옮기면 같이 따라온다.",
         " * 길이는 컨테이너 질의 단위로 적는다 — .nl-scene 이 크기 컨테이너이므로",
         " * 1cqw/1cqh 가 장면 폭·높이의 1% 다.",
         " */",
         ":root {",
         f"  --nl-scene-w: {bw:.1f};",
         f"  --nl-scene-h: {bh:.1f};",
         "  /* 장면 좌표 1 단위 */",
         f"  --nl-u-w: calc(100cqw / {bw:.1f});",
         f"  --nl-u-h: calc(100cqh / {bh:.1f});",
         ""]

    # 두 벨트의 윗면. 흐르는 층이 이 위에 얹힌다.
    # 무늬가 한 바퀴 도는 데 걸리는 시간은 상자 속도에서 나온다. 고정값으로 두면
    # 벨트만 따로 빨라져 상자가 미끄러지는 것처럼 보인다.
    #   무늬 이동거리(= 벨트 길이) / 상자 속도(= 이동거리 / 한 바퀴)
    in_len = IN_Y_FAR - GATE_HALF
    p = iso_raw(-HALF, IN_Y_FAR, BELT_TOP)
    L += ["  /* 입구 벨트 윗면: 먼 쪽 모서리에서 게이트 쪽으로 흐른다 */",
          f"  --nl-belt-in-left: {fx(p[0])};",
          f"  --nl-belt-in-top: {fy(p[1])};",
          f"  --nl-belt-in-len: {in_len * UNIT / bw * 100:.4f}%;",
          f"  --nl-belt-in-ratio: {in_len / BELT_W:.4f};",
          f"  --nl-belt-in-rate: {in_len / (IN_START - IN_END):.4f};"]
    out_len = OUT_X_FAR - GATE_HALF
    p = iso_raw(GATE_HALF, -HALF, BELT_TOP)
    L += ["  /* 출구 벨트 윗면 */",
          f"  --nl-belt-out-left: {fx(p[0])};",
          f"  --nl-belt-out-top: {fy(p[1])};",
          f"  --nl-belt-out-len: {out_len * UNIT / bw * 100:.4f}%;",
          f"  --nl-belt-out-ratio: {out_len / BELT_W:.4f};",
          f"  --nl-belt-out-rate: {out_len / (OUT_END - OUT_START):.4f};", ""]

    # 두 갈래의 출발 자리와 한 바퀴 동안의 이동량
    a, b = iso_raw(0, IN_START, BELT_TOP), iso_raw(0, IN_END, BELT_TOP)
    in_span = IN_START - IN_END
    L += ["  /* 올라오는 갈래: 출발 자리와 한 바퀴 동안의 이동량 */",
          f"  --nl-in-x: {fx(a[0])};",
          f"  --nl-in-y: {fy(a[1])};",
          f"  --nl-in-dx: {(b[0] - a[0]) / bw * 100:.4f}cqw;",
          f"  --nl-in-dy: {(b[1] - a[1]) / bh * 100:.4f}cqh;"]
    c, e = iso_raw(OUT_START, 0, BELT_TOP), iso_raw(OUT_END, 0, BELT_TOP)
    out_span = OUT_END - OUT_START
    L += ["  /* 나가는 갈래 */",
          f"  --nl-out-x: {fx(c[0])};",
          f"  --nl-out-y: {fy(c[1])};",
          f"  --nl-out-dx: {(e[0] - c[0]) / bw * 100:.4f}cqw;",
          f"  --nl-out-dy: {(e[1] - c[1]) / bh * 100:.4f}cqh;", ""]

    ci = iso_raw(-HALF, GATE_HALF, MOUTH_Z1)
    co = iso_raw(GATE_HALF, -HALF, MOUTH_Z1)
    L += ["  /* 스트립 커튼: 개구부 왼쪽 위 모서리와 크기. 세로면에 눕히는 행렬은",
          "     nullus.css 가 들고 있다 (입구는 가로축 +x, 출구는 +y). */",
          f"  --nl-curtain-in-x: {fx(ci[0])};",
          f"  --nl-curtain-in-y: {fy(ci[1])};",
          f"  --nl-curtain-out-x: {fx(co[0])};",
          f"  --nl-curtain-out-y: {fy(co[1])};",
          f"  --nl-curtain-w: calc({BELT_W * UNIT:.1f} * var(--nl-u-w));",
          f"  --nl-curtain-h: calc({(MOUTH_Z1 - MOUTH_Z0) * UNIT:.1f} * var(--nl-u-h));",
          ""]
    # 같은 규격이어야 하는 조각들은 치수를 한 벌만 내보낸다. 조각마다 따로
    # 내보내고 CSS 가 그중 하나를 골라 쓰면, 하나가 달라졌을 때 나머지가 조용히
    # 어긋난 자리에 놓인다.
    groups = {"oss-box": [n for n in sprites if n.startswith("oss-")],
              "app-box": [n for n in sprites if n.startswith("app-")]}
    grouped = {n for names in groups.values() for n in names}

    # 주의: 이 함수 맨 위에서 bx, by 가 viewBox 원점으로 잡혀 있고 sx/sy 가 그것을
    # 붙잡고 있다. 루프 변수 이름이 겹치면 그 뒤에 나오는 좌표가 전부 어긋난다.
    L += [f"  --nl-lamp: calc({LAMP * UNIT:.1f} * var(--nl-u-w));",
          f"  --nl-lamp-glow: calc({LAMP * 2.4 * UNIT:.1f} * var(--nl-u-w));"]

    L.append("")
    L.append("  /* 키오스크 화면 — 세로면에 눕히는 행렬은 nullus.css 가 들고 있다 */")
    for i, (_kx, _ky, _kind) in enumerate(KIOSKS):
        g = kiosk_screen(_kind)
        kp = iso_raw(_kx - g["half_w"], _ky + g["dy"] + 0.02, g["z1"])
        L += [f"  --nl-kiosk-{i}-x: {fx(kp[0])};",
              f"  --nl-kiosk-{i}-y: {fy(kp[1])};",
              f"  --nl-kiosk-{i}-w: calc({g['half_w'] * 2 * UNIT:.1f} * var(--nl-u-w));",
              f"  --nl-kiosk-{i}-h: calc({(g['z1'] - g['z0']) * UNIT:.1f} * var(--nl-u-h));"]
    L.append("")
    L.append("  /* 개발자 모니터에서 지금 치고 있는 줄 */")
    _da = dev_type_anchor(*DEV_POS)
    _dp = iso_raw(*_da)
    L += [f"  --nl-dev-x: {fx(_dp[0])};",
          f"  --nl-dev-y: {fy(_dp[1])};",
          f"  --nl-dev-w: calc({DEV_TYPE_W * UNIT:.1f} * var(--nl-u-w));",
          f"  --nl-dev-h: calc({DEV_LINE_H * UNIT:.1f} * var(--nl-u-h));"]
    L.append("")

    L.append("  /* 조각의 크기와 기준점(그 자리의 벨트 윗면)에서의 거리 */")

    def emit(name, sp):
        return [f"  --nl-{name}-dx: calc({sp['dx']:.1f} * var(--nl-u-w));",
                f"  --nl-{name}-dy: calc({sp['dy']:.1f} * var(--nl-u-h));",
                f"  --nl-{name}-w: calc({sp['w']:.1f} * var(--nl-u-w));",
                f"  --nl-{name}-h: calc({sp['h']:.1f} * var(--nl-u-h));"]

    for name, sp in sprites.items():
        if name not in grouped:
            L += emit(name, sp)
    for group, names in groups.items():
        dims = {tuple(round(sprites[n][k], 3) for k in ("dx", "dy", "w", "h")) for n in names}
        assert len(dims) == 1, f"{group} 안의 조각 크기가 서로 다르다: {names}"
        L += emit(group, sprites[names[0]])

    bo = iso_raw(NUL_W / 2 + 0.35, APP_SLOT_Y - 0.4, PAL_H + NUL_H + APP_H + 0.5)
    L += ["", "  /* 완성 배지가 붙는 자리 (스택 오른쪽 위 모서리) */",
          f"  --nl-badge-ox: calc({bo[0]:.1f} * var(--nl-u-w));",
          f"  --nl-badge-oy: calc({bo[1]:.1f} * var(--nl-u-h));"]
    L += ["", "  /* 집게가 벌어지는 거리와 방향. 크레인 팔과 같은 축을 따라 열린다. */"]
    for _tag, _ax in AXIS.items():
        _jo = iso_raw(GRIP_OPEN * _ax[0], GRIP_OPEN * _ax[1], 0)
        L += [f"  --nl-jaw-open-{_tag}-x: calc({_jo[0]:.2f} * var(--nl-u-w));",
              f"  --nl-jaw-open-{_tag}-y: calc({_jo[1]:.2f} * var(--nl-u-h));"]
    L += ["", "  /* 한 칸 쌓일 때의 세로 이동. z 축이 순수 세로라 이 값 하나면 된다. */",
          f"  --nl-oss-step: calc({OSS_H * UNIT:.1f} * var(--nl-u-h));",
          f"  --nl-pallet-lift: calc({PAL_H * UNIT:.1f} * var(--nl-u-h));",
          f"  --nl-nullus-lift: calc({NUL_H * UNIT:.1f} * var(--nl-u-h));"]

    # 나가는 쪽 긴 상자 위의 세 자리. 월드 x 로 옮기면 화면에서는 가로·세로가
    # 함께 움직이므로 두 값을 다 넘긴다.
    L.append("  /* 긴 상자 위의 세 자리 — 오른쪽(a)부터 차례로 채워진다 */")
    for _k, _off in zip("abc", (APP_SLOT, 0.0, -APP_SLOT)):
        _sp = iso_raw(_off, APP_SLOT_Y, 0)
        L += [f"  --nl-slot-{_k}-x: calc({_sp[0]:.2f} * var(--nl-u-w));",
              f"  --nl-slot-{_k}-y: calc({_sp[1]:.2f} * var(--nl-u-h));"]

    L += [
          "  /* 후크에 걸린 짐이 후크보다 아래에 오도록 내리는 몫 (= 상자 높이) */",
          f"  --nl-hang-in: calc({(OSS_H + _grip_top(OSS_D)) * UNIT:.1f} * var(--nl-u-h));",
          f"  --nl-hang-out: calc({(APP_H + _grip_top(APP_D)) * UNIT:.1f} * var(--nl-u-h));", ""]
    L += ["  /* 집게가 벌어질 때 가로대가 축 방향으로만 늘어나는 변환 */",
          "  --nl-bar-open-in: "
          + bar_stretch_matrix(AXIS["in"], grip_bar_open(OSS_W, OSS_D, AXIS["in"])) + ";",
          "  --nl-bar-open-out: "
          + bar_stretch_matrix(AXIS["out"], grip_bar_open(APP_W, APP_D, AXIS["out"])) + ";",
          ""]

    hook_z = CRANE_TOP - 0.4

    def rest_frac(drop, hang):
        """쉴 때 풀어 둘 케이블의 비율. 케이블 길이가 크레인마다 달라 비로 넘긴다."""
        cable = drop - hang
        assert cable > HOOK_REST + 0.3, f"케이블이 너무 짧다 ({cable:.2f})"
        return HOOK_REST / cable

    hang_in = OSS_H + _grip_top(OSS_D)
    hang_out = APP_H + _grip_top(APP_D)
    L.append("  /* 크레인: 케이블이 시작하는 자리, 내리는 거리, 짐을 놓는 순간 */")
    in_at, out_at = [], []
    for i, cy in enumerate(IN_CRANE_YS):
        k = chr(ord("a") + i)
        hook = iso_raw(0, cy, hook_z)
        drop = hook_z - (BELT_TOP + PAL_H + i * OSS_H)
        assert drop > 0, f"입구 크레인 {k} 가 스택보다 낮다"
        at = (IN_START - cy) / in_span
        in_at.append(at)
        L += [f"  --nl-rig-in-{k}-x: {fx(hook[0])};",
              f"  --nl-rig-in-{k}-y: {fy(hook[1])};",
              f"  --nl-rig-in-{k}-drop: calc({drop * UNIT:.1f} * var(--nl-u-h));",
              f"  --nl-rig-in-{k}-restf: {rest_frac(drop, hang_in):.4f};",
              # 항상 음수여야 한다. 양수 지연은 애니메이션이 "아직 시작 전" 이라
            # keyframes 가 적용되지 않고, 그동안 짐이 기본 상태(불투명)로 후크에
            # 걸린 채 보인다. 한 바퀴를 더 빼도 위상은 같다.
            f"  --nl-rig-in-{k}-shift: {at - 1.5:.4f};"]
    for i, cx in enumerate(OUT_CRANE_XS):
        k = chr(ord("a") + i)
        # 후크는 짐이 놓일 자리 바로 위에 있어야 곧장 내려앉는다.
        hook = iso_raw(cx, APP_SLOT_Y, hook_z)
        # 셋 다 같은 높이에 놓이므로 내리는 거리도 같다.
        drop = hook_z - (BELT_TOP + PAL_H + NUL_H)
        assert drop > 0, f"출구 크레인 {k} 가 상자보다 낮다"
        # 짐이 놓이는 순간은 그 "자리" 가 후크 아래를 지나는 때다. 자리가 상자
        # 한가운데가 아니므로 앵커 기준으로 그만큼 당기거나 미뤄야 한다.
        slot = (APP_SLOT, 0.0, -APP_SLOT)[i]
        at = (cx - slot - OUT_START) / out_span
        assert 0.0 < at < 1.0, f"출구 크레인 {k} 의 시점이 벨트 밖이다 (at={at:.3f})"
        out_at.append(at)
        L += [f"  --nl-rig-out-{k}-x: {fx(hook[0])};",
              f"  --nl-rig-out-{k}-y: {fy(hook[1])};",
              f"  --nl-rig-out-{k}-drop: calc({drop * UNIT:.1f} * var(--nl-u-h));",
              f"  --nl-rig-out-{k}-restf: {rest_frac(drop, hang_out):.4f};",
              f"  --nl-rig-out-{k}-shift: {at - 1.5:.4f};"]
    L += ["",
          "  /* 올라오는 컨테이너 셋. 접속할 때마다 pick-oss.js 가 카탈로그에서",
          "     다시 고른다 — 아래는 자바스크립트가 막힌 환경의 기본값이다. */"]
    for k, slug in zip("abc", OSS_DEFAULT):
        L.append(f'  --nl-oss-{k}: url("oss-{slug}.svg");')
    L += ["}", ""]

    vanish = (IN_START - IN_VANISH) / in_span

    # 매 바퀴 OSS 를 다시 고르려면, 한 바퀴가 넘어가는 순간(진행도 0)에 그 도구가
    # 화면에 하나도 없어야 한다. 한 도구가 보이는 구간은 크레인이 그것을 드는
    # 때부터 벨트에서 사라질 때까지 이어진다 —
    #   [ at - (0.5 - HOIST_SHOW) , vanish ]
    # 이 구간이 0 을 넘지 않아야 교체가 눈에 띄지 않는다.
    for _tag, _key, _w, _dp, _h in (("입구", "in", OSS_W, OSS_D, OSS_H),
                                    ("출구", "out", APP_W, APP_D, APP_H)):
        _ax = AXIS[_key]
        assert _grip_clears(_w, _dp, _h, _ax), (
            f"{_tag} 집게 가로대가 컨테이너 뚜껑을 가로지른다 — _grip_top 을 높여야 한다")
        for _side in (-1, 1):
            assert _pad_on_corner_mid(_w, _dp, _h, _ax, _side), (
                f"{_tag} 집게발이 컨테이너 모서리 한가운데를 벗어났다 (side={_side})")

    assert vanish < 1.0, "벨트 위 상자가 한 바퀴 안에 사라지지 않는다"
    assert GRAB_AT > 0.01, (
        "크레인이 한 바퀴 경계에서 짐을 물고 있다 — 교체 순간에 공중의 컨테이너가 바뀐다")
    for _k, _at in zip("abc", in_at):
        assert GRAB_AT < _at, (
            f"입구 크레인 {_k} 가 짐을 물기 전에 놓는다 (물기 {GRAB_AT}, 놓기 {_at:.3f})")

    # 출구: 컨테이너가 게이트를 빠져나온 뒤에 크레인이 내려와야 한다.
    # % 1.0 을 씌우면 안 된다 — 이전 바퀴로 감긴 값(예: -0.10 -> 0.90)이 조건을
    # 만족해 버려서, 정작 막으려던 "컨테이너보다 먼저 내려온다" 를 놓친다.
    for _k, _at in zip("abc", out_at):
        _start = HOIST_DROP_FROM + _at - 0.5
        assert _start > OUT_FADE_IN, (
            f"출구 크레인 {_k} 가 진행도 {_start:.3f} 에 내려오기 시작한다 — "
            f"컨테이너가 다 나오는 {OUT_FADE_IN:.2f} 보다 빠르다")
    badge_at = (OUT_BADGE_X - OUT_START) / out_span
    pct = lambda v: f"{max(0.0, min(1.0, v)) * 100:.2f}%"

    L += ["/*",
          " * 안무. 백분율이 곧 상자가 벨트 위 어디쯤인지다 — 크레인을 옮기면 이 숫자도",
          " * 같이 바뀌어야 하므로 손으로 적지 않고 여기서 뽑는다.",
          " * 크레인이 손을 놓는 바로 그 자리에서 그대로 나타나야 한다. 조금이라도",
          " * 떨어진 데서 미끄러져 오게 하면 교대하는 순간 툭 끊겨 보인다.",
          " */"]
    for k, at in zip("abc", in_at):
        L += [f"@keyframes nl-in-land-{k} {{",
              f"  0%, {pct(at)} {{ opacity: 0; }}",
              f"  {pct(at + 0.001)} {{ opacity: 1; }}",
              f"  {pct(vanish)} {{ opacity: 1; }}",
              f"  {pct(vanish + 0.012)}, 100% {{ opacity: 0; }}",
              "}", ""]
    L += ["/* 벨트를 타는 판넬 (입구) */",
          "@keyframes nl-in-base {",
          "  0% { opacity: 0; }",
          "  2% { opacity: 1; }",
          f"  {pct(vanish)} {{ opacity: 1; }}",
          f"  {pct(vanish + 0.012)}, 100% {{ opacity: 0; }}",
          "}", "",
          "/* 스트립 커튼. 짐이 닿기 직전에 갈라졌다가 지나가면 닫힌다. */",
          "@keyframes nl-curtain-in-l {",
          f"  0%, {pct(vanish - 0.07)} {{ transform: translateX(0); }}",
          f"  {pct(vanish - 0.03)}, {pct(vanish + 0.02)} {{ transform: translateX(-100%); }}",
          f"  {pct(vanish + 0.06)}, 100% {{ transform: translateX(0); }}",
          "}", "",
          "@keyframes nl-curtain-in-r {",
          f"  0%, {pct(vanish - 0.07)} {{ transform: translateX(0); }}",
          f"  {pct(vanish - 0.03)}, {pct(vanish + 0.02)} {{ transform: translateX(100%); }}",
          f"  {pct(vanish + 0.06)}, 100% {{ transform: translateX(0); }}",
          "}", "",
          "/* 나오는 쪽은 한 바퀴가 넘어가는 자리에서 열린다. */",
          "@keyframes nl-curtain-out-l {",
          "  0%, 4% { transform: translateX(-100%); }",
          "  9%, 92% { transform: translateX(0); }",
          "  97%, 100% { transform: translateX(-100%); }",
          "}", "",
          "@keyframes nl-curtain-out-r {",
          "  0%, 4% { transform: translateX(100%); }",
          "  9%, 92% { transform: translateX(0); }",
          "  97%, 100% { transform: translateX(100%); }",
          "}", "",
          "/* 게이트에서 나오는 완성된 스택 */",
          "@keyframes nl-out-base {",
          "  0% { opacity: 0; }",
          "  4% { opacity: 1; }",
          "  96% { opacity: 1; }",
          "  100% { opacity: 0; }",
          "}", ""]
    for k, at in zip("abc", out_at):
        L += [f"@keyframes nl-out-land-{k} {{",
              f"  0%, {pct(at)} {{ opacity: 0; }}",
              f"  {pct(at + 0.001)} {{ opacity: 1; }}",
              "  96% { opacity: 1; }",
              "  100% { opacity: 0; }",
              "}", ""]
    # 경광등은 개수·자리·색·지연이 모두 데이터라 규칙까지 여기서 뽑는다.
    # 손으로 적으면 하나 늘릴 때마다 CSS 를 같이 고쳐야 하고, 잊으면 조용히
    # 안 보인다.
    L += ["/* 표시등 — 자리·색·깜빡임 지연. 생김새와 동작은 nullus.css 가 든다. */"]
    for i, (_pos, _line, _hue, _delay) in enumerate(BEACONS):
        # 기준점은 알(정육면체)의 뒤쪽 위 모서리. 세 면은 여기서 상수만큼 밀면
        # 되므로 (한 변이 다 같아) 표시등마다 자리는 하나면 된다.
        bp = iso_raw(*beacon_pos(_pos, _line))
        L += [f".nl-beacon--{i} {{",
              f"  left: {fx(bp[0])};",
              f"  top: {fy(bp[1])};",
              f"  --nl-lamp-top: {shade(_hue, 1.22)};",
              f"  --nl-lamp-x: {_hue};",
              f"  --nl-lamp-y: {shade(_hue, 0.8)};",
              f"  --nl-glow: {_hue};",
              f"  animation-delay: {_delay:.2f}s;",
              "}", ""]

    # 크레인 안무는 크레인마다 다르다. 짐을 무는 시점을 한 바퀴의 GRAB_AT 로
    # 맞추려면, 크레인 자기 주기에서는 서로 다른 지점이 되기 때문이다.
    # (자기 주기 = 전체 주기이되 위상이 shift 만큼 밀려 있다)
    def hoist_frames(name, at, drop_var):
        show = (GRAB_AT - (at - 0.5)) % 1.0
        up = "transform: translateY(var(--nl-rest));"
        down = f"transform: translateY({drop_var});"
        out = [f"@keyframes nl-hoist-{name} {{"]
        if show < HOIST_DROP_FROM:
            out += [f"  0%, {pct(max(0.0, show - 0.015))} {{ opacity: 0; {up} }}",
                    f"  {pct(show)}, {pct(HOIST_DROP_FROM)} {{ opacity: 1; {up} }}",
                    f"  50% {{ opacity: 1; {down} }}",
                    f"  50.01% {{ opacity: 0; {down} }}",
                    f"  70%, 100% {{ opacity: 0; {up} }}"]
        else:
            # 무는 시점이 자기 주기의 뒤쪽이면 경계를 넘어 물고 있는 상태로 시작한다
            out += [f"  0%, {pct(HOIST_DROP_FROM)} {{ opacity: 1; {up} }}",
                    f"  50% {{ opacity: 1; {down} }}",
                    f"  50.01% {{ opacity: 0; {down} }}",
                    f"  70%, {pct(show - 0.015)} {{ opacity: 0; {up} }}",
                    f"  {pct(show)}, 100% {{ opacity: 1; {up} }}"]
        return out + ["}", ""]

    def clamp_frames(kind, name, at, op, cl):
        """무는 동작. 집게발(옮기기)과 가로대(늘이기)가 같은 타이밍을 쓴다."""
        show = (GRAB_AT - (at - 0.5)) % 1.0
        out = [f"@keyframes nl-{kind}-{name} {{"]
        if show < HOIST_DROP_FROM:
            out += [f"  0%, {pct(max(0.0, show - 0.02))} {{ {op} }}",
                    f"  {pct(show)}, 50% {{ {cl} }}",
                    f"  51%, 100% {{ {op} }}"]
        else:
            out += ["  0%, 50% { " + cl + " }",
                    f"  51%, {pct(show - 0.02)} {{ {op} }}",
                    f"  {pct(show)}, 100% {{ {cl} }}"]
        return out + ["}", ""]

    JAW_OP = "transform: translate(var(--nl-open-x), var(--nl-open-y));"
    JAW_CL = "transform: translate(0, 0);"
    BAR_OP = "transform: var(--nl-bar-open);"
    BAR_CL = "transform: matrix(1, 0, 0, 1, 0, 0);"

    L.append("/* 크레인 안무 — 짐을 무는 시점만 크레인마다 다르다. */")
    for _g, _ats in (("in", in_at), ("out", out_at)):
        for _k, _at in zip("abc", _ats):
            L += hoist_frames(f"{_g}-{_k}", _at,
                              "calc(var(--nl-drop) - var(--nl-hang))")
            L += clamp_frames("jaw", f"{_g}-{_k}", _at, JAW_OP, JAW_CL)
            L += clamp_frames("bar", f"{_g}-{_k}", _at, BAR_OP, BAR_CL)

    L += ["@keyframes nl-badge {",
          f"  0%, {pct(badge_at)} {{ opacity: 0; transform: scale(0.5); }}",
          f"  {pct(badge_at + 0.03)} {{ opacity: 1; transform: scale(1); }}",
          "  96% { opacity: 1; transform: scale(1); }",
          "  100% { opacity: 0; transform: scale(1); }",
          "}", ""]
    return "\n".join(L)


def pick_script():
    """접속할 때마다 카탈로그에서 셋을 골라 CSS 변수에 꽂는다.

    순수 CSS 로는 난수를 만들 수 없어 이 한 조각만 자바스크립트다. 막혀 있어도
    화면은 기본 셋으로 그대로 뜬다 — 변수에 이미 값이 들어 있기 때문이다."""
    tools = ", ".join(f'"{t}"' for t in OSS_CATALOG)
    return f"""// 자동 생성 — scripts/emit-keycloak-theme-art.py. 손으로 고치지 않는다.
//
// 왼쪽 벨트에 실릴 OSS 셋을 카탈로그에서 골라 CSS 변수에 꽂는다. 순수 CSS 로는
// 난수를 만들 수 없어 이 한 조각만 자바스크립트다. 이 파일이 막혀도 화면은
// scene.generated.css 의 기본 셋으로 그대로 뜬다.
(function () {{
  var TOOLS = [{tools}];

  // 리소스 경로는 이 스크립트 자신의 주소에서 얻는다 — Keycloak 이 경로에 버전
  // 해시를 넣어서 미리 적어 둘 수 없다. currentScript 는 지금 이 순간에만
  // 읽히므로 먼저 붙잡아 둔다.
  var src = (document.currentScript || {{}}).src;

  function apply() {{
    try {{
      if (!src) return;
      var base = src.slice(0, src.lastIndexOf("/") + 1);
      var scene = document.querySelector(".nl-scene");
      if (!scene) return;

      var pool = TOOLS.slice();
      for (var i = pool.length - 1; i > 0; i--) {{
        var j = Math.floor(Math.random() * (i + 1));
        var t = pool[i]; pool[i] = pool[j]; pool[j] = t;
      }}
      ["a", "b", "c"].forEach(function (slot, i) {{
        scene.style.setProperty("--nl-oss-" + slot,
          'url("' + base + "oss-" + pool[i] + '.svg")');
      }});
    }} catch (e) {{
      // 고르기에 실패해도 기본 셋이 남아 있으므로 조용히 넘어간다.
    }}
  }}

  function start() {{
    apply();
    // 한 바퀴가 넘어갈 때마다 다시 고른다. 그 순간에는 이 셋 중 어느 것도 화면에
    // 없다 — 크레인이 짐을 드는 시점이 경계보다 뒤이고 벨트 위 상자는 그 전에
    // 사라지도록 안무를 맞춰 두었다(생성기가 단언으로 지킨다). 그래서 새로고침
    // 없이도 바뀌는 게 눈에 띄지 않는다.
    var run = document.querySelector(".nl-run--in");
    if (!run) return;
    run.addEventListener("animationiteration", function (e) {{
      // 안에 실린 조각들의 같은 이벤트가 올라온다. 한 바퀴에 한 번만 고른다.
      if (e.target === run) apply();
    }});
  }}

  // 이 스크립트는 <head> 에서 실행된다. 그때는 아직 장면이 없다.
  if (document.readyState === "loading") {{
    document.addEventListener("DOMContentLoaded", start);
  }} else {{
    start();
  }}
}})();
"""


MESSAGES = (REPO
            / "deploy/helm/nullus/files/keycloak-theme/nullus/login/messages"
            / "messages_ko.properties")


def _screen_box(x0, x1, y0, y1, z0, z1):
    pts = [iso_raw(x, y, z) for x in (x0, x1) for y in (y0, y1) for z in (z0, z1)]
    return (min(p[0] for p in pts), min(p[1] for p in pts),
            max(p[0] for p in pts), max(p[1] for p in pts))


def _silhouette(x0, x1, y0, y1, z0, z1):
    """상자의 화면 실루엣(볼록 육각형). 경계상자로 재면 빈 모서리까지 겹친 것으로
    쳐서, 벨트 옆에 붙는 작은 것은 어디에 둬도 겹친다고 나온다."""
    pts = [iso_raw(x, y, z) for x in (x0, x1) for y in (y0, y1) for z in (z0, z1)]
    pts = sorted(set((round(a, 9), round(b, 9)) for a, b in pts))
    def half(seq):
        out = []
        for q in seq:
            while len(out) >= 2:
                (ax, ay), (bx, by) = out[-2], out[-1]
                if (bx - ax) * (q[1] - ay) - (by - ay) * (q[0] - ax) <= 0:
                    out.pop()
                else:
                    break
            out.append(q)
        return out[:-1]
    return half(pts) + half(pts[::-1])


def _convex_hit(a, b):
    """두 볼록 다각형이 겹치는지 (분리축 정리). 한 축이라도 갈라지면 안 겹친다."""
    for poly in (a, b):
        for i in range(len(poly)):
            x0, y0 = poly[i]
            x1, y1 = poly[(i + 1) % len(poly)]
            nx, ny = -(y1 - y0), x1 - x0
            pa = [nx * px + ny * py for px, py in a]
            pb = [nx * px + ny * py for px, py in b]
            if max(pa) <= min(pb) or max(pb) <= min(pa):
                return False
    return True


def _cargo_boxes(n=120):
    """한 바퀴 동안 화물이 차지하는 화면 실루엣들."""
    out = []
    ti = PAL_H + 3 * OSS_H
    to = PAL_H + NUL_H + APP_H
    for i in range(n + 1):
        y = IN_Y_FAR + (IN_VANISH - IN_Y_FAR) * i / n
        out.append(_silhouette(-PAL_IN_W / 2, PAL_IN_W / 2, y - PAL_IN_D / 2,
                               y + PAL_IN_D / 2, BELT_TOP, BELT_TOP + ti))
        x = OUT_START + (OUT_END - OUT_START) * i / n
        out.append(_silhouette(x - PAL_OUT_W / 2, x + PAL_OUT_W / 2, -PAL_OUT_D / 2,
                               PAL_OUT_D / 2, BELT_TOP, BELT_TOP + to))
    return out


def check_beacons():
    """표시등 알이 화면에서 무엇과도 겹치지 않는지.

    알은 scene-front.svg 와 움직이는 조각들 **위에** 얹히는 DOM 이라, 화면에서
    겹치면 순서를 어떻게 바꿔도 그 위로 그려진다. 자리로만 막을 수 있다 —
    프레임 앞면의 벨트 윗면 아래에 붙어 있어야 화물 밑을 지나간다."""
    screens = []
    for kx, ky, kind in KIOSKS:
        if kind == "monitor":
            screens.append(("모니터 키오스크", _silhouette(
                kx - MON_W / 2, kx + MON_W / 2, ky - MON_D / 2, ky + MON_D / 2,
                MON_POST, MON_POST + MON_H)))
        else:
            screens.append(("콘솔 키오스크", _silhouette(
                kx - CON_SCR_W / 2, kx + CON_SCR_W / 2, ky - CON_SCR_D / 2,
                ky + CON_SCR_D / 2, 0.0, CON_SCR_Z + CON_SCR_H)))
    dx, dy = DEV_POS
    mx = dx - DESK_DEP / 2 + 0.3
    screens.append(("개발자 모니터", _silhouette(
        mx - 0.14, mx + 0.13, dy - 1.2, dy + 1.2,
        DESK_H + 0.34, DESK_H + 0.34 + 1.5)))
    lanes = [("화물이 지나는 길", c) for c in _cargo_boxes()]
    for i, (_pos, _line, _hue, _delay) in enumerate(BEACONS):
        bx, by, bz = beacon_pos(_pos, _line)
        lb = _silhouette(bx, bx + LAMP, by, by + LAMP, bz - LAMP, bz)
        for name, sb in screens + lanes:
            assert not _convex_hit(lb, sb), (
                f"표시등 {i} 가 {name} 와 화면에서 겹친다 — 그 위로 그려진다. "
                "벨트 윗면보다 낮게, 설비를 피해 붙여야 한다")


def check_markup():
    """장면 마크업(messages 번들)이 여기 데이터와 개수가 맞는지 본다.
    경광등이나 키오스크를 늘리고 마크업을 안 고치면, 늘어난 것은 자리만 잡히고
    화면에는 나타나지 않는다 — 오류도 나지 않아 눈으로 보기 전엔 모른다."""
    check_beacons()
    if not MESSAGES.exists():
        return
    src = MESSAGES.read_text()
    for name, want, token in (("표시등", len(BEACONS), "nl-beacon nl-beacon--"),
                              ("키오스크", len(KIOSKS), "nl-kiosk nl-kiosk--")):
        got = src.count(token)
        assert got == want, (
            f"{name} 이 {want} 개인데 마크업에는 {got} 개다 — "
            f"{MESSAGES.name} 의 장면 마크업을 같이 고쳐야 한다")

    # 알은 프레임 앞면에 붙어 있으니 맨 앞 층 뒤에 와야 한다. 앞에 두면 벨트에
    # 가려 안 보인다.
    divider = src.index('class="nl-scene__front"')
    for i in range(len(BEACONS)):
        assert src.index(f'nl-beacon nl-beacon--{i}"') > divider, (
            f"표시등 {i} 가 마크업에서 맨 앞 층 앞에 있다 — 벨트에 가려 안 보인다")


if __name__ == "__main__":
    import sys

    check_markup()

    # 출력 경로를 받는 이유: 테스트가 임시 폴더로 뽑아 커밋된 것과 대조한다.
    # 손으로 고쳤거나 다시 뽑는 걸 잊으면 그림과 좌표가 조용히 어긋난다.
    base = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else REPO / DEFAULT_OUT
    base.mkdir(parents=True, exist_ok=True)

    back, front = build_scene()
    (base / "scene-back.svg").write_text(
        svg_doc(back, "벨트 아래 — 게이트·컨베이어·크레인 구조물."))
    (base / "scene-front.svg").write_text(
        svg_doc(front, "맨 앞 — 라인 옆 판독기. viewBox 는 scene-back.svg 와 같다."))

    sprites = build_sprites()
    for name, s in sprites.items():
        (base / f"{name}.svg").write_text(s["svg"])
    (base / "scene.generated.css").write_text(generated_css(back, sprites))
    (base / "pick-oss.js").write_text(pick_script())
    (base / "metric.svg").write_text(metric_tile())

    _, _, w, h = back.bbox()
    print(f"wrote scene-back/front + {len(sprites)} sprites + scene.generated.css "
          f"({w:.0f}x{h:.0f})")
