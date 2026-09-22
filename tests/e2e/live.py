#!/usr/bin/env python3
"""Opt-in real subscription/provider canary, isolated from existing threads.
Reads current credentials locally. Never prints or saves them in the repository.
"""
import json,os,pathlib,shutil,socket,tempfile,time,tomllib,urllib.request
from routing import Client,api,input_text

def main():
    source=pathlib.Path.home()/'.codex';config=tomllib.loads((source/'config.toml').read_text())
    provider_id=os.environ.get('SUPERIOR_LIVE_PROVIDER','grok2api');provider=config['model_providers'][provider_id]
    base=provider['base_url'].rstrip('/');headers={}
    if provider.get('env_key'):headers['Authorization']='Bearer '+os.environ[provider['env_key']]
    models=json.load(urllib.request.urlopen(urllib.request.Request(base+'/models',headers=headers),timeout=20))
    ids=[m['id'] for m in models['data']]
    model=os.environ.get('SUPERIOR_LIVE_MODEL') or next((x for x in ids if x in ['grok-4.20','grok-4.1-thinking','grok-4-fast','grok-4.1-fast']),ids[0])
    home=pathlib.Path(tempfile.mkdtemp(prefix='superior-live-'));(home/'primary').mkdir(mode=0o700)
    (home/'primary'/'config.toml').write_text('model='+json.dumps(config.get('model','gpt-6-astra'))+'\ncli_auth_credentials_store="file"\n')
    shutil.copy2(source/'auth.json',home/'primary'/'auth.json');os.chmod(home/'primary'/'auth.json',0o600)
    with socket.socket() as s:s.bind(('127.0.0.1',0));port=s.getsockname()[1]
    second=None
    router_state=pathlib.Path.home()/'.superior/state.json'
    if router_state.exists():
        saved=json.loads(router_state.read_text())
        for a in saved['accounts']:
            auth=pathlib.Path(a['codexHome'])/'auth.json'
            if a['id']!='primary' and a.get('kind')!='provider' and auth.exists():
                second=a;break
    if second:
        second_home=home/'second';second_home.mkdir(mode=0o700);shutil.copy2(pathlib.Path(second['codexHome'])/'auth.json',second_home/'auth.json');os.chmod(second_home/'auth.json',0o600)
        router=home/'router';router.mkdir(mode=0o700)
        (router/'state.json').write_text(json.dumps({'version':1,'accounts':[{'id':'primary','label':'Primary','codexHome':str(home/'primary'),'enabled':True,'controller':True,'createdAt':1},{'id':'second','label':'Second subscription','codexHome':str(second_home),'enabled':True,'createdAt':2}],'threadOwner':{}}))
    client=Client(home,port)
    def say(tid,text):
        n=len(client.events);r=client.call('turn/start',{'threadId':tid,'input':input_text(text),'effort':'low'})
        end=client.wait('turn/completed',lambda p:p['turn']['id']==r['turn']['id'],after=n,timeout=120)
        if end['turn']['status']=='failed' and ('usageLimitExceeded' in json.dumps(end) or 'out of credits' in json.dumps(end)):
            end=client.wait('turn/completed',lambda p:p['turn']['status']=='completed',after=n,timeout=120)
        if end['turn']['status']!='completed':raise RuntimeError(json.dumps(end))
        read=client.call('thread/read',{'threadId':tid,'includeTurns':True})
        messages=[i.get('text','') for t in read['thread']['turns'] for i in t['items'] if i['type']=='agentMessage']
        return messages[-1]
    try:
        toml='model='+json.dumps(model)+'\nmodel_provider='+json.dumps(provider_id)+'\n[model_providers.'+provider_id+']\n'
        for k,v in provider.items():
            if isinstance(v,(str,int,float,bool)):toml+=k+'='+json.dumps(v)+'\n'
        env={provider['env_key']:os.environ[provider['env_key']]} if provider.get('env_key') else {}
        account=api(port,'providers',{'label':'Live provider canary','configToml':toml,'environment':env})['account']['id']
        tid=client.call('thread/start',{'_superiorAccountId':'primary','cwd':str(home),'approvalPolicy':'never','sandbox':'read-only'})['thread']['id']
        marker='ORCHID_'+str(time.time_ns())
        first=say(tid,'Remember this exact marker: '+marker+'. Reply with only the marker.')
        assert marker in first,'subscription did not echo marker'
        print('PASS live Codex subscription response',flush=True)
        if second:
            api(port,'thread-switch',{'threadId':tid,'accountId':'second'})
            answer=say(tid,'Return the exact original marker, nothing else.')
            assert marker in answer,'second subscription lost history'
            owner=api(port,'thread-account?threadId='+tid)['account']['id']
            print('PASS live subscription A -> subscription B history recall' if owner=='second' else 'PASS real depleted subscription -> automatic fallback preserves history',flush=True)
        api(port,'thread-switch',{'threadId':tid,'accountId':account})
        answer=say(tid,'What exact marker did I ask you to remember? Reply with only the marker.')
        assert marker in answer,'provider lost marker: '+answer[:300]
        print('PASS live subscription -> '+provider_id+' ('+model+') history recall',flush=True)
        api(port,'thread-switch',{'threadId':tid,'accountId':'primary'})
        answer=say(tid,'Again return the original exact marker, nothing else.')
        assert marker in answer,'subscription lost marker'
        print('PASS live provider -> subscription round trip',flush=True)
        print('Live artifacts:',home,flush=True)
    finally:
        client.close()
        for name in ['auth.json','provider-env.json']:
            for private_copy in home.rglob(name):private_copy.unlink()
if __name__=='__main__':main()
