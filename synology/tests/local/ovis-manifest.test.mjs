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

test('telemetry stays user-controlled; self-update off; no ctl, no root', () => {
  // off by default comes from ovis-launch's first-start mark; the env would lock the in-app switch
  assert.doesNotMatch(manifest, /^\s*OVIS_TELEMETRY:/m);
  assert.doesNotMatch(manifest, /^\s*DO_NOT_TRACK:/m);
  line(/off by default/);
  line(/^    OVIS_AUTO_UPDATE: "0"$/m);
  assert.doesNotMatch(manifest, /^ctl:/m);
  assert.doesNotMatch(manifest, /rootOps|root-ops|run-as: *root/);
});

test('attribution: upstream maintainer, Syno Package Land distributor', () => {
  line(/^maintainer: devswha$/m);
  line(/^  distributor: Syno Package Land$/m);
  line(/^displayname: Ovis$/m);
});

test('the bundled herdr listens where Ovis reads, and its pin follows upstream', () => {
  // ovis-launch sets herdr's XDG_CONFIG_HOME to the folder above herdr/herdr.sock
  line(/^    HERDR_SOCKET: "\$\{SYNOPKG_PKGVAR\}\/herdr\/herdr\.sock"$/m);
  const build = readFileSync(new URL('../../build-source.sh', import.meta.url), 'utf8');
  const version = /^HERDR_VERSION=(\S+)$/m.exec(build)?.[1];
  assert.ok(version, 'HERDR_VERSION');
  const upstream = readFileSync(new URL('../../../scripts/build-remote-bundle.ts', import.meta.url), 'utf8');
  assert.match(upstream, new RegExp(`herdr/releases/download/v${version.replaceAll('.', '\\.')}/`));
  for (const [, name, sha] of build.matchAll(/echo "(herdr-linux-\w+) ([0-9a-f]{64})"/g))
    assert.match(upstream, new RegExp(`"${name}", "${sha}"`), name);
  assert.ok(readFileSync(new URL(`../../LICENSES/herdr-${version}-LICENSE.txt`, import.meta.url), 'utf8').includes('Apache License'));
});

test('the icon is drawn per size from icon_256.png down', () => {
  line(/^icon: assets\/icon$/m);
});
