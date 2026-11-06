#!/usr/bin/env bash
# Build on any Linux Docker/buildx host; no NAS connection or local host paths.
set -euo pipefail
packaging=$(cd -- "$(dirname -- "$0")" && pwd)
: "${BUILD_PREFIX:?Set BUILD_PREFIX to a unique disposable resource prefix}"
: "${IMAGE:?Set IMAGE to the registry repository and immutable version tag}"
[[ $(uname -s) == Linux && $BUILD_PREFIX =~ ^[a-z][a-z0-9-]+$ ]] || exit 2
builder="$BUILD_PREFIX-ovis-builder"
cleanup() {
  [[ ${KEEP_BUILDER:-0} == 1 ]] && return 0
  docker buildx prune --builder "$builder" --all --force >/dev/null 2>&1 || true
  docker buildx rm "$builder" >/dev/null 2>&1 || true
}
# Never reuse another run's builder or prune the default/shared builder.
opts=()
export HTTPS_PROXY=${BUILD_PROXY:-${HTTPS_PROXY:-}}
if [[ -n ${BUILD_PROXY:-} ]]; then
  opts+=(--driver-opt network=host --driver-opt "env.HTTPS_PROXY=$BUILD_PROXY")
fi
if ! docker buildx inspect "$builder" >/dev/null 2>&1; then
  docker buildx create --name "$builder" --driver docker-container "${opts[@]}" >/dev/null
elif [[ ${KEEP_BUILDER:-0} != 1 ]]; then
  echo "Builder already exists; choose a unique BUILD_PREFIX" >&2; exit 2
fi
trap cleanup EXIT
work="$packaging/.build"
context="$work/image"
if [[ ${SKIP_SOURCE_BUILD:-0} != 1 ]]; then
  prep="$work/prepare-image"
  rm -rf "$prep"
  mkdir -p "$prep/source" "$prep/packaging"
  source_root=${SPL_UPSTREAM_SOURCE:-$(cd -- "$packaging/.." && pwd)}
  commit=${SPL_UPSTREAM_COMMIT:-$(git -C "$source_root" rev-parse HEAD)}
  git -C "$source_root" archive "$commit" | tar -x -C "$prep/source"
  cp "$packaging/build-source.sh" "$packaging/prune-runtime.mjs" "$prep/packaging/"
  cp -R "$packaging/adapter" "$prep/packaging/adapter"
  docker buildx build --builder "$builder" \
    --build-arg "https_proxy=${BUILD_PROXY:-}" \
    --file "$packaging/container/prepare.Dockerfile" \
    --output "type=local,dest=$work/prepared" "$prep"
  rm -rf "$packaging/daemon"
  mkdir -p "$packaging/daemon"
  cp -R "$work/prepared/amd64" "$packaging/daemon/x86_64"
  cp -R "$work/prepared/arm64" "$packaging/daemon/aarch64"
fi
rm -rf "$context"
mkdir -p "$context/payload"
cp "$packaging/container/Dockerfile" "$context/Dockerfile"
cp -R "$packaging/LICENSES" "$context/LICENSES"
for pair in x86_64:amd64 aarch64:arm64; do
  arch=${pair%%:*}; target=${pair##*:}
  cp -R "$packaging/daemon/$arch" "$context/payload/$target"
  mkdir -p "$packaging/helpers/$arch"
  cp "$packaging/daemon/$arch/ovis-token" "$packaging/helpers/$arch/ovis-token"
done
# OUTPUT=load with PLATFORMS=linux/amd64 supports local smoke tests.
output=${OUTPUT:-push}
[[ $output == push || $output == load ]] || { echo "OUTPUT must be push or load" >&2; exit 2; }
archive=()
if [[ -n ${ARCHIVE:-} ]]; then archive+=(--output "type=oci,dest=$ARCHIVE"); fi
build_args=()
if [[ -n ${BUILD_PROXY:-} ]]; then build_args+=(--build-arg "https_proxy=$BUILD_PROXY"); fi
docker buildx build --builder "$builder" --platform "${PLATFORMS:-linux/amd64,linux/arm64}" \
  --provenance=false "${archive[@]}" --tag "$IMAGE" "${build_args[@]}" \
  --"$output" "$context"
