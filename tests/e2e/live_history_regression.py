"""Opt-in real Grok follow-up using a private copy of the ORCHID test history.
Never changes the user's source rollout; removes copied credentials on exit.
"""
import hashlib,json,pathlib,shutil,socket,tempfile
from routing import Client,api,input_text

def main():
    state_root=pathlib.Path.home()/'.superior'
    stored=json.loads((state_root/'state.json').read_text())
    account=next(a for a in stored['accounts'] if a.get('provider')=='grok2api' and not a.get('deletedAt'))
    source_home=pathlib.Path(account['codexHome'])
    tid='01a0c837-c6aa-73a1-8627-8bcb8f89eeca'
    source=next((source_home/'sessions').rglob('*'+tid+'.jsonl'))
    original=source.read_bytes();digest=hashlib.sha256(original).digest()
    home=pathlib.Path(tempfile.mkdtemp(prefix='codexrouter-history-live-'));(home/'primary').mkdir(mode=0o700)
    (home/'primary/config.toml').write_text('cli_auth_credentials_store="file"\n')
    with socket.socket() as sock:sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
    client=Client(home,port)
    try:
        first=api(port,'providers',{'label':'History source','configToml':'model="gpt-5.4"\nmodel_provider="fixture"\n[model_providers.fixture]\nname="Fixture"\nbase_url="http://127.0.0.1:9/v1"\nwire_api="responses"'})['account']
        copied=home/'router/accounts'/first['id']/'codex-home/sessions'/source.relative_to(source_home/'sessions');copied.parent.mkdir(parents=True,exist_ok=True);copied.write_bytes(original)
        ordinal=max((json.loads(line).get('ordinal',0) for line in original.splitlines() if line.strip()),default=0)+1
        with copied.open('a') as f:
            f.write(json.dumps({'timestamp':'2026-09-22T12:00:00Z','ordinal':ordinal,'type':'response_item','payload':{'type':'reasoning','encrypted_content':'synthetic-foreign-ciphertext','summary':[]}})+'\n')
        client.close()
        state_file=home/'router/state.json';saved=json.loads(state_file.read_text());saved.setdefault('threadOwner',{})[tid]=first['id'];state_file.write_text(json.dumps(saved))
        client=Client(home,port)
        client.call('thread/resume',{'threadId':tid,'path':str(copied)})
        target=api(port,'providers',{'label':'Live Grok','configToml':(source_home/'provider.toml').read_text(),'modelsJson':(source_home/'models.json').read_text(),'environment':json.loads((source_home/'provider-env.json').read_text())})['account']
        api(port,'thread-switch',{'threadId':tid,'accountId':target['id']})
        n=len(client.events)
        result=client.call('turn/start',{'threadId':tid,'effort':'none','collaborationMode':{'mode':'default','settings':{'model':account['model'],'reasoning_effort':'none','developer_instructions':None}},'input':input_text('Regression: reply with exactly ORCHID_7391 if that was the marker in the first message. Do not use tools.')})
        end=client.wait('turn/completed',lambda p:p['turn']['id']==result['turn']['id'],after=n,timeout=120)
        if end['turn']['status']!='completed':raise RuntimeError('Live history follow-up failed: '+json.dumps(end['turn'].get('error')))
        read=client.call('thread/read',{'threadId':tid,'includeTurns':True})
        messages=[i.get('text','') for t in read['thread']['turns'] for i in t['items'] if i['type']=='agentMessage']
        assert messages and 'ORCHID_7391' in messages[-1],'marker missing'
        assert hashlib.sha256(source.read_bytes()).digest()==digest,'source rollout changed'
        print('PASS real Grok copied-history follow-up, foreign reasoning removed, stale none repaired, source unchanged',flush=True)
    finally:
        client.close()
        for name in ['auth.json','provider-env.json']:
            for p in home.rglob(name):p.unlink()
if __name__=='__main__':main()
