"""Opt-in real Grok/Kimi image-input regression in isolated app-server homes."""
import base64,json,pathlib,socket,struct,tempfile,zlib
from routing import Client,api,input_text

def fixture_png():
    def chunk(t,data):return struct.pack('>I',len(data))+t+data+struct.pack('>I',zlib.crc32(t+data)&0xffffffff)
    w,h=64,32
    rows=b''.join(b'\0'+b'\xff\0\0'*32+b'\0\0\xff'*32 for _ in range(h))
    return b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR',struct.pack('>IIBBBBB',w,h,8,2,0,0,0))+chunk(b'IDAT',zlib.compress(rows))+chunk(b'IEND',b'')

def main():
    home=pathlib.Path(tempfile.mkdtemp(prefix='codexrouter-vision-'));(home/'primary').mkdir(mode=0o700);(home/'primary/config.toml').write_text('cli_auth_credentials_store="file"\n')
    stored=json.loads((pathlib.Path.home()/'.superior/state.json').read_text())
    with socket.socket() as s:s.bind(('127.0.0.1',0));port=s.getsockname()[1]
    client=Client(home,port)
    try:
        image='data:image/png;base64,'+base64.b64encode(fixture_png()).decode()
        for name in ['grok2api','kimi']:
            saved=next(a for a in stored['accounts'] if a.get('provider')==name and not a.get('deletedAt'))
            root=pathlib.Path(saved['codexHome'])
            account=api(port,'providers',{'label':'Vision '+name,'configToml':(root/'provider.toml').read_text(),'modelsJson':(root/'models.json').read_text(),'environment':json.loads((root/'provider-env.json').read_text())})['account']
            models=client.call('model/list',{'codexMuxAccountId':account['id'],'includeHidden':True})['data']
            model=next(m for m in models if m['model']==account['model'])
            assert 'image' in model['inputModalities'],name+' runtime image declaration missing'
            tid=client.call('thread/start',{'_superiorAccountId':account['id'],'cwd':str(home),'ephemeral':True,'approvalPolicy':'never','sandbox':'read-only'})['thread']['id']
            before=len(client.events)
            turn=client.call('turn/start',{'threadId':tid,'input':input_text('Name the color on the left half and right half of this image. Answer only LEFT=<color>, RIGHT=<color>. Do not use tools.')+[{'type':'image','url':image}],'effort':'low'})['turn']
            end=client.wait('turn/completed',lambda p:p['turn']['id']==turn['id'],after=before,timeout=120)
            if end['turn']['status']!='completed':raise RuntimeError(name+' image turn failed: '+json.dumps(end['turn'].get('error')))
            texts=[e['params']['item'].get('text','') for e in client.events[before:] if e.get('method')=='item/completed' and e.get('params',{}).get('item',{}).get('type')=='agentMessage']
            answer=texts[-1].lower();assert 'red' in answer and 'blue' in answer,name+' wrong vision result'
            print('PASS '+name+' native image declaration and real image understanding: '+texts[-1],flush=True)
    finally:
        client.close()
        for name in ['auth.json','provider-env.json']:
            for p in home.rglob(name):p.unlink()
if __name__=='__main__':main()
