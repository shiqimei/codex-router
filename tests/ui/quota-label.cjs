const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict');
const source=fs.readFileSync('ui/native-provider.js','utf8');
const context=vm.createContext({});vm.runInContext(source.slice(source.indexOf('function codexRouterQuotaLabel(')),context);
const label=vm.runInContext('codexRouterQuotaLabel',context),now=1800000000000;
for(const [remaining,hours,want] of [[0,48,'0% · ↻ 2d'],[0,12,'0% · ↻ 12h'],[0,24,'0% · ↻ 24h'],[0,24.1,'0% · ↻ 2d'],[0,0.5,'0% · ↻ 1h'],[71,12,'71% · ↻ 12h'],[0,-1,'0%']])assert.equal(label(remaining,now/1000+hours*3600,now),want);
assert.equal(label(0,null,now),'0%');assert.equal(label(null,null,now),'–');
console.log('PASS quota relative reset labels');
