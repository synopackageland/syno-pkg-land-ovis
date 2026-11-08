import assert from 'node:assert/strict';
import {mkdtempSync, mkdirSync, writeFileSync, rmSync, symlinkSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import {sshInfo, explainSetupFailure, authorized} from '../../runtime/remote-pcs.mjs';

test('SSH information contains only a bounded public key from the declared home', async () => {
 const home=mkdtempSync(join(tmpdir(),'ovis-ssh-'));
 try {
  mkdirSync(join(home,'.ssh'));writeFileSync(join(home,'.ssh/id_ed25519'),'PRIVATE KEY');
  writeFileSync(join(home,'.ssh/id_ed25519.pub'),'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGZpeHR1cmU= Ovis\n');
  const info=await sshInfo(home);
  assert.match(info.public_key,/^ssh-ed25519 /);assert.doesNotMatch(JSON.stringify(info),/PRIVATE KEY/);
  rmSync(join(home,'.ssh/id_ed25519.pub'));symlinkSync(join(home,'.ssh/id_ed25519'),join(home,'.ssh/id_ed25519.pub'));
  assert.equal((await sshInfo(home)).public_key,null);
 } finally {rmSync(home,{recursive:true,force:true});}
});

test('known SSH failures explain recovery without disclosing raw output', () => {
 for (const raw of ['No user exists for uid 123456','Bad owner or permissions on /data/home/.ssh/config','Permission denied (publickey).','Could not resolve hostname workstation: Name or service not known']) {
  const job=explainSetupFailure({phase:'failed',error:raw,ssh_output:raw});
  assert.notEqual(job.error,raw);assert.equal(job.ssh_output,null);
 }
 const challenge={phase:'challenge',error:null,ssh_output:'fingerprint',challenge:{kind:'host_key'}};
 assert.deepEqual(explainSetupFailure(challenge),challenge);
});

test('the package route requires the injected bearer credential', () => {
 assert.equal(authorized(new Request('http://localhost'),''),false);
 assert.equal(authorized(new Request('http://localhost',{headers:{authorization:'Bearer wrong'}}),'fixture'),false);
 assert.equal(authorized(new Request('http://localhost',{headers:{authorization:'Bearer fixture'}}),'fixture'),true);
});
