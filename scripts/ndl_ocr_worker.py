#!/usr/bin/env python3
"""Long-lived NDLOCR-Lite worker for miner OcrEngine.

Protocol (line-oriented JSON on stdin/stdout):
  → {"id":"1","image_path":"/path/to.png"}
  ← {"id":"1","ok":true,"text":"..."}
  ← {"id":"1","ok":false,"error":"..."}

On startup, after models load:
  ← {"ready":true}

Env:
  MINER_NDL_ROOT   required — clone of github.com/ndl-lab/ndlocr-lite
  MINER_NDL_DEVICE optional — cpu (default) or cuda
  MINER_NDL_ENABLE_TCY optional — 1 to enable 縦中横 helper
"""

from __future__ import annotations

import json
import os
import sys
import tempfile
import traceback
from argparse import Namespace
from pathlib import Path


def _fail_startup(msg: str) -> None:
    sys.stderr.write(f"ndl_ocr_worker: {msg}\n")
    sys.stderr.flush()
    sys.exit(1)


def _ndl_root() -> Path:
    root = os.environ.get("MINER_NDL_ROOT", "").strip()
    if not root:
        _fail_startup("MINER_NDL_ROOT is required (path to ndlocr-lite clone)")
    p = Path(root).expanduser().resolve()
    if not (p / "src" / "ocr.py").is_file():
        _fail_startup(f"MINER_NDL_ROOT={p} missing src/ocr.py")
    return p


def _build_args(src: Path) -> Namespace:
    device = os.environ.get("MINER_NDL_DEVICE", "cpu").strip() or "cpu"
    if device not in ("cpu", "cuda"):
        _fail_startup(f"MINER_NDL_DEVICE must be cpu or cuda, got {device!r}")
    # NDLOCR-Lite has no Metal/CoreML path; CUDA is Linux/Windows GPU only.
    if device == "cuda" and sys.platform == "darwin":
        _fail_startup(
            "MINER_NDL_DEVICE=cuda is not supported on macOS; use cpu (default)"
        )
    enable_tcy = os.environ.get("MINER_NDL_ENABLE_TCY", "").strip() in (
        "1",
        "true",
        "TRUE",
        "yes",
        "YES",
    )
    return Namespace(
        det_weights=str(src / "model" / "deim-s-1024x1024.onnx"),
        det_classes=str(src / "config" / "ndl.yaml"),
        det_score_threshold=0.2,
        det_conf_threshold=0.25,
        det_iou_threshold=0.2,
        rec_weights30=str(
            src
            / "model"
            / "parseq-ndl-24x256-30-tiny-189epoch-tegaki3-r8data-202604.onnx"
        ),
        rec_weights50=str(
            src
            / "model"
            / "parseq-ndl-24x384-50-tiny-300epoch-tegaki3-r8data-202604.onnx"
        ),
        rec_weights=str(
            src
            / "model"
            / "parseq-ndl-24x768-100-tiny-153epoch-tegaki3-r8data-202604.onnx"
        ),
        rec_classes=str(src / "config" / "NDLmoji.yaml"),
        device=device,
        enable_tcy=enable_tcy,
        viz=False,
        json_only=True,
        output=tempfile.gettempdir(),
    )


def _load_engine(src: Path):
    sys.path.insert(0, str(src))
    # NDLOCR modules import siblings (deim, parseq, …) from cwd-less path.
    os.chdir(src)
    from ocr import (  # type: ignore  # noqa: E402
        _run_ocr_on_image_array,
        get_detector,
        get_recognizer,
    )

    args = _build_args(src)
    detector = get_detector(args)
    recognizer100 = get_recognizer(args=args)
    recognizer30 = get_recognizer(args=args, weights_path=args.rec_weights30)
    recognizer50 = get_recognizer(args=args, weights_path=args.rec_weights50)
    return {
        "detector": detector,
        "recognizer30": recognizer30,
        "recognizer50": recognizer50,
        "recognizer100": recognizer100,
        "run": _run_ocr_on_image_array,
        "args": args,
    }


def _recognize(engine, image_path: str) -> str:
    from PIL import Image
    import numpy as np

    path = Path(image_path)
    if not path.is_file():
        raise FileNotFoundError(f"image not found: {image_path}")

    pil_image = Image.open(path).convert("RGB")
    img = np.array(pil_image)
    out_dir = tempfile.mkdtemp(prefix="miner-ndl-")
    try:
        result = engine["run"](
            detector=engine["detector"],
            recognizer30=engine["recognizer30"],
            recognizer50=engine["recognizer50"],
            recognizer100=engine["recognizer100"],
            inputname=path.name,
            img=img,
            outputpath=out_dir,
            save_viz=False,
        )
    finally:
        # _run may not write files when save_viz=False; still clean dir.
        try:
            os.rmdir(out_dir)
        except OSError:
            for child in Path(out_dir).glob("*"):
                try:
                    child.unlink()
                except OSError:
                    pass
            try:
                os.rmdir(out_dir)
            except OSError:
                pass

    text = result.get("text") or ""
    if not isinstance(text, str):
        text = str(text)
    return text.strip()


def _emit(obj: dict) -> None:
    sys.stdout.write(json.dumps(obj, ensure_ascii=False) + "\n")
    sys.stdout.flush()


def main() -> int:
    root = _ndl_root()
    src = root / "src"
    try:
        engine = _load_engine(src)
    except Exception as exc:  # noqa: BLE001 — surface load errors to Go
        _fail_startup(f"model load failed: {exc}")
        return 1

    _emit({"ready": True})

    for raw in sys.stdin:
        line = raw.strip()
        if not line:
            continue
        req_id = ""
        try:
            req = json.loads(line)
            if not isinstance(req, dict):
                _emit({"id": "", "ok": False, "error": "request must be a JSON object"})
                continue
            req_id = str(req.get("id", ""))
            image_path = req.get("image_path")
            if not image_path or not isinstance(image_path, str):
                _emit({"id": req_id, "ok": False, "error": "image_path required"})
                continue
            text = _recognize(engine, image_path)
            _emit({"id": req_id, "ok": True, "text": text})
        except Exception as exc:  # noqa: BLE001 — per-request isolation
            _emit(
                {
                    "id": req_id,
                    "ok": False,
                    "error": f"{type(exc).__name__}: {exc}",
                    "trace": traceback.format_exc(limit=3),
                }
            )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
