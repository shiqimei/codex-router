// Uses the desktop's own dialog, typography, inputs and button primitives.
function SuperiorProviderDialog({onClose,provider}) {
  const editing=Boolean(provider?.id);
  const [label,setLabel]=F6s.useState(provider?.label||'');
  const [config,setConfig]=F6s.useState(provider?.configToml||'');
  const [models,setModels]=F6s.useState(provider?.modelsJson||'');
  const [environment,setEnvironment]=F6s.useState('');
  const [busy,setBusy]=F6s.useState(false);
  const [error,setError]=F6s.useState('');
  async function save(event){event.preventDefault();setBusy(true);setError('');try{
    await codexMuxRequest(editing?'/providers/'+provider.id:'/providers',{method:editing?'PUT':'POST',body:JSON.stringify({label,configToml:config,revision:editing?provider.revision:undefined,modelsJson:models,environment:editing&&!environment.trim()?undefined:JSON.parse(environment||'{}')})});onClose();
  }catch(e){setError(e.message)}finally{setBusy(false)}}
  const inputClass='w-full rounded-lg border border-default bg-surface px-3 py-2 text-sm text-default outline-none focus-visible:ring-2 focus-visible:ring-ring';
  const field=(title,child)=>d6.jsxs('label',{className:'flex flex-col gap-2 text-sm',children:[d6.jsx('span',{className:'font-medium',children:title}),child]});
  return d6.jsx(AM,{open:true,size:'default',onOpenChange:open=>{if(!open&&!busy)onClose()},dialogCloseLabel:'Close provider settings',children:d6.jsxs(RM,{as:'form',onSubmit:save,children:[
    d6.jsx(BM,{children:d6.jsx(jM,{asChild:true,children:d6.jsx('h2',{className:'heading-dialog',children:editing?'Edit provider':'Add provider'})})}),
    d6.jsxs(BM,{className:'gap-4',children:[
      d6.jsx('p',{className:'text-sm text-secondary',children:'Use any provider supported by your Codex config.toml.'}),
      field('Name',d6.jsx('input',{value:label,onChange:e=>setLabel(e.target.value),required:true,autoFocus:true,placeholder:'My provider',className:inputClass})),
      field('Provider configuration',d6.jsx('textarea',{value:config,onChange:e=>setConfig(e.target.value),required:true,rows:8,spellCheck:false,className:inputClass+' font-mono',placeholder:'model = "your-model"\nmodel_provider = "custom"\n\n[model_providers.custom]\nname = "Custom"\nbase_url = "https://your-endpoint/v1"\nwire_api = "responses"\nenv_key = "PROVIDER_API_KEY"'})),
      field('models.json (optional)',d6.jsx('textarea',{value:models,onChange:e=>setModels(e.target.value),rows:8,spellCheck:false,className:inputClass+' font-mono',placeholder:'{"models":[{"slug":"your-model","display_name":"Your model"}]}'})),
      field('Environment variables (JSON, optional)',d6.jsx('textarea',{value:environment,onChange:e=>setEnvironment(e.target.value),rows:2,spellCheck:false,className:inputClass+' font-mono',placeholder:'{"PROVIDER_API_KEY":"…"}'})),
      editing?d6.jsx('p',{className:'text-xs text-secondary',children:`Saved environment keys: ${provider.environmentKeys?.join(', ')||'none'}. Leave blank to keep them; {} clears them.`}):null,
      error?d6.jsx('p',{role:'alert',className:'text-sm text-danger',children:error}):null,
      d6.jsxs('div',{className:'flex justify-end gap-2',children:[d6.jsx(ki,{size:'medium',color:'secondary',disabled:busy,onClick:onClose,children:'Cancel'}),d6.jsx(ki,{size:'medium',color:'primary',type:'submit',disabled:busy,children:busy?'Saving…':editing?'Save changes':'Add provider'})]})
    ]})
  ]})});
}
function SuperiorUsageIcon(props){return d6.jsx('svg',{viewBox:'0 0 20 20',fill:'none',...props,children:d6.jsx('path',{d:'M4 14V9m6 5V4m6 10V7',stroke:'currentColor',strokeWidth:1.5,strokeLinecap:'round'})})}
function SuperiorSafeImage(url){try{return new URL(url).protocol==='https:'?url:null}catch{return null}}
function SuperiorUsageDialog({onClose}) {
  const [accounts,setAccounts]=F6s.useState((globalThis.__codexMuxConnectedAccounts||[]).filter(a=>a.kind!=='provider'));
  const [selected,setSelected]=F6s.useState('primary');
  const [credits,setCredits]=F6s.useState(null);
  const [error,setError]=F6s.useState(null);
  const [busy,setBusy]=F6s.useState(false);
  const attempts=F6s.useRef({});
  async function refresh(){const result=await codexMuxRequest('/accounts');setAccounts(result.accounts.filter(a=>a.connected&&a.enabled&&a.kind!=='provider'))}
  F6s.useEffect(()=>{refresh().catch(e=>setError(e.message))},[]);
  F6s.useEffect(()=>{let live=true;setCredits(null);codexMuxRateLimitResets(selected).then(v=>{if(live)setCredits(v)}).catch(e=>{if(live)setError(e.message)});return()=>{live=false}},[selected]);
  const account=accounts.find(a=>a.id===selected)||accounts[0];
  globalThis.__superiorUsageSelector=d6.jsx('div',{className:'mt-4 flex flex-wrap gap-2',children:accounts.map(a=>d6.jsx('button',{type:'button',disabled:busy,onClick:()=>{setSelected(a.id);setError(null)},className:'flex items-center gap-2 rounded-lg px-2 py-2 text-sm',style:{background:a.id===selected?'var(--color-background-elevated, #8882)':'transparent'},children:[d6.jsx(CodexMuxAccountAvatar,{label:a.label,provider:a.provider,imageUrl:a.profileImageUrl,className:'size-5'}),a.label]},a.id))});
  F6s.useEffect(()=>()=>{delete globalThis.__superiorUsageSelector},[]);
  async function reset(creditId){setBusy(true);setError(null);const key=creditId||'automatic';const id=attempts.current[key]||crypto.randomUUID();attempts.current[key]=id;try{
    const r=await codexMuxConsumeRateLimitReset(selected,{creditId,redeemRequestId:id});
    if(r.code!=='reset'&&r.code!=='already_redeemed'){setError('Reset not applied: '+r.code);return {status:'failed'}}
    delete attempts.current[key];await refresh();const next=await codexMuxRateLimitResets(selected);setCredits(next);return {status:'completed',creditId,remainingCount:next.available_count||0};
  }catch(e){setError(e.message);return {status:'retry',creditId}}finally{setBusy(false)}}
  return d6.jsx(Pms,{availableCount:credits?.available_count||0,availableResetCredits:credits?.credits?.filter(c=>c.status==='available')||[],defaultResetCreditsOpen:false,errorMessage:error,isLoadingResetCredits:credits==null,isResetting:busy,onClose,onResetCredit:reset,usageWindows:codexMuxUsageWindows(account?.rateLimits)});
}
function SuperiorConnectionsDialog({onClose}) {
 tqi();
 const headingRef=F6s.useRef(null);
 const [providerEditor,setProviderEditor]=F6s.useState(null);
 const [deleteTarget,setDeleteTarget]=F6s.useState(null);
 const [deleted,setDeleted]=F6s.useState(null);
 const [accounts,setAccounts]=F6s.useState(globalThis.__superiorAccounts||[]);
 const [error,setError]=F6s.useState('');const [busy,setBusy]=F6s.useState(false);const [login,setLogin]=F6s.useState(null);
 async function refresh(){const v=await codexMuxRequest('/accounts');setAccounts(v.accounts);globalThis.__superiorAccounts=v.accounts;if(login&&v.accounts.some(a=>a.id===login.accountId&&a.connected))setLogin(null)}
 F6s.useEffect(()=>{refresh().catch(e=>setError(e.message));const timer=setInterval(()=>refresh().catch(()=>{}),5000);return()=>clearInterval(timer)},[login?.accountId]);
 async function toggle(a){setBusy(true);try{await codexMuxRequest('/accounts/'+a.id,{method:'PATCH',body:JSON.stringify({enabled:!a.enabled})});await refresh()}catch(e){setError(e.message)}finally{setBusy(false)}}
 async function signIn(a){setBusy(true);try{const v=await codexMuxRequest('/accounts/'+a.id+'/login',{method:'POST',body:JSON.stringify({mode:'chatgptDeviceCode'})});setLogin({...v.login,accountId:a.id})}catch(e){setError(e.message)}finally{setBusy(false)}}
 async function addSubscription(){setBusy(true);setError('');try{const v=await codexMuxRequest('/accounts',{method:'POST',body:JSON.stringify({label:'Subscription '+(accounts.filter(a=>a.kind!=='provider').length+1)})});await signIn(v.account);await refresh()}catch(e){setError(e.message)}finally{setBusy(false)}}
 async function editProvider(a){setBusy(true);setError('');try{setProviderEditor(await codexMuxRequest('/providers/'+a.id))}catch(e){setError(e.message)}finally{setBusy(false)}}
 async function deleteProvider(){if(!deleteTarget)return;setBusy(true);setError('');try{await codexMuxRequest('/providers/'+deleteTarget.id,{method:'DELETE'});setDeleted(deleteTarget);setDeleteTarget(null);await refresh()}catch(e){setError(e.message)}finally{setBusy(false)}}
 async function undoDelete(){setBusy(true);setError('');try{await codexMuxRequest('/providers/'+deleted.id+'/restore',{method:'POST'});setDeleted(null);await refresh()}catch(e){setError(e.message)}finally{setBusy(false)}}
 if(providerEditor)return d6.jsx(SuperiorProviderDialog,{provider:providerEditor,onClose:()=>{setProviderEditor(null);refresh().catch(e=>setError(e.message))}});
 return d6.jsx(AM,{open:true,size:'default',onOpenChange:v=>{if(!v)onClose()},dialogCloseLabel:'Close subscriptions',contentProps:{onOpenAutoFocus:event=>{event.preventDefault();headingRef.current?.focus()}},children:d6.jsxs(RM,{children:[d6.jsx(BM,{children:d6.jsx(jM,{asChild:true,children:d6.jsx('h2',{ref:headingRef,tabIndex:-1,className:'heading-dialog outline-none',children:'Subscriptions'})})}),d6.jsxs(BM,{className:'gap-3',children:[d6.jsxs('div',{className:'flex flex-wrap gap-2 pb-1',children:[d6.jsx(ki,{size:'medium',color:'secondary',disabled:busy,onClick:addSubscription,children:'Add another subscription'}),d6.jsx(ki,{size:'medium',color:'secondary',disabled:busy,onClick:()=>setProviderEditor({}),children:'Add provider'})]}),d6.jsx('div',{className:'overflow-hidden rounded-2xl border border-default',children:accounts.map((a,index)=>d6.jsxs('div',{className:'flex items-center justify-between gap-4 px-4 py-3 border-default',style:{borderTopWidth:index===0?0:1},children:[d6.jsxs('div',{className:'flex min-w-0 flex-1 items-center gap-3',children:[d6.jsx(CodexMuxAccountAvatar,{label:a.label,provider:a.provider,imageUrl:a.profileImageUrl,className:'size-7'}),d6.jsxs('div',{className:'min-w-0',children:[d6.jsx('div',{className:'truncate text-sm',children:a.label}),d6.jsx('div',{className:'truncate text-xs text-secondary',children:a.kind==='provider'?`${a.provider} · ${a.model}`:a.connected?[a.email,a.planLabel].filter(Boolean).join(' · '):'Not signed in'})]})]}),d6.jsxs('div',{className:'flex shrink-0 items-center gap-2',children:[a.kind==='provider'?d6.jsx(ki,{size:'medium',color:'secondary',disabled:busy,onClick:()=>editProvider(a),children:'Edit'}):null,a.kind!=='provider'&&!a.connected?d6.jsx(ki,{size:'medium',color:'secondary',disabled:busy,onClick:()=>signIn(a),children:'Sign in'}):null,a.kind==='provider'?d6.jsx(ki,{size:'medium',color:'secondary',disabled:busy,onClick:()=>{setDeleteTarget(a);setError('')},children:'Delete'}):null,d6.jsx(YKi,{checked:a.enabled,disabled:busy||a.controller,ariaLabel:`Enable ${a.label}`,onChange:()=>toggle(a),className:'ms-2'})]})]},a.id))}),deleteTarget?d6.jsxs('div',{className:'rounded-lg border border-default p-3 text-sm',children:[d6.jsx('p',{children:`Delete ${deleteTarget.label}? Existing conversations and history will be kept.`}),d6.jsxs('div',{className:'mt-2 flex gap-2',children:[d6.jsx(ki,{size:'medium',color:'secondary',disabled:busy,onClick:()=>setDeleteTarget(null),children:'Cancel'}),d6.jsx(ki,{size:'medium',color:'primary',disabled:busy,onClick:deleteProvider,children:busy?'Deleting…':'Delete provider'})]})]}):null,deleted?d6.jsxs('div',{className:'flex items-center justify-between text-sm',children:[d6.jsx('span',{children:`${deleted.label} deleted.`}),d6.jsx(ki,{size:'medium',color:'secondary',disabled:busy,onClick:undoDelete,children:'Undo'})]}):null,login?d6.jsxs('div',{className:'rounded-lg border border-default p-3 text-sm',children:[d6.jsx('div',{children:'Enter code: '+login.userCode}),d6.jsx('a',{href:login.verificationUrl||'https://auth.openai.com/codex/device',target:'_blank',rel:'noreferrer',children:'Open sign-in page'})]}):null,error?d6.jsx('p',{role:'alert',className:'text-sm text-danger',children:error}):null]})]})});
}

