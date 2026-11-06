// Opt-in acceptance against an installed App. Never imported by build/install hooks.
import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { mkdir, readFile, stat, writeFile } from 'node:fs/promises';
import { resolve, join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { promisify } from 'node:util';
import { pathToFileURL } from 'node:url';

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
const { dsmAdapter } = await import(moduleName('OVIS_E2E_CLEF_MODULE', '@synology-vincentt/e2e-clef/dsm'));
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
let stage = '';
const quote = value => `'${value.replaceAll("'", "'\\''")}'`;

// Discard SSH diagnostics: credentials, hostnames and remote paths never enter results.
async function ssh(command) {
  try {
    return (await run('ssh', ['-o', 'BatchMode=yes', '-o', 'LogLevel=ERROR',
      '-o', 'ConnectTimeout=15', config.sshTarget, command],
    { timeout: 300_000, maxBuffer: 2 * 1024 * 1024 })).stdout.trim();
  } catch { throw new Error(`Remote operation failed at ${stage}`); }
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
async function reopenFromDSM() {
  // A gateway restart can invalidate its browser session. Re-enter with the
  // retained DSM session before checking the App's persistent data.
  await until(async () => {
    await app.goto(new URL('/webman/3rdparty/Ovis/sso/open.cgi', config.dsmUrl).href,
      { timeout: 15_000, waitUntil: 'domcontentloaded' });
    await app.waitForURL(url => url.origin === new URL(config.appUrl).origin, { timeout: 15_000 });
    return true;
  }, Boolean, 'DSM entry did not recover');
}
async function login(role) {
  const context = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width: 1440, height: 1000 } });
  context.setDefaultTimeout(180_000);
  context.setDefaultNavigationTimeout(180_000);
  try {
    const page = await context.newPage();
    await page.goto(config.dsmUrl);
    await page.locator('input[name="username"]').fill(config[role].username);
    await page.keyboard.press('Enter');
    const password = page.locator('input[type="password"]:visible');
    await password.fill(config[role].password);
    await password.press('Enter');
    await page.locator('#sds-desktop').waitFor({ state: 'visible', timeout: 180_000 });
    await page.locator('#sds-taskbar-tray-item-user').waitFor();
    // Reuse Clef's read-only DSM bootstrap/desktop semantics; no model or AI key needed.
    const adapter = dsmAdapter({ startupWaitMs: 60_000 });
    const snapshot = await until(() => adapter.observe(page, {
      revision: 'desktop', viewport: { width: 1440, height: 1000 }, text: '',
      truncated: false, tree: { id: 'root', role: 'document', children: [] },
    }), value => value.desktopReady === true, 'Clef did not confirm DSM desktop readiness');
    assert.equal(snapshot.desktopReady, true, 'Clef did not confirm DSM desktop readiness');
    return { context, page };
  } catch (error) { await context.close(); throw error; }
}
async function servicesOff() {
  const telemetry = await api('/api/telemetry');
  const relay = await api('/api/relay');
  const portal = await api('/api/portal');
  assert.equal(telemetry.enabled, false, 'Telemetry is enabled');
  assert.equal(relay.phase, 'off', 'Relay is not off');
  assert.equal(relay.sessions, 0, 'Relay sessions exist');
  assert.equal(relay.blocked, 'serve_only', 'Relay guard is not configured');
  assert.equal(portal.phase, 'idle', 'Portal is not idle');
  const policy = await container("sh -c 'test \"$OVIS_TAILSCALE_SERVE_ONLY\" = 1 && test -f \"$OVIS_STATE_DIR/telemetry-off\" && echo off'");
  assert.equal(policy, 'off', 'Runtime guard/telemetry marker missing');
  return { telemetry: 'off', relay: 'off', portal: 'idle', guard: 'serve_only' };
}
async function shot(name) {
  // Capture only this run's terminal; never capture other people's terminal/sidebar state.
  await app.locator('.xterm-screen:visible').screenshot({ path: join(evidence, name) });
}
async function terminalReady() {
  await app.getByRole('button', { name: 'Terminal', exact: true }).click();
  await app.locator('.terminal-stack[data-pane-content]').waitFor({ state: 'visible' });
  // The key bar is CSS-hidden on desktops; its disabled state still tracks input readiness.
  const key = app.getByRole('group', { name: 'Terminal keys', exact: true, includeHidden: true })
    .getByRole('button', { includeHidden: true }).first();
  await until(() => key.isEnabled(), Boolean, 'Terminal input is not ready');
  const input = app.locator('.pane-terminal .xterm-helper-textarea');
  await input.focus();
  return input;
}
async function check(name, action) {
  if (focused && !focusedCases.has(name)) return;
  stage = name;
  try {
    const detail = await action();
    results.push({ name, status: 'pass', detail });
    console.log(`PASS ${name}`);
  } catch (error) {
    // Assertion/Playwright errors may contain account, URL, file or DOM values.
    results.push({ name, status: 'fail', detail: { stage, kind: error.name } });
    console.log(`FAIL ${name} (${error.name})`);
  }
  await writeFile(join(evidence, 'results.json'), JSON.stringify({ fixture, results }, null, 2), { mode: 0o600 });
}

