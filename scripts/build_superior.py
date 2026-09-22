#!/usr/bin/env python3
"""Build an independent CodexRouter desktop from the locally installed official app.
Only the specified, hashed source is supported; originals are never modified.
"""
import argparse,hashlib,json,os,pathlib,plistlib,re,secrets,shutil,subprocess,tempfile
ROOT=pathlib.Path(__file__).resolve().parents[1]
from native_ui import patch_native_ui
def run(*args):subprocess.run(args,check=True,stdout=subprocess.DEVNULL)
def build():
 p=argparse.ArgumentParser();p.add_argument('--source',type=pathlib.Path,default=pathlib.Path('/Applications/ChatGPT.app'));p.add_argument('--output',type=pathlib.Path,default=pathlib.Path.home()/'Applications/CodexRouter.app');p.add_argument('--identity',default='-');p.add_argument('--state',type=pathlib.Path,default=pathlib.Path.home()/'.superior');args=p.parse_args()
 source=args.source.resolve();dest=args.output.expanduser().resolve();state=args.state.expanduser().resolve()
 if source==dest:raise SystemExit('Output must be independent of official app')
 info=plistlib.loads((source/'Contents/Info.plist').read_bytes());version=(info['CFBundleShortVersionString'],info['CFBundleVersion'])
 if version!=('26.915.31945','9922') or hashlib.sha256((source/'Contents/Resources/app.asar').read_bytes()).hexdigest()!='1f7939c1c781887c167043c4d1d307af3400d324685cfc315dfe2f80e634f483':raise SystemExit(f'Unsupported upstream build: {version}; inspect and adapt anchors first')
 state.mkdir(parents=True,exist_ok=True,mode=0o700);os.chmod(state,0o700)
 token_path=state/'control-token'
 if not token_path.exists():token_path.write_text(secrets.token_hex(32));os.chmod(token_path,0o600)
 token=token_path.read_text().strip()
 if len(bytes.fromhex(token))!=32:raise SystemExit('Invalid control token')
 run('go','build','-trimpath','-o',str(ROOT/'build/codex-mux'),'./cmd/codex-mux')
 with tempfile.TemporaryDirectory(prefix='superior-build-') as tmp:
  tmp=pathlib.Path(tmp);app=tmp/'CodexRouter.app';run('ditto',str(source),str(app));resources=app/'Contents/Resources';extracted=tmp/'asar'
  run(str(ROOT/'node_modules/.bin/asar'),'extract',str(resources/'app.asar'),str(extracted))
  early=extracted/'.vite/build/early-bootstrap.js'
  prefix='process.env.CODEX_ELECTRON_USER_DATA_PATH='+json.dumps(str(pathlib.Path.home()/'Library/Application Support/Superior'))+';process.env.CODEX_MUX_HOME='+json.dumps(str(state))+';process.env.CODEX_MUX_CONTROL_PORT="48124";process.env.CODEX_CLI_PATH=require("node:path").join(process.resourcesPath,"codex");'
  early.write_text(prefix+early.read_text())
  bootstrap=next((extracted/'.vite/build').glob('bootstrap-*.js'));text=bootstrap.read_text();anchor='initializeUpdater(){return this.options.enableUpdater?'
  if text.count(anchor)!=1:raise SystemExit('Updater anchor mismatch')
  bootstrap.write_text(text.replace(anchor,'initializeUpdater(){return false?',1))
  index=extracted/'webview/index.html';text=index.read_text();anchor='connect-src &#39;self&#39;'
  if text.count(anchor)!=1:raise SystemExit('Renderer CSP anchor mismatch')
  text=text.replace(anchor,anchor+' http://127.0.0.1:48124');index.write_text(text.replace('<title>ChatGPT</title>','<title>CodexRouter</title>'))
  patch_native_ui(extracted,token,48124)
  run('node',str(ROOT/'tests/ui/catalog.cjs'),str(next((extracted/'webview/assets').glob('app-initial-*.js'))))
  # Keep native modules outside ASAR, as in the upstream application.
  packed=tmp/'app.asar';run(str(ROOT/'node_modules/.bin/asar'),'pack',str(extracted),str(packed),'--unpack','*.node')
  shutil.copy2(packed,resources/'app.asar')
  unpacked=tmp/'app.asar.unpacked'
  if unpacked.exists():shutil.copytree(unpacked,resources/'app.asar.unpacked',dirs_exist_ok=True)
  (resources/'codex').rename(resources/'codex.real');shutil.copy2(ROOT/'build/codex-mux',resources/'codex')
  launcher=tmp/'launcher.c';launcher.write_text((ROOT/'native/launcher.c').read_text().replace('Codex Subscription Router','CodexRouter').replace('Application Support/CodexRouter','Application Support/Superior'))
  run('cc','-O2','-o',str(app/'Contents/MacOS/CodexRouterLauncher'),str(launcher))
  info['CFBundleExecutable']='CodexRouterLauncher'
  info['CFBundleIdentifier']='app.superior.router';info['CFBundleName']='CodexRouter';info['CFBundleDisplayName']='CodexRouter';info.pop('SUFeedURL',None);info['CFBundleURLTypes']=[{'CFBundleURLName':'CodexRouter','CFBundleURLSchemes':['codexrouter','superior']}]
  info['ElectronAsarIntegrity']={'Resources/app.asar':{'algorithm':'SHA256','hash':hashlib.sha256(packed.read_bytes()).hexdigest()}}
  (app/'Contents/Info.plist').write_bytes(plistlib.dumps(info))
  run('codesign','--force','--deep','--sign',args.identity,str(app));run('codesign','--verify','--deep','--strict',str(app))
  dest.parent.mkdir(parents=True,exist_ok=True)
  if dest.exists():
   backup=dest.with_name(dest.name+'.backup-'+str(__import__('time').time_ns()));dest.rename(backup)
  run('ditto',str(app),str(dest))
  legacy=dest.parent/'Superior.app'
  if dest.name=='CodexRouter.app' and legacy!=dest:
   if legacy.exists() and not legacy.is_symlink():
    backupDir=dest.parent/'.CodexRouter-backups';backupDir.mkdir(exist_ok=True);legacy.rename(backupDir/('Superior-'+str(__import__('time').time_ns())+'.app'))
   if not legacy.exists():legacy.symlink_to(dest.name)
  manifest={'product':'CodexRouter','sourceVersion':version,'sourceAsarSHA256':hashlib.sha256((source/'Contents/Resources/app.asar').read_bytes()).hexdigest(),'signing':'ad-hoc' if args.identity=='-' else 'certificate','output':str(dest)}
  (ROOT/'build/desktop-manifest.json').write_text(json.dumps(manifest,indent=2));print(json.dumps(manifest,indent=2))
if __name__=='__main__':build()
