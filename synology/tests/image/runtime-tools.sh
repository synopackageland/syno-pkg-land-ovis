#!/usr/bin/env bash
# Explicit image gate: tests the built payload as an ordinary pane user.
set -euo pipefail
: "${OVIS_TEST_IMAGE:?Set OVIS_TEST_IMAGE to the built runtime image}"
: "${BUILD_PREFIX:?Set BUILD_PREFIX to a disposable container prefix}"
[[ $BUILD_PREFIX =~ ^[a-z][a-z0-9-]+$ ]] || exit 2
for platform in ${OVIS_TEST_PLATFORMS:-linux/amd64 linux/arm64}; do
  arch=${platform##*/}
  docker run --rm -i --name "$BUILD_PREFIX-tools-$arch" --platform "$platform" \
    --user 1000:1000 --read-only --cap-drop ALL --security-opt no-new-privileges \
    --network none --tmpfs /tmp:rw,exec,nosuid,nodev,mode=1777 \
    --entrypoint /bin/sh "$OVIS_TEST_IMAGE" -s <<'CHECK'
set -eu
test "$(id -u)" = 1000
# OpenSSH reads passwd, independently of HOME.
test "$(getent passwd root | cut -d: -f6)" = /data/home
for tool in bash git ssh curl unzip xz ps node npm npx bun herdr \
  jq rg fd less file tree zip rsync wget nano python python3 pip3 sqlite3 ping; do
  command -v "$tool" >/dev/null || { echo "Missing runtime tool: $tool" >&2; exit 1; }
done
for tool in cc gcc g++ make sudo docker tmux; do
  if command -v "$tool" >/dev/null; then
    echo "Unexpected runtime tool: $tool" >&2; exit 1
  fi
done
test -z "${PIP_BREAK_SYSTEM_PACKAGES:-}"
test "$(readlink -f /usr/local/bin/fd)" = "$(readlink -f /usr/bin/fdfind)"
test "$(readlink -f /usr/local/bin/python)" = "$(readlink -f /usr/bin/python3)"
jq --version
rg --version
fd --version
python --version
python -c 'import sqlite3, ssl; assert sqlite3.connect(":memory:").execute("select 1").fetchone() == (1,)'
sqlite3 :memory: 'select 1;'
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
python3 -m venv "$work/venv"
"$work/venv/bin/pip" --version
# Exercise an offline pure-Python wheel install without touching system Python.
python - "$work" <<'PY'
import pathlib, sys, zipfile
root = pathlib.Path(sys.argv[1])
with zipfile.ZipFile(root / 'ovis_smoke-1.0-py3-none-any.whl', 'w') as wheel:
    wheel.writestr('ovis_smoke.py', 'VALUE = 42\n')
    wheel.writestr('ovis_smoke-1.0.dist-info/METADATA', 'Metadata-Version: 2.1\nName: ovis-smoke\nVersion: 1.0\n')
    wheel.writestr('ovis_smoke-1.0.dist-info/WHEEL', 'Wheel-Version: 1.0\nGenerator: smoke\nRoot-Is-Purelib: true\nTag: py3-none-any\n')
    wheel.writestr('ovis_smoke-1.0.dist-info/RECORD', '')
PY
"$work/venv/bin/pip" install --no-index --disable-pip-version-check "$work/ovis_smoke-1.0-py3-none-any.whl"
"$work/venv/bin/python" -c 'import ovis_smoke; assert ovis_smoke.VALUE == 42'
echo 'Runtime tools and isolated Python installation passed as uid 1000'
CHECK
done
