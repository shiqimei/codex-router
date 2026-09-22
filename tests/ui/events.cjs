const vm=require('node:vm'),fs=require('node:fs'),assert=require('node:assert/strict');
const source=fs.readFileSync('ui/native-provider.js','utf8');
const helper=source.slice(source.indexOf('const codexRouterEventListeners'),source.indexOf('function CodexRouterUseModelContext'));
const streams=[];
class EventSource { constructor(){this.closed=false;streams.push(this)} close(){this.closed=true} }
const context=vm.createContext({EventSource,CODEX_MUX_API:'http://localhost',CODEX_MUX_TOKEN:'test',encodeURIComponent});
vm.runInContext(helper,context);
const subscribe=vm.runInContext('CodexRouterSubscribe',context);
let calls=0;
const stops=Array.from({length:20},()=>subscribe(()=>calls++));
assert.equal(streams.length,1,'many pickers must share one HTTP stream');
streams[0].onmessage({data:'{}'});assert.equal(calls,20);
stops.slice(0,19).forEach(stop=>stop());assert.equal(streams[0].closed,false);
stops[19]();assert.equal(streams[0].closed,true);
const stop=subscribe(()=>{});assert.equal(streams.length,2);stop();
console.log('PASS shared model/menu event stream and cleanup');
