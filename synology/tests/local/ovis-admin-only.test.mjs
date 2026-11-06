// Administrators only: the manifest declares it, and a built SPK carries it.
// The SPK half runs on whatever `synopkgland build` left in dist/ and is
// skipped when nothing has been built yet.
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {existsSync, mkdtempSync, readdirSync, readFileSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import {fileURLToPath} from 'node:url';

const root = fileURLToPath(new URL('../..', import.meta.url));
const manifest = readFileSync(join(root, 'dsm-package.yaml'), 'utf8');

test('manifest: admin-only entry and gateway, upstream on loopback, no way around the gate', () => {
  assert.match(manifest, /^entry: admin-only$/m);
  assert.match(manifest, /^  access: dsm-admins$/m);
  assert.match(manifest, /^    HOST: "127\.0\.0\.1"$/m);
  // relay sessions and Portal would reach Ovis without the DSM gate
  assert.match(manifest, /^    OVIS_TAILSCALE_SERVE_ONLY: "1"$/m);
  for (const key of ['upstream_auth_paths', 'passthrough', 'public_paths', 'redirects', 'ctl', 'portal'])
    assert.doesNotMatch(manifest, new RegExp(`^\\s*${key}:`, 'm'), key);
  assert.match(manifest, /administrators only/);
});

test('backend permissions deny everyone but administrators by default', () => {
  const permissions = JSON.parse(readFileSync(join(root, 'backend/internal/api/permissions.json'), 'utf8'));
  assert.equal(permissions.default, 'administrators');
  for (const [method, who] of Object.entries(permissions.methods)) assert.equal(who, 'administrators', method);
});

const spks = existsSync(join(root, 'dist')) ? readdirSync(join(root, 'dist')).filter((name) => name.endsWith('.spk')) : [];

for (const spk of spks) {
  test(`${spk}: admin-only icon, gateway with no public paths, only the SDK's CGIs`, () => {
    const dir = mkdtempSync(join(tmpdir(), 'ovis-spk-'));
    try {
      execFileSync('tar', ['-xf', join(root, 'dist', spk), '-C', dir, 'package.tgz']);
      const files = execFileSync('tar', ['-tzf', join(dir, 'package.tgz')], {encoding: 'utf8'}).split('\n');
      execFileSync('tar', ['-xzf', join(dir, 'package.tgz'), '-C', dir, 'ui/config', 'sso/gateway.json', 'sso/native-nginx.conf']);

      const entries = Object.values(JSON.parse(readFileSync(join(dir, 'ui/config'), 'utf8'))['.url']);
      assert.ok(entries.length > 0);
      for (const entry of entries) assert.equal(entry.allUsers, false, 'DSM icon for administrators only');

      const gateway = JSON.parse(readFileSync(join(dir, 'sso/gateway.json'), 'utf8'));
      assert.equal(gateway.access, 'dsm-admins');
      assert.equal(gateway.entry, 'admin-only');
      assert.equal(gateway.mode, 'token');
      assert.deepEqual(gateway.passthrough, []);
      assert.deepEqual(gateway.redirects, {});
      assert.equal(new URL(gateway.upstream).hostname, '127.0.0.1');

      // DSM nginx only forwards the dedicated port to the gateway's private socket
      const nginx = readFileSync(join(dir, 'sso/native-nginx.conf'), 'utf8');
      assert.deepEqual([...nginx.matchAll(/^\s*location\s+(\S+)/gm)].map((m) => m[1]), ['/']);
      assert.match(nginx, /proxy_pass https:\/\/unix:\/tmp\/synopkgland-gateway-Ovis\.sock:;/);

      // the only DSM CGIs are the SDK's entry gate and identity-gateway CGIs
      const cgis = files.filter((f) => /\.cgi$/.test(f)).sort();
      assert.deepEqual(cgis, ['ui/cgi/api.cgi', 'ui/sso/authorize.cgi', 'ui/sso/open.cgi', 'ui/sso/settings.cgi']);
      assert.ok(!files.some((f) => /^ui\/.*\.(lib|api)$/.test(f) || f.startsWith('ui/webapi/')), 'no DSM WebAPI of its own');
    } finally {
      rmSync(dir, {recursive: true, force: true});
    }
  });
}
