#!/usr/bin/env python3
"""Generate synthetic Japanese novel-page OCR fixture images.

Requires: Pillow, a CJK TTF (default: env OCR_FONT or common IPAex/Noto paths).
Regenerate:  OCR_FONT=/path/to/ipaexg.ttf python3 testdata/ocr/gen_fixtures.py

Outputs under testdata/ocr/images/ and refreshes cases.json.
Images are synthetic prose renders for adapter contract / L2/L3 fixtures —
not real book photos. Primary material assumption: novel prose (see ticket 06).
"""

from __future__ import annotations

import json
import os
import random
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter, ImageFont

ROOT = Path(__file__).resolve().parent
IMG_DIR = ROOT / "images"
CASES_PATH = ROOT / "cases.json"

FONT_CANDIDATES = [
    os.environ.get("OCR_FONT", ""),
    "/tmp/IPAexfont00401/ipaexg.ttf",
    str(ROOT / "fonts" / "ipaexg.ttf"),
    "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
    "/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
    "/usr/share/fonts/opentype/noto/NotoSerifCJK-Regular.ttc",
]


def find_font() -> str:
    for p in FONT_CANDIDATES:
        if p and Path(p).is_file():
            return p
    raise SystemExit(
        "No CJK font found. Set OCR_FONT=/path/to.ttf "
        "(e.g. IPAexGothic ipaexg.ttf) and re-run."
    )


def load_font(path: str, size: int) -> ImageFont.FreeTypeFont:
    return ImageFont.truetype(path, size=size)


def new_page(w: int, h: int, bg=(255, 255, 250)) -> Image.Image:
    return Image.new("RGB", (w, h), bg)


def draw_h_lines(
    img: Image.Image,
    lines: list[str],
    font: ImageFont.FreeTypeFont,
    *,
    margin: int = 40,
    line_gap: int = 12,
    fill=(20, 20, 20),
    start_y: int | None = None,
) -> None:
    draw = ImageDraw.Draw(img)
    y = margin if start_y is None else start_y
    for line in lines:
        draw.text((margin, y), line, font=font, fill=fill)
        bbox = draw.textbbox((margin, y), line, font=font)
        y = bbox[3] + line_gap


# Fullwidth punctuation often set upright in tate-gaki; keep simple: one cell per rune.
_TATE_SKIP = set("\n\r\t")