function SuperiorSubscriptionsIcon(props) {
 return d6.jsx('svg',{stroke:'currentColor',fill:'currentColor',strokeWidth:0,viewBox:'0 0 256 256',width:'1em',height:'1em','aria-hidden':true,...props,children:__SUPERIOR_SUBSCRIPTIONS_ICON_ELEMENTS__.map((element,index)=>d6.jsx(element.tag,element.props,index))});
}
function SuperiorManageSubscriptionsItem() {
 const scope=xf($);
 return d6.jsx(QB,{LeftIcon:SuperiorSubscriptionsIcon,onSelect:()=>Wj(scope,SuperiorConnectionsDialog,{}),children:'Manage subscriptions'});
}

function SuperiorGrokIcon(props) {
 return d6.jsx('svg',{viewBox:'0 0 35 33',fill:'none',width:'1em',height:'1em','aria-hidden':true,...props,children:__SUPERIOR_GROK_ICON_PATHS__.map((path,index)=>d6.jsx('path',{d:path,fill:'currentColor'},index))});
}

function superiorProviderUsageLabel(account) {
 if(!account.usage)return 'API';
 if(account.usage.kind==='weekly_remaining'){const n=account.usage.remainingPercent;return codexRouterPercentLabel(n)}
 const n=account.usage.totalTokens;
 if(n==null)return '–';
 if(n>=1e9)return `${Math.floor(n/1e9)}B`;
 if(n>=1e6)return `${Math.floor(n/1e6)}M`;
 if(n>=1e3)return `${Math.floor(n/1e3)}K`;
 return String(n);
}
function superiorProviderUsageTitle(account) {
 if(!account.usage)return undefined;
 if(account.usage.error)return account.usage.error;
 if(account.usage.kind==='weekly_remaining'){
  const n=account.usage.remainingPercent;if(n==null)return 'Weekly usage unavailable';
  const usage=account.usage,reset=usage.resetsAt;return `${n.toFixed(2)}% weekly remaining${usage.accountCount?' · '+usage.accountCount+' accounts ('+usage.weighting+' weights)':''}${reset?' · Next reset '+new Date(reset*1000).toLocaleString():''}${usage.totalTokens!=null?' · '+usage.totalTokens.toLocaleString('en-US')+' total tokens':''}${usage.syncedAt?' · Quota synced '+new Date(usage.syncedAt*1000).toLocaleString():''}`;
 }
 const n=account.usage.totalTokens;
 return n==null?'Token usage unavailable':`${n.toLocaleString('en-US')} total tokens`;
}

