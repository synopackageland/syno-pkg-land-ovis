# AI authoring

Start with `synopkgland skill list`, then `synopkgland skill show orchestrator`.
Use `synopkgland skill path` to locate the bundled references; read the relevant
skill with `synopkgland skill show <id>` before editing.
For UI work, read `synopkgland skill show ui-vue` and `synopkgland skill show ui-json`.
Use the installed SDK contracts and preserve package.json/lockfile pins.

For UI discovery, run `npx --no syno-pkg-land-ui catalog --json` and
`npx --no syno-pkg-land-ui describe Button --json` from the installed frontend.
Read its prompt and exported Vue docs/types before choosing component APIs.
Use `syno-pkg-land-ui` directly in npm scripts; keep AI guidance version-independent.

Open the App at once: render its shell immediately and run session, permission
and status checks in the background, never a full-window "Checking…" placeholder.
Only a confirmed refusal (signed out, expired session, not an administrator)
shows the sign-in page or a `@synopackageland/ui` Dialog; loading text inside a
section is fine.

Everything below holds for every native backend language; the backend
sections at the end name each language's files, types and test commands.

From the project root, run `synopkgland check --json`,
`synopkgland build --json`, then `synopkgland inspect <spk> --json` on the emitted SPK.
For Vue, first run `npm ci --include=dev` and `npm run build` in frontend. Run its tests only if the user chose tests.
Do not connect to or change a NAS without explicit task authorization.

## Optional testing

Ask once whether the user wants tests: none, local/non-AI, or optional AI too.
Accept no tests, remember the choice, do not nag and never block a build or release
on tests. Scaffolding is included by default; writing and running tests are optional.
Read `synopkgland skill show testing` when setting up or extending tests.

The default `npm test` runs only fast local unit/ctl registry contracts.
At the project root, run `npm install --include=dev` first; after committing its
lockfile, use `npm ci --include=dev`.
`NODE_ENV=production` skips devDependencies; `test:local` needs the CLI and frontend builds need dev tools.
Before `test:local`, prepare the backend/frontend as described below and run
`synopkgland ctl generate` for a native App.
`npm run test:local` opts into check/build/inspect. Only on explicit user request,
run `npm run test:nas:ctl`, `npm run e2e:smoke`, `npm run e2e:ui` or `npm run e2e:ai`.
The full suite is slow. Never call these from build, install, DSM hooks or release.
For non-AI use, no model/key is required; AI dependencies are optional installs.

Assert meaningful ctl values and the UI primary action's real result, not only
schemas or page rendering. Adapt the generated examples to actual App operations.
Long jobs use start -> progress -> pause -> resume without reset -> cancel with
bounded polling and fixture cleanup; never wait for full completion.

AI steps require a user-selected endpoint, key location and model. If no profile
is configured and setup is unknown, ask the user for those three values. Choose
the cheapest adequate model and low effort; never default to an expensive model.
Use E2E_LLM_BASE_URL, E2E_LLM_MODEL, E2E_LLM_API_KEY_FILE (mode 600) or
E2E_LLM_API_KEY, and optional E2E_LLM_REASONING_EFFORT (default low).

## Upstream web UI only

For an `--upstream-fork` wrapper whose only UI is the upstream web panel, set
`ui: false` in `synology/dsm-package.yaml` and configure `identity_gateway`
for that panel. `--ui plain` selects `ui/app.js`; it does not mean `ui: false`.
The current scaffold still creates UI and browser-test examples: remove
`synology/ui/` (or `synology/frontend/` for Vue), `synology/tests/e2e/`,
`synology/e2e.config.ts` and `synology/e2e.ai.config.ts`, and remove the
`e2e:*` scripts from `synology/package.json`.
Keep `synology/backend/`, `synology/i18n/messages/`, `synology/operations.json` and
the local/ctl tests: the entry gate still needs the backend and message catalogs.
Read `synopkgland skill show upstream-fork` and
`synopkgland skill show orchestrator` for the wrapper workflow.

## Lifecycle scripts (`dsm-scripts/`)

`dsm-scripts/` holds DSM's own lifecycle scripts (preinst, postinst, preupgrade,
postupgrade, preuninst, postuninst, start-stop-status); each does nothing until
edited. The build calls each from the generated script of the same name, as the
package account, around the SDK's own steps. Read
`synopkgland skill show lifecycle` before editing them; never add root work,
never repeat SDK steps, and keep only DSM script names in that folder
(build tools belong in `scripts/`).