try {
  if (focused) {
    stage = 'setup:installed-version';
    assert.equal(await ssh(`${packageCLI} version Ovis`), config.expectedVersion);
    assert.equal(JSON.parse(await ssh(`${packageCLI} status Ovis`)).status, 'running');
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
  browser = await chromium.launch({
    ...(process.env.OVIS_E2E_BROWSER ? { executablePath: process.env.OVIS_E2E_BROWSER } : {}),
    args: ['--no-sandbox'],
  });
  if (focused) {
    stage = 'setup:administrator-login';
    const { context, page } = await login('admin');
    adminContext = context;
    app = page;
    await app.goto(new URL('/webman/3rdparty/Ovis/sso/open.cgi', config.dsmUrl).href);
    await app.waitForURL(url => url.origin === new URL(config.appUrl).origin, { timeout: 180_000 });
    await app.waitForLoadState('domcontentloaded');
    await until(() => api('/api/health'), value => value.ok && value.herdr, 'Herdr startup did not complete');
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
    await app.waitForURL(url => url.origin === new URL(config.appUrl).origin, { timeout: 180_000 });
    await app.waitForLoadState('domcontentloaded');
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
    assert.ok(app, 'Administrator entry prerequisite failed');
    await container(`mkdir ${quote(fixtureDir)}`);
    directoryCreated = true;
    stage = 'shell-command:open-dialog';
    await app.getByRole('button', { name: 'New workspace', exact: true }).click();
    const dialog = app.getByRole('dialog');
    stage = 'shell-command:select-shell';
    await dialog.getByRole('combobox').click();
    await app.getByRole('option', { name: 'Shell', exact: true }).click();
    stage = 'shell-command:directory-field';
    await dialog.getByLabel('Directory', { exact: true }).fill(fixtureDir);
    stage = 'shell-command:name-field';
    await dialog.getByRole('textbox', { name: /^Name/ }).fill(fixture);
    stage = 'shell-command:create-workspace';
    const created = app.waitForResponse(response => new URL(response.url()).pathname === '/api/workspace/create'
      && response.request().method() === 'POST');
    await dialog.getByRole('button', { name: 'Start', exact: true }).click();
    const response = await created;
    assert.equal(response.status(), 200, 'Workspace creation failed');
    ({ workspace_id: workspaceId, pane_id: paneId } = await response.json());
    assert.ok(workspaceId && paneId, 'Workspace/pane IDs missing');
    stage = 'shell-command:terminal-ready';
    const screen = app.locator('.xterm-screen:visible');
    await screen.waitFor({ timeout: 180_000 });
    const input = await terminalReady();
    // Split the expected output so command echo alone cannot satisfy readback.
    const command = `printf '%s%s\\n' 'terminal-ok-' '${fixture}'; node -v; printf '%s' '${fixture}' > marker`;
    await input.pressSequentially(command);
    await input.press('Enter');
    stage = 'shell-command:output-readback';
    const read = await until(() => api(`/api/pane/read?pane_id=${encodeURIComponent(paneId)}&source=visible&format=text`),
      value => value.read?.text.includes(`terminal-ok-${fixture}`) && /v22\.\d+\.\d+/.test(value.read.text),
      'Shell output not observed');
    assert.equal(await container(`cat ${quote(`${fixtureDir}/marker`)}`), fixture);
    await writeFile(join(evidence, 'terminal.txt'), read.read.text, { mode: 0o600 });
    await shot('terminal.png');
    return { echoOutput: true, nodeVersion: read.read.text.match(/v22\.\d+\.\d+/)[0], marker: true, screenshot: 'terminal.png' };
  });
  await check('restart-persistence', async () => {
    assert.ok(workspaceId && paneId && directoryCreated, 'Shell fixture prerequisite failed');
    assert.equal(await container(`cat ${quote(`${fixtureDir}/marker`)}`), fixture);
    const oldStarted = await ssh(`${docker} inspect --format '{{.State.StartedAt}}' ${quote(config.containerName)}`);
    restartAttempted = true;
    await ssh(`${packageCLI} stop Ovis`);
    assert.notEqual(JSON.parse(await ssh(`${packageCLI} status Ovis || :`)).status, 'running');
    await ssh(`${packageCLI} start Ovis`);
    await until(async () => JSON.parse(await ssh(`${packageCLI} status Ovis`)), value => value.status === 'running',
      'Package did not restart');
    const newStarted = await ssh(`${docker} inspect --format '{{.State.StartedAt}}' ${quote(config.containerName)}`);
    assert.notEqual(newStarted, oldStarted, 'Container did not restart');
    await reopenFromDSM();
    await until(() => api('/api/health'), value => value.ok && value.herdr, 'Herdr did not recover');
    assert.equal(await container(`cat ${quote(`${fixtureDir}/marker`)}`), fixture);
    const { snapshot } = await api('/api/session');
    assert.ok(snapshot.workspaces.some(workspace => workspace.workspace_id === workspaceId), 'Workspace ID was not restored');
    await app.getByText(fixture, { exact: true }).first().click();
    await terminalReady();
    await shot('after-restart.png');
    const off = await servicesOff();
    return { markerRetained: true, workspaceIdRetained: true, containerRestarted: true, ...off, screenshot: 'after-restart.png' };
  });
} catch (error) {
  results.push({ name: 'prerequisite', status: 'fail', detail: { stage, kind: error.name } });
} finally {
  await check('cleanup-and-running', async () => {
    // A failing case must never leave the package stopped.
    if (restartAttempted) await ssh(`${packageCLI} start Ovis`);
    await until(async () => JSON.parse(await ssh(`${packageCLI} status Ovis`)), value => value.status === 'running',
      'Package is not running after cleanup');
    if (workspaceId) {
      await reopenFromDSM();
      await until(() => api('/api/health'), value => value.ok && value.herdr, 'Cleanup connection unavailable');
      const { snapshot } = await api('/api/session');
      const own = snapshot.workspaces.find(workspace => workspace.workspace_id === workspaceId);
      assert.ok(own && own.label === fixture, 'Refuse to close an unowned workspace');
      await api('/api/workspace/close', { workspace_id: workspaceId });
      await until(() => api('/api/session'), value => !value.snapshot.workspaces.some(workspace => workspace.workspace_id === workspaceId),
        'Fixture workspace was not removed');
    }
    if (directoryCreated) {
      await container(`sh -c ${quote(`rm -f ${quote(`${fixtureDir}/marker`)} && rmdir ${quote(fixtureDir)}`)}`);
      assert.equal(await container(`sh -c ${quote(`test ! -e ${quote(fixtureDir)} && echo removed`)}`), 'removed');
    }
    return { workspaceRemoved: !!workspaceId, fixtureRemoved: directoryCreated, packageRunning: true };
  });
  await adminContext?.close();
  await browser?.close();
  await writeFile(join(evidence, 'results.json'), JSON.stringify({ fixture, results }, null, 2), { mode: 0o600 });
}
process.exitCode = results.some(result => result.status !== 'pass') ? 1 : 0;
