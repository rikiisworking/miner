# NDLOCR-Lite spike notes (2026-07-28)

## Setup

- Clone: `/home/nick/src/ndlocr-lite` (from github.com/ndl-lab/ndlocr-lite)
- Python: 3.12 venv via uv (`/tmp/ndlocr-venv` during spike)
- Deps: onnxruntime 1.26, opencv-headless, pillow, numpy, … (see `requirements-ocr.txt`)

## Fixture results (stock CLI, cold start ~6–7s each)

| ID | Result (summary) |
|----|------------------|
| `01_single_sentence` | exact `私は本を読む。` |
| `02_multi_sentence` | exact three sentences |
| `21_novel_vertical_page` | strong novel prose, non-empty |
| `25_novel_vertical_single_col` | near-exact (minor 本→木 once) |
| `28_novel_vertical_stub_sentences` | both stems present |
| `16_blur` | readable |
| `34_tilt_h_strong` | partial |
| `41_brightness_dim` | good |
| `44_brightness_mixed_lr` | partial / soft |

Warm detect+recog on simple page ≈ **1.1s** after models loaded.

## Worker strategy

**In-process import** of `get_detector` / `get_recognizer` / `_run_ocr_on_image_array` in long-lived `scripts/ndl_ocr_worker.py`.  
JSON lines protocol. No per-request model reload.

## Defaults

- Device: cpu
- enable-tcy: off
- Soft contract IDs: mixed lighting / compound blur-tilt

## License

NDLOCR-Lite CC BY 4.0 — attribute NDL Lab in README / CONTEXT.
