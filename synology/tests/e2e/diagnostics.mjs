import { appendFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

// Keep browser destinations useful without retaining sign-in parameters or fragments.
export function diagnosticURL(value) {
  try {
    const url = new URL(value);
    return ['https:', 'http:'].includes(url.protocol) ? `${url.origin}${url.pathname}` : url.protocol;
  } catch { return 'unavailable'; }
}

// Only fixed view controls and empty terminal structure are visible in failure captures.
// Hide all user content, including other workspaces, chats, inputs and xterm canvases.
export const failureStyle = `
  body * { visibility: hidden !important; }
  .app, .app-header, .view-switch, .view-switch *, .terminal-stack,
  .terminal-surface, .terminal-viewport-tools, .terminal-viewport-tools * {
    visibility: visible !important;
  }
`;

export function createDiagnostics({ evidence, page: currentPage = () => undefined }) {
  let current = { stage: 'setup', locator: null };
  let failureNumber = 0;
  const captured = new WeakMap();
  const records = [];

  async function record(event, detail = {}) {
    const entry = { at: new Date().toISOString(), event, ...current, ...detail };
    records.push(entry);
    await appendFile(join(evidence, 'stages.jsonl'), `${JSON.stringify(entry)}\n`, { mode: 0o600 });
    console.log(`STAGE ${entry.stage} ${event}${entry.locator ? ` [${entry.locator}]` : ''}`);
  }

  async function mark(stage, locator = null) {
    current = { stage, locator };
    await record('begin');
  }

  async function failure(error, page = currentPage(), target) {
    if (captured.has(error)) return captured.get(error);
    // No raw exception, DOM text, input value, headers or storage state crosses this boundary.
    const number = ++failureNumber;
    const detail = { ...current, kind: error.name, url: diagnosticURL(page?.url() ?? '') };
    captured.set(error, detail);
    if (target) {
      try {
        detail.matches = await target.count();
        detail.visible = await Promise.all(Array.from({ length: Math.min(detail.matches, 10) },
          (_, i) => target.nth(i).isVisible()));
      } catch { detail.locatorProbe = 'unavailable'; }
    }
    if (page) {
      const name = `failure-${number}.png`;
      try {
        await page.screenshot({ path: join(evidence, name), style: failureStyle,
          timeout: 5000, animations: 'disabled' });
        detail.screenshot = name;
      } catch { detail.screenshot = 'unavailable'; }
    }
    try {
      await record('failure', detail);
      await writeFile(join(evidence, `failure-${number}.json`), JSON.stringify(detail, null, 2), { mode: 0o600 });
    } catch { /* Evidence failure must preserve the original test failure. */ }
    return detail;
  }

  async function step(stage, locator, action, { page = currentPage(), target } = {}) {
    await mark(stage, locator);
    try {
      const result = await action();
      await record('pass', { stage, locator });
      return result;
    } catch (error) {
      await failure(error, page, target);
      throw error;
    }
  }

  return { mark, step, failure, observe: detail => record('observation', detail), records,
    get current() { return current; } };
}