Root cleanup or migration after an install or upgrade (for example removing
what an older version left in a root-owned place) is not a lifecycle script:
name a `root/` program as `root.start` in `dsm-package.yaml`. The executor runs
it as root once after each install or upgrade, at the first start of the new
version and only while the administrator's approval is live; it must not
declare `run` or `job`, should be safe to run twice, and declaring it means the
administrator approves the App again. Package start only requests the
executor, so `postinst`, `postupgrade` and `start` cannot wait for the step:
keep the unprivileged follow-up in `dsm-scripts/`, and read the step's result
from the executor `status` (`start_step`).

## Who may use the app (`entry`)

Omit `entry` to keep legacy access. `entry: admin-only` admits DSM administrators
only. `entry: app-privilege` also admits accounts that Control Panel >
Application Privileges allows for `SYNO.SDS.<package>.Instance`; DSM's install
default allows administrators only, so nobody else enters until an admin grants
access. One CGI check covers every route; failures refuse; changes apply within
30 seconds. Never re-implement AppPriv rules in the App.
Declare every App method once in operations.json with a required ai class. Run
`ctl generate` to generate permissions, routes, commands, AI tools and the App
reference; run `ctl generate --check` and `check` to refuse drift. Read generated
operation descriptions instead of maintaining a second list in a skill. Dynamic method names and unregistered HTTP dispatchers are rejected; use
fixed SDK route constructors. Use identity_gateway.upstream_auth_paths for
client API paths authenticated by the upstream.
`api.HandleHTTP("/proxy/", "app.proxy", handler)` serves raw HTTP
for literal, disjoint full-segment prefixes outside SDK reserved paths. Declare
the key as authenticated or administrators in operations.json, then generate. Origin, entry,
identity and permission checks precede raw streaming/initial-upgrade handlers;
CGI itself cannot upgrade a connection. For package-private worker HTTP use
`ipc.ServeUnix(ctx, path, handler)` and `ipc.DialUnix`: owned mode-0600 sockets
directly beneath the package's private `var/run`, with no custom TCP listener.
The audit follows configured binaries' Linux import graphs and excludes unshipped
dev commands.
Hyper Backup lists the App as an application by default: the build adds a
fixed `scripts/backup/` shim that Hyper Backup runs as root, which only
switches to the package account and moves one archive of the data folder. The
App itself still runs as the package user. Never add or edit files in
`scripts/backup/`; opt out with `backup: { hyper_backup: false }` in
dsm-package.yaml.
Mark routes ordinary users call `authenticated`, mark whole admin routes
`administrators`, guard global settings inside shared routes with
the SDK's require-admin check, and hide admin controls with `ctx.user?.isAdmin`
(UI only, never the authority). Declare actual App permissions in operations.json and regenerate the policy.

## DSM Application Portal (`portal`)

From DSM 7.2-64570, Apps can declare `portal` without an executor. DSM's
web-config resource registers the entry, and INFO
`instuninst_restart_services="nginx.service"` asks DSM to reload nginx after
install, upgrade and uninstall. The reload is asynchronous; package hooks stay
unprivileged and never reload nginx themselves. Schema-2 Compose/gateway Apps
may also use a top-level `portal`; legacy `integration.web.portal` is unsupported.

`synopkgland add portal` adds `portal: {}` (or set
`portal: {alias: my-app}` in dsm-package.yaml). Without an executor, set
`os_min_ver` to 7.2-64570 or later before adding Portal; the command refuses a
lower target and never raises it silently. With an executor, a portal needs
`os_min_ver` 7.1-42661 or later. Below DSM 7.2 the executor registers the entry:
DSM 7.1 rejects the unsigned web-config worker, there is no alias wizard step,
and an administrator enables it through `ctx.portal.register()` after executor
approval. From DSM 7.2, the SPK registers it as a DSM resource.

