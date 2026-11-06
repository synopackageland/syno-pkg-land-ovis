# Ovis for DSM

This folder packages [Ovis](https://github.com/devswha/ovis), the browser and
phone client for the [herdr](https://github.com/herdrdev/herdr) terminal
multiplexer, as a Synology DSM package. Everything here is ours; files outside
`synology/` are upstream's and are never edited.

- **Maintainer:** devswha (upstream). **Distributor:** Syno Package Land.
- **Version:** `<upstream version>-<build>`, for example `0.4.22-0001`. The
  build restarts at 1 with every upstream release.

## What runs on the NAS

The NAS is Ovis's always-on *connection server*, the setup upstream describes in
[One always-on PC for all your PCs](../docs/guide.md#one-always-on-pc-for-all-your-pcs).
Add your computers under **Settings → Remote PCs**: Ovis connects to them over
SSH as the package account and starts its own bridge next to their herdr.

The NAS also runs herdr itself, so the sidebar's row for the NAS (this computer)
works instead of showing "cannot reach herdr". Upstream has no setting that
turns this computer's row off; the per-browser **Show in sidebar** switch cannot
hide the last PC, and the package never patches upstream's code. So the package
ships herdr:

- **Binary:** the official herdr Linux release (static, x86_64 and aarch64),
  pinned by version and SHA-256 in `build-source.sh`. It is the version
  upstream's own Linux remote bundle ships (`scripts/build-remote-bundle.ts`),
  and the build stops when upstream moves that pin. Apache-2.0, in `LICENSES/`.
- **Process:** `ovis-launch` starts `herdr server` (headless) as the package
  account, then Ovis; package stop ends Ovis, then herdr (SIGTERM, SIGKILL after
  10 s). A herdr that exits on its own is started again with backoff.
- **Socket:** herdr listens on `$XDG_CONFIG_HOME/herdr/herdr.sock`; the launcher
  sets herdr's `XDG_CONFIG_HOME` to the package var folder, so it is the
  manifest's `HERDR_SOCKET` (`${SYNOPKG_PKGVAR}/herdr/herdr.sock`).
  `OVIS_HERDR_BIN` names the bundled binary.
- **Panes on the NAS:** shells run as the package account (`/bin/bash`), with
  the package var folder as home. No PTY host is shipped (no Node or Rust
  host), so the NAS's own panes are mirrored from `pane.read`, as upstream does
  without one. Agents run there only if installed for that account.
- **Updates:** each package version ships the pinned herdr. herdr itself only
  reports newer releases (it waits for an explicit install). Ovis's
  **Settings → About → Update herdr** still works on Linux and has no off switch:
  it replaces the bundled binary in place with herdr's latest release, and the
  next package upgrade puts the pinned version back.

```text
browser ──HTTPS :27317 (DSM nginx, DSM certificate)──▶ SDK identity gateway
          DSM administrator sign-in; token adapter adds Authorization: Bearer <OVIS_TOKEN>
                                                    ──▶ Ovis (Bun) on 127.0.0.1:37317
                                                    ──SSH──▶ bridge + herdr on each PC
```

- **Entry:** a dedicated HTTPS port, chosen in the install wizard (default
  27317). Ovis serves its app, `/api`, `/ws`, service worker and manifest at `/`
  and has no base-path setting, so a DSM path (ingress) cannot carry it. A
  top-level path on DSM's own origin is not safe either: the root-absolute
  `/api` and `/ws` would collide with DSM's routes, and a service worker at
  `/sw.js` would take scope over the DSM origin and could intercept DSM's own
  pages. A port of its own gives Ovis a separate origin.
- **Administrators only:**
  - **Icon:** the DSM main-menu icon is shown only to administrators (`allUsers: false`).
  - **Gateway:** every request on the dedicated port (the app, `/api`, `/ws`, static files and
    device pairing) passes the identity gateway's `access: dsm-admins` check. It has no
    passthrough or public paths.
  - **Ovis itself:** listens on `127.0.0.1` only, so nothing reaches it around the gateway.
  - **DSM CGIs:** the package's only CGIs are the SDK's entry gate and gateway CGIs; they are
    admin-only and their permissions default to `administrators`.
  - **Relay and Portal:** both would reach Ovis without the gateway, so
    `OVIS_TAILSCALE_SERVE_ONLY=1` keeps them off. With `OVIS_TOKEN` set, its Tailscale rules
    come after the token check and admit no one.
  - **Regression test:** `tests/local/ovis-admin-only.test.mjs` checks all of this in the
    manifest and in a built SPK.
- **Sign-in:** DSM administrators only (`entry: admin-only`,
  `access: dsm-admins`). Ovis's documented sign-in-proxy setup: the server runs
  with `OVIS_TOKEN`, and the gateway sends it on every admitted request. The
  token is random per install, stored in the package's private var folder
  (`gateway/ovis-token`, 0600) and never reaches the browser. Ovis's own device
  pairing is not needed.
- **Shared identity:** every DSM administrator who opens Ovis is the same Ovis
  user: they see and change the same remote PCs, sessions and settings. That is
  acceptable because only DSM administrators get through the gate; normal DSM
  users are refused before Ovis.
- **Runtime:** official Bun 1.4.2 (upstream's pin; x86_64 uses the baseline
  build for CPUs without AVX2) runs upstream's `server/index.ts`, next to the
  bundled herdr (above). No Node, node-pty or Rust PTY host is shipped. armv7 is not built: Bun has no
  armv7 release.
- **Updates:** Package Center only. `server/index.ts` has no updater (that is
  `server/managed.ts`, which also needs a Git checkout); Settings → Updates says
  updates are unavailable. `OVIS_AUTO_UPDATE` stays `0`.
- **Anonymous usage counts:** off by default, and the user's choice.
  `ovis-launch` writes upstream's own off mark (`telemetry-off` in the state
  folder) on the first start only; **Settings → About → Anonymous usage counts**
  can turn them on, which removes the mark, and later starts keep that choice.
  `OVIS_TELEMETRY` is not set, because it would lock the switch off.
- **Outbound connections:** herdr may check for its own releases; remote-PC bridge bundles come from upstream's GitHub
  release `remote-v<N>`, checked against the SHA-256 in its manifest. The Ovis
  relay (`relay.ovis.sh`) and Portal are disabled (see Administrators only).

## Files

| Path | Purpose |
| --- | --- |
| `dsm-package.yaml` | Package manifest: daemon, identity gateway, metadata |
| `build-source.sh` | Native payload hook: builds upstream with Bun, stages `daemon/<arch>` |
| `prune-runtime.mjs` | Keeps only the `node_modules` packages the server imports |
| `adapter/` | `ovis-launch` (creates the token, marks usage counts off on a first start, runs herdr and Bun) and `ovis-token` (gateway adapter), with tests |
| `assets/icon/`, `assets/make-icon.py` | DSM icon per size: upstream's sheep on a rounded tile; the script that draws it (maintainer tool) |
| `upgrade.sh`, `version.mjs` | Merge an upstream release tag and set the version |
| `upstream-source.json` | The upstream tag, commit and build packaged |
| `backend/`, `i18n/` | SDK entry gate CGI and its messages |
| `LICENSES/` | Our wrapper's license, Bun's and herdr's licenses; upstream's license and notices ship in the payload's `app/` |

## Build

Needs Go (1.26.10 or later), git, curl and unzip. The hook downloads Bun and herdr itself
and checks their SHA-256.

```sh
npm ci                                   # in synology/, for the pinned CLI
bash synology/upgrade.sh v0.4.23 1      # from the fork root: merge a release tag and build
# or rebuild the current tag:
SPL_UPSTREAM_COMMIT=$(git rev-parse v0.4.22^{commit}) bash synology/build-source.sh
cd synology && npx synopkgland check && npx synopkgland build && npx synopkgland inspect dist/<spk>
(cd synology/adapter && go test ./...)
```

When upstream changes its Bun pin (`packageManager`), the hook stops; update
`BUN_VERSION` and the digests from that Bun release's `SHASUMS256.txt`.
