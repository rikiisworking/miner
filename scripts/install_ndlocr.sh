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

# Major.minor from a python binary (empty on failure).
python_mm() {
  local bin="$1"
  "$bin" -c 'import sys; print("%d.%d" % sys.version_info[:2])' 2>/dev/null || true
}

# Supported for requirements-ocr.txt wheels (esp. onnxruntime on macOS).
python_supported() {
  case "$(python_mm "$1")" in
    3.10|3.11|3.12) return 0 ;;
    *) return 1 ;;
  esac
}

health_ok() {
  local py="$NDL_ROOT/.venv/bin/python"
  [[ -f "$NDL_ROOT/src/ocr.py" ]] || return 1
  # -x can be flaky on some network FS; require file + runnable import instead.
  [[ -f "$py" ]] || return 1
  python_supported "$py" || return 1
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

# Resolve a pythonX.Y binary: PATH, then Homebrew opt prefixes (even if not linked).
find_python() {
  local ver="$1"
  local cand
  if command -v "python${ver}" >/dev/null 2>&1; then
    command -v "python${ver}"
    return 0
  fi
  for cand in \
    "/opt/homebrew/opt/python@${ver}/bin/python${ver}" \
    "/usr/local/opt/python@${ver}/bin/python${ver}" \
    "/opt/homebrew/bin/python${ver}" \
    "/usr/local/bin/python${ver}"; do
    if [[ -x "$cand" ]]; then
      printf '%s\n' "$cand"
      return 0
    fi
  done
  return 1
}

unsupported_python_hint() {
  if is_darwin; then
    cat <<'EOF' >&2
ocr-install: ERROR: need Python 3.12, 3.11, or 3.10 (3.13/3.14 lack matching OCR wheels).
Install one of:
  brew install python@3.12
  # or: https://github.com/astral-sh/uv
Then retry:
  make ocr-install
EOF
  else
    cat <<'EOF' >&2
ocr-install: ERROR: need Python 3.12, 3.11, or 3.10 (or install uv).
  https://github.com/astral-sh/uv
EOF
  fi
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
  unsupported_python_hint
  exit 1
}

create_venv_stdlib() {
  local ver py=""
  for ver in $(python_candidates); do
    if py="$(find_python "$ver")"; then
      break
    fi
    py=""
  done
  if [[ -z "$py" ]]; then
    # Only accept plain python3 when it is already a supported minor.
    if command -v python3 >/dev/null 2>&1 && python_supported python3; then
      py="$(command -v python3)"
    else
      local pv="missing"
      if command -v python3 >/dev/null 2>&1; then
        pv="$(python_mm python3)"
        pv="${pv:-unknown}"
      fi
      log "no python3.12/3.11/3.10 found (python3 on PATH: $pv)"
      unsupported_python_hint
      exit 1
    fi
  fi
  log "creating venv with $py ($(python_mm "$py"); $OS_NAME/$ARCH_NAME)"
  "$py" -m venv "$NDL_ROOT/.venv"
}

create_venv() {
  local py_venv="$NDL_ROOT/.venv/bin/python"
  if [[ -f "$py_venv" ]]; then
    if python_supported "$py_venv"; then
      log "venv already exists: $NDL_ROOT/.venv (Python $(python_mm "$py_venv"))"
      return
    fi
    log "removing unsupported venv Python $(python_mm "$py_venv") at $NDL_ROOT/.venv (need 3.10–3.12)"
    rm -rf "$NDL_ROOT/.venv"
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

  cp .env.example .env   # set MINER_PIN
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

# Allow unit tests to `source` helpers without running install.
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
