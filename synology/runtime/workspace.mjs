import {authorized} from './remote-pcs.mjs';

// Upstream defaults to HOME. Only the local, gateway-authenticated create
// request receives the package's NAS workspace default. Keep malformed and
// oversized requests on upstream's normal validation path without reading an
// unbounded clone before its authentication/body-size checks.
export async function workspaceDefault(request, workspace, token) {
 if (!workspace || !authorized(request,token) || request.method!=='POST' || new URL(request.url).pathname!=='/api/workspace/create' || !request.body) return request;
 const reader=request.clone().body.getReader();const chunks=[];let size=0;
 try {
  for(;;) {
   const {done,value}=await reader.read();if(done)break;
   size+=value.byteLength;
   if(size>65536){void reader.cancel().catch(()=>{});return request;}
   chunks.push(value);
  }
  const bytes=new Uint8Array(size);let offset=0;
  for(const chunk of chunks){bytes.set(chunk,offset);offset+=chunk.byteLength;}
  const payload=JSON.parse(new TextDecoder().decode(bytes));
  if(!payload || typeof payload!=='object' || Array.isArray(payload) || ![undefined,null,''].includes(payload.cwd))return request;
  const headers=new Headers(request.headers);headers.delete('content-length');
  return new Request(request,{headers,body:JSON.stringify({...payload,cwd:workspace})});
 } catch {return request;} finally {reader.releaseLock();}
}
