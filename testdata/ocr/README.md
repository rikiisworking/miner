# OCR fixtures (ticket 06+)

Synthetic **page-image** cases for local OCR ingest. Primary material = novel prose; non-novel is best-effort.

| Layer | How these are used |
|-------|--------------------|
| **L1** MiningApp | Real `ocr.Tesseract` + fixture images (size/busy still pure rules). |
| **L2** HTTP | Multipart upload of fixture images (`02_multi_sentence`, `19_not_an_image`, oversize buffer). |
| **L3** UI | File-input journey with happy image + non-image error fixture. |
| **Adapter contract** (optional) | `MINER_OCR_CONTRACT=1`: suites by tag — happy, vertical, blur, brightness, font, thickness, colour — vs `expected_text` / `min_overlap`. Known-weak IDs soft-log only. |

## Layout

```
testdata/ocr/
  cases.json          # manifest (source of truth for case list)
  gen_fixtures.py     # regenerate images (needs CJK TTF)
  images/             # PNG/JPEG/bin payloads
  README.md
```

## Cases (55)

### Horizontal / general

| id | tags | intent |
|----|------|--------|
| `01_single_sentence` | happy, stub-aligned | `私は本を読む。` |
| `02_multi_sentence` | multi-sentence | feeds `splitSentences` after OCR |
| `03_paragraph` | paragraph | denser prose lines |
| `05_punctuation_mix` | 。！？ | terminator coverage |
| `06_blank` | empty | no glyphs |
| `07_noise_only` | noise | no readable prose |
| `08_low_contrast` | hard | gray-on-gray |
| `09_dense_small` | dense | small font block |
| `10_large_sparse` | easy | few large glyphs |
| `11_dark_background` | inverted | light on dark |
| `12_rotated` | skew | ~7° phone tilt |
| `13_non_novel_english` | best-effort | non-novel material |
| `14_jpeg_artifacts` | jpeg | heavy compression |
| `15_tiny` | L2-smoke | minimal valid image |
| `16_blur` | blur | soft focus |
| `17_quotes_dash` | quotes | 「」——． |
| `18_chapter_heading` | heading | 第3章 style |
| `19_not_an_image` | error-path | non-image bytes |
| `20_largeish_noise` | size | under 10 MB; real oversize built in tests |

### Novel tate-gaki (縦書き bunkobon style)

Cream paper, mincho when available, columns **right→left**, glyphs **top→bottom**, footer page numbers.

| id | tags | intent |
|----|------|--------|
| `04_vertical_columns` | tate-gaki | short two-column page |
| `21_novel_vertical_page` | novel, multi-column | full bunkobon-like prose page |
| `22_novel_vertical_dialogue` | novel, dialogue | 「」 vertical dialogue |
| `23_novel_vertical_dense` | novel, dense | many columns, smaller type |
| `24_novel_vertical_chapter_open` | novel, chapter | title column + body |
| `25_novel_vertical_single_col` | novel, phone-crop | one column strip |
| `26_novel_vertical_skewed` | novel, skew | handheld ~5° + desk border |
| `27_novel_vertical_punctuation` | novel, 。！？ | splitSentences after OCR |
| `28_novel_vertical_stub_sentences` | novel, stub-aligned | known analyzer sentences vertical |
| `29_novel_vertical_warm_light` | novel, lighting | warm lamp night reading |
| `30_novel_vertical_soft_focus` | novel, blur | mild AF miss on book |

Filter in Go: `m.WithTag("tate-gaki")` or `m.WithTag("novel")`.

### Slight phone tilt (handheld)

Desk letterbox + small rotation. Tag: `tilt`. Angles are approximate.

| id | layout | ~angle | notes |
|----|--------|--------|-------|
| `12_rotated` | horizontal | 7° | legacy |
| `26_novel_vertical_skewed` | vertical | 5° | legacy |
| `31_tilt_h_slight_cw` | horizontal | 3° CW | slight |
| `32_tilt_h_slight_ccw` | horizontal | 3° CCW | slight |
| `33_tilt_h_moderate` | horizontal | 8° | moderate |
| `34_tilt_h_strong` | horizontal | 15° | upper “slight” bound |
| `35_tilt_v_slight_cw` | vertical | 3° CW | tate-gaki |
| `36_tilt_v_slight_ccw` | vertical | 3° CCW | tate-gaki |
| `37_tilt_v_moderate` | vertical | 7° | tate-gaki |
| `38_tilt_h_multi_sentence` | horizontal | 4.5° | multi-sentence |
| `39_tilt_h_with_blur` | horizontal | 6° + blur | compound |
| `40_tilt_v_dialogue` | vertical | 4° | 「」 dialogue |

Filter: `m.WithTag("tilt")` or `m.WithTag("slight")`.

### Brightness (global + mixed in one shot)

| id | kind |
|----|------|
| `41_brightness_dim` | global dim |
| `42_brightness_bright` | global bright |
| `43_brightness_very_dark` | very dark |
| `44_brightness_mixed_lr` | left dark / right bright |
| `45_brightness_mixed_tb` | top bright / bottom dark |
| `46_brightness_gradient` | smooth LR gradient |
| `47_brightness_mixed_vertical` | tate-gaki + LR mix |

### Intra-shot style mix (blur / font / thickness / colour)

| id | kind |
|----|------|
| `48_partial_blur_bottom` | bottom half soft |
| `49_partial_blur_center_band` | middle band soft |
| `50_mixed_fonts` | gothic + mincho lines |
| `51_mixed_thickness` | regular + bold |
| `52_mixed_colours` | black / blue / red / gray |
| `53_mixed_font_thickness_colour` | all three style axes |
| `54_mixed_style_partial_blur` | style + partial blur |
| `55_mixed_brightness_colour_blur` | light + colour + blur |

Filter: `m.WithTag("brightness")`, `m.WithTag("mixed")`, `m.WithTag("partial")`.

**>10 MB reject:** do not commit multi-MB blobs. L1/L2 build a `[]byte` larger than `max_upload_bytes` (10 MiB) in the test.

## Regenerate

```bash
# IPAexGothic + Mincho (novel body) example — fonts not committed:
OCR_FONT=/path/to/ipaexg.ttf \
OCR_FONT_MINCHO=/path/to/ipaexm.ttf \
  python3 testdata/ocr/gen_fixtures.py
```

Runtime tests only need `cases.json` + `images/` — not the font.

## Go helper

```go
import "github.com/rikiisworking/miner/internal/ocrtest"

m, err := ocrtest.Load()
c := m.Must("01_single_sentence")
b, err := c.Bytes()
```

Contract tests (real engine) skip unless `MINER_OCR_CONTRACT=1`:

```bash
export MINER_OCR_CONTRACT=1
# MINER_TESSERACT / MINER_TESSDATA_PREFIX if not on PATH
go test ./internal/adapters/ocr/ -run 'Contract' -count=1 -timeout 5m -v
```

Soft (log-only) under default engine: some vertical columns, strong tilt+blur compounds, mixed lighting extremes — see `contractSoftIDs` in `tesseract_test.go`.
