#!/usr/bin/env python3
"""Real Codex app-server + mux + deterministic HTTP Responses servers.
No real provider credentials are needed; model traffic is captured only in RAM.
"""
import json, os, pathlib, queue, subprocess, tempfile, threading, time, urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT=pathlib.Path(__file__).resolve().parents[2]
CODEX=os.environ.get('SUPERIOR_TEST_CODEX','/Applications/ChatGPT.app/Contents/Resources/codex')
class Provider(BaseHTTPRequestHandler):
    requests=[]
    def log_message(self,*args):pass
    def do_POST(self):
        import gzip
        raw=self.rfile.read(int(self.headers['Content-Length']))
        if self.headers.get('Content-Encoding')=='gzip':raw=gzip.decompress(raw)
        body=json.loads(raw);Provider.requests.append((self.path,body))
        text=json.dumps(body.get('input',[])).replace(chr(92),'')
        if self.path.startswith('/Q'):
            self.send_response(429);self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(json.dumps({'error':{'type':'usage_limit_reached','code':'usage_limit_reached','message':'usage limit reached'}}).encode());return
        if 'ACTIVE_EDIT_GUARD' in text and hasattr(Provider,'edit_gate'):
            Provider.edit_started.set();Provider.edit_gate.wait(30)
        if 'HOLD_FOR_STEER' in text: time.sleep(getattr(Provider,'delay',2))
        answer='MEMORY_OK' if 'ORCHID_7391' in text else 'NO_MEMORY'
        if 'STEER_8426' in text:answer+=' STEER_OK'
        item={'id':'msg_'+str(time.time_ns()),'type':'message','role':'assistant','status':'completed','content':[{'type':'output_text','text':answer,'annotations':[]}]}
        response={'id':'resp_'+str(time.time_ns()),'object':'response','status':'completed','model':body['model'],'output':[item],'usage':{'input_tokens':30,'output_tokens':4,'total_tokens':34}}
        events=[{'type':'response.created','response':{**response,'status':'in_progress','output':[]}},
          {'type':'response.output_item.added','output_index':0,'item':{**item,'status':'in_progress','content':[]}},
          {'type':'response.content_part.added','item_id':item['id'],'output_index':0,'content_index':0,'part':{'type':'output_text','text':'','annotations':[]}},
          {'type':'response.output_text.delta','item_id':item['id'],'output_index':0,'content_index':0,'delta':answer},
          {'type':'response.output_text.done','item_id':item['id'],'output_index':0,'content_index':0,'text':answer},
          {'type':'response.output_item.done','output_index':0,'item':item}, {'type':'response.completed','response':response}]
        self.send_response(200);self.send_header('Content-Type','text/event-stream');self.end_headers()
        try:
            for event in events:self.wfile.write(('data: '+json.dumps(event)+'\n\n').encode());self.wfile.flush()
        except (BrokenPipeError,ConnectionResetError):pass

