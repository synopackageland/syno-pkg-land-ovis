# Installed Ovis acceptance

This opt-in suite exercises an already installed Ovis package. It opens the DSM
main menu, starts a private shell workspace through Ovis, types a harmless command,
reads its actual output, and stops/starts **Ovis only** to check persistence.
It also checks administrator admission, anonymous/nonadministrator refusal,
container isolation, and telemetry/relay/Portal state. It never installs,
upgrades, uninstalls, reboots, changes accounts, or changes outbound settings.

Run only against a device authorized for a brief Ovis outage. Other Ovis clients
will disconnect during stop/start. Existing workspaces are left alone. The suite
creates a uniquely named directory in the container's data volume and a workspace
using that directory; its final case removes those fixtures and leaves Ovis running.
Nine cases run once, without case retries. Startup/reconnect predicates allow three
minutes; individual browser actions allow the same budget.

## Setup

Use Node 24.8 or newer, SSH key access to the authorized device, and an installed
`playwright` (1.63 tested) with Chromium. Install/build the
`@synology-vincentt/e2e-clef` library in your test environment. Clef's DSM adapter
confirms desktop readiness; Playwright handles deterministic application controls.
No model, provider, AI key, saved browser profile, or additional NAS API is needed.
The test does not modify the package's dependency pins.

Create a private JSON configuration outside Git (`chmod 600`):

```json
{
  "dsmUrl": "https://<device>:<dsm-port>",
  "appUrl": "https://<device>:<ovis-port>",
  "sshTarget": "root@<device>",
  "containerName": "<installed-ovis-container>",
  "expectedVersion": "0.4.22-0005",
  "admin": {"username": "<administrator>", "password": "<from-secret-store>"},
  "normal": {"username": "<existing-nonadministrator>", "password": "<from-secret-store>"}
}
```

Use an existing local DSM nonadministrator account with password login and desktop
access; the suite never creates one or changes its permissions. SSH is used for
package status/stop/start, filtered container inspection and this run's fixture
directory only. It requires access to DSM's `synopkg` and Container Manager's
Docker executable. It never dumps container environment variables or tokens.
The DSM/app URLs must use the same device hostname for DSM browser sign-in.
Test contexts accept the device's HTTPS certificate and use fresh, isolated
browser storage.

```sh
export OVIS_E2E_CONFIG="<private-config-file>"
export OVIS_E2E_OUTPUT="<private-evidence-directory>"
npm --prefix synology run e2e:nas
```

The default imports resolve `playwright` and `@synology-vincentt/e2e-clef/dsm`
from the runner's environment. To reuse separately installed or locally built
dependencies, set `OVIS_E2E_PLAYWRIGHT_MODULE` to Playwright's `index.mjs` and
`OVIS_E2E_CLEF_MODULE` to Clef's built `dist/dsm/index.js`. Set `OVIS_E2E_BROWSER`
only when using a system Chromium executable. No developer-specific paths are
baked into the suite. Run browser checks on the project's designated Linux test
host/container; supply any SSH tunnel or proxy in that environment before starting.

## Evidence and failure recovery

`results.json` contains each case's outcome, selected safe readback and screenshot
filenames. Screenshots include only the Ovis menu icon, the refusal explanation,
and this run's terminal canvas. Credentials, browser traces, storage state,
HTTP bodies and other people's terminal/sidebar contents are not recorded.
Keep the whole evidence directory private and outside Git.

An unexpected version stops the run before fixture creation or restart. Failures
in individual cases are recorded while independent cases continue; failures are
not retried. Inspect `results.json` and the named case/stage before proposing a
product fix. The suite deliberately omits raw exception text because it may
contain secrets or private DOM values.

If an external interruption prevents final cleanup, the private `results.json`
names this run's `ovis-e2e-<uuid>` fixture. Confirm that exact workspace name and
its `/data/home/ovis-e2e-<uuid>` directory before removing the workspace, its
`marker` file and the now-empty directory. Start Ovis through Package Center if
needed. Never remove an unrelated workspace or the persistent volume.

Passing this focused suite establishes the observed installed version and tested
device only. It does not establish reboot, upgrade, fresh-install, agent-install,
other architectures, or exhaustive isolation/security acceptance.
