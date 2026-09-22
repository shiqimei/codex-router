"""Version-pinned native menu adaptation of upstream account-menu.js."""
import pathlib,re,json
import xml.etree.ElementTree as ET
ROOT=pathlib.Path(__file__).resolve().parents[1]
def replace_once(s,old,new):
 if s.count(old)!=1:raise RuntimeError('Native UI anchor mismatch: '+old[:90])
 return s.replace(old,new,1)
def patch_native_ui(extracted,token,port):
 initial=next((extracted/'webview/assets').glob('app-initial-*.js'));bundle=initial.read_text()
 component=(ROOT/'ui/account-menu.js').read_text().replace('__CODEX_MUX_CONTROL_PORT__',str(port)).replace('__CODEX_MUX_CONTROL_TOKEN__',token)
 aliases={'kXc':'F6s','e7':'d6','_H':'QB','CH':'iV','BW':'Wj','QLs':'$ms','S2':'SuperiorUsageIcon','jLa':'SuperiorSafeImage','lt':'Kf'}
 for old,new in aliases.items():component=re.sub(r'\b'+re.escape(old)+r'\b',lambda _:new,component)
 component=component.replace('Lo(Q)','xf($)')
 component=component.replace('return (0, d6.jsx)($ms, {','return (0, d6.jsx)(SuperiorUsageDialog, {',1)
 component=component.replace('const [accounts, setAccounts] = F6s.useState([]);','const [accounts, setAccounts] = F6s.useState(globalThis.__superiorAccounts || []);',1)
 component=component.replace('const [loading, setLoading] = F6s.useState(true);','const [loading, setLoading] = F6s.useState(!globalThis.__superiorAccounts?.length);',1)
 component=component.replace('setAccounts(nextAccounts);','globalThis.__superiorAccounts=nextAccounts;setAccounts(nextAccounts);',1)
 component=component.replace('const modalScope = xf($);','''const modalScope = xf($);
  const currentThreadId = C0();
  const routerModelSettings = _Ra(currentThreadId);
  const [selectedId,setSelectedId]=F6s.useState('');''')
 component=component.replace('setAccounts(nextAccounts);','''setAccounts(nextAccounts);
      const routing=await codexMuxRequest('/routing');
      if(currentThreadId){try{const owner=await codexMuxRequest('/thread-account?threadId='+encodeURIComponent(currentThreadId));setSelectedId(owner.account.id)}catch{setSelectedId('')}}else{setSelectedId(routing.defaultAccountId||'')}
      if(loginAccountId && nextAccounts.some(a=>a.id===loginAccountId&&a.connected)){codexMuxLoginActive=false;setLogin(null)}''')
 component=component.replace('  }, []);\n\n  F6s.useEffect(() => {\n    refresh();','  }, [currentThreadId,loginAccountId]);\n\n  F6s.useEffect(() => {\n    refresh();',1)
 component=component.replace('  const rows = [];','''  async function selectAccount(event,account){
    event.preventDefault();if(busy)return;setBusy(true);setError('');
    try{if(currentThreadId){await codexMuxRequest('/thread-switch',{method:'POST',body:JSON.stringify({threadId:currentThreadId,accountId:account.id})})}else{await codexMuxRequest('/routing',{method:'POST',body:JSON.stringify({defaultAccountId:account.id})})}const context=await codexMuxRequest('/model-context'+(currentThreadId?'?threadId='+encodeURIComponent(currentThreadId):''));if(context.model){if(currentThreadId){await routerModelSettings.setModelAndReasoningEffortForNextTurn(context.model,context.reasoning)}else{await routerModelSettings.setModelAndReasoningEffort(context.model,context.reasoning,{persistAsDefault:false})}}setSelectedId(account.id)}catch(e){setError(e.message)}finally{setBusy(false)}
  }
  const rows = [];''',1)
 component=component.replace('className: "group",','className: "group",\n          isActive: selectedId===account.id,\n          disabled:busy,\n          onSelect:event=>selectAccount(event,account),',1)
 component=component.replace(': account.planType || "ChatGPT subscription",',': account.kind==="provider" ? account.provider : account.planType || "ChatGPT subscription",')
 component=component.replace('children: codexRouterQuotaLabel(remaining, weekly?.resetsAt),','children: account.kind==="provider" ? superiorProviderUsageLabel(account) : codexRouterPercentLabel(remaining),\n            title: account.kind==="provider" ? superiorProviderUsageTitle(account) : undefined,')
 component+=(ROOT/'ui/native-provider.js').read_text()
 icon=ET.parse(ROOT/'ui/icons/subscriptions.svg').getroot()
 elements=[{'tag':element.tag.split('}')[-1],'props':element.attrib} for element in icon]
 component=component.replace('__SUPERIOR_SUBSCRIPTIONS_ICON_ELEMENTS__',json.dumps(elements))
 grok_paths=[p.attrib['d'] for p in ET.parse(ROOT/'ui/icons/grok.svg').getroot().findall('{http://www.w3.org/2000/svg}path')]
 component=component.replace('__SUPERIOR_GROK_ICON_PATHS__',json.dumps(grok_paths))
 kimi_paths=[p.attrib for p in ET.parse(ROOT/'ui/icons/kimi.svg').getroot().findall('{http://www.w3.org/2000/svg}path')]
 component=component.replace('__CODEX_ROUTER_KIMI_PATHS__',json.dumps(kimi_paths))
 bundle=replace_once(bundle,'function k6s(e){',component+'\nfunction k6s(e){')
 bundle=replace_once(bundle,'children:[r,S,s,C,w,v,D,i,null,O,A,null,j]','children:[w,v,D,i,null,O,(0,u6.jsx)(SuperiorManageSubscriptionsItem,{}),A,null,j]')
 bundle=replace_once(bundle,'children:[qt,Jt,Xt]','children:[qt,Jt,(0,d6.jsx)(SuperiorManageSubscriptionsItem,{}),Xt]')
 bundle=replace_once(bundle,'usageItems:Vt','usageItems:(0,d6.jsx)(CodexMuxAccountMenu,{})')
 # Both the sidebar variant and compact profile menu use the same connection list.
 bundle=replace_once(bundle,'children:[It,Kt,Zt,null,Qt,null,null,null,$t,tn,Vt,nn]','children:[Zt,null,Qt,null,null,null,$t,tn,(0,d6.jsx)(CodexMuxAccountMenu,{}),nn]')
 bundle=bundle.replace('onOpenChange:j,','onOpenChange:CodexMuxProfileMenuOpenChange(j),') if False else bundle
 for old in ['triggerButton:Gt,onOpenChange:j,children:[M,null]','open:l,onOpenChange:j,contentWidth:`panel`']:
  bundle=replace_once(bundle,old,old.replace('onOpenChange:j','onOpenChange:CodexMuxProfileMenuOpenChange(j)'))
 # Native usage header receives the account selector, outside compiler memo caching.
 bundle=replace_once(bundle,'let Se=I.length===2?', 'xe=(0,B0.jsxs)(B0.Fragment,{children:[xe,globalThis.__superiorUsageSelector?(0,B0.jsx)(BM,{children:globalThis.__superiorUsageSelector}):null]});let Se=I.length===2?')
 # Make the existing native model picker query the selected connection and
 # use its custom-catalog filtering path instead of the Primary-only whitelist.
 start=bundle.index('function jLa(e){');end=bundle.index('function MLa(',start)
 bundle=bundle[:start]+"""function jLa(e){
 const context=CodexRouterUseModelContext(),host=e?.hostId??`local`,connection=ZC(host),{data}=bf(Eb,host,{enabled:false}),config=data==null?null:vh(data.config),custom=context?.kind===`provider`;
 return bf(PLa,{additionalAvailableModels:Array.from(e?.additionalAvailableModels??[]).sort(),authMethod:custom?`apikey`:connection?.authMethod??null,hasConfiguredModelCatalog:custom?context.hasCatalog:mqt(config),hostId:host,includeUltraReasoningEffort:e?.includeUltraReasoningEffort!==false,isCustomModelProvider:custom||hqt(config),limit:e?.limit??100,modelCatalogPath:custom?context.id+`:`+context.catalogRevision:(config?.model_catalog_json??null),modelProvider:custom?context.provider:(config?.model_provider??null),routerAccountId:context?.id},{enabled:e?.enabled!==false&&connection?.isLoading!==true});
}"""+bundle[end:]
 bundle=replace_once(bundle,'modelCatalogPath:s,modelProvider:c},{get:l,queryClient:u,scope:d}', 'modelCatalogPath:s,modelProvider:c,routerAccountId:routerAccountId},{get:l,queryClient:u,scope:d}')
 bundle=replace_once(bundle,'ep(d,r).sendRequest(`model/list`,{includeHidden:!0,cursor:null,limit:o})', 'ep(d,r).sendRequest(`model/list`,{includeHidden:!0,cursor:null,limit:o,...routerAccountId?{codexMuxAccountId:routerAccountId}:{}})')
 bundle=replace_once(bundle,'UXn(e)&&i.has(e)','UXn(e)&&(s||i.has(e))')
 # Custom provider choices stay in the draft, never in Primary config.toml.
 bundle=replace_once(bundle,'function _Ra(e,t){let n=', 'function _Ra(e,t){const routerContext=CodexRouterUseModelContext();let n=')
 bundle=replace_once(bundle,'a=p&&n?.persistAsDefault===!1','a=p&&(n?.persistAsDefault===!1||routerContext?.kind===`provider`)')
 # This callback now closes over the routing context, so invalidate its memo.
 bundle=replace_once(bundle,'n[7]!==v||n[8]!==r||n[9]!==c.modelSettings', 'true||n[7]!==v||n[8]!==r||n[9]!==c.modelSettings')
 initial.write_text(bundle)
