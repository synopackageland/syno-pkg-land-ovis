// Package invariants for Ovis: settings the brief and upstream docs depend on.
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';

const manifest = readFileSync(new URL('../../dsm-package.yaml', import.meta.url), 'utf8');
const line = (re) => assert.match(manifest, re);

test('Ovis listens on loopback only, behind the DSM gateway', () => {
  line(/^    HOST: "127\.0\.0\.1"$/m);
  line(/^  upstream: 37317$/m);
  line(/^    PORT: "37317"$/m);
  line(/^entry: admin-only$/m);
  line(/^  access: dsm-admins$/m);
  line(/^  mode: token$/m);
  assert.doesNotMatch(manifest, /^\s*upstream_auth_paths:/m);
});

test('telemetry and self-update stay off; no ctl, no root', () => {
  line(/^    OVIS_TELEMETRY: "0"$/m);
  line(/^    OVIS_AUTO_UPDATE: "0"$/m);
  assert.doesNotMatch(manifest, /^ctl:/m);
  assert.doesNotMatch(manifest, /rootOps|root-ops|run-as: *root/);
});

test('attribution: upstream maintainer, Syno Package Land distributor', () => {
  line(/^maintainer: devswha$/m);
  line(/^  distributor: Syno Package Land$/m);
  line(/^displayname: Ovis$/m);
});
