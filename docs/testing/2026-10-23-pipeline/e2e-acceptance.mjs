import fs from 'node:fs';
import {execFileSync} from 'node:child_process';
const base=new URL('.',import.meta.url).pathname, projectID='OZK6T6UTSA72SJGPJTK7ONAE7N';
const results=[], prefix='验收1023';
export async function rpc(op,raw,extra={}){const r=await fetch('http://127.0.0.1:9278',{method:'POST',body:JSON.stringify({op,raw,projectID,...extra})});return r.json();}
const unwrap=r=>{if(r.error)throw Error(r.error);return r.data};
const call=async(...a)=>unwrap(await rpc(...a));
const record=(id,expected,actual,pass)=>{const row={id,expected,actual,pass};results.push(row);fs.writeFileSync(base+'e2e-results.json',JSON.stringify(results,null,2));console.log(id,pass?'PASS':'FAIL',JSON.stringify(actual));};
const node=(id,type,branches=[],params={})=>({id,type,kind:type.startsWith('end.')?'end':type.startsWith('agent.')?'agent':'static',name:id,params,branches:branches.map(([name,target])=>({name,target,description:name})),position:{x:0,y:0},prompt:'',criteria:'',permission:'workspace-write',maxRetries:0,timeoutSeconds:120,failureTarget:'',stepSlot:'',stepId:''});
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

const ctx=JSON.parse(fs.readFileSync(base+'created-context.json','utf8'));
const dir=repo('full-chain');
const step=await call('step',{title:'完整模板链路写入并提交验收文件',prompt:'执行：在当前独立工作区创建 acceptance-e2e.txt，内容精确为 PIPELINE_E2E_1023 加换行。读取核验，只提交该文件，标题 test: Verify pipeline e2e，追加 Co-authored-by: Codex <codex@openai.com>。不修改其他文件，不合并，不清理。调用 pipeline_complete_node，outcome=success，branch=success，summary=文件核验与提交证据，output 包含 file 和 commit。',permissionMode:'danger-full-access'},{featureID:ctx.feature.id});
const template=await call('template',task('E01 完整模板链路',[node('create','worktree.create',[['success','develop']]),agent('develop',[['success','merge']],'执行绑定步骤，完成文件写入、核验与提交，按要求汇报。',{stepSlot:'develop',permission:'danger-full-access',timeoutSeconds:300}),node('merge','worktree.merge',[['success','report'],['dirty',''],['diverged','']]),report('report','cleanup',{result:'develop'}),node('cleanup','worktree.remove',[['success','done']]),end()]));
const instance=await call('instantiate',{templateId:template.id,projectId:projectID,bindings:{develop:step.id},name:'验收1023 E01 完整模板实例'});
const r=await run(instance,{},dir);
const exists=fs.existsSync(dir+'/acceptance-e2e.txt');
record('E01','模板→绑定步骤→独立工作区→真实 Agent 提交→合并→报告→清理→成功',{...summary(r),templateID:template.id,taskID:instance.id,stepID:step.id,file:exists?fs.readFileSync(dir+'/acceptance-e2e.txt','utf8'):null},r.status==='completed'&&exists&&fs.readFileSync(dir+'/acceptance-e2e.txt','utf8')==='PIPELINE_E2E_1023\n'&&!fs.existsSync(r.worktree.directory));
