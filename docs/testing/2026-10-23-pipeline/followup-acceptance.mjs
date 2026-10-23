import fs from 'node:fs';
const base=new URL('.',import.meta.url).pathname, projectID='OZK6T6UTSA72SJGPJTK7ONAE7N', rows=[];
async function rpc(op,raw,extra={}) {return fetch('http://127.0.0.1:9278',{method:'POST',body:JSON.stringify({op,raw,projectID,...extra})}).then(r=>r.json())}
async function call(...a){const r=await rpc(...a);if(r.error)throw Error(r.error);return r.data}
const record=(id,pass,detail)=>{rows.push({id,pass,...detail});fs.writeFileSync(base+'followup-results.json',JSON.stringify(rows,null,2));console.log(id,pass,JSON.stringify(detail))};
const node=(id,type,branches=[],params={})=>({id,type,kind:type.startsWith('end.')?'end':type.startsWith('agent.')?'agent':'static',params,branches:branches.map(([name,target])=>({name,target})),position:{x:0,y:0}});
const save=async(name,actions)=>call('task',{name:'验收1023 '+name,entry:actions[0].id,actions:actions.map((a,i)=>({...a,position:{x:i*300,y:0}}))});
const wait=async(id)=>{for(let i=0;i<400;i++){const r=(await call('state')).runs.find(x=>x.id===id);if(!['running','queued'].includes(r.status)){fs.writeFileSync(base+'run-'+id+'.json',JSON.stringify(r,null,2));return r}await new Promise(r=>setTimeout(r,1000))}throw Error('timeout')}
const doc=await call('knowledge',{Title:'验收1023 流水线输入知识',Content:'# 验收知识\n\nPIPELINE_KNOWLEDGE_1023\n'});
const kt=await save('S03 读取知识成功',[node('knowledge','knowledge.read',[['success','report']],{documentId:doc.id}),node('report','report.write',[['success','done']],{fileName:'knowledge-evidence.md',content:{result:'knowledge'}}),node('done','end.success')]);
let r=await wait((await call('run',{taskId:kt.id})).id);record('S03',r.status==='completed',{runID:r.id,status:r.status,documentID:doc.id});
const timeout=await save('A05 超时重试耗尽',[{...node('timeout','agent.run',[['success','done']]),prompt:'等待30秒后调用 pipeline_complete_node，outcome=success branch=success。执行。',criteria:'完成等待',permission:'read-only',timeoutSeconds:2,maxRetries:1,failureTarget:'handled'},node('handled','report.write',[['success','done']],{fileName:'timeout-handled.md',content:'超时重试耗尽后处理'}),node('done','end.success')]);
r=await wait((await call('run',{taskId:timeout.id})).id);record('A05',r.status==='completed'&&r.actions.filter(a=>a.id==='timeout').length===2,{runID:r.id,status:r.status,errors:r.actions.map(a=>a.error),path:r.actions.map(a=>a.id)});
const pause=await save('L01 暂停继续',[{...node('gate','agent.run',[['success','report']]),prompt:'等待30秒后汇报。执行。',criteria:'等待',permission:'read-only',timeoutSeconds:5,failureTarget:'report'},node('report','report.write',[['success','done']],{fileName:'resume.md',content:'恢复后的报告'}),node('done','end.success')]);
const started=await call('run',{taskId:pause.id});await call('control',null,{runID:started.id,operation:'pause'});r=await wait(started.id);record('L01-pause',r.status==='paused',{runID:r.id,status:r.status});
await call('control',null,{runID:r.id,operation:'resume'});r=await wait(started.id);record('L01-resume',r.status==='completed',{runID:r.id,status:r.status});
const stop=await call('run',{taskId:pause.id});await call('control',null,{runID:stop.id,operation:'stop'});r=await wait(stop.id);record('L02-stop',r.status==='cancelled',{runID:r.id,status:r.status});
const state=await call('state');const original=state.tasks.find(t=>t.name==='验收1023 A01 三分支实例');
const extended=await call('task',{...original,id:'',name:'验收1023 A06 延长时限真实分支',actions:original.actions.map(a=>a.kind==='agent'?{...a,timeoutSeconds:300,prompt:a.prompt+' 用户已授权执行本次验收，立即执行工具汇报。'}:a)});
r=await wait((await call('run',{taskId:extended.id,inputs:{route:'ready',payload:{count:3,enabled:true,message:'延长时限验证'}}})).id);
record('A06',r.status==='completed'&&r.actions[0].branch==='ready',{runID:r.id,status:r.status,error:r.error,report:r.actions[0].report});
