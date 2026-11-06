import assert from 'node:assert/strict';
import { test } from 'node:test';
import { cli } from '../support/cli.mjs';

// Invoke only on user request, after ctl discovery/approval for this installed App.
// Replace this starter command and assertions when the App's contract changes.
test('installed status reports the expected package and administrator', {
  skip: !process.env.APP_TEST_HOST && 'Set APP_TEST_HOST to an authorized registered host',
}, () => {
  const command = JSON.parse(process.env.APP_TEST_CTL_COMMAND || '["app","status"]');
  assert.ok(Array.isArray(command) && command.length && command.every(x => typeof x === 'string'));
  const data = cli('ctl', process.env.APP_TEST_APP || 'Ovis', ...command,
    '--host', process.env.APP_TEST_HOST);
  assert.equal(data.package, 'Ovis');
  assert.equal(data.admin, true);
  assert.ok(data.process.uid > 0, 'the App runs as a nonroot package account');
});
