# Ovis for DSM

This folder packages unmodified [Ovis](https://github.com/devswha/ovis) behind
DSM administrator sign-in. Upstream files outside `synology/` remain unchanged.

Version 0.4.22-0005 replaces the native runtime with an App-owned Ubuntu 24.04
container. Bun 1.4.2 and herdr 0.9.3 retain the checksum pins in `build-source.sh`;
Node 22.23.4 (with npm) comes from a digest-pinned official image in
`container/Dockerfile`; only the binary and npm are copied into Ubuntu. The Node
PTY fallback uses upstream's pinned @lydell/node-pty 1.1.0 target-native prebuild.

## Runtime and access

The SDK's official Container Manager project worker runs the image as the fixed
DSM package UID/GID. The root filesystem is read-only, all capabilities are
dropped, and no-new-privileges is enabled. There is no privileged mode, host
network, published container port, device, Docker socket or Shared Folder mount.
The only writable persistent mount is the project's own named data volume.
The SDK mounts its initializer and generated authentication seed read-only.

The private bridge admits HTTPS traffic only through the host SDK gateway:

```
browser -> DSM HTTPS port (default 27317) -> DSM admin gateway -> Ovis:37317
```

The icon and every gateway route require a DSM administrator. All administrators
share one Ovis identity. No passthrough, Portal or relay is enabled. Telemetry
starts off and remains user-controlled. Package Center owns Ovis updates.
The gateway adapter reads the SDK-generated private runtime seed, which the SDK
also injects into Ovis. It never sends that credential to the browser. The seed
rotates when the installed package generation is replaced; home data is retained.

`HOME=/data/home` holds agents, npm's user prefix (`~/.local`), workspaces,
herdr configuration and Ovis state. `/data` is a sticky writable mount base;
the SDK creates private owned subdirectories without root/chown. The initial
image UID 1000 is only a standalone default; DSM uses the package identity.
The container has internet egress for agent installation and remote PCs.
The memory limit is 2 GiB and CPU scheduling weight is 512. DSM kernels
without CFS quota support reject a hard `cpus` limit; CPU shares allow the
container to start on those systems. Large agent workloads may need more memory.

These limits protect the host from ordinary shell writes, not from kernel
vulnerabilities. Containers do not provide per-administrator isolation.

## Image and SPK build

Run on a Linux machine with Docker buildx, Bash, git and tar. The source
preparation stage supplies Go 1.26.10, curl, unzip and Bun inside Docker.
Node/npm are needed only for the separate SDK CLI commands. Cross-architecture RUN steps need binfmt/QEMU on that
same build host. Authenticate to GHCR using an existing credential outside the
build context. The namespace's package must allow anonymous pulls before pinning
or building an SPK; the CLI does not inherit private registry credentials.

```sh
npm ci --include=dev --prefix synology
export BUILD_PREFIX=ovis-build
export IMAGE=ghcr.io/synopackageland/syno-pkg-land-ovis-runtime:<upstream>-<build>
bash synology/build-image.sh
cd synology
npx synopkgland images pin
npx synopkgland check --json
npx synopkgland build --json
npx synopkgland inspect dist/<spk> --json
```

The build script exports upstream source privately into a pinned Go toolchain
container, builds the web client using
upstream's Bun command, verifies both Bun/herdr downloads and stages both
architectures. An isolated, prefixed buildx builder builds and pushes the image
index. Its cache is pruned and builder removed on exit; existing volumes and
shared builders are untouched. `BUILD_PROXY` optionally supplies an operator's
HTTP CONNECT proxy to the builder and HTTPS build downloads. No host paths are
hardcoded. `OUTPUT=load PLATFORMS=linux/amd64` supports a local single-architecture
smoke build. `ARCHIVE=/path/to/image.oci.tar` also exports a multi-architecture
OCI archive during a push build; `docker save` can export that image for transfer to a test NAS.
`SKIP_SOURCE_BUILD=1` reuses already verified local payloads during iteration.
`KEEP_BUILDER=1` keeps only that run's builder/cache for an active retry; finish
with `docker buildx prune --builder "$BUILD_PREFIX-ovis-builder" --all --force`
and `docker buildx rm "$BUILD_PREFIX-ovis-builder"`.

For an upstream version change, set the image tag and run
`bash synology/upgrade.sh <upstream-tag> <build>`; it merges the selected tag,
builds the image, pins it and builds the package. No merge to main or release is
performed by these scripts.

## Userland packages

Ubuntu supplies glibc for Bun, Node and agents. Explicit additional packages:

| Package | Purpose |
| --- | --- |
| bash | Interactive panes and common installer scripts |
| git | Agent repositories and worktrees |
| openssh-client | Ovis remote PCs and Git SSH transport |
| curl, ca-certificates | HTTPS installers and certificate validation |
| unzip, xz-utils | Common release archive extraction |
| procps | Agent process inspection |

No compiler, sudo, Docker client or system daemon is installed. Agent installers
needing extra system packages require a future image revision.

## Upgrade and data

Native releases stored their home and Ovis/herdr state directly in the package
var directory. The container uses a named volume; this version does not copy old
native state automatically. Retain the native var directory for recovery and
set up remote PCs/agents again in the new container home. Do not delete legacy
state or existing volumes during upgrade. Container stop/start and later upgrades
reuse the same named volume. SDK uninstall retains named volumes even when its
delete-data option is selected; the wizard reports that boundary.

## Distribution choices

The supported SPK contains the image's pinned registry metadata, not image
layers. Container Manager obtains those layers separately. Public GHCR visibility
is required for ordinary anonymous CLI builds and fresh installations. A
build-operator metadata mirror can authenticate to a private registry during
local verification; this does not give the shipped package registry credentials.

The pinned CLI's `buildStandalone` API accepts saved `composeImageProofsByArch`
for offline rebuilding. A build operator can extract the index, platform
manifests and configs from an OCI archive; the SDK verifies their SHA-256,
repository and platform before packaging. This supplies metadata only and does
not load the image on the NAS.

The current CLI has no image-tar-in-SPK/offline-load option. Preloading an exact
image with `docker save` / `docker load` is a separate administrator test/setup
step, not a shipped installer hook. Do not add a root hook or give the package
Docker access to implement loading. An offline bundle would add the full image
archive to each architecture's SPK and needs a supported privileged image-import
contract before implementation. It avoids registry availability at installation
but makes every package download/upgrade carry the runtime layers again.

## Validation

`npm test` runs packaging access/pin contracts; `(cd adapter && go test ./...)`
checks launcher/token behavior. `npm run test:local` also builds/inspects the SPK.
Device acceptance must separately verify native-to-container upgrade, fresh
administrator entry, anonymous/nonadmin rejection, an agent installed from an
Ovis terminal, UID/mount readback and persistence. Static checks alone do not
establish DSM acceptance. Keep screenshots outside Git.