Read `synopkgland skill show portal` before changing it. The install wizard only
proposes the alias; an administrator activates it in the App's portal panel
(`ctx.portal`, DSM's own `SYNO.Core.AppPortal` in the admin session) or Control
Panel > Login Portal > Applications. Other signed-in users get the link from
read-only `portal.status`, else "ask an administrator". Never write
`AppPortal.json` or `@appconf/<package>`, never add a backend route that sets
the alias, and never relay QuickConnect responses or build QuickConnect links.
Beyond the packaging floor, support is detected with SYNO.API.Info.

### Login page look (`login_style`)

A portal build gives the Login Portal entry a default look: a neutral
wallpaper, its thumbnail in Control Panel > Login Portal > Applications, the
login button colour `#2F6BD8`, and the App icon beside the NAS name on the
alias login page. To use your own images, add to dsm-package.yaml (each key is
optional; `wallpaper` and `wallpaper_thumbnail` go together):

```yaml
login_style:
  color: "#0F766E"                                   # login button, #RGB or #RRGGBB
  wallpaper: branding/login-wallpaper.jpg            # JPEG, at least 1920x1080, at most 2 MiB
  wallpaper_thumbnail: branding/login-thumbnail.jpg  # JPEG 320x200, the same picture
  title: branding/login-title.png                    # PNG 680x124, replaces icon and NAS name
```

`synopkgland check` refuses a missing image or one of the wrong format or size
(`STANDALONE_LOGIN_STYLE_IMAGE_INVALID`) and prints the style in effect;
`synopkgland inspect <spk>` shows what the SPK carries. Use only images you
may redistribute; never copy Synology's.

## Background jobs (`jobs`)

Submit only files the App's own producer has already imported into its private
data directory, with `{key, name, input}`. The existing worker processes a
bounded private copy. Schedules reprocess that retained copy with explicit
consent; they never reread a source file. Keep submission, progress and
cancellation on the existing SDK contracts.

The worker runs the App's Work for one claimed job at a time, as the package
account. The Work receives the job's Task: it reads the private input copy,
reports progress (`done`/`total`, `bytes`/`totalBytes`, `item`) and saves a
checkpoint, at most 16 KiB of text, only once the work it describes is done.
Owners pause and resume their jobs (`bg.jobs.pause`, `bg.jobs.resume`; the
SDK BackgroundPanel shows the buttons): a queued job pauses at once, a running
one at its next checkpoint poll. A package stop pauses the running job too, and
the next start resumes it. A resumed job runs again from its last checkpoint,
or from the start when it saved none, so a Work must be safe to repeat from any
checkpoint it saved. Delegated jobs cannot pause, and the pause routes answer
`conflict` for them. The template's Work hashes the input, reports bytes read
and checkpoints the hash state every 64 KiB; replace it with the App's own work.

## Uninstall hook (`uninstall`)

Declare `uninstall.purge` in dsm-package.yaml (the template ships it
commented) to run a fixed package executable on a real uninstall (not an
upgrade), as the package account, before the executor is removed. It gets
`SYNOPKGLAND_DELETE_STATE=0|1` and `SYNOPKGLAND_REMOVE_DATA=0|1` from the
uninstall wizard and nothing else from the environment. Release what the App
holds outside the package on every uninstall; delete the state listed in
`uninstall.state.items` (each key needs `uninstall.state.<key>` in
i18n/messages) only with `SYNOPKGLAND_DELETE_STATE=1`, and keep what a
reinstall can reuse otherwise. A failure or timeout (2–120 s, default 60) is
logged to syslog tag `synopkgland-uninstall` with output in
`var/uninstall-hook.log` and never blocks the uninstall. `purge.root: <op>`
(rootOps only) adds a root step `root/<op>.sh [delete-state]` that runs only
while the administrator's approval of that exact policy stands; sessions
cannot call it through `root.run`. The template backend's `uninstall` mode
deletes the background job spool when asked.

## Permanent destination integration

`init --with delegate` / `add delegate` default to `operations: [files.moveFromStaging]`,
`destinations: [save]`, `lifetime: until-revoked`, `scope: install-selected`. Existing legacy
TTL declarations retain their 1–3600 second service / 30-day wire ceiling and
64 MiB / 60-second URL example. Do not apply those limits to the new move API.
Delegate can coexist with daemon; ordinary standalone jobs cannot. Preserve
App business route floors. Binary/compose remain unsupported.

Use File Station `/home[/...]`, never `/homes/<user>`. The SDK destination picker
selection itself grants access after fresh DSM session/UID/write verification,
without a second consent dialog. Ordinary and administrator subjects use their
own nonzero UID/current groups, never root. An online App can create an in-scope
grant using a valid session; there is no trusted external consent or notification.

Use the default `createBackgroundSaveFacade(ctx)` / CapabilityPanels management
page. `delegate.status` exposes verified `disabled` and manifest mode; missing
protected evidence must not imply enabled. `delegate.operations.list` accepts
only `{}`, uses the backend move mirrors, and returns the caller's username+UID
operations only, including for admins. Mirrors are not completion receipts:
query original `delegate.operations.get` and show Saved only for committed with
a receipt. Cancel by the same reference; revoke by grant ID. Never resubmit
uncertain/interrupted work. Admin disable is protected and revokes grants;
enable/mailbox start does not resurrect them. Keep IDs, never session credentials.

App-owned daemon completion and scheduled CSV export call the configured
delegate service's move-from-staging with `delegation_id`, stable `job_id`, private relative
`staging`, `name`, `size`. No owner/UID selector. Bounds: 16 GiB/file, queued16,
active1/package, 24-hour attempt, 5-second revalidation, 10 MiB operation records,
50 MiB audit. Scope is a usage restriction, not hard subtree confinement.
Local production-executor fixtures use fake DSM/peer/bulk writer; do not claim
NAS ACL, actual writer UID, logout/reboot or full lifecycle acceptance from them.
Gopeed product wiring remains a separate task.

Admission recovery (per job): on `EXECUTOR_UNAVAILABLE`, `EXECUTOR_TIMEOUT`,
`EXECUTOR_PROTOCOL`, `canceled`, `EXECUTOR_PAUSED`, `queue-full`,
`STORAGE_ERROR`, `ADMISSION_PENDING`, `jobs_busy` or `jobs_unavailable` (the
package journal was busy or failed transiently; a caller deadline or cancel
spent waiting for it is `canceled`), call move-from-staging again later with
the same exact params. It queries the original reference first and re-sends
admission only when the executor does not know it and the pinned source is
unchanged; executor tuple deduplication keeps one operation, so a lost reply or a
racing caller never copies twice. At most 10 possibly-delivered sends, 2 s
backoff doubling to 5 min; the worker sweep does the same. Terminal, do not
retry: `APP_DENIED`, `DELEGATION_DENIED`, `operation-conflict`, `disabled`,
`SCOPE_DENIED` and other executor refusals, `STAGING_CHANGED`,
`ADMISSION_EXHAUSTED`, `spool_unsafe` and `journal_corrupt` (repair the package
spool first), and `operation-not-found` for an accepted operation.
Never change job/grant IDs, delete mirrors or call the socket directly to retry.

Cancellation (per job): the service's cancel-operation / `delegate.operations.cancel` with the
original ref is idempotent; repeat it with the same ref after any retryable
error or a lost reply. A reply `queued`→`canceled` is final at once. A copy in
flight answers `copying` with code `CANCEL_REQUESTED`: the cancel is accepted and
durable, not finished; poll the operation status on the same ref until terminal
(repeating the cancel is harmless). `canceled` (`OPERATION_CANCELED`) guarantees the writer
stopped, its part file is removed, nothing was published and staging is kept; it
stays canceled across executor crash, package stop/start and logout. `committed`
means publication won the race: the file stays (cancel never removes it) and only
the app record may be dropped. `uncertain` means a publish attempt failed before
the cancel; keep querying. A cancel of a ref the executor has not admitted yet
(reserved/sent mirror) returns `canceled` and leaves a tombstone, so a late or
recovered admission of that ref returns `canceled` and never copies; a producer
cancel is also written to the mirror first, so move-from-staging and the worker
sweep forward the cancel instead of admitting it.


### Install-selected scope and the destination API

- The DSM install wizard dynamically lists local shares (`SYNO.Core.Share list`
  via `api_store`) and offers home. It saves only a package-owned pre-selection
  in `var/delegate-install.json`; never trust that file for authority.
- The existing administrator PasswordConfirm form displays this pre-selection
  at first enable. `ctx.executor.authorize(password, selectedScope)` confirms it
  through a pinned, exact-readback one-time root task. Settings changes use the
  same form/password path; no App CGI route may set protected scope directly.
- The executor trusts `/usr/local/etc/synopkgland/<pkg>/delegations/selected-scope.record`
  only. Its scope/generation are runtime protected state; shipped policy pins
  lock the selection mode and operations. Changes revoke old grants and cancel
  pending writes. Change-away-and-back never revives an old grant.
- Follow `selection` from verified `delegate.status` / `sdk.destination(key)`.
  If the draft differs, display the difference and follow the protected value.
  Home and fixed `shared:<name>` manifests stay compatible.
- Frontend: `const sdk = createBackgroundSaveFacade(ctx); const current = await
  sdk.destination('save')` returns typed `{key, selection, destination, status}`.
  `selection` contains `{scope, root, generation}`; `destination` is this caller's
  grant or null. Handle active/missing/reselection-required/needs-scope-confirmation/disabled.
- Backend: the service's destination lookup verifies the fresh DSM session and
  username+UID. Capture its active grant ID into the App job. The offline
  producer uses the existing move-from-staging; no user/UID selector, cookie,
  token, or browser path cache. Executor revalidates at use/publication.
- The generated Background saves page includes a working **Save example file**
  (`sdk.exportExample({key, job_id})`, `delegate.example.export`). The SDK's
  export-example creates a private JSON file, calls move-from-staging once, and
  repeats the original request on repeated IDs. Review it in the SDK's delegate
  package to adapt a real producer; show Saved only from a protected committed
  receipt.
- Verify the wizard on your own DSM release over an SSH local forward. The
  template ships the wizard contract; it is not proof of every DSM release or of
  full writer/reboot behavior.

## HTTPS and stale DSM tokens

Do not force HTTPS for ordinary CRUD/settings/file Apps. Only secure context
features, pairing or a strict secure Origin policy justify it. Return
`https_required`, not a generic authentication error, and render the SDK's
`identityCopy(locale).httpsTitle` / `errors.https_required` with reopening
instructions. Secure-origin enforcement is opt-in and does not replace
authentication, origin or permission checks.

Call `readDsmPorts` for `dms_https` only when `ctx.user?.isAdmin === true`;
`readDsmHttpsURL(ctx, location.href)` applies this guard. Ordinary users see
instructions only: PortInfo can refuse their session or close the DSM window.
Do not guess a configured port.

For an old tab's stale `X-SYNO-TOKEN`, SDK `createClient` can opt in through
`sessionRetry` and `createDsmSession.refresh(rejectedToken)`. Only trusted
`effect: read`, `kind: rpc` calls retry once on `session_required`. Writes, AI,
streams, batches and unknown methods never retry. Existing `ctx.call` is not
automatically wrapped. Verify the installed SDK exports these APIs before
using them. Follow the [CLI skill](https://github.com/synopackageland/syno-pkg-land-cli/blob/main/skills/SKILL.md)
and [SDK guide](https://github.com/synopackageland/syno-pkg-land-sdk/blob/main/docs/https-and-session-recovery.md).
When the user requests NAS acceptance, test both HTTP/HTTPS entries, both user roles and cross-tab session recovery.

## Go backend

- `api.RequireSecureOrigin(request)` opts into secure-origin enforcement; it does
  not replace authentication, origin or permission checks.
- Routes: `backend/internal/app` (`routes.go` registers them); App
  permissions: `backend/internal/api/permissions.json`. Test with
  `(cd backend && go test ./...)`, then `go vet ./...` and `gofmt -l .`.
- Admin guard inside a shared route: `api.RequireAdmin(caller)`.
- Jobs: `background.Work` is `func(ctx, *background.Task) (background.Result,
  error)`; call `t.Report(background.Progress{...})` and `t.Checkpoint(state)`,
  and resume from `t.Job.Checkpoint`. The template's Work is
  `app.HashWork` (`background_work.go`), set as `Runner.Work` in
  `cmd/background-worker` and as `delegate.Service.Work` with delegate.
- Uninstall hook: `app.Uninstall(dataDir, deleteState)` (`uninstall.go`),
  reached by `api.cgi uninstall`.
- Delegate: `delegate.Service.Destination(ctx, caller, request, key)`,
  `Service.MoveFromStaging`, `Service.CancelOperation`,
  `Service.OperationStatus`, `Service.ExportExample` (the SDK's
  `delegate/example.go`).

## Python, Node.js and Rust backends

The SDK runtime is the CGI: it checks the DSM session, the entry gate and
permissions.json, then runs the App once per call, as the package account, and
answers the browser itself for every refusal. The App only registers methods
and asks the runtime for services with `ctx.runtime(op, params)`: `bg.submit`/`bg.get`/`bg.cancel` (the
caller's jobs, `jobs: true`, whose worker hashes each job's input), `root.run`
(`rootOps`), `delegate.move_from_staging` (`delegate`) and `portal.status`. A
service refusal is a code the browser may see; pass it on. The admin guard
inside a shared method is `ctx.admin`. State lives in
`SYNOPKGLAND_DATA_DIR`. `synopkgland add` does not change these backends: choose
their capabilities with `synopkgland init --with`.

## Python backend

- Methods: `backend/app/routes.py` (`build` registers them, the capability
  modules `jobs.py`, `root_ops.py`, `delegate.py` add theirs); permissions:
  `backend/app/permissions.json`. DSM runs it with its own python3 (3.8):
  standard library only, nothing newer than 3.8.
- Params: `fields(params, name=type)` (`params.py`); errors:
  `Error(code, message)`, `Error.bad_request(message)`; anything else is
  `internal`. `ctx.runtime` raises the runtime's refusal as an `Error`.
- Test with `(cd backend && python3 -m unittest discover -s tests)`;
  `tests/support.py`'s `call` plays the runtime, with the services a test answers.

## Node.js backend

- Methods: `backend/src/routes.ts` (`build` registers them, the capability
  modules `jobs.ts`, `root-ops.ts`, `delegate.ts` add theirs); permissions:
  `backend/app/permissions.json`. TypeScript with `"module": "NodeNext"`;
  `npm run build` compiles it into `backend/app`, which the SPK ships, so run it
  before `synopkgland check` or `build`.
- Params: `strings(params, ...names)` (`params.ts`); errors:
  `new AppError(code, message)`, `AppError.badRequest(message)`. Call services
  through `service(ctx, op, params)` (`service.ts`), which passes the runtime's
  refusals on; a bare `ctx.runtime` refusal would reach the browser as `internal`.
- Test with `(cd backend && npm ci --include=dev && npm test && npm run typecheck)`; `test/support.ts`'s
  `call` plays the runtime.

## Rust backend

- Methods: `backend/src/routes.rs` (`build` registers them, the capability
  modules `jobs.rs`, `root_ops.rs`, `delegate.rs` add theirs); permissions:
  `backend/permissions.json`. The build cross-compiles the binary, which cannot
  run on the build machine: list every method in `backend.methods` of
  dsm-package.yaml too.
- Params: a `#[serde(deny_unknown_fields)]` struct and `params.decode()?`;
  errors: `Error::new(code, message)`, `Error::bad_request(message)`,
  `routes::internal(e)` for the unexpected. `ctx.runtime::<T>(op, &params)?`
  passes the runtime's refusal on.
- Test with `(cd backend && cargo test --locked)`, then
  `cargo clippy --locked --all-targets -- -D warnings` and `cargo fmt --check`;
  `test_support::call` plays the runtime. `rust-toolchain.toml` pins the
  toolchain and the DSM targets; keep `Cargo.lock` committed.

## App launch mode

Native Apps declare `ui.launch: dsm-window | new-page` in `dsm-package.yaml`.
Use `dsm-window` for frontends built with `@synopackageland/ui`; use `new-page`
for every other frontend. Plain/binary scaffolds explicitly choose new-page; Vue uses the native default. An omitted
native `ui.launch` resolves to `dsm-window`; the CLI does not guess a frontend's
framework from its build command. `new-page` registers DSM `type: "standalone"`,
so the main menu and Package Center open a full page on the DSM origin.
`ctx.fullPage`, `ctx.appIcon`, session/SynoToken, launch parameters and entry
permissions use the same wrapper in both modes. The wrapper sets the standalone
page title, favicon and device-width viewport. It retains ExtJS; `pureEnv` is
not appropriate, and alternate Portal ports are not needed (`allowAltPort` is omitted).

A full page does not need Application Portal. Omit `portal` unless an alias is
explicitly wanted; only that opt-in adds `defaultAlias`, login styling, Portal
panels and capabilities. No Portal is a healthy default for inspect/info/doctor.

Web service Apps declare `integration.web.launch`; its built-in default is `new-page`.
Native SDK Apps default to `dsm-window`; non-UI-SDK native frontends must
explicitly set `ui.launch: new-page`. Headless Compose Apps have no launcher.
The new-page main menu uses `type: "url"` with an explicit `url` at the existing trusted entry: the authenticated
gateway, or a Web Station alias only with explicit `access: public`. DSM rewrites standalone URLs to a launchApp shell, which needs an AppInstance
and AppWindow JS class. A schema-1 web service has no such class, so it uses url.
The schema-2 identity_gateway + compose SDK wrapper has the class and uses standalone.
Authenticated web Apps do not create a Web Station alias or Application Portal.
`dsm-window` remains an explicit web launch option with its URL launcher.

Schema 2 `dsm-package.yaml` with `identity_gateway` and `compose` is also a web
service: omitted `ui.launch` resolves to `new-page`, without Application Portal.
Both Compose routes share `contract/app-launch.ts` for defaults and launcher types.
An explicit `ui.launch` overrides this default. Native SDK Apps without a web
service keep `dsm-window` when `ui.launch` is absent.

A native App that wraps a web service with `identity_gateway` and has its own
settings may declare `console: {}`. The main menu then shows the service UI
(`displayname`, a url launcher that opens a new tab) and an administrator-only
DSM window, "<displayname> for DSM", running the SDK UI; Package Center's Open
starts the console. Do not declare `ui.launch` or `portal` with it, and do not
redirect from the console to the service. Simple wrappers keep one icon.

## Integrating an existing open-source App

If you package someone else's open-source project, set maintainer to the original project/owner (prefer the project's own team/org name, else the owner's display name, else login), metadata.maintainer_url to the upstream repo, and yourself as metadata.distributor/metadata.distributor_url. Resolve all attribution TODOs before release; run check and review its warnings. Only your own Apps use your publisher as maintainer.

Keep the upstream fork/history and remote upstream. All wrapper-owned files belong in synology/; never edit upstream files or treat the App as an SDK component. Read synopkgland skill show upstream-fork.

Leave ctl absent from the manifest by default. Inventory upstream APIs with `synopkgland ctl import-openapi openapi.json --output synology/operations.json` (or import a handwritten v2 registry); every new entry is hidden. Group discovered APIs by area, recommend a small useful set, and ask the user about each API. Only explicitly approved APIs may be exposed; never use an all-or-nothing opt-in. Keep all specs and adapters in synology/ and reuse the existing external/ingress transport. Add ctl.registry only after implementing the selected adapters, then run ctl generate and ctl generate --check. If requested, read synopkgland skill show ctl and produce ctl-plan covering existing UI actions and public API endpoints, including common reads and writes. Status-only commands do not meet completion. Wrap existing native APIs without changing upstream code or inventing features. Long operations must return through existing asynchronous APIs without waiting for completion; report synchronous-only long APIs as blockers for discussion.

Commit the scaffold, then run bash synology/upgrade.sh <upstream-tag> <build-number>. It fetches the exact tag, verifies ancestry/untouched upstream files, merges, derives version/provenance, checks and builds. Commit synology/ before upgrading again. It does not publish or touch a NAS.

Native App: implement build-source.sh as the native payload hook; it deliberately fails until implemented. Stage official release binaries pinned by SHA-256 (the OpenList route), or compile the merged source with a source build hook. Verify the release digest before staging architecture-specific daemon.targets; keep the SDK management shell separate.

Use DSM administrator identity without an additional App password, an OIDC/SSO server or upstream source changes. Prefer supported trusted-header/proxy auth (FreshRSS), otherwise disable upstream login/enable guest behind the administrator-only gateway after a bounded first bootstrap (Uptime Kuma disableAuth). The third shipped pattern is the existing SDK token adapter: mode: token, access: dsm-admins, entry: admin-only. OpenList obtains its native API token with the unmodified admin token CLI while stopped, stores it package-private 0600 under a 0700 directory, and the gateway injects Authorization server-side after DSM admission; packaging never delivers it to the browser. All DSM admins share one upstream admin; token reset requires package restart; WebDAV keeps its own native credentials. The upstream admin token/settings APIs remain unchanged. Verify actual DSM admission/revocation separately. Use supported config, environment, argv or proxy adapters. Every App lifecycle phase runs as the package UID. For upstream fixed-UID data directories, currently provide a synology/ startup adapter or writable path without root/chown; replace it with the framework ownership mechanism when available. Preserve licenses and the full source/component inventory.
