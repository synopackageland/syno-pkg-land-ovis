# Ovis for DSM

This folder packages unmodified [Ovis](https://github.com/devswha/ovis) behind
DSM administrator sign-in. Upstream files outside `synology/` remain unchanged.

Version 0.5.1-0004 packages unmodified upstream v0.5.1 with SDK 0.53.26
and CLI 0.52.17. The runtime is built for x86_64 and aarch64, with Bun 1.4.2,
herdr 0.9.3 and remote bundle protocol 43. Node 22.23.4 (with npm) and the
Node PTY fallback retain the immutable previous runtime as their tool layer.

Fresh installations are the supported path for this storage change. Select an
existing NAS folder in the install wizard. DSM 7.2.2 and newer use the native
folder chooser; DSM 7.2.1 uses a share dropdown and relative subfolder field.
The folder must exist. The SDK grants the package account access to the share
and checks the selected folder at startup; missing access uses its permission
Dialog and Retry. This grant covers the share even when a subtree is mounted.

## Runtime and access

The SDK assigns the DSM package UID:GID to the Ovis container. Shells,
herdr, agents and SSH all run as that identity. A terminal may run single
commands as container root with `sudo` (no password), for example
`sudo apt-get install -y build-essential`; see "sudo inside the container"
below. The SDK drops all capabilities and adds back only those sudo needs
(AUDIT_WRITE, CHOWN, DAC_OVERRIDE, FOWNER, FSETID, SETGID, SETUID), with no
privileged mode, devices, Docker socket or host network. The package user's
effective capabilities stay zero until it runs `sudo`.

The root filesystem is writable so that apt can install into the container
layer. `/tmp` is the image's own directory (mode 1777); `TMPDIR` is `/scratch`,
the SDK's disk scratch in the package's own storage, owned by the package
account, mode 0700 and emptied at every start. There is no memory tmpfs:
DSM's Docker recreates a tmpfs as root mode 0755 on a container restart,
ignoring its declared mode, which stopped the previous build's `/run/ovis`
runtime directory from starting again after a restart. tini is PID 1 and reaps
processes orphaned by terminals and agents.

The persistent mount is the selected NAS folder at `/data`, with read/write
access. The SDK creates `home/` and `workspace/` as the package account,
without chown or changing the selected parent folder. Existing files stay in
place. No named volume is declared or mounted and no legacy data is imported.

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

`HOME=/data/home` holds agents, npm's user prefix (`~/.local`), SSH
configuration, herdr configuration and Ovis state. `/data/workspace` is the
default directory for new local workspaces; explicit paths keep their meaning.
The SDK's container init (`synopkgland container init`) runs before the
launcher on every start: it appends the account `ovis` for the package UID to
`/etc/passwd` and `/etc/group` (writable by that UID in the image, because
setuid sudo ignores libnss_wrapper), sets `HOME`, `USER`, `LOGNAME`, `TMPDIR`
and `/synopkgland/bin` on `PATH`, and removes group/other access from existing
credential paths the package UID owns (`~/.ssh`, its keys, config and
`authorized_keys`, `~/.gnupg`, `~/.netrc` and similar). It never follows
symlinks or changes another account's files.

## sudo inside the container

Container Manager has no user namespace, so container root is root on the NAS
for whatever is mounted: through `sudo`, the selected NAS folder can be read and
changed regardless of DSM permissions, and files created with `sudo` belong to
root. Agents in the container can use `sudo` too. Ovis is for DSM
administrators only; choose a folder that belongs to them.

Packages installed with `apt-get` live in the container layer. They are gone
when the container is recreated (package update, repair or changed settings);
install them again afterwards, for example from a script in HOME.

The container has internet egress for agent installation and remote PCs. Its
memory limit is 2 GiB and CPU scheduling weight is 512. DSM kernels without CFS
quota support use CPU shares. All admitted administrators share this package
identity and its home; there is no per-administrator filesystem isolation.

## SSH setup

Settings > Devices and PCs > Add a connection > SSH > Add PC shows **SSH keys and configuration**, the
package's public key and a **Copy public key** button. On first start the launcher
creates an Ed25519 key in `/data/home/.ssh/id_ed25519` and its `.pub` file, then
runs `synopkgland container fix-permissions` once, because DSM ACL inheritance
can widen the modes mkdir and ssh-keygen ask for. Later starts get the same
treatment from the container init. After loosening a key or `.ssh` by hand, run
`synopkgland container fix-permissions` in an Ovis terminal. Existing private
keys are retained and never overwritten. The private key is never returned by the setup endpoint.

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
HTTP CONNECT proxy to the builder, HTTP/HTTPS build downloads and apt. `BUILD_NO_PROXY`
optionally names hosts those downloads reach directly. No host paths are
hardcoded. `OUTPUT=load PLATFORMS=linux/amd64` supports a local single-architecture
smoke build. `ARCHIVE=/path/to/image.oci.tar` also exports a multi-architecture
OCI archive during a push build; `OUTPUT=oci ARCHIVE=...` writes only that archive,
which `skopeo copy --all --preserve-digests oci-archive:<archive> docker://<IMAGE>`
then publishes with the same digest. `docker save` can export that image for transfer to a test NAS.
`SKIP_SOURCE_BUILD=1` reuses already verified local payloads during iteration.
The image reuses the previous released runtime's immutable system/tool layer,
replaces the entire application payload, and restores the unchanged target-native
PTY packages. sudo and tini come from Ubuntu's apt archive at build time.
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
| sudo | Single commands as container root (SDK `run_as.sudo`) |
| tini | PID 1; reaps orphaned processes |

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

`/tmp` and `/scratch` are on disk and allow execution, so temporary venv
entrypoints work.

No compiler, Docker client or system daemon is installed. Install what an agent
needs with `sudo apt-get install`; it lasts until the container is recreated.

## Storage mapping and retention

| Key | NAS source | Container path | Access | Purpose |
| --- | --- | --- | --- | --- |
| data | Selected folder | /data | rw | The SDK install-selected folder bind |
| data | Selected folder/home | /data/home | rw | HOME, SSH keys, agents, Ovis and herdr state |
| data | Selected folder/workspace | /data/workspace | rw | Default local workspace |

The previous package used `ovis_data`. This version performs no migration,
ownership transfer or compatibility conversion. A fresh installation creates
new home/workspace children in the selected folder. Historical named volumes
remain outside this package's mounts and are not deleted by it.
Uninstall retains the selected folder and its contents. DSM's data-share grant
may also remain; administrators manage retained permissions in DSM.

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
no capabilities and no network. It verifies the tools and symlinks, the sudo
contract (setuid sudo, the `ovis` sudoers rule, appendable account files) and
tini, that no compiler or Docker client/daemon is present, creates a
temporary venv and installs/imports a synthetic pure-Python wheel offline.
`OVIS_TEST_PLATFORMS=linux/amd64` selects a native-only development check;
the default also needs arm64 emulation on an amd64 build host.
Device acceptance verifies a fresh installation with a two-level subfolder,
administrator entry, SSH/herdr operation, UID/mount readback and package-owned
HOME/workspace files. Static checks alone do not
establish DSM acceptance. Keep screenshots outside Git.

## Upstream v0.5.1 adapter compatibility

The upgrade was reviewed against upstream's v0.5.0/v0.5.1 changelog and tagged
source. The new account, relay, desktop and herdr-core features do not replace
the package's DSM gateway or its bundled herdr TUI runtime. The package keeps
`OVIS_TAILSCALE_SERVE_ONLY=1`, so upstream's relay and Portal remain blocked;
`OVIS_AUTO_UPDATE=0` keeps updates with Package Center.

The entrypoint still calls `Bun.serve({fetch})`. The packaging preload therefore
continues to protect its script and public-key routes with the injected bearer
credential, request an uncompressed HTML shell before injection, and translate
known failures in `/api/machines/setup` jobs. The Add PC dialog still uses
`#machine-connect-form`, `.machine-dialog`, `phase`, `error` and `ssh_output`.
Its entry now lives under Devices and PCs > Add a connection > SSH. Obsolete
pre-Ovis environment aliases were removed to match the current naming contract.
The SDK's container init supplies the package account. The remote
bundle version is read from `shared/machines.ts`; fixtures must use version 43
for this release instead of the former version 41.
