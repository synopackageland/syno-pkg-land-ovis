// DSM-owned presentation and read-only SSH setup information. Upstream stays intact.
import {lstat, open} from 'node:fs/promises';
import {join} from 'node:path';
import {timingSafeEqual} from 'node:crypto';
import {constants} from 'node:fs';

export function authorized(request, token) {
 const actual=Buffer.from(request.headers.get('authorization') || '');
 const expected=Buffer.from(`Bearer ${token}`);
 return Boolean(token) && actual.length===expected.length && timingSafeEqual(actual,expected);
}

export async function sshInfo(home) {
 let public_key=null;
 try {
  const dir=await lstat(join(home,'.ssh'));
  if (!dir.isDirectory() || dir.isSymbolicLink()) throw Error('invalid directory');
  const f=await open(join(home,'.ssh/id_ed25519.pub'),constants.O_RDONLY|constants.O_NOFOLLOW|constants.O_NONBLOCK);
  try {
   const st=await f.stat();
   if (st.isFile() && st.size>0 && st.size<=4096) {
    const buffer=Buffer.alloc(4097);const {bytesRead}=await f.read(buffer,0,4097,0);
    const value=bytesRead<=4096 ? buffer.subarray(0,bytesRead).toString('utf8').trim() : '';
    if (/^ssh-ed25519 [A-Za-z0-9+/]+=*(?: [^\r\n]*)?$/.test(value)) public_key=value;
   }
  } finally {await f.close();}
 } catch { /* Missing/unsafe public keys never expose another file. */ }
 return {directory:'/data/home/.ssh',public_key};
}

export function explainSetupFailure(job) {
 if (job.phase!=='failed') return job;
 const raw=`${job.error || ''}\n${job.ssh_output || ''}`;
 let message;
 if (/No user exists for uid/i.test(raw)) message='The container account cannot be resolved by SSH. Update or repair the Ovis package in Package Center, then retry';
 else if (/Bad owner or permissions|UNPROTECTED PRIVATE KEY FILE|Permissions .*too open/i.test(raw)) message='SSH refused an unsafe key or config file. In an Ovis terminal, keep /data/home/.ssh private (mode 700), private keys/config at mode 600, and use files owned by the Ovis container account';
 else if (/No such identity|identity file .*not accessible/i.test(raw)) message='The selected SSH key could not be read. Check its absolute path inside the Ovis container, normally /data/home/.ssh/id_ed25519';
 else if (/Permission denied.*(?:publickey|password)|Authentication failed/i.test(raw)) message='The PC refused SSH sign-in. Add the Ovis public key to the remote account’s ~/.ssh/authorized_keys, check its permissions, and confirm the SSH username';
 else if (/Could not resolve hostname|Name or service not known/i.test(raw)) message='The SSH address or alias could not be resolved. Check the address and /data/home/.ssh/config inside Ovis';
 else if (/Host key verification failed|REMOTE HOST IDENTIFICATION HAS CHANGED/i.test(raw)) message='The PC’s SSH host key could not be verified. Compare its fingerprint on the PC before updating /data/home/.ssh/known_hosts inside Ovis';
 else if (/Connection refused|Connection timed out|No route to host/i.test(raw)) message='The PC could not be reached over SSH. Check that it is awake, SSH is enabled, and its address, port, and firewall allow this connection';
 return message ? {...job,error:message,ssh_output:null} : job;
}
