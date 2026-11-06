import assert from 'node:assert/strict';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { createDiagnostics, diagnosticURL } from './diagnostics.mjs';

test('diagnostic destinations omit credentials, queries, fragments and non-HTTP payloads', () => {
  assert.equal(diagnosticURL('https://account:secret@example.invalid/entry?token=secret#credential'),
    'https://example.invalid/entry');
  assert.equal(diagnosticURL('data:text/plain,secret'), 'data:');
  assert.equal(diagnosticURL('invalid'), 'unavailable');
});

test('a timeout retains its exact locator and URL after cleanup changes the active stage', async () => {
  const evidence = await mkdtemp(join(tmpdir(), 'ovis-diagnostics-'));
  try {
    let captures = 0;
    const page = {
      url: () => 'https://example.invalid/?token=private-secret#private-secret',
      screenshot: async options => {
        captures++;
        assert.equal(options.timeout, 5000);
        assert.match(options.style, /body \* \{ visibility: hidden !important/);
        assert.doesNotMatch(options.style, /\.xterm-screen|\.sidebar|\.chat-view/);
      },
    };
    const target = { count: async () => 1, nth: () => ({ isVisible: async () => false }) };
    const diagnostics = createDiagnostics({ evidence, page: () => page });
    const error = Object.assign(new Error('private-secret DOM and input values'), { name: 'TimeoutError' });
    await assert.rejects(diagnostics.step('restart:select-workspace', 'fixture row.first()',
      async () => { throw error; }, { target }), value => value === error);
    await diagnostics.mark('cleanup:running');
    const detail = await diagnostics.failure(error);
    assert.equal(detail.stage, 'restart:select-workspace');
    assert.equal(detail.locator, 'fixture row.first()');
    assert.equal(detail.url, 'https://example.invalid/');
    assert.deepEqual(detail.visible, [false]);
    assert.equal(detail.screenshot, 'failure-1.png');
    assert.equal(captures, 1);
    assert.doesNotMatch(await readFile(join(evidence, 'stages.jsonl'), 'utf8'), /private-secret/);
    assert.doesNotMatch(await readFile(join(evidence, 'failure-1.json'), 'utf8'), /private-secret/);
  } finally { await rm(evidence, { recursive: true, force: true }); }
});

test('a closed page and failed evidence write do not replace the original timeout', async () => {
  const evidence = await mkdtemp(join(tmpdir(), 'ovis-diagnostics-'));
  try {
    const diagnostics = createDiagnostics({ evidence, page: () => ({
      url: () => 'https://example.invalid/', screenshot: async () => { throw new Error('closed page'); },
    }) });
    const error = Object.assign(new Error('original'), { name: 'TimeoutError' });
    await assert.rejects(diagnostics.step('restart:terminal-button', 'Terminal button', async () => {
      await rm(evidence, { recursive: true, force: true });
      throw error;
    }), value => value === error);
    assert.equal((await diagnostics.failure(error)).screenshot, 'unavailable');
  } finally { await rm(evidence, { recursive: true, force: true }); }
});
