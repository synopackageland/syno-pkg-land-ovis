import assert from 'node:assert/strict';
import test from 'node:test';
import {workspaceDefault} from '../../runtime/workspace.mjs';
const create=(value,{path='/api/workspace/create',authorized=true}={})=>new Request('http://fixture'+path,{method:'POST',headers:{...(authorized?{authorization:'Bearer fixture'}:{}),'content-type':'application/json'},body:JSON.stringify(value)});

test('the authenticated local workspace uses the selected NAS workspace only when no directory is specified',async()=>{
 for(const cwd of [undefined,null,'']) {
  const request=await workspaceDefault(create({cwd,label:'new'}),'/data/workspace','fixture');
  assert.deepEqual(await request.json(),{cwd:'/data/workspace',label:'new'});
 }
 const request=create({cwd:'/data/home/project'});
 assert.equal(await workspaceDefault(request,'/data/workspace','fixture'),request);
});
test('remote and unauthenticated requests remain on upstream admission and validation',async()=>{
 for(const options of [{authorized:false},{path:'/api/machines/remote/workspace/create'}]) {
  const request=create({cwd:null},options);
  assert.equal(await workspaceDefault(request,'/data/workspace','fixture'),request);
  assert.equal(request.bodyUsed,false);
 }
});
test('malformed and oversized requests keep upstream validation without a rewritten body',async()=>{
 for(const request of [new Request('http://fixture/api/workspace/create',{method:'POST',headers:{authorization:'Bearer fixture'},body:'{' }),create({cwd:null,padding:'x'.repeat(70000)})]) {
  assert.equal(await workspaceDefault(request,'/data/workspace','fixture'),request);
  assert.equal(request.bodyUsed,false);
 }
});
