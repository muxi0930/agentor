import fs from 'node:fs';
import {execFileSync} from 'node:child_process';
const base=new URL('.',import.meta.url).pathname, projectID='OZK6T6UTSA72SJGPJTK7ONAE7N';
const results=[], prefix='验收1023';
export async function rpc(op,raw,extra={}){const r=await fetch('http://127.0.0.1:9278',{method:'POST',body:JSON.stringify({op,raw,projectID,...extra})});return r.json();}
const unwrap=r=>{if(r.error)throw Error(r.error);return r.data};
const call=async(...a)=>unwrap(await rpc(...a));
const record=(id,expected,actual,pass)=>{const row={id,expected,actual,pass};results.push(row);fs.writeFileSync(base+'resume-results.json',JSON.stringify(results,null,2));console.log(id,pass?'PASS':'FAIL',JSON.stringify(actual));};
const node=(id,type,branches=[],params={})=>({id,type,kind:type.startsWith('end.')?'end':type.startsWith('agent.')?'agent':'static',name:id,params,branches:branches.map(([name,target])=>({name,target,description:name})),position:{x:0,y:0},prompt:'',criteria:'',permission:'workspace-write',maxRetries:0,timeoutSeconds:300,failureTarget:'',stepSlot:'',stepId:''});
const end=(id='done',failure=false)=>node(id,failure?'end.failure':'end.success');
const report=(id,next,content)=>node(id,'report.write',[['success',next]],{fileName:id+'.md',content});
const agent=(id,branches,prompt,opts={})=>({...node(id,'agent.run',branches),prompt,criteria:'按要求调用 pipeline_complete_node 汇报可验证的结果，且正常结束本轮。',...opts});
const task=(name,actions,inputs=[])=>({name:prefix+' '+name,entry:actions[0].id,actions:actions.map((a,i)=>({...a,position:{x:i*300,y:i%2*130}})),inputs});
const save=async t=>call('task',t);
const wait=async id=>{for(let i=0;i<360;i++){const state=await call('state');const r=state.runs.find(r=>r.id===id);if(!['running','queued'].includes(r.status)){fs.writeFileSync(base+'run-'+id+'.json',JSON.stringify(r,null,2));return r}if(i%20===0)console.log('WAIT',id,r.currentNode);await new Promise(r=>setTimeout(r,1000));}throw Error('timeout '+id)};
const start=async(t,inputs={},directory)=>call('run',{taskId:t.id,inputs},{directory});
const run=async(t,inputs={},directory)=>wait((await start(t,inputs,directory)).id);
const summary=r=>({runID:r.id,status:r.status,error:r.error,path:r.actions.map(a=>a.id+':'+a.branch+':'+a.status),attempts:r.actions.length});
const git=(dir,...args)=>execFileSync('git',['-C',dir,...args],{encoding:'utf8'}).trim();
const repoRoot='/Users/zhui/Work/p/trash/aa/Feat/pipeline-acceptance-20261023';
function repo(name){const dir=repoRoot+'/'+name;fs.mkdirSync(dir,{recursive:true});git(dir,'init','-b','main');git(dir,'config','user.name','Codex Acceptance');git(dir,'config','user.email','codex@openai.com');fs.writeFileSync(dir+'/conflict.txt','BASE\n');git(dir,'add','conflict.txt');git(dir,'commit','-m','test: Initialize pipeline fixture');return dir}

const instance=(await call('state')).tasks.find(t=>t.name==='验收1023 A06 延长时限真实分支');
let r;
const review=(await call('state')).runs.find(r=>r.id==='QTEG6XCHLLJ5PTBAH4BCKQOM4F'); record('A01-review','只走 review 分支',summary(review),review.status==='completed'&&review.actions[0].branch==='review');
for(const route of ['reject']){
 r=await run(instance,{route,payload:{message:'真实输入-'+route,enabled:true,count:3}});
 const selected=r.actions.find(a=>a.id==='choose');
 record('A01-'+route,'只走 '+route+' 分支，输入原样透传',summary(r),selected?.branch===route && r.actions.length===2 && r.status===(route==='reject'?'error':'completed')&&selected.output?.payload?.count===3);
}
const retry=await save(task('A02 重试后成功',[agent('retry',[['success','done']],'这是重试验收。禁止运行命令。若“上一次失败”为空，调用 pipeline_complete_node，outcome=failure，branch=""，summary="ACCEPTANCE_EXPECTED_FIRST_FAILURE"。若上一次失败包含该标记，则 outcome=success, branch=success, summary="第二次尝试通过", output={attemptRecovered:true}。正常结束。',{maxRetries:1}),end()]));
r=await run(retry);record('A02','首次失败、第二次成功，共2次',summary(r),r.status==='completed'&&r.actions.length===2&&r.actions[0].status==='error');
const exhaust=await save(task('A03 耗尽后处理',[agent('fail', [['success','done']], '仅调用 pipeline_complete_node，outcome=failure, branch="", summary="ACCEPTANCE_EXPECTED_FAILURE"，不执行命令，然后正常结束。',{maxRetries:1,failureTarget:'handled'}),report('handled','done','重试耗尽后的处理已执行'),end()]));
r=await run(exhaust);record('A03','失败两次→处理报告→成功终点',summary(r),r.status==='completed'&&r.actions.filter(a=>a.id==='fail').length===2&&r.actions.at(-1).id==='handled');
const unconnected=await save(task('A04 未连接分支',[agent('loose',[['success','']], '仅调用 pipeline_complete_node，outcome=success, branch=success, summary="验收未连接分支"，正常结束。')]));
r=await run(unconnected);record('A04','已汇报但未连接分支导致终止',summary(r),r.status==='error'&&r.error.includes('没有连接'));
