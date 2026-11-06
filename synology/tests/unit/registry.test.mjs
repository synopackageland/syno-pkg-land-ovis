import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { test } from 'node:test';

// Pin meaningful command behavior, not just JSON schema validity.
const registry = existsSync('operations.json')
  ? JSON.parse(readFileSync('operations.json', 'utf8')) : undefined;
test('the starter status command remains a read of app.status', {
  skip: !registry?.operations?.['app.status'] && 'No starter status operation; add your own unit contract',
}, () => {
  const status = registry.operations['app.status'];
  assert.equal(status.effect, 'read');
  assert.deepEqual(status.cli, ['app', 'status']);
  assert.equal(status.flow.type, 'invoke');
  assert.equal(status.flow.method, 'app.status');
});
