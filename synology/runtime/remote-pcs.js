// This adapter adds DSM package SSH guidance to the upstream Add PC dialog.
// It changes presentation only; upstream owns connection and host-key decisions.
(() => {
 const copies={
 en:{hint:'Ovis uses its container’s SSH config, known_hosts, and keys in /data/home/.ssh. This is separate from your DSM account and the NAS SSH service. Agent tools and logins use the target PC’s environment',summary:'SSH keys and configuration',guide:'Copy the public key into ~/.ssh/authorized_keys for the account on your PC. Set its .ssh directory to mode 700 and authorized_keys to mode 600. On that PC, enable SSH and verify its host-key fingerprint before trusting it in Ovis',where:'Use an Ovis terminal to edit /data/home/.ssh/config for aliases and /data/home/.ssh/known_hosts for trusted hosts. Use absolute key paths. An SSH agent from DSM or your browser is not forwarded into this container',key:'Ovis SSH public key',copy:'Copy public key',copied:'Public key copied',select:'Select and copy the public key above',missing:'No public key is available. In an Ovis terminal, create one with ssh-keygen -t ed25519 -f /data/home/.ssh/id_ed25519, or restore id_ed25519.pub for your existing key. Keep the private key inside Ovis'},
 zh:{hint:'Ovis 使用容器内 /data/home/.ssh 中的 SSH config、known_hosts 和密钥，与 DSM 账户及 NAS 的 SSH 服务独立。Agent 工具和登录使用目标 PC 的环境',summary:'SSH 密钥与配置',guide:'将公钥复制到 PC 上该账户的 ~/.ssh/authorized_keys。将 .ssh 目录权限设为 700，authorized_keys 设为 600。在 PC 上启用 SSH，并在 Ovis 中信任主机密钥之前核对指纹',where:'在 Ovis 终端中编辑 /data/home/.ssh/config 设置别名，编辑 /data/home/.ssh/known_hosts 管理可信主机。密钥请使用绝对路径。DSM 或浏览器的 SSH agent 不会转发到此容器',key:'Ovis SSH 公钥',copy:'复制公钥',copied:'已复制公钥',select:'请选择并复制上方公钥',missing:'没有可用公钥。在 Ovis 终端运行 ssh-keygen -t ed25519 -f /data/home/.ssh/id_ed25519 创建密钥，或恢复已有密钥的 id_ed25519.pub。请将私钥保留在 Ovis 内'},
 ja:{hint:'Ovis はコンテナ内の /data/home/.ssh にある SSH config、known_hosts、鍵を使います。DSM アカウントや NAS の SSH サービスとは別です。エージェントのツールとログインは接続先 PC の環境を使います',summary:'SSH 鍵と設定',guide:'公開鍵を PC の接続先アカウントの ~/.ssh/authorized_keys に追加してください。.ssh の権限を 700、authorized_keys を 600 にします。PC で SSH を有効にし、Ovis で信頼する前にホスト鍵の指紋を確認してください',where:'Ovis のターミナルで /data/home/.ssh/config に別名を設定し、/data/home/.ssh/known_hosts で信頼するホストを管理します。鍵は絶対パスで指定します。DSM やブラウザーの SSH agent はコンテナに転送されません',key:'Ovis SSH 公開鍵',copy:'公開鍵をコピー',copied:'公開鍵をコピーしました',select:'上の公開鍵を選択してコピーしてください',missing:'公開鍵がありません。Ovis のターミナルで ssh-keygen -t ed25519 -f /data/home/.ssh/id_ed25519 を実行するか、既存の鍵の id_ed25519.pub を復元してください。秘密鍵は Ovis 内に保管してください'},
 ko:{hint:'Ovis는 컨테이너의 /data/home/.ssh에 있는 SSH config, known_hosts 및 키를 사용합니다. DSM 계정 및 NAS SSH 서비스와 별개입니다. 에이전트 도구와 로그인은 대상 PC 환경을 사용합니다',summary:'SSH 키 및 설정',guide:'공개 키를 PC 대상 계정의 ~/.ssh/authorized_keys에 추가하세요. .ssh 디렉터리 권한은 700, authorized_keys는 600으로 설정하세요. PC에서 SSH를 활성화하고 Ovis에서 신뢰하기 전에 호스트 키 지문을 확인하세요',where:'Ovis 터미널에서 /data/home/.ssh/config에 별칭을 설정하고 /data/home/.ssh/known_hosts에서 신뢰할 호스트를 관리하세요. 키에는 절대 경로를 사용하세요. DSM 또는 브라우저의 SSH agent는 컨테이너로 전달되지 않습니다',key:'Ovis SSH 공개 키',copy:'공개 키 복사',copied:'공개 키를 복사했습니다',select:'위 공개 키를 선택하여 복사하세요',missing:'사용 가능한 공개 키가 없습니다. Ovis 터미널에서 ssh-keygen -t ed25519 -f /data/home/.ssh/id_ed25519를 실행하거나 기존 키의 id_ed25519.pub를 복원하세요. 개인 키는 Ovis 내부에 보관하세요'}
 };
 const style=document.createElement('style');
 style.textContent='.machine-dialog .dsm-ssh-key{display:block;box-sizing:border-box;width:100%;height:auto;min-height:5em;resize:vertical;margin-block:0.5em}';
 document.head.append(style);
 let info;
 function update() {
  const language=document.documentElement.lang.split('-')[0];const t=copies[language] || copies.en;
  for (const form of document.querySelectorAll('#machine-connect-form')) {
   if (!form.querySelector('input[required]')) continue;
   const hint=form.querySelector(':scope > p.field-hint');
   if (hint && hint.textContent!==t.hint) {hint.textContent=t.hint;}
   const previous=form.querySelector('[data-dsm-ssh-setup]');
   if(previous?.dataset.language===language)continue;previous?.remove();
   const details=document.createElement('details');details.dataset.dsmSshSetup='true';details.dataset.language=language;
   const summary=document.createElement('summary');summary.textContent=t.summary;details.append(summary);
   const guide=document.createElement('p');guide.className='field-hint';guide.textContent=t.guide;details.append(guide);
   const where=document.createElement('p');where.className='field-hint';where.textContent=t.where;details.append(where);
   const key=document.createElement('textarea');key.readOnly=true;key.rows=3;key.className='input dsm-ssh-key';key.setAttribute('aria-label',t.key);details.append(key);
   const copy=document.createElement('button');copy.type='button';copy.className='btn';copy.textContent=t.copy;copy.disabled=true;details.append(copy);
   const status=document.createElement('p');status.className='field-hint';status.setAttribute('role','status');details.append(status);
   copy.addEventListener('click',async()=>{try {await navigator.clipboard.writeText(key.value);status.textContent=t.copied;} catch {key.focus();key.select();status.textContent=t.select;}});
   form.append(details);
   info ||= fetch('/api/synology/ssh',{cache:'no-store'}).then(r=>{if(!r.ok)throw Error('unavailable');return r.json();}).catch(()=>null);
   info.then(value=>{
    if(value?.public_key){key.value=value.public_key;copy.disabled=false;}
    else {key.hidden=true;status.textContent=t.missing;}
   });
  }
 }
 new MutationObserver(update).observe(document.documentElement,{childList:true,subtree:true,attributes:true,attributeFilter:['lang']});update();
})();