// All mounted model pickers share one stream; per-hook EventSources exhaust
// Chromium's HTTP/1 connection pool and block accounts/model HTTP requests.
const codexRouterEventListeners=new Set();
let codexRouterEventSource;
function CodexRouterSubscribe(listener){
 codexRouterEventListeners.add(listener);
 if(!codexRouterEventSource){
  codexRouterEventSource=new EventSource(`${CODEX_MUX_API}/events?token=${encodeURIComponent(CODEX_MUX_TOKEN)}`);
  codexRouterEventSource.onmessage=event=>{for(const fn of codexRouterEventListeners)fn(event)};
 }
 return()=>{codexRouterEventListeners.delete(listener);if(!codexRouterEventListeners.size){codexRouterEventSource.close();codexRouterEventSource=null}};
}
function CodexRouterUseModelContext() {
 const React=X();
 const threadId=C0();
 wLa();
 const {isNewThreadDraft,updateDraftSettings}=yLa();
 const [context,setContext]=React.useState(null);
 React.useEffect(()=>{
  let live=true;
  const refresh=()=>codexMuxRequest('/model-context'+(threadId?'?threadId='+encodeURIComponent(threadId):'')).then(value=>{if(live)setContext(old=>JSON.stringify(old)===JSON.stringify(value)?old:value)}).catch(()=>{});
  refresh();
  const unsubscribe=CodexRouterSubscribe(event=>{try{const value=JSON.parse(event.data);if(['account-updated','thread-switched','thread-failed-over','thread-routed','routing-updated'].includes(value.type))refresh()}catch{}});
  return()=>{live=false;unsubscribe()};
 },[threadId]);
 React.useEffect(()=>{
  if(threadId||!isNewThreadDraft||!context?.model)return;
  const key=context.id+':'+context.catalogRevision;
  updateDraftSettings(old=>old.codexRouterModelContext===key?old:{...old,codexRouterModelContext:key,isManuallyChanged:true,modelSettings:{model:context.model,reasoningEffort:context.reasoning,profile:null},modelSettingsPersistAsDefault:false});
 },[threadId,isNewThreadDraft,updateDraftSettings,context]);
 return context;
}

