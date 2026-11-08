#!/usr/bin/env bash
# Image payload preparation: build the merged upstream tree with its own toolchain
# (Bun, as upstream's `bun run build` and remote-PC bundle do) and stage one
# daemon payload per declared architecture below synology/daemon/<arch>.
#
# upgrade.sh runs it from synology/ with SPL_UPSTREAM_SOURCE (the merged fork
# root), SPL_UPSTREAM_TAG and SPL_UPSTREAM_COMMIT. It never edits the upstream
# tree: it builds in a private export of the selected commit under .build/.
#
# Each payload holds:
#   ovis-launch   launcher and supervisor (adapter/): creates the per-install
#                 OVIS_TOKEN, runs herdr's headless server, then Bun
#   ovis-token    DSM identity gateway token adapter (adapter/)
#   bun           official Bun release for the NAS, pinned by SHA-256
#   herdr         official herdr release for the NAS, pinned by SHA-256: the
#                 version upstream's own Linux remote bundle ships
#   app/          upstream server/, shared/, integrations/, dist/, production
#                 node_modules/ and package.json: the file set of upstream's
#                 own Linux runtime bundle (scripts/build-remote-bundle.ts),
#                 node_modules/ pruned to what the server imports,
#                 without Node or node-pty (no PTY host: the NAS's own
#                 panes are drawn from pane.read). Remote PCs bring their own bridge.
set -euo pipefail

packaging=$(cd -- "$(dirname -- "$0")" && pwd)
source_root=${SPL_UPSTREAM_SOURCE:-$(cd -- "$packaging/.." && pwd)}
commit=${SPL_UPSTREAM_COMMIT:-$(git -C "$source_root" rev-parse HEAD)}

# Bun 1.4.2, upstream's packageManager pin. Digests: oven-sh/bun bun-v1.4.2 SHASUMS256.txt.
# x86_64 uses the baseline build: many NAS CPUs (Celeron, Atom) have no AVX2.
BUN_VERSION=1.4.2
bun_pin() {
  case $1 in
    darwin-arm64) echo "bun-darwin-aarch64 90987a3a16d7db556d886ac3d551e7b6d3edf0a1cf43acaed622e8676be1d12f" ;;
    darwin-x64) echo "bun-darwin-x64 80520d7e17526308c9185d261679ac6d27798d3803a0e9f7ff9121ab8affb012" ;;
    linux-x64 | x86_64) echo "bun-linux-x64-baseline c678040f14fe0440eb839d37cbd0ce4c051a32da72806ac97de6a6aab6bf728f" ;;
    linux-arm64 | aarch64) echo "bun-linux-aarch64 54328bbc2d9c8e0c9f892c544d66c57a83b84139e34909e5ee81758f1ac8fda7" ;;
    *) echo "No pinned Bun for $1" >&2; return 1 ;;
  esac
}

# herdr 0.9.3, the release upstream's scripts/build-remote-bundle.ts pins for its
# Linux bundles (same digests). Static binaries; Apache-2.0 (LICENSES/).
HERDR_VERSION=0.9.3
herdr_pin() {
  case $1 in
    x86_64) echo "herdr-linux-x86_64 18a8dc65f1c2fa485884344356dea1cfd911c6f06cf46fa78e193f4087f4dba7" ;;
    aarch64) echo "herdr-linux-aarch64 4de7aa3e25678812e92960de64f7c2aaa1bca1f0f80a3c5e559837e231e1f5c0" ;;
    *) echo "No pinned herdr for $1" >&2; return 1 ;;
  esac
}

for tool in git go curl unzip shasum tar; do
  command -v "$tool" >/dev/null 2>&1 || { echo "$tool is not on PATH" >&2; exit 2; }
done

work="$packaging/.build"
downloads="$work/downloads"
mkdir -p "$downloads"

# Fetch one pinned Bun build and print the path of its verified binary.
fetch_bun() {
  local name sha zip
  read -r name sha < <(bun_pin "$1")
  zip="$downloads/$name.zip"
  if [[ ! -f $zip ]] || [[ $(shasum -a 256 "$zip" | cut -d' ' -f1) != "$sha" ]]; then
    curl -fsSL --retry 3 -o "$zip.part" "https://github.com/oven-sh/bun/releases/download/bun-v$BUN_VERSION/$name.zip"
    [[ $(shasum -a 256 "$zip.part" | cut -d' ' -f1) == "$sha" ]] || { echo "Bun checksum mismatch: $name.zip" >&2; exit 1; }
    mv "$zip.part" "$zip"
  fi
  rm -rf "$downloads/$name"
  unzip -qo "$zip" -d "$downloads"
  echo "$downloads/$name/bun"
}

