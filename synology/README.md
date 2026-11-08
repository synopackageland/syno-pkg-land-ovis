# Ovis for DSM

This folder packages unmodified [Ovis](https://github.com/devswha/ovis) behind
DSM administrator sign-in. Upstream files outside `synology/` remain unchanged.

Version 0.4.22-0005 replaces the native runtime with an App-owned Ubuntu 24.04
container. Bun 1.4.2 and herdr 0.9.3 retain the checksum pins in `build-source.sh`;
Node 22.23.4 (with npm) comes from a digest-pinned official image in
`container/Dockerfile`; only the binary and npm are copied into Ubuntu. The Node
PTY fallback uses upstream's pinned @lydell/node-pty 1.1.0 target-native prebuild.
Version 0.4.22-0006 adds common shell tools and Python with pip and venv support.
Version 0.4.22-0007 explicitly runs the Ovis container as root so SSH can resolve
its account, while keeping HOME and the root account home at /data/home. It uses SDK 0.53.8 for the root runtime environment.
Version 0.4.22-0006 introduced SDK 0.52.0 with CLI 0.50.0 to retain the
Container Manager project profile across package stop/start; this release
keeps the CLI pin and moves the SDK to 0.53.8.

## Runtime and access

The SDK's official Container Manager project worker assigns UID/GID 0 inside the
Ovis container through the explicit `run_as: container-root` service policy.
Host package hooks, the Compose control tool and the identity gateway remain the
DSM package account. This option does not install an executor or request SDK
administrator authorization, rootOps approval or DSM password reconfirmation.
Normal DSM administrator sign-in remains the App entry gate.

The root filesystem is read-only. The wrapper declares `cap_drop: [ALL]` and
explicitly adds only `DAC_OVERRIDE` and `CHOWN`, with no-new-privileges enabled.
The observed DSM Container Manager runtime also supplies `NET_BIND_SERVICE`,
which was present in the previous package runtime too. There is no privileged mode, host
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
the launcher creates private home subdirectories. The initial image UID 1000 is
only a standalone default; DSM explicitly assigns container UID 0. On DSM without
user namespace remapping this is host UID 0, with only the declared volume and
read-only SDK mounts visible.
The container has internet egress for agent installation and remote PCs.
The memory limit is 2 GiB and CPU scheduling weight is 512. DSM kernels
without CFS quota support reject a hard `cpus` limit; CPU shares allow the
container to start on those systems. Large agent workloads may need more memory.

These limits protect the host from ordinary shell writes, not from kernel
vulnerabilities. Containers do not provide per-administrator isolation.

## SSH setup

Settings > Remote PCs > Add PC shows **SSH keys and configuration**, the
package's public key and a **Copy public key** button. On first start the launcher
creates an Ed25519 key in `/data/home/.ssh/id_ed25519` and its `.pub` file, using
private directory/file modes. Existing private keys are retained and never
overwritten. The private key is never returned by the setup endpoint.

Copy the public key to `~/.ssh/authorized_keys` for the account on the remote PC,
set that PC's `.ssh` directory to mode 700 and `authorized_keys` to mode 600, and
enable its SSH service. Verify the PC's host-key fingerprint on the PC before
accepting the upstream Ovis trust prompt. Password/passphrase challenges for the
remote PC still belong to upstream; there is no extra DSM password prompt.

In an Ovis terminal, edit `/data/home/.ssh/config` for SSH aliases and
`/data/home/.ssh/known_hosts` for trusted hosts. Use absolute key paths. These
files and keys belong to the Ovis container, separate from DSM users and the NAS
SSH service. A host or browser ssh-agent is not forwarded into the container.
All admitted DSM administrators share the same Ovis home and SSH identity.

The package-owned Bun preload adapter adds this guidance without changing
upstream source or translations. Its public-key endpoint requires the same
server-side gateway bearer credential; it exposes only a bounded public key.
Known connection failures get recovery instructions, while upstream still owns
connection jobs, password handling, fingerprint decisions and terminal streams.

The guidance follows [Zed’s SSH remote-development model](https://zed.dev/docs/remote-development):
use OpenSSH configuration and key authentication, and keep remote authentication
and host-key prompts in the existing connection flow. Here the SSH client lives
in the DSM container, so the guidance names its home explicitly. No Zed code or
visual assets are copied.

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
| jq | JSON inspection and scripting |
| ripgrep, fd-find | Fast content and file searches (`rg`, `fd`) |
| less, file, tree | Page output, identify files and inspect directory trees |
| zip | Create ZIP archives |
| rsync | Incremental file transfers |
| wget | Downloads for scripts that expect wget |
| nano | Small interactive text editor |
| python3, python3-pip, python3-venv | Python scripts and isolated package installation |
| sqlite3 | Inspect and query local databases |
| iputils-ping | Basic network diagnostics, subject to runtime network permissions |

Ubuntu's `universe` component is enabled for these tools. All packages still use
`--no-install-recommends`. `fd` links to Ubuntu's `fdfind`; `python` links to
`python3`. herdr already provides persistent multiplexed terminals, so the image
does not add tmux. The small ping package is used instead of the larger DNS suite.

System Python retains Ubuntu's externally managed environment protection. Use a
writable virtual environment for pure-Python packages; no
`PIP_BREAK_SYSTEM_PACKAGES` override is needed:

```sh
python3 -m venv "$HOME/.venvs/project"
"$HOME/.venvs/project/bin/pip" install <package>
```

The bounded `/tmp` tmpfs explicitly allows execution so temporary venv entrypoints
also work. It retains `nosuid` and `nodev`; the root filesystem stays read-only.

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

### Upgrade from package-UID containers

`DAC_OVERRIDE` lets the startup adapter traverse retained mode-0700 directories
and read the package-owned runtime seed. `CHOWN` permits a one-time migration of
`/data/home` to UID/GID 0, needed both for OpenSSH's config ownership check and
Git's repository ownership check. Modes and contents are retained. The adapter
walks only this App's named-volume home through a confined filesystem root,
changes link ownership without following links, and refuses filesystem boundaries.
It writes a private version marker only after the walk succeeds; interrupted
migration resumes safely and later starts skip completed migration.

No host root hook, FOWNER capability or Git safe.directory wildcard is used.
The capabilities apply to files visible in the container; no writable host or
Shared Folder bind is declared. The root filesystem and SDK bind mounts stay
read-only. Downgrading to an older
package-UID runtime requires a separate recovery procedure and is not verified.

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
`OVIS_TEST_IMAGE=<image> BUILD_PREFIX=<disposable-prefix> npm run test:image`
checks both image architectures as UID 1000, with a read-only root filesystem,
no capabilities and no network. It verifies the tools and symlinks, creates a
temporary venv and installs/imports a synthetic pure-Python wheel offline.
`OVIS_TEST_PLATFORMS=linux/amd64` selects a native-only development check;
the default also needs arm64 emulation on an amd64 build host.
Device acceptance must separately verify native-to-container upgrade, fresh
administrator entry, anonymous/nonadmin rejection, an agent installed from an
Ovis terminal, UID/mount readback and persistence. Static checks alone do not
establish DSM acceptance. Keep screenshots outside Git.
