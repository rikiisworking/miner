#!/usr/bin/env bash
# Idempotent NDLOCR-Lite install for miner photo OCR.
# Linux and macOS (Intel + Apple Silicon). Default root: $PWD/.deps/ndlocr-lite
# (override with MINER_NDL_ROOT).
#
# Requires: git, and either uv (recommended) or python3.12 / 3.11 / 3.10.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NDL_ROOT="${MINER_NDL_ROOT:-$REPO_ROOT/.deps/ndlocr-lite}"
REQS="$REPO_ROOT/requirements-ocr.txt"
WORKER="$REPO_ROOT/scripts/ndl_ocr_worker.py"
NDL_REPO_URL="${MINER_NDL_REPO_URL:-https://github.com/ndl-lab/ndlocr-lite.git}"
# Preferred interpreter major.minor; install tries this then fallbacks.
PYTHON_VERSION="${MINER_NDL_PYTHON_VERSION:-3.12}"
UPDATE="${OCR_UPDATE:-0}"

OS_NAME="$(uname -s 2>/dev/null || echo unknown)"
ARCH_NAME="$(uname -m 2>/dev/null || echo unknown)"

log() { printf 'ocr-install: %s\n' "$*"; }
die() { printf 'ocr-install: ERROR: %s\n' "$*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

# True on macOS (Darwin).
is_darwin() {
  [[ "$OS_NAME" == "Darwin" ]]
}

health_ok() {
  local py="$NDL_ROOT/.venv/bin/python"
  [[ -f "$NDL_ROOT/src/ocr.py" ]] || return 1
  # -x can be flaky on some network FS; require file + runnable import instead.
  [[ -f "$py" ]] || return 1
  "$py" -c "import onnxruntime, cv2, PIL, yaml, numpy" >/dev/null 2>&1
}

clone_or_update() {
  if [[ -d "$NDL_ROOT/.git" ]]; then
    if [[ "$UPDATE" == "1" ]]; then
      log "updating existing clone at $NDL_ROOT"
      git -C "$NDL_ROOT" pull --ff-only
    else
      log "clone already present: $NDL_ROOT (set OCR_UPDATE=1 to git pull)"
    fi
    return
  fi
  if [[ -e "$NDL_ROOT" ]]; then
    die "$NDL_ROOT exists but is not a git clone; remove it or set MINER_NDL_ROOT"
  fi
  need_cmd git
  log "cloning $NDL_REPO_URL → $NDL_ROOT"
  mkdir -p "$(dirname "$NDL_ROOT")"
  git clone --depth 1 "$NDL_REPO_URL" "$NDL_ROOT"
}

# Ordered list of Python versions to try (major.minor).
python_candidates() {
  local preferred="$PYTHON_VERSION"
  printf '%s\n' "$preferred"
  case "$preferred" in
    3.12) printf '%s\n' 3.11 3.10 ;;
    3.11) printf '%s\n' 3.12 3.10 ;;
    3.10) printf '%s\n' 3.12 3.11 ;;
    *) printf '%s\n' 3.12 3.11 3.10 ;;
  esac
}

