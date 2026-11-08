// Package invariants for Ovis: settings the brief and upstream docs depend on.
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import {parse} from 'yaml';

const manifest = readFileSync(new URL('../../dsm-package.yaml', import.meta.url), 'utf8');
const compose = readFileSync(new URL('../../compose.yaml', import.meta.url), 'utf8');
const line = (re) => assert.match(manifest, re);

test('Ovis has only private bridge ingress behind the DSM gateway', () => {
  assert.match(compose, /HOST: '0\.0\.0\.0'/);
  assert.doesNotMatch(compose, /^\s*ports:/m);
  line(/^    port: 37317$/m);
  assert.match(compose, /PORT: '37317'/);
  line(/^entry: admin-only$/m);
  line(/^  access: dsm-admins$/m);
  line(/^  mode: token$/m);
  assert.doesNotMatch(manifest, /^\s*upstream_auth_paths:/m);
});

test('telemetry stays user-controlled; self-update off; no ctl or host root hooks', () => {
  // off by default comes from ovis-launch's first-start mark; the env would lock the in-app switch
  assert.doesNotMatch(manifest, /^\s*OVIS_TELEMETRY:/m);
  assert.doesNotMatch(manifest, /^\s*DO_NOT_TRACK:/m);
  line(/off by default/);
  assert.match(compose, /OVIS_AUTO_UPDATE: '0'/);
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
  assert.match(compose, /HERDR_SOCKET: \/data\/home\/config\/herdr\/herdr\.sock/);
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

// Commas in an unquoted YAML flow list become separate, invalid mount targets.
test('DSM storage uses an explicit local volume and one bounded tmpfs mount', () => {
  assert.equal(parse(compose).volumes.data.driver, 'local');
  assert.deepEqual(parse(compose).services.ovis.tmpfs,
    ['/tmp:rw,exec,nosuid,nodev,size=536870912,mode=1777']);
});


test('container root is explicit with only retained-data migration capabilities', () => {
 const m=parse(manifest);const policy=m.compose.services.ovis;
 assert.equal(policy.run_as.mode,'container-root');
 assert.ok(policy.run_as.reason);
 assert.deepEqual(policy.run_as.capabilities,['DAC_OVERRIDE', 'CHOWN']);
 assert.equal(policy.storage_init.directories,undefined);
 assert.deepEqual(policy.storage_init.runtime.environment.OVIS_TOKEN,{secret:'ovis'});
 const image=readFileSync(new URL('../../container/Dockerfile',import.meta.url),'utf8');
 assert.match(image,/usermod --home \/data\/home root/);
 assert.equal(parse(compose).services.ovis.environment.HOME,'/data/home');
});
