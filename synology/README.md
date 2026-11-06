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
SSH as the package account and starts its own bridge next to their herdr. The
NAS runs no herdr and no agents, so the sidebar's row for the NAS itself always
says herdr cannot be reached.

```text
browser ──HTTPS :27317 (DSM nginx, DSM certificate)──▶ SDK identity gateway
          DSM administrator sign-in; token adapter adds Authorization: Bearer <OVIS_TOKEN>
                                                    ──▶ Ovis (Bun) on 127.0.0.1:37317
                                                    ──SSH──▶ bridge + herdr on each PC
```

- **Entry:** a dedicated HTTPS port, chosen in the install wizard (default
  27317). Ovis serves its app, `/api`, `/ws`, service worker and manifest at `/`
  and has no base-path setting, so a DSM path (ingress) cannot carry it.
- **Sign-in:** DSM administrators only (`entry: admin-only`,
  `access: dsm-admins`). Ovis's documented sign-in-proxy setup: the server runs
  with `OVIS_TOKEN`, and the gateway sends it on every admitted request. The
  token is random per install, stored in the package's private var folder
  (`gateway/ovis-token`, 0600) and never reaches the browser. All DSM
  administrators share one Ovis identity, and Ovis's own device pairing is not
  needed.
- **Runtime:** official Bun 1.4.2 (upstream's pin; x86_64 uses the baseline
  build for CPUs without AVX2) runs upstream's `server/index.ts`. No Node,
  node-pty, Rust PTY host or herdr is shipped. armv7 is not built: Bun has no
  armv7 release.
- **Updates:** Package Center only. `server/index.ts` has no updater (that is
  `server/managed.ts`, which also needs a Git checkout); Settings → Updates says
  updates are unavailable. `OVIS_AUTO_UPDATE` stays `0`.
- **Anonymous usage counts:** off. `OVIS_TELEMETRY=0` is set in the daemon
  environment, which wins over the in-app switch.
- **Outbound connections:** remote-PC bridge bundles come from upstream's GitHub
  release `remote-v<N>`, checked against the SHA-256 in its manifest. The Ovis
  relay (`relay.ovis.sh`) stays off until a user turns it on.

## Files

| Path | Purpose |
| --- | --- |
| `dsm-package.yaml` | Package manifest: daemon, identity gateway, metadata |
| `build-source.sh` | Native payload hook: builds upstream with Bun, stages `daemon/<arch>` |
| `prune-runtime.mjs` | Keeps only the `node_modules` packages the server imports |
| `adapter/` | `ovis-launch` (creates the token, execs Bun) and `ovis-token` (gateway adapter), with tests |
| `upgrade.sh`, `version.mjs` | Merge an upstream release tag and set the version |
| `upstream-source.json` | The upstream tag, commit and build packaged |
| `backend/`, `i18n/` | SDK entry gate CGI and its messages |
| `LICENSES/` | Our wrapper's license and Bun's license; upstream's license and notices ship in the payload's `app/` |

## Build

Needs Go (1.26.10 or later), git, curl and unzip. The hook downloads Bun itself
and checks its SHA-256.

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