create_venv_uv() {
  local ver
  local last_err=""
  for ver in $(python_candidates); do
    log "creating venv with uv (Python $ver) for $OS_NAME/$ARCH_NAME"
    if uv venv "$NDL_ROOT/.venv" --python "$ver" 2>/tmp/miner-ocr-uv-venv.err; then
      log "venv ready: Python $ver"
      return 0
    fi
    last_err="$(cat /tmp/miner-ocr-uv-venv.err 2>/dev/null || true)"
    log "uv could not create Python $ver venv; trying next…"
    rm -rf "$NDL_ROOT/.venv"
  done
  printf '%s\n' "$last_err" >&2
  if is_darwin; then
    die "uv failed to create a Python 3.12/3.11/3.10 venv on macOS.
Install uv (https://github.com/astral-sh/uv) and retry, or:
  brew install python@3.12
  MINER_NDL_PYTHON_VERSION=3.12 make ocr-install"
  fi
  die "uv failed to create a Python 3.12/3.11/3.10 venv"
}

create_venv_stdlib() {
  local ver py=""
  for ver in $(python_candidates); do
    if command -v "python${ver}" >/dev/null 2>&1; then
      py="python${ver}"
      break
    fi
  done
  if [[ -z "$py" ]]; then
    if command -v python3 >/dev/null 2>&1; then
      py="python3"
      local pv
      pv="$("$py" -c 'import sys; print("%d.%d" % sys.version_info[:2])' 2>/dev/null || echo unknown)"
      log "warning: no python3.12/3.11/3.10 on PATH; using $py ($pv)"
      if is_darwin; then
        log "on macOS prefer: brew install python@3.12  (or install uv)"
      fi
    else
      die "need uv or python3 to create venv (https://github.com/astral-sh/uv)"
    fi
  fi
  log "creating venv with $py ($OS_NAME/$ARCH_NAME)"
  "$py" -m venv "$NDL_ROOT/.venv"
}

create_venv() {
  local py_venv="$NDL_ROOT/.venv/bin/python"
  if [[ -f "$py_venv" ]]; then
    log "venv already exists: $NDL_ROOT/.venv"
    return
  fi

  if command -v uv >/dev/null 2>&1; then
    create_venv_uv
    return
  fi
  create_venv_stdlib
}

install_deps() {
  [[ -f "$REQS" ]] || die "missing $REQS"
  local py="$NDL_ROOT/.venv/bin/python"
  [[ -f "$py" ]] || die "venv python missing: $py"

  log "platform: $OS_NAME/$ARCH_NAME (macOS uses onnxruntime pin from requirements-ocr.txt)"
  if command -v uv >/dev/null 2>&1; then
    log "installing deps via uv pip -r requirements-ocr.txt"
    if ! uv pip install --python "$py" -r "$REQS"; then
      if is_darwin; then
        die "pip install failed on macOS.
Tips:
  - Use Python 3.12 or 3.11 (not 3.14): brew install python@3.12
  - OCR_UPDATE=1 make ocr-install after fixing Python
  - Apple Silicon and Intel both use CPU (MINER_NDL_DEVICE=cpu)"
      fi
      die "pip install failed"
    fi
  else
    log "installing deps via pip -r requirements-ocr.txt"
    "$py" -m pip install --upgrade pip
    if ! "$py" -m pip install -r "$REQS"; then
      if is_darwin; then
        die "pip install failed on macOS (see tips: Python 3.12/3.11, CPU only)"
      fi
      die "pip install failed"
    fi
  fi
}

print_exports() {
  local py="$NDL_ROOT/.venv/bin/python"
  cat <<EOF

ocr-install: ready ($OS_NAME/$ARCH_NAME)

  MINER_NDL_ROOT=$NDL_ROOT
  MINER_NDL_PYTHON=$py
  MINER_NDL_WORKER=$WORKER

Shell exports (if not using make run):

  export MINER_NDL_ROOT='$NDL_ROOT'
  export MINER_NDL_PYTHON='$py'
  export MINER_NDL_WORKER='$WORKER'

Then:

  export MINER_PIN='your-shared-pin'
  make run
EOF
  if is_darwin; then
    cat <<'EOF'

macOS notes:
  - Use MINER_NDL_DEVICE=cpu (default). CUDA is not available on Mac.
  - First boot loads ONNX models (several seconds on Apple Silicon/Intel).
EOF
  fi
}

main() {
  [[ -f "$WORKER" ]] || die "missing worker script: $WORKER"
  log "host: $OS_NAME/$ARCH_NAME"
  clone_or_update
  if health_ok && [[ "$UPDATE" != "1" ]]; then
    log "health check OK — skip reinstall (OCR_UPDATE=1 to force)"
    print_exports
    return 0
  fi
  create_venv
  install_deps
  if ! health_ok; then
    if is_darwin; then
      die "install finished but import check failed (onnxruntime/cv2/PIL/yaml/numpy).
On macOS, recreate with Python 3.12:
  rm -rf '$NDL_ROOT/.venv'
  brew install python@3.12   # if needed
  MINER_NDL_PYTHON_VERSION=3.12 OCR_UPDATE=1 make ocr-install"
    fi
    die "install finished but import check failed (onnxruntime/cv2/PIL/yaml/numpy)"
  fi
  log "health check OK"
  print_exports
}

main "$@"