# Fetch one pinned herdr build and print the path of its verified binary.
fetch_herdr() {
  local name sha file
  read -r name sha < <(herdr_pin "$1")
  file="$downloads/herdr-$HERDR_VERSION-$name"
  if [[ ! -f $file ]] || [[ $(shasum -a 256 "$file" | cut -d' ' -f1) != "$sha" ]]; then
    curl -fsSL --retry 3 -o "$file.part" "https://github.com/herdrdev/herdr/releases/download/v$HERDR_VERSION/$name"
    [[ $(shasum -a 256 "$file.part" | cut -d' ' -f1) == "$sha" ]] || { echo "herdr checksum mismatch: $name" >&2; exit 1; }
    mv "$file.part" "$file"
  fi
  echo "$file"
}

# A private export of the selected upstream commit: the fork's tree is never touched.
src="$work/src"
rm -rf "$src"
mkdir -p "$src"
git -C "$source_root" archive "$commit" | tar -x -C "$src"

pm=$(sed -n 's/.*"packageManager": *"bun@\([^"]*\)".*/\1/p' "$src/package.json")
[[ $pm == "$BUN_VERSION" ]] || {
  echo "Upstream now pins bun@$pm; update BUN_VERSION and the digests in build-source.sh" >&2
  exit 1
}

grep -q "herdr/releases/download/v$HERDR_VERSION/" "$src/scripts/build-remote-bundle.ts" || {
  echo "Upstream's remote bundle no longer pins herdr $HERDR_VERSION; update HERDR_VERSION and the digests in build-source.sh" >&2
  exit 1
}

case "$(uname -s)-$(uname -m)" in
  Darwin-arm64) host=darwin-arm64 ;;
  Darwin-x86_64) host=darwin-x64 ;;
  Linux-x86_64) host=linux-x64 ;;
  Linux-aarch64) host=linux-arm64 ;;
  *) echo "Unsupported build host $(uname -sm)" >&2; exit 2 ;;
esac
host_bun=$(fetch_bun "$host")

echo "building the Ovis web client (upstream bun run build)" >&2
(
  cd "$src"
  export BUN_INSTALL_CACHE_DIR="$work/bun-cache" NO_COLOR=1
  "$host_bun" install --frozen-lockfile
  "$host_bun" run build
  # Runtime dependencies only, from the same lockfile and patches.
  rm -rf node_modules
  "$host_bun" install --frozen-lockfile --production
)

app="$work/app"
rm -rf "$app"
mkdir -p "$app/scripts"
for dir in server shared integrations dist node_modules; do
  cp -R "$src/$dir" "$app/$dir"
done
cp "$src/package.json" "$app/package.json"
# Upstream's own license and third-party notices travel with its code.
cp "$src/LICENSE" "$src/THIRD_PARTY_NOTICES.md" "$app/"
for script in install-gjc-integration.ts install-pi-integration.ts; do
  cp "$src/scripts/$script" "$app/scripts/$script"
done
find "$app/server" "$app/shared" -name '*.test.ts' -delete
# Only the packages the server entrypoints import: the web client's packages are
# already bundled into dist/. The native PTY addon (@lydell/node-pty) runs only
# under Node, which is not shipped, and .bin holds only links.
"$host_bun" build "$src/server/index.ts" "$src/server/managed.ts" --target=bun \
  --external @lydell/node-pty --external node-pty \
  --outdir "$work/trace" --metafile="$work/trace/meta.json" >/dev/null
rm -rf "$app/node_modules/.bin"
"$host_bun" "$packaging/prune-runtime.mjs" "$app" "$work/trace/meta.json"
if find "$app" -type l -print -quit | grep -q .; then
  echo 'payload must not contain symbolic links' >&2
  find "$app" -type l >&2
  exit 1
fi

export CGO_ENABLED=0 GOOS=linux GOFLAGS=-mod=readonly GOTOOLCHAIN=local
for pair in x86_64:amd64 aarch64:arm64; do
  arch=${pair%%:*}
  goarch=${pair##*:}
  out="$packaging/daemon/$arch"
  rm -rf "$out"
  mkdir -p "$out"
  echo "staging the $arch payload" >&2
  (
    cd "$packaging/adapter"
    GOARCH=$goarch go build -trimpath -ldflags "-s -w" -o "$out/ovis-launch" ./cmd/ovis-launch
    GOARCH=$goarch go build -trimpath -ldflags "-s -w" -o "$out/ovis-token" ./cmd/ovis-token
  )
  cp "$(fetch_bun "$arch")" "$out/bun"
  cp "$(fetch_herdr "$arch")" "$out/herdr"
  cp -R "$app" "$out/app"
  cp -R "$packaging/runtime" "$out/runtime"
  chmod 0755 "$out/ovis-launch" "$out/ovis-token" "$out/bun" "$out/herdr"
done
echo "payload staged in $packaging/daemon" >&2
