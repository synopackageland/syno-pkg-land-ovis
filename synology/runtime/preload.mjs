// Install a packaging-owned response adapter before the unmodified Bun entrypoint.
import {authorized,sshInfo,explainSetupFailure} from './remote-pcs.mjs';
import {readFileSync} from 'node:fs';
const script=readFileSync(new URL('./remote-pcs.js',import.meta.url),'utf8');
const originalServe=Bun.serve.bind(Bun);
Bun.serve=function(options) {
 const originalFetch=options.fetch;
 return originalServe({...options,fetch:async function(request,server) {
  const path=new URL(request.url).pathname;
  if (path==='/api/synology/ssh' || path==='/synology/remote-pcs.js') {
   if (!authorized(request,process.env.OVIS_TOKEN)) return new Response('Unauthorized',{status:401});
   if (request.method!=='GET') return new Response('Method not allowed',{status:405,headers:{allow:'GET'}});
   const headers={'cache-control':'no-store','x-content-type-options':'nosniff'};
   return path==='/api/synology/ssh' ? Response.json(await sshInfo(process.env.HOME),{headers}) : new Response(script,{headers:{...headers,'content-type':'text/javascript; charset=utf-8'}});
  }
  // The shell must be transformed before compression; browser requests normally
  // select upstream's precompressed HTML, which cannot be rewritten as text.
  let upstreamRequest=request;
  if(path==='/' || path==='/index.html') {
   const headers=new Headers(request.headers);headers.delete('accept-encoding');
   upstreamRequest=new Request(request,{headers});
  }
  const response=await originalFetch.call(this,upstreamRequest,server);
  if (!(response instanceof Response)) return response; // WebSocket upgrades return undefined.
  if ((path==='/' || path==='/index.html') && response.headers.get('content-type')?.includes('text/html')) {
   return new HTMLRewriter().on('head',{element(el){el.append('<script defer src="/synology/remote-pcs.js"></script>',{html:true});}}).transform(response);
  }
  if (/^\/api\/machines\/setup(?:\/[^/]+)?$/.test(path) && response.headers.get('content-type')?.includes('application/json')) {
   const job=await response.json();
   const headers=new Headers(response.headers);headers.delete('content-length');headers.delete('content-encoding');headers.set('cache-control','no-store');
   return Response.json(explainSetupFailure(job),{status:response.status,headers});
  }
  return response;
 }});
};
