import {afterAll, expect, test} from 'bun:test';
import {mkdtempSync, mkdirSync, rmSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
const home=mkdtempSync(join(tmpdir(),'ovis-preload-'));
mkdirSync(join(home,'.ssh'));writeFileSync(join(home,'.ssh/id_ed25519.pub'),'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGZpeHR1cmU= fixture');
process.env.HOME=home;process.env.OVIS_TOKEN='fixture-only';
await import('../../runtime/preload.mjs');
const server=Bun.serve({hostname:'127.0.0.1',port:0,fetch(request){
 if(new URL(request.url).pathname.startsWith('/api/machines/setup')) return Response.json({phase:'failed',error:'No user exists for uid 1234',ssh_output:'No user exists for uid 1234'});
 const html='<html><head></head><body>fixture</body></html>';
 if(request.headers.get('accept-encoding')?.includes('gzip')) return new Response(Bun.gzipSync(html),{headers:{'content-type':'text/html','content-encoding':'gzip'}});
 return new Response(html,{headers:{'content-type':'text/html'}});
}});
const url=`http://127.0.0.1:${server.port}`;
afterAll(()=>{server.stop(true);rmSync(home,{recursive:true,force:true});});
test('the app shell loads the packaging adapter',async()=>{
 expect(await (await fetch(url,{headers:{'accept-encoding':'gzip'}})).text()).toContain('/synology/remote-pcs.js');
});
test('public key reads require gateway authentication and never expose a private key',async()=>{
 expect((await fetch(url+'/api/synology/ssh')).status).toBe(401);
 const response=await fetch(url+'/api/synology/ssh',{headers:{authorization:'Bearer fixture-only'}});
 expect(response.headers.get('cache-control')).toBe('no-store');
 expect((await response.json()).public_key).toStartWith('ssh-ed25519 ');
 expect((await fetch(url+'/api/synology/ssh',{method:'POST',headers:{authorization:'Bearer fixture-only'}})).status).toBe(405);
});
test('setup errors provide recovery and suppress matching raw SSH output',async()=>{
 const response=await fetch(url+'/api/machines/setup/fixture');const job=await response.json();
 expect(job.error).toContain('Update or repair');expect(job.error).not.toContain('1234');expect(job.ssh_output).toBeNull();
});