function CodexRouterKimiIcon(props) {
 return d6.jsx('svg',{viewBox:'0 0 24 24',fill:'none',width:'1em',height:'1em','aria-hidden':true,...props,children:__CODEX_ROUTER_KIMI_PATHS__.map((path,index)=>d6.jsx('path',path,index))});
}

function codexRouterQuotaLabel(remaining,resetsAt,now=Date.now()) {
 if(remaining==null)return '–';
 const percent=Math.round(remaining);
 let label=`${percent}%`;
 const delta=Number(resetsAt)*1000-now;
 if(!Number.isFinite(delta)||delta<=0)return label;
 const days=delta>24*60*60*1000;
 const amount=Math.ceil(delta/(days?24*60*60*1000:60*60*1000));
 return `${label} · ↻ ${amount}${days?'d':'h'}`;
}

function codexRouterPercentLabel(remaining){return remaining==null?'–':`${Math.round(remaining)}%`}
function codexRouterAccountSubtitle(account){
 const base=account.kind==='provider'?account.provider:(account.email||account.planType||'ChatGPT subscription');
 const reset=account.kind==='provider'?account.usage?.resetsAt:codexMuxWeeklyWindow(account.rateLimits)?.resetsAt;
 const label=codexRouterQuotaLabel(0,reset);
 const suffix=label.includes(' · ')?label.slice(label.indexOf(' · ')+3):'';
 return d6.jsxs('span',{className:'flex min-w-0 items-baseline',style:{columnGap:4,font:'inherit',letterSpacing:'inherit',fontVariantNumeric:'proportional-nums'},children:[
  d6.jsx('span',{className:'truncate',children:base}),
  suffix?d6.jsx('span',{className:'shrink-0','aria-hidden':true,children:'·'}):null,
  suffix?d6.jsx('span',{className:'shrink-0 whitespace-nowrap',children:suffix}):null
 ]});
}
