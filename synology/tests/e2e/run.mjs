// Opt-in acceptance against an installed App. Never imported by build/install hooks.
import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { mkdir, readFile, stat, writeFile } from 'node:fs/promises';
import { resolve, join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { promisify } from 'node:util';
import { pathToFileURL } from 'node:url';
import { createDiagnostics } from './diagnostics.mjs';
import { fixtureWorkspaceControl } from './workspace-control.mjs';

const run = promisify(execFile);
assert.ok(!process.env.OVIS_E2E_FOCUS || process.env.OVIS_E2E_FOCUS === 'shell-persistence',
  'OVIS_E2E_FOCUS supports only shell-persistence');
const focused = process.env.OVIS_E2E_FOCUS === 'shell-persistence';
const focusedCases = new Set(['shell-command', 'restart-persistence', 'cleanup-and-running']);
const configPath = process.env.OVIS_E2E_CONFIG;
assert.ok(configPath, 'Set OVIS_E2E_CONFIG to a private JSON file');
assert.equal((await stat(configPath)).mode & 0o077, 0, 'Configuration must be mode 600');
const config = JSON.parse(await readFile(configPath, 'utf8'));
for (const key of ['dsmUrl', 'appUrl', 'sshTarget', 'containerName', 'expectedVersion'])
  assert.ok(typeof config[key] === 'string' && config[key], `Missing configuration: ${key}`);
assert.match(config.containerName, /^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/);
assert.match(config.sshTarget, /^[a-zA-Z0-9_.@-]+$/);
assert.ok(!config.sshTarget.startsWith('-'));
for (const role of ['admin', 'normal'])
  assert.ok(config[role]?.username && config[role]?.password, `Missing ${role} account`);
assert.notEqual(config.admin.username, config.normal.username);
for (const key of ['dsmUrl', 'appUrl']) {
  const url = new URL(config[key]);
  assert.equal(url.protocol, 'https:');
  assert.ok(!url.username && !url.password && !url.search && !url.hash);
}
const moduleName = (env, fallback) => process.env[env]
  ? pathToFileURL(resolve(process.env[env])).href : fallback;
const { dsmAdapter } = await import(moduleName('OVIS_E2E_CLEF_MODULE', 'e2e-dsm-adapter/dsm'));
const { chromium } = await import(moduleName('OVIS_E2E_PLAYWRIGHT_MODULE', 'playwright'));
const evidence = resolve(process.env.OVIS_E2E_OUTPUT || '.e2e/ovis');
await mkdir(evidence, { recursive: true, mode: 0o700 });
const fixture = `ovis-e2e-${randomUUID()}`;
const fixtureDir = `/data/home/${fixture}`;
const docker = '/var/packages/ContainerManager/target/usr/bin/docker';
const packageCLI = '/usr/syno/bin/synopkg';
const results = [];
let browser, adminContext, app, workspaceId, paneId, directoryCreated = false;
let restartAttempted = false;
let startAttempted = false;
const diagnostics = createDiagnostics({ evidence, page: () => app });
const { mark, step } = diagnostics;
const quote = value => `'${value.replaceAll("'", "'\\''")}'`;

// Discard SSH diagnostics: credentials, hostnames and remote paths never enter results.
async function ssh(command) {
  try {
    return (await run('ssh', ['-o', 'BatchMode=yes', '-o', 'LogLevel=ERROR',
      '-o', 'ConnectTimeout=15', config.sshTarget, command],
    { timeout: 300_000, maxBuffer: 2 * 1024 * 1024 })).stdout.trim();
  } catch { throw new Error(`Remote operation failed at ${diagnostics.current.stage}`); }
}
const container = args => ssh(`${docker} exec ${quote(config.containerName)} ${args}`);
async function until(read, predicate, message) {
  const deadline = Date.now() + 180_000;
  let last;
  do {
    try { last = await read(); if (predicate(last)) return last; } catch { /* startup/reconnect */ }
    await delay(1000);
  } while (Date.now() < deadline);
  throw new Error(message);
}
async function api(path, body) {
  const result = await app.evaluate(async ({ path, body }) => {
    const response = await fetch(path, { method: body === undefined ? 'GET' : 'POST',
      headers: body === undefined ? {} : { 'content-type': 'application/json' },
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
      signal: AbortSignal.timeout(30_000) });
    return { status: response.status, value: await response.json() };
  }, { path, body });
  assert.equal(result.status, 200, `App request failed: ${path}`);
  return result.value;
}
async function reopenFromDSM(prefix) {
  // A gateway restart can invalidate its browser session. Re-enter with the
  // retained DSM session before checking the App's persistent data.
  await step(`${prefix}:dsm-entry`, 'goto DSM entry; waitForURL(app origin)', () => until(async () => {
    await mark(`${prefix}:dsm-entry:navigate`, 'goto /webman/3rdparty/Ovis/sso/open.cgi');
    await app.goto(new URL('/webman/3rdparty/Ovis/sso/open.cgi', config.dsmUrl).href,
      { timeout: 15_000, waitUntil: 'domcontentloaded' });
    await mark(`${prefix}:dsm-entry:app-origin`, 'waitForURL(app origin)');
    await app.waitForURL(url => url.origin === new URL(config.appUrl).origin, { timeout: 15_000 });
    return true;
  }, Boolean, 'DSM entry did not recover'));
}
async function login(role) {
  const context = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width: 1440, height: 1000 } });
  context.setDefaultTimeout(180_000);
  context.setDefaultNavigationTimeout(180_000);
  const page = await context.newPage();
  try {
    await step(`login:${role}:navigate`, 'goto DSM', () => page.goto(config.dsmUrl), { page });
    await step(`login:${role}:username`, 'input[name=username]',
      () => page.locator('input[name="username"]').fill(config[role].username), { page });
    await step(`login:${role}:next`, 'keyboard Enter', () => page.keyboard.press('Enter'), { page });
    const password = page.locator('input[type="password"]:visible');
    await step(`login:${role}:password`, 'input[type=password]:visible',
      () => password.fill(config[role].password), { page });
    await step(`login:${role}:submit`, 'password Enter', () => password.press('Enter'), { page });
    await step(`login:${role}:desktop`, '#sds-desktop',
      () => page.locator('#sds-desktop').waitFor({ state: 'visible', timeout: 180_000 }), { page, target: page.locator('#sds-desktop') });
    await step(`login:${role}:user-menu`, '#sds-taskbar-tray-item-user',
      () => page.locator('#sds-taskbar-tray-item-user').waitFor(), { page });
    // Reuse Clef's read-only DSM bootstrap/desktop semantics; no model or AI key needed.
    const adapter = dsmAdapter({ startupWaitMs: 60_000 });
    const snapshot = await step(`login:${role}:clef-desktop`, 'Clef desktopReady', () => until(() => adapter.observe(page, {
      revision: 'desktop', viewport: { width: 1440, height: 1000 }, text: '',
      truncated: false, tree: { id: 'root', role: 'document', children: [] },
    }), value => value.desktopReady === true, 'Clef did not confirm DSM desktop readiness'), { page });
    assert.equal(snapshot.desktopReady, true, 'Clef did not confirm DSM desktop readiness');
    return { context, page };
  } catch (error) { await diagnostics.failure(error, page); await context.close(); throw error; }
}
async function servicesOff(prefix = 'services-off') {
  const telemetry = await step(`${prefix}:telemetry`, 'GET /api/telemetry', () => api('/api/telemetry'));
  const relay = await step(`${prefix}:relay`, 'GET /api/relay', () => api('/api/relay'));
  const portal = await step(`${prefix}:portal`, 'GET /api/portal', () => api('/api/portal'));
  await step(`${prefix}:policy-assertions`, 'telemetry/relay/portal policy', async () => {
    assert.equal(telemetry.enabled, false, 'Telemetry is enabled');
    assert.equal(relay.phase, 'off', 'Relay is not off');
    assert.equal(relay.sessions, 0, 'Relay sessions exist');
    assert.equal(relay.blocked, 'serve_only', 'Relay guard is not configured');
    assert.equal(portal.phase, 'idle', 'Portal is not idle');
  });
  const policy = await step(`${prefix}:runtime-guard`, 'owned runtime guard readback', () =>
    container("sh -c 'test \"$OVIS_TAILSCALE_SERVE_ONLY\" = 1 && test -f \"$OVIS_STATE_DIR/telemetry-off\" && echo off'"));
  await step(`${prefix}:guard-assertion`, 'runtime guard off', async () =>
    assert.equal(policy, 'off', 'Runtime guard/telemetry marker missing'));
  return { telemetry: 'off', relay: 'off', portal: 'idle', guard: 'serve_only' };
}
async function profileState(prefix) {
  const path = `/var/packages/ContainerManager/etc/${config.containerName}.config`;
  return step(`${prefix}:container-profile`, 'owned Container Manager profile metadata only', async () => {
    const containerProfile = await ssh(`if test -f ${quote(path)}; then echo present; else echo absent; fi`);
    await diagnostics.observe({ containerProfile });
    return containerProfile;
  });
}
async function shot(name) {
  // Capture only this run's terminal; never capture other people's terminal/sidebar state.
  await app.locator('.xterm-screen:visible').screenshot({ path: join(evidence, name) });
}
async function terminalReady(prefix) {
  const button = app.getByRole('button', { name: 'Terminal', exact: true });
  await step(`${prefix}:terminal-button`, 'getByRole(button, Terminal, exact)',
    () => button.click(), { target: button });
  const stack = app.locator('.terminal-stack[data-pane-content]');
  await step(`${prefix}:terminal-presentation`, '.terminal-stack[data-pane-content]',
    () => stack.waitFor({ state: 'visible' }), { target: stack });
  // The key bar is CSS-hidden on desktops; its disabled state still tracks input readiness.
  const key = app.getByRole('group', { name: 'Terminal keys', exact: true, includeHidden: true })
    .getByRole('button', { includeHidden: true }).first();
  await step(`${prefix}:terminal-input-ready`, 'Terminal keys > first button (includeHidden)',
    () => until(() => key.isEnabled(), Boolean, 'Terminal input is not ready'), { target: key });
  const input = app.locator('.pane-terminal .xterm-helper-textarea');
  await step(`${prefix}:terminal-focus`, '.pane-terminal .xterm-helper-textarea',
    () => input.focus(), { target: input });
  return input;
}
async function check(name, action) {
  if (focused && !focusedCases.has(name)) return;
  await mark(name);
  try {
    const detail = await action();
    results.push({ name, status: 'pass', detail });
    console.log(`PASS ${name}`);
  } catch (error) {
    // Assertion/Playwright errors may contain account, URL, file or DOM values.
    results.push({ name, status: 'fail', detail: await diagnostics.failure(error) });
    console.log(`FAIL ${name} (${error.name})`);
  }
  await writeFile(join(evidence, 'results.json'), JSON.stringify({ fixture, results }, null, 2), { mode: 0o600 });
}