def draw_v_column(
    img: Image.Image,
    text: str,
    font: ImageFont.FreeTypeFont,
    *,
    x: int,
    top: int = 40,
    bottom: int | None = None,
    char_gap: int = 2,
    fill=(28, 24, 20),
    cell: int | None = None,
) -> int:
    """One vertical column, top→bottom. Returns final y.

    Characters sit in fixed-height cells (novel monospaced grid feel).
    """
    draw = ImageDraw.Draw(img)
    if cell is None:
        # Em-box from a fullwidth sample glyph.
        sample = draw.textbbox((0, 0), "国", font=font)
        cell = (sample[3] - sample[1]) + char_gap
    max_y = (img.size[1] - 40) if bottom is None else bottom
    y = top
    for ch in text:
        if ch in _TATE_SKIP:
            continue
        if y + cell > max_y:
            break
        # Center glyph roughly in cell width using textbbox.
        bbox = draw.textbbox((0, 0), ch, font=font)
        gw = bbox[2] - bbox[0]
        gh = bbox[3] - bbox[1]
        cx = x - gw // 2
        cy = y + max(0, (cell - gh) // 2) - bbox[1]
        draw.text((cx, cy), ch, font=font, fill=fill)
        y += cell
    return y


def split_into_columns(text: str, chars_per_col: int) -> list[str]:
    """Fill columns top→bottom; caller lays columns right→left."""
    flat = "".join(ch for ch in text if ch not in _TATE_SKIP)
    if chars_per_col < 1:
        chars_per_col = 1
    cols: list[str] = []
    for i in range(0, len(flat), chars_per_col):
        cols.append(flat[i : i + chars_per_col])
    return cols


def render_novel_vertical_page(
    columns: list[str],
    font: ImageFont.FreeTypeFont,
    *,
    page_w: int = 720,
    page_h: int = 1024,
    bg=(250, 246, 236),  # warm paper
    margin_top: int = 72,
    margin_bottom: int = 80,
    margin_right: int = 72,
    col_gap: int = 44,
    fill=(32, 28, 24),
    page_num: str | None = None,
    page_num_font: ImageFont.FreeTypeFont | None = None,
    rule_line: bool = False,
) -> Image.Image:
    """Bunkobon-like page: cream paper, columns right→left, top→bottom."""
    img = new_page(page_w, page_h, bg)
    draw = ImageDraw.Draw(img)
    sample = draw.textbbox((0, 0), "国", font=font)
    cell = (sample[3] - sample[1]) + 2
    # Column x = center of glyph column; start from right margin.
    x = page_w - margin_right
    bottom = page_h - margin_bottom
    for col in columns:
        draw_v_column(
            img,
            col,
            font,
            x=x,
            top=margin_top,
            bottom=bottom,
            fill=fill,
            cell=cell,
        )
        x -= col_gap
        if x < 48:
            break
    if rule_line:
        # faint top margin rule (some editions)
        draw.line(
            (margin_right // 2, margin_top - 16, page_w - margin_right // 2, margin_top - 16),
            fill=(210, 200, 185),
            width=1,
        )
    if page_num and page_num_font is not None:
        # center bottom page number (common novel footer)
        bb = draw.textbbox((0, 0), page_num, font=page_num_font)
        tw = bb[2] - bb[0]
        draw.text(
            ((page_w - tw) // 2, page_h - margin_bottom + 24),
            page_num,
            font=page_num_font,
            fill=(90, 80, 70),
        )
    return img


def paper_texture(img: Image.Image, amount: int = 8, seed: int = 7) -> Image.Image:
    """Subtle fiber noise on cream paper."""
    rnd = random.Random(seed)
    px = img.load()
    w, h = img.size
    for _ in range(w * h // 25):
        x, y = rnd.randrange(w), rnd.randrange(h)
        r, g, b = px[x, y]
        d = rnd.randint(-amount, amount)
        px[x, y] = (
            max(0, min(255, r + d)),
            max(0, min(255, g + d // 2)),
            max(0, min(255, b + d // 3)),
        )
    return img


def phone_tilt(
    img: Image.Image,
    degrees: float,
    *,
    desk=(36, 34, 32),
    pad: int = 24,
) -> Image.Image:
    """Slight handheld rotation with desk/letterbox border (phone over book).

    Positive degrees = CCW (PIL rotate convention). Expand so no crop of page.
    """
    if pad:
        canvas = Image.new("RGB", (img.size[0] + pad * 2, img.size[1] + pad * 2), desk)
        canvas.paste(img, (pad, pad))
        img = canvas
    return img.rotate(degrees, expand=True, fillcolor=desk, resample=Image.Resampling.BICUBIC)


def apply_brightness(img: Image.Image, factor: float) -> Image.Image:
    return _brightness(img, factor)


def split_brightness_vertical(
    img: Image.Image,
    *,
    left_factor: float,
    right_factor: float,
) -> Image.Image:
    """Left half one brightness, right half another (mixed lighting in one shot)."""
    w, h = img.size
    left = apply_brightness(img.crop((0, 0, w // 2, h)), left_factor)
    right = apply_brightness(img.crop((w // 2, 0, w, h)), right_factor)
    out = img.copy()
    out.paste(left, (0, 0))
    out.paste(right, (w // 2, 0))
    return out


def split_brightness_horizontal(
    img: Image.Image,
    *,
    top_factor: float,
    bottom_factor: float,
) -> Image.Image:
    """Top half one brightness, bottom half another."""
    w, h = img.size
    top = apply_brightness(img.crop((0, 0, w, h // 2)), top_factor)
    bottom = apply_brightness(img.crop((0, h // 2, w, h)), bottom_factor)
    out = img.copy()
    out.paste(top, (0, 0))
    out.paste(bottom, (0, h // 2))
    return out


def gradient_brightness(
    img: Image.Image,
    *,
    left_factor: float,
    right_factor: float,
) -> Image.Image:
    """Smooth left→right brightness ramp (window light / shadow falloff)."""
    w, h = img.size
    base = img.convert("RGB")
    dark = apply_brightness(base, left_factor)
    bright = apply_brightness(base, right_factor)
    # Alpha ramp 0..255 across width
    ramp = Image.new("L", (w, h))
    rp = ramp.load()
    for x in range(w):
        v = int(255 * x / max(1, w - 1))
        for y in range(h):
            rp[x, y] = v
    return Image.composite(bright, dark, ramp)


def blur_region(
    img: Image.Image,
    box: tuple[int, int, int, int],
    *,
    radius: float = 2.0,
) -> Image.Image:
    """Blur only one rectangle (partial defocus in same shot). box = (x0,y0,x1,y1)."""
    out = img.copy()
    x0, y0, x1, y1 = box
    region = out.crop((x0, y0, x1, y1)).filter(ImageFilter.GaussianBlur(radius=radius))
    out.paste(region, (x0, y0))
    return out


def faux_bold_text(
    draw: ImageDraw.ImageDraw,
    xy: tuple[int, int],
    text: str,
    font: ImageFont.FreeTypeFont,
    fill: tuple[int, int, int],
    *,
    radius: int = 1,
) -> None:
    """Simulate heavier stroke by multi-offset draw (no bold face required)."""
    x, y = xy
    for dx in range(-radius, radius + 1):
        for dy in range(-radius, radius + 1):
            if dx * dx + dy * dy <= radius * radius + 1:
                draw.text((x + dx, y + dy), text, font=font, fill=fill)
    draw.text((x, y), text, font=font, fill=fill)


def draw_h_line_styled(
    img: Image.Image,
    text: str,
    font: ImageFont.FreeTypeFont,
    *,
    x: int,
    y: int,
    fill=(20, 20, 20),
    bold: bool = False,
) -> int:
    """Draw one horizontal line; return next y."""
    draw = ImageDraw.Draw(img)
    if bold:
        faux_bold_text(draw, (x, y), text, font, fill, radius=1)
    else:
        draw.text((x, y), text, font=font, fill=fill)
    bbox = draw.textbbox((x, y), text, font=font)
    return bbox[3] + 14


def save(img: Image.Image, name: str, fmt: str = "PNG", **kw) -> str:
    IMG_DIR.mkdir(parents=True, exist_ok=True)
    path = IMG_DIR / name
    img.save(path, format=fmt, **kw)
    return name


def add_noise(img: Image.Image, amount: int = 28) -> Image.Image:
    rnd = random.Random(42)
    px = img.load()
    w, h = img.size
    for _ in range(w * h // 40):
        x, y = rnd.randrange(w), rnd.randrange(h)
        r, g, b = px[x, y]
        d = rnd.randint(-amount, amount)
        px[x, y] = (
            max(0, min(255, r + d)),
            max(0, min(255, g + d)),
            max(0, min(255, b + d)),
        )
    return img


def main() -> None:
    font_path = find_font()
    print(f"font: {font_path}")
    IMG_DIR.mkdir(parents=True, exist_ok=True)

    f22 = load_font(font_path, 22)
    f28 = load_font(font_path, 28)
    f32 = load_font(font_path, 32)
    f36 = load_font(font_path, 36)
    f18 = load_font(font_path, 18)
    f48 = load_font(font_path, 48)

    cases: list[dict] = []

    def add(
        case_id: str,
        file: str,
        *,
        expected_text: str,
        tags: list[str],
        notes: str,
        want_success: bool = True,
        min_overlap: float | None = None,
    ) -> None:
        cases.append(
            {
                "id": case_id,
                "file": f"images/{file}",
                "expected_text": expected_text,
                "want_success": want_success,
                "min_overlap": min_overlap,
                "tags": tags,
                "notes": notes,
            }
        )

    # --- 01 happy: single sentence (stub fixture text) ---
    t = "私は本を読む。"
    img = new_page(640, 200)
    draw_h_lines(img, [t], f36)
    save(img, "01_single_sentence.png")
    add(
        "01_single_sentence",
        "01_single_sentence.png",
        expected_text=t,
        tags=["happy", "horizontal", "single", "stub-aligned"],
        notes="Clean single sentence; matches analyzer stub fixture.",
        min_overlap=0.9,
    )

    # --- 02 multi-sentence page ---
    multi = [
        "病院に行った。",
        "今日は雨だ。",
        "私は本を読む。",
    ]
    img = new_page(720, 320)
    draw_h_lines(img, multi, f32, line_gap=18)
    save(img, "02_multi_sentence.png")
    add(
        "02_multi_sentence",
        "02_multi_sentence.png",
        expected_text="\n".join(multi),
        tags=["happy", "horizontal", "multi-sentence"],
        notes="Three short novel-like sentences; feeds splitSentences after OCR.",
        min_overlap=0.85,
    )

    # --- 03 multi-line paragraph ---
    para_lines = [
        "彼は静かな部屋で古い小説を開いた。",
        "窓の外では風が木の葉を揺らしている。",
        "お茶を一口飲んでから、次のページへ進んだ。",
    ]
    img = new_page(800, 360)
    draw_h_lines(img, para_lines, f28, line_gap=16)
    save(img, "03_paragraph.png")
    add(
        "03_paragraph",
        "03_paragraph.png",
        expected_text="\n".join(para_lines),
        tags=["happy", "horizontal", "paragraph"],
        notes="Longer prose lines; typical phone capture density.",
        min_overlap=0.8,
    )

    # --- 04 vertical short (legacy id; bunkobon two-column) ---
    vtext = "彼は夜の街を歩いた。雨が静かに降っていた。"
    cols = split_into_columns(vtext, chars_per_col=12)
    img = render_novel_vertical_page(
        cols, f36, page_w=520, page_h=720, col_gap=52, page_num="12", page_num_font=f18
    )
    img = paper_texture(img, amount=6)
    save(img, "04_vertical_columns.png")
    add(
        "04_vertical_columns",
        "04_vertical_columns.png",
        expected_text=vtext,
        tags=["hard", "vertical", "novel-layout", "tate-gaki"],
        notes="Short tate-gaki two-column page. Real engines often struggle; edit path is safety net.",
        min_overlap=0.4,
    )

    # --- 05 punctuation mix 。！？ ---
    punct = "本当か？信じられない！彼は笑った。"
    img = new_page(700, 200)
    draw_h_lines(img, [punct], f36)
    save(img, "05_punctuation_mix.png")
    add(
        "05_punctuation_mix",
        "05_punctuation_mix.png",
        expected_text=punct,
        tags=["happy", "punctuation", "splitSentences"],
        notes="。！？ terminators for sentence split after OCR.",
        min_overlap=0.85,
    )

    # --- 06 blank page (no text) ---
    img = new_page(640, 480, (255, 255, 255))
    save(img, "06_blank.png")
    add(
        "06_blank",
        "06_blank.png",
        expected_text="",
        tags=["edge", "empty", "failure-or-empty"],
        notes="No glyphs. Engine may return empty string or soft fail; must not crash ingest.",
        want_success=True,  # empty text OK; product may treat as empty candidates
        min_overlap=None,
    )

    # --- 07 pure noise / no prose ---
    img = new_page(640, 480, (180, 180, 180))
    img = add_noise(img, amount=80)
    save(img, "07_noise_only.png")
    add(
        "07_noise_only",
        "07_noise_only.png",
        expected_text="",
        tags=["edge", "noise", "best-effort"],
        notes="No readable prose. Best-effort empty/garbage; queue must stay intact.",
        want_success=True,
        min_overlap=None,
    )

    # --- 08 low contrast ---
    t = "薄い文字は読みにくい。"
    img = new_page(640, 200, (240, 240, 240))
    draw_h_lines(img, [t], f36, fill=(200, 200, 200))
    save(img, "08_low_contrast.png")
    add(
        "08_low_contrast",
        "08_low_contrast.png",
        expected_text=t,
        tags=["hard", "contrast"],
        notes="Gray-on-gray; engine quality varies. Edit/paste fallback path.",
        min_overlap=0.3,
    )

    # --- 09 dense small font ---
    dense = [
        "古い屋敷の奥にある書斎には、無数の本が積み上げられていた。",
        "埃っぽい空気の中で、時計だけが規則正しく時を刻んでいる。",
        "少年は息を潜め、禁じられた棚の一冊に手を伸ばした。",
        "表紙には誰も読めない文字が金色に輝いていたという。",
    ]
    img = new_page(900, 400)
    draw_h_lines(img, dense, f18, line_gap=10, margin=24)
    save(img, "09_dense_small.png")
    add(
        "09_dense_small",
        "09_dense_small.png",
        expected_text="\n".join(dense),
        tags=["hard", "dense", "small-font"],
        notes="Phone photo of dense page approximation.",
        min_overlap=0.5,
    )

    # --- 10 large sparse ---
    t = "夏が来た。"
    img = new_page(640, 320)
    draw_h_lines(img, [t], f48, margin=80, start_y=120)
    save(img, "10_large_sparse.png")
    add(
        "10_large_sparse",
        "10_large_sparse.png",
        expected_text=t,
        tags=["happy", "large-font"],
        notes="Few large glyphs; easy OCR baseline.",
        min_overlap=0.95,
    )

    # --- 11 dark mode / inverted ---
    t = "夜の海は静かだった。"
    img = new_page(640, 220, (15, 15, 20))
    draw_h_lines(img, [t], f36, fill=(230, 230, 235))
    save(img, "11_dark_background.png")
    add(
        "11_dark_background",
        "11_dark_background.png",
        expected_text=t,
        tags=["hard", "inverted"],
        notes="Light text on dark; some engines need preprocess.",
        min_overlap=0.5,
    )

    # --- 12 slight rotation / skew ---
    t = "机の上に手紙が残されていた。"
    img = new_page(700, 260)
    draw_h_lines(img, [t], f32, margin=50, start_y=90)
    img = img.rotate(7, expand=True, fillcolor=(255, 255, 250))
    save(img, "12_rotated.png")
    add(
        "12_rotated",
        "12_rotated.png",
        expected_text=t,
        tags=["hard", "skew", "tilt", "phone-capture", "moderate"],
        notes="~7° rotation like handheld phone shot (legacy id; see 31–40 tilt suite).",
        min_overlap=0.5,
    )

    # --- 13 English / non-novel best-effort ---
    t = "EXIT 12B  emergency only"
    img = new_page(640, 200)
    # Latin from same font (IPAex has Latin glyphs)
    draw_h_lines(img, [t], f28)
    save(img, "13_non_novel_english.png")
    add(
        "13_non_novel_english",
        "13_non_novel_english.png",
        expected_text=t,
        tags=["best-effort", "non-novel"],
        notes="Primary assumption is novel prose; non-novel is best-effort, no crash.",
        min_overlap=0.5,
    )

    # --- 14 JPEG heavily compressed ---
    t = "圧縮された写真でも読めるか。"
    img = new_page(640, 200)
    draw_h_lines(img, [t], f36)
    img = add_noise(img, amount=12)
    save(img, "14_jpeg_artifacts.jpg", fmt="JPEG", quality=12, optimize=True)
    add(
        "14_jpeg_artifacts",
        "14_jpeg_artifacts.jpg",
        expected_text=t,
        tags=["hard", "jpeg", "artifacts"],
        notes="Low-quality JPEG as phone might produce after re-encode.",
        min_overlap=0.4,
    )

    # --- 15 tiny image ---
    t = "小さい。"
    img = new_page(120, 48)
    f_tiny = load_font(font_path, 16)
    draw_h_lines(img, [t], f_tiny, margin=4, line_gap=2)
    save(img, "15_tiny.png")
    add(
        "15_tiny",
        "15_tiny.png",
        expected_text=t,
        tags=["edge", "tiny", "L2-smoke"],
        notes="Minimal valid image bytes for L2/L3 upload smoke.",
        min_overlap=0.3,
    )

    # --- 16 blur / out of focus ---
    t = "焦点が合っていないページ。"
    img = new_page(700, 220)
    draw_h_lines(img, [t], f36)
    img = img.filter(ImageFilter.GaussianBlur(radius=2.2))
    save(img, "16_blur.png")
    add(
        "16_blur",
        "16_blur.png",
        expected_text=t,
        tags=["hard", "blur"],
        notes="Soft focus; may need retake messaging in product.",
        min_overlap=0.3,
    )

    # --- 17 fullwidth punctuation / mixed scripts ---
    t = "「こんにちは」——彼は言った．"
    img = new_page(720, 200)
    draw_h_lines(img, [t], f36)
    save(img, "17_quotes_dash.png")
    add(
        "17_quotes_dash",
        "17_quotes_dash.png",
        expected_text=t,
        tags=["edge", "punctuation", "quotes"],
        notes="Japanese quotes and dash; OCR often mangles brackets.",
        min_overlap=0.5,
    )

    # --- 18 numbers + kanji (page-ish) ---
    t = "第3章　始まりの朝"
    img = new_page(640, 200)
    draw_h_lines(img, [t], f36)
    save(img, "18_chapter_heading.png")
    add(
        "18_chapter_heading",
        "18_chapter_heading.png",
        expected_text=t,
        tags=["happy", "heading", "numbers"],
        notes="Chapter-style heading common in novels.",
        min_overlap=0.7,
    )

    # --- 19 force-fail hook image (corrupt / not an image payload) ---
    # Valid tiny PNG that is blank — product "OCR fail" may use engine error hook;
    # also ship a non-image bytes file for HTTP reject paths.
    bad = IMG_DIR / "19_not_an_image.bin"
    bad.write_bytes(b"this is not an image payload for ingest error paths\n")
    add(
        "19_not_an_image",
        "19_not_an_image.bin",
        expected_text="",
        tags=["edge", "invalid", "error-path"],
        notes="Non-image bytes. Ingest should surface clear error; discard payload; queue unchanged.",
        want_success=False,
        min_overlap=None,
    )

    # --- 20 oversize marker (generate small stub + document 10MB rule; real oversize built in tests) ---
    # Ship a ~64KB noisy PNG as "large-ish" fixture; L1 oversize uses generated 10MB+ buffer in code.
    img = new_page(800, 600)
    draw_h_lines(img, ["大きさの確認用。", "実10MB超過はテスト内で生成。"], f28)
    img = add_noise(img, amount=40)
    save(img, "20_largeish_noise.png")
    add(
        "20_largeish_noise",
        "20_largeish_noise.png",
        expected_text="大きさの確認用。\n実10MB超過はテスト内で生成。",
        tags=["edge", "size", "not-oversize"],
        notes=(
            "Under 10 MB. True >10 MB reject uses in-test generated buffer "
            "(do not commit multi-MB blobs)."
        ),
        min_overlap=0.5,
    )

    # =====================================================================
    # Novel / bunkobon tate-gaki suite (vertical Japanese prose book style)
    # Prefer mincho (serif) when IPAexm or Noto Serif available.
    # =====================================================================
    font_mincho = font_path
    for cand in (
        os.environ.get("OCR_FONT_MINCHO", ""),
        "/tmp/IPAexfont00401/ipaexm.ttf",
        str(ROOT / "fonts" / "ipaexm.ttf"),
        font_path,
    ):
        if cand and Path(cand).is_file():
            font_mincho = cand
            break
    fm32 = load_font(font_mincho, 32)
    fm28 = load_font(font_mincho, 28)
    fm24 = load_font(font_mincho, 24)
    fm36 = load_font(font_mincho, 36)
    fm40 = load_font(font_mincho, 40)
    fm18 = load_font(font_mincho, 18)
    print(f"mincho: {font_mincho}")

    # --- 21 full bunkobon-like page: multi-column novel prose ---
    novel_prose = (
        "朝の光が障子を白く染めていた。"
        "彼は布団の中でしばらく天井を見つめてから、ゆっくりと上半身を起こした。"
        "昨日の出来事が、まだ夢の続きのように頭の中を巡っている。"
        "机の上には読みかけの本と、冷めた茶碗が置かれていた。"
    )
    cols = split_into_columns(novel_prose, chars_per_col=18)
    img = render_novel_vertical_page(
        cols,
        fm28,
        page_w=780,
        page_h=1100,
        col_gap=40,
        margin_top=80,
        margin_bottom=90,
        margin_right=70,
        page_num="137",
        page_num_font=fm18,
        rule_line=True,
    )
    img = paper_texture(img, amount=7, seed=21)
    save(img, "21_novel_vertical_page.png")
    add(
        "21_novel_vertical_page",
        "21_novel_vertical_page.png",
        expected_text=novel_prose,
        tags=["hard", "vertical", "tate-gaki", "novel", "bunkobon", "multi-column"],
        notes=(
            "Full synthetic bunkobon page: cream paper, mincho, columns right→left, "
            "top→bottom, footer page number. Primary novel OCR target layout."
        ),
        min_overlap=0.35,
    )

    # --- 22 dialogue-heavy vertical (「」 on tate page) ---
    dialogue = (
        "「今日はどこへ行くんだい」"
        "彼女は窓の外を見ながら言った。"
        "「まだ決めていない」"
        "彼は短く答えると、コートの襟を立てた。"
        "駅前はすでに人通りで賑わっていた。"
    )
    cols = split_into_columns(dialogue, chars_per_col=16)
    img = render_novel_vertical_page(
        cols,
        fm28,
        page_w=700,
        page_h=1000,
        col_gap=42,
        page_num="88",
        page_num_font=fm18,
    )
    img = paper_texture(img, amount=6, seed=22)
    save(img, "22_novel_vertical_dialogue.png")
    add(
        "22_novel_vertical_dialogue",
        "22_novel_vertical_dialogue.png",
        expected_text=dialogue,
        tags=["hard", "vertical", "tate-gaki", "novel", "dialogue", "quotes"],
        notes="Vertical novel dialogue with 「」. Quote glyphs often confuse OCR order.",
        min_overlap=0.3,
    )

    # --- 23 dense vertical (many columns, smaller type) ---
    dense_v = (
        "古い屋敷の奥にある書斎には、無数の本が積み上げられていた。"
        "埃っぽい空気の中で、時計だけが規則正しく時を刻んでいる。"
        "少年は息を潜め、禁じられた棚の一冊に手を伸ばした。"
        "表紙には誰も読めない文字が金色に輝いていたという。"
        "外では蝉の声が途切れることなく続いていた。"
    )
    cols = split_into_columns(dense_v, chars_per_col=22)
    img = render_novel_vertical_page(
        cols,
        fm24,
        page_w=820,
        page_h=1120,
        col_gap=34,
        margin_top=64,
        margin_bottom=72,
        margin_right=56,
        page_num="204",
        page_num_font=fm18,
    )
    img = paper_texture(img, amount=5, seed=23)
    save(img, "23_novel_vertical_dense.png")
    add(
        "23_novel_vertical_dense",
        "23_novel_vertical_dense.png",
        expected_text=dense_v,
        tags=["hard", "vertical", "tate-gaki", "novel", "dense", "small-font"],
        notes="Dense multi-column tate-gaki; phone crop often loses edge columns.",
        min_overlap=0.3,
    )

    # --- 24 chapter opening (vertical title + first lines) ---
    # Title as rightmost short column; body continues leftward.
    chapter_title = "第一章　旅立ち"
    chapter_body = (
        "列車は朝霧の中をゆっくりと走り始めた。"
        "窓に顔を寄せると、見知らぬ町の屋根が次々と流れていく。"
        "切符を握りしめた掌が、少し汗ばんでいた。"
    )
    body_cols = split_into_columns(chapter_body, chars_per_col=14)
    img = render_novel_vertical_page(
        body_cols,
        fm28,
        page_w=680,
        page_h=960,
        col_gap=48,
        margin_top=100,
        margin_right=130,  # leave room for title column
        page_num="3",
        page_num_font=fm18,
        rule_line=True,
    )
    draw_v_column(
        img,
        chapter_title,
        fm40,
        x=680 - 72,
        top=120,
        bottom=960 - 80,
        fill=(32, 28, 24),
    )
    img = paper_texture(img, amount=6, seed=24)
    save(img, "24_novel_vertical_chapter_open.png")
    add(
        "24_novel_vertical_chapter_open",
        "24_novel_vertical_chapter_open.png",
        expected_text=chapter_title + chapter_body,
        tags=["hard", "vertical", "tate-gaki", "novel", "chapter", "heading"],
        notes="Chapter opening: larger title column + body. Mixed sizes stress OCR.",
        min_overlap=0.3,
    )

    # --- 25 single column vertical strip (phone portrait crop of one line) ---
    strip = "彼は静かに本を閉じた。そして窓の外を見た。"
    img = render_novel_vertical_page(
        [strip],
        fm32,
        page_w=280,
        page_h=900,
        col_gap=40,
        margin_top=48,
        margin_bottom=48,
        margin_right=120,
        page_num=None,
    )
    img = paper_texture(img, amount=5, seed=25)
    save(img, "25_novel_vertical_single_col.png")
    add(
        "25_novel_vertical_single_col",
        "25_novel_vertical_single_col.png",
        expected_text=strip,
        tags=["hard", "vertical", "tate-gaki", "novel", "single-column", "phone-crop"],
        notes="One vertical column only — common tight phone crop of a novel page.",
        min_overlap=0.4,
    )

    # --- 26 vertical page photographed at slight angle (skew) ---
    skew_text = (
        "雨はいつまでも止む気配がなかった。"
        "傘の縁から落ちる水滴が、靴の先を黒く染めていく。"
        "それでも彼は歩き続けた。"
    )
    cols = split_into_columns(skew_text, chars_per_col=15)
    img = render_novel_vertical_page(
        cols,
        fm28,
        page_w=640,
        page_h=920,
        col_gap=40,
        page_num="56",
        page_num_font=fm18,
    )
    img = paper_texture(img, amount=8, seed=26)
    img = img.rotate(5, expand=True, fillcolor=(40, 40, 40))
    # letterbox dark like desk under book
    save(img, "26_novel_vertical_skewed.png")
    add(
        "26_novel_vertical_skewed",
        "26_novel_vertical_skewed.png",
        expected_text=skew_text,
        tags=["hard", "vertical", "tate-gaki", "novel", "skew", "tilt", "phone-capture", "moderate"],
        notes="Vertical novel page + ~5° handheld skew + dark desk border (see also 35–37).",
        min_overlap=0.25,
    )

    # --- 27 vertical punctuation-heavy (。、！？　) ---
    punct_v = (
        "本当か？信じられない！"
        "彼は首を振った。しかし、もう後には引けなかった。"
        "明日になれば、すべてが変わるだろう。"
    )
    cols = split_into_columns(punct_v, chars_per_col=14)
    img = render_novel_vertical_page(
        cols,
        fm32,
        page_w=600,
        page_h=880,
        col_gap=46,
        page_num="19",
        page_num_font=fm18,
    )
    img = paper_texture(img, amount=5, seed=27)
    save(img, "27_novel_vertical_punctuation.png")
    add(
        "27_novel_vertical_punctuation",
        "27_novel_vertical_punctuation.png",
        expected_text=punct_v,
        tags=["hard", "vertical", "tate-gaki", "novel", "punctuation", "splitSentences"],
        notes="Tate-gaki with 。！？、 — sentence split after OCR must still work.",
        min_overlap=0.3,
    )

    # --- 28 vertical stub-aligned sentences (bridge to analyzer fixtures) ---
    stub_v = "私は本を読む。病院に行った。"
    cols = split_into_columns(stub_v, chars_per_col=8)
    img = render_novel_vertical_page(
        cols,
        fm36,
        page_w=420,
        page_h=640,
        col_gap=50,
        page_num="1",
        page_num_font=fm18,
    )
    img = paper_texture(img, amount=4, seed=28)
    save(img, "28_novel_vertical_stub_sentences.png")
    add(
        "28_novel_vertical_stub_sentences",
        "28_novel_vertical_stub_sentences.png",
        expected_text=stub_v,
        tags=["hard", "vertical", "tate-gaki", "novel", "stub-aligned"],
        notes="Known stub analyzer sentences rendered as vertical novel text.",
        min_overlap=0.4,
    )

    # --- 29 vertical low-light / warm lamp (phone night reading) ---
    night = (
        "ランプの灯りだけが机を照らしていた。"
        "影が長く壁に伸び、文字の端がわずかににじんで見える。"
    )
    cols = split_into_columns(night, chars_per_col=14)
    img = render_novel_vertical_page(
        cols,
        fm28,
        page_w=620,
        page_h=900,
        bg=(255, 236, 200),
        fill=(55, 40, 25),
        col_gap=42,
        page_num="301",
        page_num_font=fm18,
    )
    img = paper_texture(img, amount=10, seed=29)
    img = _brightness(img, 0.85)
    save(img, "29_novel_vertical_warm_light.png")
    add(
        "29_novel_vertical_warm_light",
        "29_novel_vertical_warm_light.png",
        expected_text=night,
        tags=["hard", "vertical", "tate-gaki", "novel", "lighting"],
        notes="Warm lamp reading light on vertical page; contrast shift vs daylight scan.",
        min_overlap=0.25,
    )

    # --- 30 vertical + mild blur (soft phone AF miss on book) ---
    soft = (
        "風がページをめくった。"
        "彼は慌てて本を押さえ、読みかけの行を目で追った。"
    )
    cols = split_into_columns(soft, chars_per_col=13)
    img = render_novel_vertical_page(
        cols,
        fm28,
        page_w=600,
        page_h=860,
        col_gap=42,
        page_num="77",
        page_num_font=fm18,
    )
    img = paper_texture(img, amount=6, seed=30)
    img = img.filter(ImageFilter.GaussianBlur(radius=1.4))
    save(img, "30_novel_vertical_soft_focus.png")
    add(
        "30_novel_vertical_soft_focus",
        "30_novel_vertical_soft_focus.png",
        expected_text=soft,
        tags=["hard", "vertical", "tate-gaki", "novel", "blur"],
        notes="Vertical novel + mild defocus; retake vs edit safety net.",
        min_overlap=0.2,
    )

    # =====================================================================
    # Slightly tilted phone shots (handheld over page)
    # Tag: tilt — small angles only (not full landscape reorientation).
    # =====================================================================
    tilt_h_text = "机の上に手紙が残されていた。彼はそれを手に取った。"
    tilt_h_lines = [
        "机の上に手紙が残されていた。",
        "彼はそれを手に取った。",
        "窓の外では雨が降っている。",
    ]
    tilt_v_text = (
        "夕方の駅は通勤の人波で混雑していた。"
        "彼女は改札を抜け、ホームの端で電車を待った。"
    )

    def make_horizontal_page(lines: list[str], font: ImageFont.FreeTypeFont) -> Image.Image:
        img = new_page(720, 280, (250, 246, 236))
        draw_h_lines(img, lines, font, margin=36, line_gap=14)
        return paper_texture(img, amount=5, seed=hash(lines[0]) % 10_000)

    def make_vertical_page(text: str, font: ImageFont.FreeTypeFont, *, cpc: int = 14) -> Image.Image:
        cols = split_into_columns(text, chars_per_col=cpc)
        img = render_novel_vertical_page(
            cols,
            font,
            page_w=560,
            page_h=820,
            col_gap=40,
            page_num="42",
            page_num_font=fm18,
        )
        return paper_texture(img, amount=5, seed=hash(text[:8]) % 10_000)

    # --- 31 horizontal slight CW (~3°) ---
    img = phone_tilt(make_horizontal_page(tilt_h_lines, f28), -3.0)
    save(img, "31_tilt_h_slight_cw.png")
    add(
        "31_tilt_h_slight_cw",
        "31_tilt_h_slight_cw.png",
        expected_text="\n".join(tilt_h_lines),
        tags=["hard", "tilt", "phone-capture", "horizontal", "slight"],
        notes="Horizontal prose, ~3° clockwise tilt + desk border. Typical slight misalign.",
        min_overlap=0.55,
    )

    # --- 32 horizontal slight CCW (~3°) ---
    img = phone_tilt(make_horizontal_page(tilt_h_lines, f28), 3.0)
    save(img, "32_tilt_h_slight_ccw.png")
    add(
        "32_tilt_h_slight_ccw",
        "32_tilt_h_slight_ccw.png",
        expected_text="\n".join(tilt_h_lines),
        tags=["hard", "tilt", "phone-capture", "horizontal", "slight"],
        notes="Horizontal prose, ~3° counter-clockwise. Pair with 31 for both lean directions.",
        min_overlap=0.55,
    )

    # --- 33 horizontal moderate (~8°) ---
    img = phone_tilt(make_horizontal_page(tilt_h_lines, f28), -8.0)
    save(img, "33_tilt_h_moderate.png")
    add(
        "33_tilt_h_moderate",
        "33_tilt_h_moderate.png",
        expected_text="\n".join(tilt_h_lines),
        tags=["hard", "tilt", "phone-capture", "horizontal", "moderate"],
        notes="Horizontal prose ~8° tilt. Still 'slight' handheld, harder for deskew-less OCR.",
        min_overlap=0.4,
    )

    # --- 34 horizontal stronger (~15°) still phone-shot not rotated layout ---
    img = phone_tilt(make_horizontal_page([tilt_h_text], f32), 15.0)
    save(img, "34_tilt_h_strong.png")
    add(
        "34_tilt_h_strong",
        "34_tilt_h_strong.png",
        expected_text=tilt_h_text,
        tags=["hard", "tilt", "phone-capture", "horizontal", "strong"],
        notes="~15° tilt. Upper bound of 'slightly tilted' phone shot before re-shoot.",
        min_overlap=0.3,
    )

    # --- 35 vertical novel slight CW (~3°) ---
    img = phone_tilt(make_vertical_page(tilt_v_text, fm28), -3.0, pad=28)
    save(img, "35_tilt_v_slight_cw.png")
    add(
        "35_tilt_v_slight_cw",
        "35_tilt_v_slight_cw.png",
        expected_text=tilt_v_text,
        tags=["hard", "tilt", "phone-capture", "vertical", "tate-gaki", "novel", "slight"],
        notes="Tate-gaki bunkobon page ~3° CW. Combines vertical layout + small tilt.",
        min_overlap=0.3,
    )

    # --- 36 vertical novel slight CCW (~3°) ---
    img = phone_tilt(make_vertical_page(tilt_v_text, fm28), 3.0, pad=28)
    save(img, "36_tilt_v_slight_ccw.png")
    add(
        "36_tilt_v_slight_ccw",
        "36_tilt_v_slight_ccw.png",
        expected_text=tilt_v_text,
        tags=["hard", "tilt", "phone-capture", "vertical", "tate-gaki", "novel", "slight"],
        notes="Tate-gaki ~3° CCW. Pair with 35.",
        min_overlap=0.3,
    )

    # --- 37 vertical novel moderate (~7°) ---
    img = phone_tilt(make_vertical_page(tilt_v_text, fm28, cpc=13), -7.0, pad=32)
    save(img, "37_tilt_v_moderate.png")
    add(
        "37_tilt_v_moderate",
        "37_tilt_v_moderate.png",
        expected_text=tilt_v_text,
        tags=["hard", "tilt", "phone-capture", "vertical", "tate-gaki", "novel", "moderate"],
        notes="Vertical novel ~7° tilt. Common couch/phone capture error.",
        min_overlap=0.25,
    )

    # --- 38 horizontal multi-sentence + slight tilt (splitSentences path) ---
    multi_tilt = [
        "病院に行った。",
        "今日は雨だ。",
        "私は本を読む。",
    ]
    img = phone_tilt(make_horizontal_page(multi_tilt, f32), -4.5)
    save(img, "38_tilt_h_multi_sentence.png")
    add(
        "38_tilt_h_multi_sentence",
        "38_tilt_h_multi_sentence.png",
        expected_text="\n".join(multi_tilt),
        tags=["hard", "tilt", "phone-capture", "horizontal", "multi-sentence", "slight", "stub-aligned"],
        notes="Multi-sentence horizontal + ~4.5° tilt; OCR then splitSentences.",
        min_overlap=0.45,
    )

    # --- 39 tilt + mild blur (compound phone fail) ---
    img = make_horizontal_page(
        ["焦点も傾きも少しずれている。"],
        f32,
    )
    img = phone_tilt(img, 6.0)
    img = img.filter(ImageFilter.GaussianBlur(radius=1.1))
    save(img, "39_tilt_h_with_blur.png")
    add(
        "39_tilt_h_with_blur",
        "39_tilt_h_with_blur.png",
        expected_text="焦点も傾きも少しずれている。",
        tags=["hard", "tilt", "phone-capture", "horizontal", "blur", "compound"],
        notes="Slight tilt (~6°) plus soft focus. Compound capture defect.",
        min_overlap=0.25,
    )

    # --- 40 vertical dialogue + slight tilt ---
    dial_tilt = (
        "「まだ帰らないの」"
        "「もう少しだけ」"
        "二人は黙って夕暮れを見ていた。"
    )
    img = phone_tilt(make_vertical_page(dial_tilt, fm28, cpc=12), 4.0, pad=30)
    save(img, "40_tilt_v_dialogue.png")
    add(
        "40_tilt_v_dialogue",
        "40_tilt_v_dialogue.png",
        expected_text=dial_tilt,
        tags=["hard", "tilt", "phone-capture", "vertical", "tate-gaki", "novel", "dialogue", "slight"],
        notes="Vertical dialogue 「」 with ~4° tilt.",
        min_overlap=0.25,
    )

    # =====================================================================
    # Brightness suite (global + mixed within one shot)
    # =====================================================================
    bright_lines = [
        "明るさの違う写真でも読めるか。",
        "彼はページをめくって読み続けた。",
        "窓辺の光が文字を白く飛ばしている。",
    ]
    bright_expected = "\n".join(bright_lines)

    def base_bright_page() -> Image.Image:
        return make_horizontal_page(bright_lines, f32)

    # --- 41 dim overall ---
    img = apply_brightness(base_bright_page(), 0.45)
    save(img, "41_brightness_dim.png")
    add(
        "41_brightness_dim",
        "41_brightness_dim.png",
        expected_text=bright_expected,
        tags=["hard", "brightness", "dim", "horizontal"],
        notes="Globally underexposed (~0.45). Phone in dark room.",
        min_overlap=0.4,
    )

    # --- 42 bright / overexposed overall ---
    img = apply_brightness(base_bright_page(), 1.55)
    save(img, "42_brightness_bright.png")
    add(
        "42_brightness_bright",
        "42_brightness_bright.png",
        expected_text=bright_expected,
        tags=["hard", "brightness", "bright", "horizontal"],
        notes="Globally overexposed (~1.55). Window glare style.",
        min_overlap=0.4,
    )

    # --- 43 very dark ---
    img = apply_brightness(base_bright_page(), 0.28)
    save(img, "43_brightness_very_dark.png")
    add(
        "43_brightness_very_dark",
        "43_brightness_very_dark.png",
        expected_text=bright_expected,
        tags=["hard", "brightness", "dim", "horizontal"],
        notes="Very dark capture; may need retake messaging.",
        min_overlap=0.25,
    )

    # --- 44 left dark / right bright (split mixed) ---
    img = split_brightness_vertical(base_bright_page(), left_factor=0.4, right_factor=1.35)
    save(img, "44_brightness_mixed_lr.png")
    add(
        "44_brightness_mixed_lr",
        "44_brightness_mixed_lr.png",
        expected_text=bright_expected,
        tags=["hard", "brightness", "mixed", "horizontal", "split"],
        notes="Single shot: left half dark, right half bright. Hard shadow edge.",
        min_overlap=0.35,
    )

    # --- 45 top bright / bottom dark ---
    img = split_brightness_horizontal(base_bright_page(), top_factor=1.4, bottom_factor=0.42)
    save(img, "45_brightness_mixed_tb.png")
    add(
        "45_brightness_mixed_tb",
        "45_brightness_mixed_tb.png",
        expected_text=bright_expected,
        tags=["hard", "brightness", "mixed", "horizontal", "split"],
        notes="Top overexposed, bottom underexposed in one frame.",
        min_overlap=0.35,
    )

    # --- 46 smooth gradient left-dark → right-bright ---
    img = gradient_brightness(base_bright_page(), left_factor=0.35, right_factor=1.45)
    save(img, "46_brightness_gradient.png")
    add(
        "46_brightness_gradient",
        "46_brightness_gradient.png",
        expected_text=bright_expected,
        tags=["hard", "brightness", "mixed", "horizontal", "gradient"],
        notes="Smooth lighting falloff across page (window + room shadow).",
        min_overlap=0.35,
    )

    # --- 47 vertical novel + mixed LR brightness ---
    v_bright = (
        "光と影がページを斜めに横切っていた。"
        "それでも彼は黙って読み進めた。"
    )
    img = split_brightness_vertical(
        make_vertical_page(v_bright, fm28, cpc=12),
        left_factor=0.38,
        right_factor=1.4,
    )
    save(img, "47_brightness_mixed_vertical.png")
    add(
        "47_brightness_mixed_vertical",
        "47_brightness_mixed_vertical.png",
        expected_text=v_bright,
        tags=["hard", "brightness", "mixed", "vertical", "tate-gaki", "novel"],
        notes="Tate-gaki page with left-dark/right-bright mixed lighting.",
        min_overlap=0.25,
    )

    # =====================================================================
    # Intra-shot variation: partial blur, mixed font, thickness, colour
    # =====================================================================

    # --- 48 partial blur (bottom lines soft, top sharp) ---
    partial_blur_lines = [
        "上の行ははっきり見える。",
        "中ほどの行もまだ読める。",
        "下の行だけ焦点が甘い。",
    ]
    img = make_horizontal_page(partial_blur_lines, f32)
    w, h = img.size
    img = blur_region(img, (0, h // 2, w, h), radius=2.4)
    save(img, "48_partial_blur_bottom.png")
    add(
        "48_partial_blur_bottom",
        "48_partial_blur_bottom.png",
        expected_text="\n".join(partial_blur_lines),
        tags=["hard", "blur", "partial", "mixed", "horizontal"],
        notes="Same shot: top sharp, bottom half blurred (field curvature / AF miss).",
        min_overlap=0.35,
    )

    # --- 49 partial blur (center band only) ---
    center_lines = [
        "端の文字は鮮明だ。",
        "中央だけが少し滲んでいる。",
        "再び端は鮮明に戻る。",
    ]
    img = make_horizontal_page(center_lines, f32)
    w, h = img.size
    img = blur_region(img, (0, int(h * 0.28), w, int(h * 0.72)), radius=2.0)
    save(img, "49_partial_blur_center_band.png")
    add(
        "49_partial_blur_center_band",
        "49_partial_blur_center_band.png",
        expected_text="\n".join(center_lines),
        tags=["hard", "blur", "partial", "mixed", "horizontal"],
        notes="Only middle horizontal band blurred; edges stay sharp.",
        min_overlap=0.35,
    )

    # --- 50 mixed fonts (gothic + mincho lines in one page) ---
    mix_font_lines = [
        ("ゴシック体の見出し行。", f32, (20, 20, 20), False),
        ("明朝体で本文が続く。", fm32, (20, 20, 20), False),
        ("またゴシックに戻る行。", f28, (20, 20, 20), False),
        ("最後も明朝で締める。", fm28, (20, 20, 20), False),
    ]
    img = new_page(760, 320, (250, 246, 236))
    y = 36
    for text, font, fill, bold in mix_font_lines:
        y = draw_h_line_styled(img, text, font, x=36, y=y, fill=fill, bold=bold)
    img = paper_texture(img, amount=4, seed=50)
    save(img, "50_mixed_fonts.png")
    add(
        "50_mixed_fonts",
        "50_mixed_fonts.png",
        expected_text="\n".join(t for t, *_ in mix_font_lines),
        tags=["hard", "mixed", "font", "horizontal"],
        notes="Gothic + mincho alternating lines in one shot.",
        min_overlap=0.45,
    )

    # --- 51 mixed thickness (regular + faux-bold) ---
    thick_lines = [
        ("細い字の普通の行。", f32, (20, 20, 20), False),
        ("太い字で強調された行。", f32, (20, 20, 20), True),
        ("再び細い字に戻る。", f32, (20, 20, 20), False),
        ("もう一度太い見出し風。", f36, (20, 20, 20), True),
    ]
    img = new_page(760, 340, (250, 246, 236))
    y = 36
    for text, font, fill, bold in thick_lines:
        y = draw_h_line_styled(img, text, font, x=36, y=y, fill=fill, bold=bold)
    img = paper_texture(img, amount=4, seed=51)
    save(img, "51_mixed_thickness.png")
    add(
        "51_mixed_thickness",
        "51_mixed_thickness.png",
        expected_text="\n".join(t for t, *_ in thick_lines),
        tags=["hard", "mixed", "thickness", "horizontal"],
        notes="Regular + faux-bold strokes in same page.",
        min_overlap=0.45,
    )

    # --- 52 mixed ink colours ---
    color_lines = [
        ("黒い本文の文字。", f32, (20, 20, 20), False),
        ("青い注釈のような行。", f32, (25, 55, 160), False),
        ("赤い強調の一言。", f32, (170, 30, 30), False),
        ("灰色で薄いメモ書き。", f28, (120, 120, 120), False),
    ]
    img = new_page(760, 320, (250, 246, 236))
    y = 36
    for text, font, fill, bold in color_lines:
        y = draw_h_line_styled(img, text, font, x=36, y=y, fill=fill, bold=bold)
    img = paper_texture(img, amount=4, seed=52)
    save(img, "52_mixed_colours.png")
    add(
        "52_mixed_colours",
        "52_mixed_colours.png",
        expected_text="\n".join(t for t, *_ in color_lines),
        tags=["hard", "mixed", "colour", "horizontal"],
        notes="Black / blue / red / gray ink mixed in one shot.",
        min_overlap=0.4,
    )

    # --- 53 compound mixed: font + thickness + colour ---
    compound = [
        ("章タイトルは太い黒。", f36, (15, 15, 15), True),
        ("本文は明朝の細い黒。", fm28, (30, 28, 24), False),
        ("傍注は細い青字。", f22, (40, 70, 150), False),
        ("注意書きは赤い太字。", f32, (160, 25, 25), True),
    ]
    img = new_page(800, 360, (250, 246, 236))
    y = 32
    for text, font, fill, bold in compound:
        y = draw_h_line_styled(img, text, font, x=36, y=y, fill=fill, bold=bold)
    img = paper_texture(img, amount=5, seed=53)
    save(img, "53_mixed_font_thickness_colour.png")
    add(
        "53_mixed_font_thickness_colour",
        "53_mixed_font_thickness_colour.png",
        expected_text="\n".join(t for t, *_ in compound),
        tags=["hard", "mixed", "font", "thickness", "colour", "horizontal", "compound"],
        notes="Font face + stroke weight + ink colour all vary in one shot.",
        min_overlap=0.35,
    )

    # --- 54 mixed style + partial blur ---
    style_blur = [
        ("鮮明な太い見出し。", f36, (20, 20, 20), True),
        ("鮮明な本文の一行。", fm28, (25, 25, 25), False),
        ("ここから下は少し滲む。", f32, (30, 30, 30), False),
        ("色も青く薄い注記。", f28, (50, 80, 160), False),
    ]
    img = new_page(800, 360, (250, 246, 236))
    y = 32
    for text, font, fill, bold in style_blur:
        y = draw_h_line_styled(img, text, font, x=36, y=y, fill=fill, bold=bold)
    img = paper_texture(img, amount=4, seed=54)
    w, h = img.size
    img = blur_region(img, (0, int(h * 0.45), w, h), radius=2.2)
    save(img, "54_mixed_style_partial_blur.png")
    add(
        "54_mixed_style_partial_blur",
        "54_mixed_style_partial_blur.png",
        expected_text="\n".join(t for t, *_ in style_blur),
        tags=["hard", "mixed", "blur", "partial", "font", "thickness", "colour", "compound"],
        notes="Style mix plus bottom half soft focus — multi-defect single shot.",
        min_overlap=0.3,
    )

    # --- 55 mixed brightness + mixed colour + partial blur ---
    kitchen_sink_lines = [
        "光の当たる行は白い。",
        "影の中の行は暗い。",
        "色付きの行も混ざる。",
    ]
    img = new_page(760, 300, (250, 246, 236))
    y = 40
    fills = [(20, 20, 20), (20, 20, 20), (150, 40, 40)]
    for text, fill in zip(kitchen_sink_lines, fills, strict=True):
        y = draw_h_line_styled(img, text, f32, x=40, y=y, fill=fill)
    img = paper_texture(img, amount=5, seed=55)
    img = gradient_brightness(img, left_factor=0.4, right_factor=1.35)
    w, h = img.size
    img = blur_region(img, (int(w * 0.55), 0, w, h), radius=1.8)
    save(img, "55_mixed_brightness_colour_blur.png")
    add(
        "55_mixed_brightness_colour_blur",
        "55_mixed_brightness_colour_blur.png",
        expected_text="\n".join(kitchen_sink_lines),
        tags=["hard", "mixed", "brightness", "colour", "blur", "partial", "compound"],
        notes="Gradient lighting + red ink line + right-side partial blur.",
        min_overlap=0.25,
    )

    # Sort by id for stability
    cases.sort(key=lambda c: c["id"])

    manifest = {
        "version": 1,
        "description": (
            "Synthetic OCR fixtures for novel-miner ticket 06+. "
            "L1 product rules use fake OcrEngine; these images support adapter "
            "contract tests, L2 multipart smoke, and L3 upload journeys. "
            "Includes tate-gaki, phone-tilt, brightness, and intra-shot mixed style suites."
        ),
        "font_note": "Generated with IPAex/Noto CJK; font not required at test runtime.",
        "max_upload_bytes": 10 * 1024 * 1024,
        "cases": cases,
    }
    CASES_PATH.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {len(cases)} cases -> {CASES_PATH}")
    for c in cases:
        p = ROOT / c["file"]
        sz = p.stat().st_size if p.exists() else 0
        print(f"  {c['id']:28} {sz:7} B  tags={','.join(c['tags'][:3])}")


def _brightness(img: Image.Image, factor: float) -> Image.Image:
    from PIL import ImageEnhance

    return ImageEnhance.Brightness(img).enhance(factor)


if __name__ == "__main__":
    main()