class Client:
    def __init__(self,home,port):
        self.seq=0;self.pending={};self.events=[];self.cv=threading.Condition()
        env={**os.environ,'CODEX_MUX_HOME':str(home/'router'),'CODEX_HOME':str(home/'primary'),'CODEX_MUX_REAL_CODEX':CODEX,'CODEX_MUX_CONTROL_PORT':str(port),'CODEX_MUX_CONTROL_TOKEN':'ab'*32}
        self.err=open(home/'stderr.log','w')
        self.p=subprocess.Popen([str(ROOT/'build/codex-mux'),'app-server'],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=self.err,text=True,bufsize=1)
        threading.Thread(target=self.read,daemon=True).start()
        self.call('initialize',{'clientInfo':{'name':'superior_e2e','version':'1'},'capabilities':{'experimentalApi':True}})
        self.send({'method':'initialized'})
    def read(self):
        for line in self.p.stdout:
            msg=json.loads(line)
            with self.cv:
                if 'id' in msg and 'method' not in msg:self.pending[msg['id']]=msg
                else:self.events.append(msg)
                self.cv.notify_all()
    def send(self,msg):self.p.stdin.write(json.dumps(msg)+'\n');self.p.stdin.flush()
    def call(self,method,params=None,timeout=60):
        self.seq+=1;rid=self.seq;self.send({'id':rid,'method':method,'params':params or {}})
        end=time.time()+timeout
        with self.cv:
            while rid not in self.pending:
                if time.time()>end:raise RuntimeError('RPC timeout '+method)
                self.cv.wait(0.2)
            msg=self.pending.pop(rid)
        if 'error' in msg:raise RuntimeError(method+': '+json.dumps(msg['error']))
        return msg.get('result',{})
    def wait(self,method,predicate=lambda p:True,after=0,timeout=45):
        end=time.time()+timeout
        with self.cv:
            while time.time()<end:
                for msg in self.events[after:]:
                    if msg.get('method')==method and predicate(msg.get('params',{})):return msg['params']
                self.cv.wait(0.1)
        raise RuntimeError('event timeout '+method)
    def close(self):
        self.p.stdin.close()
        try:self.p.wait(timeout=10)
        except subprocess.TimeoutExpired:self.p.kill();self.p.wait()
        self.err.close()

def api(port,path,data=None,method=None):
    req=urllib.request.Request(f'http://127.0.0.1:{port}/v1/'+path,data=json.dumps(data).encode() if data is not None else None,headers={'X-Codex-Mux-Token':'ab'*32,'Content-Type':'application/json'},method=method)
    try:
        with urllib.request.urlopen(req,timeout=100) as res:return json.load(res)
    except urllib.error.HTTPError as e:raise RuntimeError(e.read().decode())
def input_text(text):return [{'type':'text','text':text,'text_elements':[]}]
def turn(c,tid,text):
    n=len(c.events);result=c.call('turn/start',{'threadId':tid,'input':input_text(text)})
    completed=c.wait('turn/completed',lambda p:p['turn']['id']==result['turn']['id'],after=n)
    assert completed['turn']['status']=='completed',completed
    return completed