try {
  if (focused) {
    await step('setup:installed-version', 'synopkg version Ovis', async () =>
      assert.equal(await ssh(`${packageCLI} version Ovis`), config.expectedVersion));
    await step('setup:package-running', 'synopkg status Ovis', async () =>
      assert.equal(JSON.parse(await ssh(`${packageCLI} status Ovis`)).status, 'running'));
  }
  await check('installed-version', async () => {
    const version = await ssh(`${packageCLI} version Ovis`);
    assert.equal(version, config.expectedVersion, 'Unexpected installed version; no installation attempted');
    const state = JSON.parse(await ssh(`${packageCLI} status Ovis`));
    assert.equal(state.status, 'running', 'Ovis is not running');
    return { version, status: state.status };
  });
  // Version mismatch must not lead to mutations against an unintended package generation.
  if (!focused) assert.equal(results[0].status, 'pass', 'Installed-version prerequisite failed');
  browser = await step('setup:browser-launch', null, () => chromium.launch({
    ...(process.env.OVIS_E2E_BROWSER ? { executablePath: process.env.OVIS_E2E_BROWSER } : {}),
    args: ['--no-sandbox'],
  }));
  if (focused) {
    await mark('setup:administrator-login');
    const { context, page } = await login('admin');
    adminContext = context;
    app = page;
    await step('setup:app-entry', 'goto DSM Ovis entry',
      () => app.goto(new URL('/webman/3rdparty/Ovis/sso/open.cgi', config.dsmUrl).href));
    await step('setup:app-origin', 'waitForURL(app origin)',
      () => app.waitForURL(url => url.origin === new URL(config.appUrl).origin, { timeout: 180_000 }));
    await step('setup:app-dom', 'domcontentloaded', () => app.waitForLoadState('domcontentloaded'));
    await step('setup:herdr-health', 'GET /api/health: ok + herdr',
      () => until(() => api('/api/health'), value => value.ok && value.herdr, 'Herdr startup did not complete'));
  }
  await check('admin-main-menu', async () => {
    const { context, page } = await login('admin');
    adminContext = context;
    await page.locator('#sds-taskbar .start-button').click();
    const icon = page.locator('.syno-sds-appview [syno-id="SYNO.SDS.Ovis.Instance"]');
    await icon.waitFor({ state: 'visible' });
    await icon.screenshot({ path: join(evidence, 'admin-menu.png') });
    const opened = context.waitForEvent('page');
    await icon.dblclick();
    app = await opened;
    await step('setup:app-origin', 'waitForURL(app origin)',
      () => app.waitForURL(url => url.origin === new URL(config.appUrl).origin, { timeout: 180_000 }));
    await step('setup:app-dom', 'domcontentloaded', () => app.waitForLoadState('domcontentloaded'));
    await app.getByRole('button', { name: 'New workspace', exact: true }).waitFor();
    const health = await until(() => api('/api/health'), value => value.ok && value.herdr,
      'Herdr startup did not complete');
    assert.equal(await app.getByText(/herdr is not running|socket.*unavailable|could not connect to herdr/i).count(), 0);
    return { openedFromMenu: true, herdr: true, authenticated: health.auth.authenticated, screenshot: 'admin-menu.png' };
  });
  await check('container-isolation', async () => {
    // Never inspect Config.Env: it contains the private gateway credential.
    const format = '{"user":{{json .Config.User}},"readonly":{{json .HostConfig.ReadonlyRootfs}},'
      + '"privileged":{{json .HostConfig.Privileged}},"network":{{json .HostConfig.NetworkMode}},'
      + '"mounts":{{json .Mounts}},"ports":{{json .HostConfig.PortBindings}},'
      + '"caps":{{json .HostConfig.CapDrop}},"security":{{json .HostConfig.SecurityOpt}}}';
    const info = JSON.parse(await ssh(`${docker} inspect --format ${quote(format)} ${quote(config.containerName)}`));
    const uid = await container('id -u');
    assert.match(uid, /^[1-9][0-9]*$/, 'Container process is root');
    assert.match(info.user, /^[1-9][0-9]*:[1-9][0-9]*$/);
    assert.equal(info.readonly, true);
    assert.equal(info.privileged, false);
    assert.notEqual(info.network, 'host');
    assert.ok(info.caps.includes('ALL'));
    assert.ok(info.security.some(value => value.startsWith('no-new-privileges')));
    assert.equal(Object.keys(info.ports || {}).length, 0, 'Published container port');
    assert.ok(!info.mounts.some(mount => /docker\.sock|containerd\.sock/.test(`${mount.Source} ${mount.Destination}`)));
    assert.ok(info.mounts.every(mount => !mount.RW || mount.Type === 'volume' && mount.Destination === '/data'));
    return { uid, readonly: true, privileged: false, hostNetwork: false, dockerSocket: false, publishedPorts: false };
  });
  await check('anonymous-rejection', async () => {
    const context = await browser.newContext({ ignoreHTTPSErrors: true });
    try {
      const statuses = [];
      for (const path of ['/', '/api/health', '/api/session', '/ws']) {
        const response = await context.request.get(new URL(path, config.appUrl).href, { maxRedirects: 0 });
        statuses.push(response.status());
        assert.ok([301, 302, 303, 307, 308, 401, 403].includes(response.status()), 'Anonymous access was admitted');
        if (response.status() >= 300 && response.status() < 400) {
          const target = new URL(response.headers().location, config.appUrl);
          assert.equal(target.origin, new URL(config.dsmUrl).origin);
          assert.match(target.pathname, /\/Ovis\/sso\/authorize\.cgi$/);
        }
      }
      return { routes: ['/', '/api/health', '/api/session', '/ws'], statuses };
    } finally { await context.close(); }
  });
  await check('normal-user-rejection', async () => {
    const { context, page } = await login('normal');
    try {
      await page.locator('#sds-taskbar .start-button').click();
      await page.locator('.syno-sds-appview:visible').waitFor();
      assert.equal(await page.locator('.syno-sds-appview [syno-id="SYNO.SDS.Ovis.Instance"]').count(), 0,
        'Ovis menu entry visible to a normal user');
      await page.goto(config.appUrl);
      const denial = page.locator('[data-error-code="dsm_admin_required"]');
      await denial.waitFor({ state: 'visible', timeout: 180_000 });
      await denial.screenshot({ path: join(evidence, 'normal-denied.png') });
      return { menuHidden: true, denial: 'dsm_admin_required', screenshot: 'normal-denied.png' };
    } finally { await context.close(); }
  });
  await check('services-off', servicesOff);
  await check('shell-command', async () => {
    await step('shell:app-prerequisite', null, async () => assert.ok(app, 'Administrator entry prerequisite failed'));
    await step('shell:create-directory', 'owned fixture mkdir', () => container(`mkdir ${quote(fixtureDir)}`));
    directoryCreated = true;
    await step('shell:open-dialog', 'getByRole(button, New workspace, exact)',
      () => app.getByRole('button', { name: 'New workspace', exact: true }).click());
    const dialog = app.getByRole('dialog');
    await step('shell:agent-selector', 'dialog combobox', () => dialog.getByRole('combobox').click());
    await step('shell:select-shell', 'option Shell', () => app.getByRole('option', { name: 'Shell', exact: true }).click());
    await step('shell:directory-field', 'dialog Directory', () => dialog.getByLabel('Directory', { exact: true }).fill(fixtureDir));
    await step('shell:name-field', 'dialog Name', () => dialog.getByRole('textbox', { name: /^Name/ }).fill(fixture));
    const created = app.waitForResponse(response => new URL(response.url()).pathname === '/api/workspace/create'
      && response.request().method() === 'POST');
    await step('shell:start-workspace', 'dialog Start', () => dialog.getByRole('button', { name: 'Start', exact: true }).click());
    const response = await step('shell:create-response', 'POST /api/workspace/create', () => created);
    await step('shell:workspace-created', 'created workspace/pane IDs', async () => {
      assert.equal(response.status(), 200, 'Workspace creation failed');
      ({ workspace_id: workspaceId, pane_id: paneId } = await response.json());
      assert.ok(workspaceId && paneId, 'Workspace/pane IDs missing');
    });
    const screen = app.locator('.xterm-screen:visible');
    await step('shell:screen-visible', '.xterm-screen:visible', () => screen.waitFor({ timeout: 180_000 }), { target: screen });
    const input = await terminalReady('shell');
    // Split the expected output so command echo alone cannot satisfy readback.
    const tools = ['bash', 'git', 'ssh', 'curl', 'jq', 'rg', 'fd', 'python', 'python3', 'pip3', 'sqlite3'];
    const toolCheck = `(set -e; for tool in ${tools.join(' ')}; do command -v "$tool" >/dev/null; done; `
      + `test "$(printf '{"ok":true}' | jq -r .ok)" = true; `
      + `printf 'runtime-tools\\n' | rg -q '^runtime-tools$'; fd --version >/dev/null; curl --version >/dev/null; `
      + `python -c 'import sqlite3, ssl; assert sqlite3.connect(":memory:").execute("select 1").fetchone() == (1,)'; `
      + `pip3 --version >/dev/null; test "$(sqlite3 :memory: 'select 1;')" = 1; `
      + `printf '%s%s\\n' 'tools-ok-' '${fixture}')`;
    const command = `${toolCheck}; printf '%s%s\\n' 'terminal-ok-' '${fixture}'; node -v; printf '%s' '${fixture}' > marker`;
    await step('shell:type-command', '.xterm-helper-textarea (fixture command)', () => input.pressSequentially(command));
    await step('shell:send-command', '.xterm-helper-textarea Enter', () => input.press('Enter'));
    const read = await step('shell:output-readback', 'GET /api/pane/read: fixture output', () => until(() => api(`/api/pane/read?pane_id=${encodeURIComponent(paneId)}&source=visible&format=text`),
      value => value.read?.text.includes(`terminal-ok-${fixture}`)
        && value.read.text.includes(`tools-ok-${fixture}`) && /v22\.\d+\.\d+/.test(value.read.text),
      'Shell output not observed'));
    await step('shell:marker-readback', 'owned marker readback', async () =>
      assert.equal(await container(`cat ${quote(`${fixtureDir}/marker`)}`), fixture));
    await writeFile(join(evidence, 'terminal.txt'), read.read.text, { mode: 0o600 });
    await step('shell:terminal-screenshot', '.xterm-screen:visible', () => shot('terminal.png'));
    return { echoOutput: true, runtimeTools: tools, toolsExecuted: true, nodeVersion: read.read.text.match(/v22\.\d+\.\d+/)[0], marker: true, screenshot: 'terminal.png' };
  });
  await check('restart-persistence', async () => {
    await step('restart:fixture-prerequisite', null, async () =>
      assert.ok(workspaceId && paneId && directoryCreated, 'Shell fixture prerequisite failed'));
    await step('restart:marker-before', 'owned marker readback', async () =>
      assert.equal(await container(`cat ${quote(`${fixtureDir}/marker`)}`), fixture));
    const oldStarted = await step('restart:container-before', 'docker inspect State.StartedAt', () =>
      ssh(`${docker} inspect --format '{{.State.StartedAt}}' ${quote(config.containerName)}`));
    const profileBefore = await profileState('restart:before');
    restartAttempted = true;
    await step('restart:stop', 'synopkg stop Ovis (once)', () => ssh(`${packageCLI} stop Ovis`));
    await step('restart:stopped', 'synopkg status Ovis', async () =>
      assert.notEqual(JSON.parse(await ssh(`${packageCLI} status Ovis || :`)).status, 'running'));
    startAttempted = true;
    await step('restart:start', 'synopkg start Ovis (once)', () => ssh(`${packageCLI} start Ovis`));
    await step('restart:running', 'synopkg status Ovis: running', () =>
      until(async () => JSON.parse(await ssh(`${packageCLI} status Ovis`)), value => value.status === 'running',
        'Package did not restart'));
    const profileAfter = await profileState('restart:after');
    const newStarted = await step('restart:container-after', 'docker inspect State.StartedAt', () =>
      ssh(`${docker} inspect --format '{{.State.StartedAt}}' ${quote(config.containerName)}`));
    await step('restart:container-changed', null, async () =>
      assert.notEqual(newStarted, oldStarted, 'Container did not restart'));
    await reopenFromDSM('restart');
    await step('restart:herdr-health', 'GET /api/health: ok + herdr', () =>
      until(() => api('/api/health'), value => value.ok && value.herdr, 'Herdr did not recover'));
    await step('restart:marker-after', 'owned marker readback', async () =>
      assert.equal(await container(`cat ${quote(`${fixtureDir}/marker`)}`), fixture));
    const { snapshot } = await step('restart:session', 'GET /api/session', () => api('/api/session'));
    await step('restart:workspace-retained', 'snapshot workspace ID', async () =>
      assert.ok(snapshot.workspaces.some(workspace => workspace.workspace_id === workspaceId), 'Workspace ID was not restored'));
    const row = fixtureWorkspaceControl(app, fixture);
    await step('restart:select-workspace', '.sidebar .workspace-select (exact fixture label)',
      () => row.click(), { target: row });
    await terminalReady('restart');
    await step('restart:terminal-screenshot', '.xterm-screen:visible', () => shot('after-restart.png'),
      { target: app.locator('.xterm-screen:visible') });
    const off = await servicesOff('restart:services-off');
    return { markerRetained: true, workspaceIdRetained: true, containerRestarted: true, profileBefore, profileAfter, ...off, screenshot: 'after-restart.png' };
  });
} catch (error) {
  results.push({ name: 'prerequisite', status: 'fail', detail: await diagnostics.failure(error) });
} finally {
  await check('cleanup-and-running', async () => {
    // Restore after a stop only if no start was sent; an attempted start may
    // still be pending in DSM even after the client has timed out.
    if (restartAttempted && !startAttempted) {
      const state = await step('cleanup:restore-status', 'synopkg status Ovis', async () =>
        JSON.parse(await ssh(`${packageCLI} status Ovis || :`)));
      if (['stop', 'stopped'].includes(state.status)) {
        startAttempted = true;
        await step('cleanup:restore-start', 'synopkg start Ovis (only if never sent)', () => ssh(`${packageCLI} start Ovis`));
      }
    }
    await step('cleanup:running', 'synopkg status Ovis: running', () =>
      until(async () => JSON.parse(await ssh(`${packageCLI} status Ovis`)), value => value.status === 'running',
        'Package is not running after cleanup'));
    if (workspaceId) {
      await reopenFromDSM('cleanup');
      await step('cleanup:health', 'GET /api/health: ok + herdr', () =>
        until(() => api('/api/health'), value => value.ok && value.herdr, 'Cleanup connection unavailable'));
      const { snapshot } = await step('cleanup:session', 'GET /api/session', () => api('/api/session'));
      const own = snapshot.workspaces.find(workspace => workspace.workspace_id === workspaceId);
      await step('cleanup:owned-workspace', 'workspace ID and fixture label', async () =>
        assert.ok(own && own.label === fixture, 'Refuse to close an unowned workspace'));
      await step('cleanup:close-workspace', 'POST /api/workspace/close (owned ID)', () =>
        api('/api/workspace/close', { workspace_id: workspaceId }));
      await step('cleanup:workspace-removed', 'GET /api/session: owned ID absent', () =>
        until(() => api('/api/session'), value => !value.snapshot.workspaces.some(workspace => workspace.workspace_id === workspaceId),
          'Fixture workspace was not removed'));
    }
    if (directoryCreated) {
      await step('cleanup:remove-directory', 'owned marker rm + fixture rmdir', () =>
        container(`sh -c ${quote(`rm -f ${quote(`${fixtureDir}/marker`)} && rmdir ${quote(fixtureDir)}`)}`));
      await step('cleanup:directory-absent', 'owned directory absence', async () =>
        assert.equal(await container(`sh -c ${quote(`test ! -e ${quote(fixtureDir)} && echo removed`)}`), 'removed'));
    }
    const containerProfile = await profileState('cleanup:final');
    return { workspaceRemoved: !!workspaceId, fixtureRemoved: directoryCreated, packageRunning: true, containerProfile };
  });
  await adminContext?.close();
  await browser?.close();
  await writeFile(join(evidence, 'results.json'), JSON.stringify({ fixture, results }, null, 2), { mode: 0o600 });
}
process.exitCode = results.some(result => result.status !== 'pass') ? 1 : 0;
