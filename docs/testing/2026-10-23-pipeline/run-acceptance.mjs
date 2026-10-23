import fs from 'node:fs';
import {execFileSync} from 'node:child_process';
const base=new URL('.',import.meta.url).pathname, projectID='OZK6T6UTSA72SJGPJTK7ONAE7N';
const results=[], prefix='验收1023';
export async function rpc(op,raw,extra={}){const r=await fetch('http://127.0.0.1:9278',{method:'POST',body:JSON.stringify({op,raw,projectID,...extra})});return r.json();}
const unwrap=r=>{if(r.error)throw Error(r.error);return r.data};
const call=async(...a)=>unwrap(await rpc(...a));
const record=(id,expected,actual,pass)=>{const row={id,expected,actual,pass};results.push(row);fs.writeFileSync(base+'acceptance-results.json',JSON.stringify(results,null,2));console.log(id,pass?'PASS':'FAIL',JSON.stringify(actual));};
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
const version=await call('version',{name:'流水线验收 2026-10-23',description:'保留完整验收模板、需求、步骤与运行记录'});
const feature=await call('feature',{title:'模板、静态分支和 Agent 多分支验收',description:'成功、业务分支、失败重试、Git 合并及输入引用验证',prompt:'只执行本轮验收定义，保留测试现场。'},{versionID:version.id});
const step=await call('step',{title:'验收 Agent 分支路由和输入',prompt:'根据节点明确指定的分支进行真实工具汇报，不修改项目文件。'},{featureID:feature.id});
fs.writeFileSync(base+'created-context.json',JSON.stringify({projectID,version,feature,step},null,2));
const t1=await save(task('S01 快照与报告',[node('snapshot','workspace.snapshot',[['success','report']]),report('report','done',{result:'snapshot'}),end()]));
let r=await run(t1);record('S01','snapshot → report → success，报告保留',summary(r),r.status==='completed'&&r.artifactIds.length===1);
const t2=await save(task('S02 知识不存在',[node('knowledge','knowledge.read',[['success','done']],{documentId:'acceptance-missing-document'}),end()]));
r=await run(t2);record('S02','静态异常只执行一次并失败',summary(r),r.status==='error'&&r.actions.length===1);
const branchTask=task('T01 三分支模板',[agent('choose',[['ready','ready-report'],['review','review-report'],['reject','reject-report']],'这是分支路由验收。禁止执行命令或改文件。读取已解析输入 route 与 payload，立即调用 pipeline_complete_node，outcome=success，branch 精确等于 route，summary 说明输入，output={route:实际route,payload:实际payload}。汇报后正常结束。',{params:{route:{input:'route'},payload:{input:'payload'}},stepSlot:'choose'}),report('ready-report','done',{result:'choose'}),report('review-report','done',{result:'choose'}),report('reject-report','failed',{result:'choose'}),end(),end('failed',true)],[{name:'route',type:'string',required:true},{name:'payload',type:'any',required:true}]);
const template=await call('template',branchTask);
const missing=await rpc('instantiate',{templateId:template.id,projectId:projectID,bindings:{}});
record('T01','未绑定必填 Step 拒绝实例化',missing,!!missing.error);
const instance=await call('instantiate',{templateId:template.id,projectId:projectID,bindings:{choose:step.id},name:prefix+' A01 三分支实例'});
const inputMissing=await rpc('run',{taskId:instance.id,inputs:{route:'ready'}});
record('T02','缺少必填运行输入拒绝启动',inputMissing,!!inputMissing.error);
await call('template',{...template,name:template.name+'（已更新）'});
const copy=(await call('state')).tasks.find(t=>t.id===instance.id);
record('T03','修改模板不改变实例', {templateID:template.id,taskID:instance.id,name:copy.name},copy.name===instance.name);
for(const route of ['ready','review','reject']){
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
const invalid=structuredClone(branchTask);invalid.projectId=projectID;invalid.actions[0].stepSlot='';invalid.actions[0].branches[0].target='choose';
record('T04','循环连线拒绝保存',await rpc('validate',invalid),!!(await rpc('validate',invalid)).error);
for(const mode of ['success','dirty','diverged','resolve']){
 const dir=repo(mode);
 const prompt=mode==='dirty'?'在当前工作区创建 dirty.txt 内容 DIRTY，不暂存不提交。':mode==='success'?'在当前工作区创建 success.txt 内容 SUCCESS，git add success.txt 后提交（标题 test: Pipeline acceptance），附加 Co-authored-by: Codex <codex@openai.com>。':'先执行 sleep 5。把当前工作区 conflict.txt 改为 WORKTREE，git add conflict.txt 后提交（标题 test: Worktree change），附加 Co-authored-by: Codex <codex@openai.com>。';
 const actions=[node('create','worktree.create',[['success','prepare']]),agent('prepare',[['success','merge']],prompt+' 禁止修改工作区外文件。完成后调用 pipeline_complete_node，outcome=success, branch=success, summary="测试前置条件已创建"。',{permission:'danger-full-access',timeoutSeconds:180}),node('merge','worktree.merge',[['success','done'],['dirty','dirty-report'],['diverged',mode==='resolve'?'resolve':'diverged-report']]),report('dirty-report','done','检测到未提交修改'),end()];
 if(mode==='resolve') actions.push({...agent('resolve',[['success','done']],'根据自动注入 worktree，在独立工作区将 sourceBranch 合入当前 branch。出现 conflict.txt 冲突时，保留双方意图，最终内容精确为 SOURCE 加换行，再 WORKTREE 加换行；暂存并完成合并提交，标题 fix: Resolve acceptance conflict，并追加 Co-authored-by: Codex <codex@openai.com>。不得修改来源工作区。用 git 确认无冲突、工作区干净、sourceBranch 为 HEAD 祖先。调用 pipeline_complete_node outcome=success branch=success summary=实际结果 output 包含 mergeCommit、resolvedFiles、unresolvedFiles。失败如实汇报。',{permission:'danger-full-access',timeoutSeconds:240,failureTarget:'failed'}),type:'agent.resolve-conflicts'},end('failed',true));
 else actions.push(report('diverged-report','done','检测到分支分歧'));
 const t=await save(task('G-'+mode,actions));const started=await start(t,{},dir);
 if(mode==='diverged'||mode==='resolve'){
  for(let i=0;i<100;i++){const state=await call('state');const current=state.runs.find(x=>x.id===started.id);if(current.worktree && fs.existsSync(current.worktree.directory)){fs.writeFileSync(dir+'/conflict.txt','SOURCE\n');git(dir,'add','conflict.txt');git(dir,'commit','-m','test: Source divergence');break;}await new Promise(r=>setTimeout(r,100));}
 }
 r=await wait(started.id);
 const merge=r.actions.find(a=>a.id==='merge');
 const pass= r.status==='completed'&&merge?.branch===(mode==='success'?'success':mode==='dirty'?'dirty':'diverged')&&(mode!=='resolve'||!!r.worktree.mergedCommit);
 record('G-'+mode,'静态合并分支 '+mode+'，保留工作区',summary(r),pass);
}
fs.writeFileSync(base+'final-pipeline-state.json',JSON.stringify(await call('state'),null,2));
fs.writeFileSync(base+'final-workspace.json',JSON.stringify((await call('workspace')).projects.find(p=>p.id===projectID),null,2));
console.log('DONE',results.length);