def main():
    import socket
    provider=ThreadingHTTPServer(('127.0.0.1',0),Provider);threading.Thread(target=provider.serve_forever,daemon=True).start()
    with socket.socket() as sock:sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
    home=pathlib.Path(tempfile.mkdtemp(prefix='superior-e2e-'));(home/'primary').mkdir();(home/'primary'/'config.toml').write_text('cli_auth_credentials_store="file"\n')
    client=Client(home,port)
    try:
        accounts=[]
        for label in ['Provider A','Provider B']:
            config=f'model="gpt-5.4"\nmodel_provider="fixture"\n[model_providers.fixture]\nname="E2E"\nbase_url="http://127.0.0.1:{provider.server_port}/{label[-1]}"\nwire_api="responses"\n'
            accounts.append(api(port,'providers',{'label':label,'configToml':config})['account']['id'])
        tid=client.call('thread/start',{'_superiorAccountId':accounts[0],'cwd':str(home),'approvalPolicy':'never','sandbox':'read-only'})['thread']['id']
        turn(client,tid,'Remember ORCHID_7391. Reply briefly.')
        rollout=pathlib.Path(client.call('thread/read',{'threadId':tid})['thread']['path'])
        # Reproduce account-bound reasoning + encrypted compaction in a drained
        # synthetic source history. The real target engine must replay plain history.
        opaque=[{'timestamp':'2026-09-22T08:00:00Z','type':'response_item','payload':{'type':'reasoning','id':'rs_test','summary':[],'encrypted_content':'fixture-account-bound'}},
                {'timestamp':'2026-09-22T08:00:00Z','type':'compacted','payload':{'message':'','replacement_history':[{'type':'compaction','id':'cmp_test','encrypted_content':'fixture-encrypted-compaction'}]}}]
        with rollout.open('a') as f:
            for record in opaque:f.write(json.dumps(record)+'\n')
        api(port,'thread-switch',{'threadId':tid,'accountId':accounts[1]})
        settings=client.wait('thread/settings/updated',lambda p:p['threadId']==tid)
        assert settings['threadSettings']['model']=='gpt-5.4',settings
        print('Native settings:',settings['threadSettings']['model'],settings['threadSettings']['collaborationMode']['settings']['model'],flush=True)
        turn(client,tid,'What was the marker?')
        assert any(path.startswith('/B') and 'ORCHID_7391' in json.dumps(body['input']) for path,body in Provider.requests)
        assert 'fixture-account-bound' in rollout.read_text(), 'source history was modified'
        target_request=next(body for path,body in reversed(Provider.requests) if path.startswith('/B'))
        assert 'fixture-account-bound' not in json.dumps(target_request) and 'fixture-encrypted-compaction' not in json.dumps(target_request)
        print('PASS idle cross-provider follow-up preserves history and thread ID',flush=True)
        print('PASS encrypted reasoning/compaction stays out of target requests; source preserved',flush=True)
        before=len(client.events)
        active=client.call('turn/start',{'threadId':tid,'input':input_text('HOLD_FOR_STEER: keep working')})['turn']['id']
        switched=api(port,'thread-switch',{'threadId':tid,'accountId':accounts[0]})
        assert switched['continued'] and switched['turnId']!=active,switched
        client.call('turn/steer',{'threadId':tid,'expectedTurnId':active,'input':input_text('STEER_8426: include this marker')})
        client.wait('turn/completed',lambda p:p['turn']['id']==switched['turnId'],after=before)
        assert any(path.startswith('/A') and 'STEER_8426' in json.dumps(body['input']) for path,body in Provider.requests)
        print('PASS active handoff + stale-turn steer reaches target provider',flush=True)
        listed=client.call('thread/list',{})['data'];assert sum(t['id']==tid for t in listed)==1
        assert api(port,'thread-account?threadId='+tid)['account']['id']==accounts[0]
        print('PASS merged list deduplicates and preserves migrated owner',flush=True)
        client.close();client=Client(home,port)
        client.call('thread/resume',{'threadId':tid})
        turn(client,tid,'Recall the marker again after restart.')
        assert api(port,'thread-account?threadId='+tid)['account']['id']==accounts[0]
        print('PASS restart follow-up keeps owner and history',flush=True)
        ghost=client.call('thread/start',{'_superiorAccountId':accounts[0],'cwd':str(home),'ephemeral':True})['thread']['id']
        client.call('thread/unsubscribe',{'threadId':ghost})
        details=api(port,'providers/'+accounts[0])
        edit={'label':'Edited provider A','configToml':details['configToml'].replace('/A','/C').replace('model="gpt-5.4"','model="fixture-edited-model"'),'revision':details['revision']}
        Provider.edit_gate=threading.Event();Provider.edit_started=threading.Event()
        n=len(client.events);active=client.call('turn/start',{'threadId':tid,'input':input_text('ACTIVE_EDIT_GUARD')})['turn']['id']
        assert Provider.edit_started.wait(15), 'model request did not enter edit guard'
        try:api(port,'providers/'+accounts[0],edit,method='PUT');raise AssertionError('active edit was accepted')
        except RuntimeError as e:assert 'stop' in str(e).lower(),str(e)
        finally:Provider.edit_gate.set()
        client.wait('turn/completed',lambda p:p['turn']['id']==active,after=n)
        updated=api(port,'providers/'+accounts[0],edit,method='PUT')['account']
        assert updated['model']=='fixture-edited-model' and updated['id']==accounts[0]
        turn(client,tid,'Recall the marker with the edited provider.')
        assert any(path.startswith('/C') and body['model']=='fixture-edited-model' and 'ORCHID_7391' in json.dumps(body['input']) for path,body in Provider.requests)
        print('PASS edit hot-reloads endpoint/model, ignores unloaded history, rejects running turns, preserves follow-up',flush=True)
        good=api(port,'providers/'+accounts[0])
        bad={'label':'Broken edit','configToml':'model="m"\nmodel_provider="nonexistent_provider"\n','revision':good['revision']}
        try:api(port,'providers/'+accounts[0],bad,method='PUT');raise AssertionError('invalid runtime edit accepted')
        except RuntimeError as e:assert 'restor' in str(e),str(e)
        assert api(port,'providers/'+accounts[0])['revision']==good['revision']
        turn(client,tid,'Recall the marker after rejected configuration rollback.')
        print('PASS failed runtime edit rolls back and existing thread still works',flush=True)

        try:api(port,'providers/'+accounts[0],edit,method='PUT');raise AssertionError('stale edit accepted')
        except RuntimeError as e:assert 'changed' in str(e),str(e)
        api(port,'routing',{'defaultAccountId':accounts[0]})
        api(port,'providers/'+accounts[0],method='DELETE')
        assert accounts[0] not in [a['id'] for a in api(port,'accounts')['accounts']]
        assert api(port,'routing')['defaultAccountId']==''
        turn(client,tid,'Recall the marker after deleting the prior provider.')
        assert api(port,'thread-account?threadId='+tid)['account']['id']!=accounts[0]
        api(port,'providers/'+accounts[0]+'/restore',{})
        assert accounts[0] in [a['id'] for a in api(port,'accounts')['accounts']]
        print('PASS delete clears selection, preserves/migrates history, and supports Undo',flush=True)
        import copy
        template=json.loads((ROOT/'catalogs/grok2api.models.json').read_text())['models'][0]
        models=[]
        for slug in ['fixture-one','fixture-two']:
            model=copy.deepcopy(template);model.update(slug=slug,display_name=slug);models.append(model)
        catalog=json.dumps({'models':models})
        config=f'model="fixture-one"\nmodel_provider="catalog"\n[model_providers.catalog]\nname="Catalog"\nbase_url="http://127.0.0.1:{provider.server_port}/catalog"\nwire_api="responses"\n'
        cat=api(port,'providers',{'label':'Catalog provider','configToml':config,'modelsJson':catalog})['account']['id']
        listed=client.call('model/list',{'codexMuxAccountId':cat,'includeHidden':True})['data']
        assert {m['model'] for m in listed}=={'fixture-one','fixture-two'},listed
        api(port,'thread-switch',{'threadId':tid,'accountId':cat})
        n=len(client.events);r=client.call('turn/start',{'threadId':tid,'model':'fixture-two','input':input_text('Recall the marker using the second catalog model.')})
        client.wait('turn/completed',lambda p:p['turn']['id']==r['turn']['id'],after=n)
        assert any(path.startswith('/catalog') and body['model']=='fixture-two' for path,body in Provider.requests)
        client.close();client=Client(home,port);resumed=client.call('thread/resume',{'threadId':tid})
        assert resumed['model']=='fixture-two',resumed['model']
        turn(client,tid,'Recall the original marker after restarting the catalog provider.')
        print('PASS per-provider catalog listing, model switching and restart persistence',flush=True)
        config=f'model="gpt-5.4"\nmodel_provider="quota"\n[model_providers.quota]\nname="Quota fixture"\nbase_url="http://127.0.0.1:{provider.server_port}/Q"\nwire_api="responses"\nrequest_max_retries=0\nstream_max_retries=0\n'
        quota=api(port,'providers',{'label':'Depleted','configToml':config})['account']['id']
        api(port,'thread-switch',{'threadId':tid,'accountId':quota})
        n=len(client.events);client.call('turn/start',{'threadId':tid,'input':input_text('Recall the marker after quota failure.')})
        client.wait('turn/completed',lambda p:p['turn']['status']=='completed',after=n,timeout=90)
        assert api(port,'thread-account?threadId='+tid)['account']['id']!=quota
        print('PASS asynchronous model-side quota failure automatically continues elsewhere',flush=True)
        print('E2E artifacts:',home,flush=True)
    finally:client.close();provider.shutdown()
if __name__=='__main__':main()
