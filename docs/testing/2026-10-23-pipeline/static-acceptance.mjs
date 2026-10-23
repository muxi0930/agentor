import fs from 'node:fs';import {execFileSync} from 'node:child_process';
const base=new URL('.',import.meta.url).pathname, projectID='OZK6T6UTSA72SJGPJTK7ONAE7N',results=JSON.parse(fs.readFileSync(base+'static-results.json','utf8'));
async function rpc(op,raw,extra={}){return fetch('http://127.0.0.1:9278',{method:'POST',body:JSON.stringify({op,raw,projectID,...extra})}).then(r=>r.json())}
async function call(...args){const r=await rpc(...args);if(r.error)throw Error(r.error);return r.data}
const git=(d,...a)=>execFileSync('git',['-C',d,...a],{encoding:'utf8'}).trim();
const n=(id,type,branches=[],params={})=>({id,type,kind:type.startsWith('end')?'end':type.startsWith('agent')?'agent':'static',params,branches:branches.map(([name,target])=>({name,target})),position:{x:0,y:0}});
for(const mode of ['unhandled','cleanup']){
 const dir='/Users/zhui/Work/p/trash/aa/Feat/pipeline-acceptance-20261023/static-final-'+mode;fs.mkdirSync(dir,{recursive:true});git(dir,'init','-b','main');git(dir,'config','user.name','Codex Acceptance');git(dir,'config','user.email','codex@openai.com');fs.writeFileSync(dir+'/fixture.txt','BASE\n');git(dir,'add','fixture.txt');git(dir,'commit','-m','test: Static fixture');
 const actions=[n('create','worktree.create',[['success','gate']]),{...n('gate','agent.run',[['success','merge']]),prompt:'验收暂停点，不修改任何文件。等待30秒后汇报 success。执行。',criteria:'等待验收',permission:'read-only',timeoutSeconds:3,failureTarget:'merge'},n('merge','worktree.merge',[['success',mode==='cleanup'?'cleanup':'done'],['dirty',mode==='unhandled'?'':'dirty'],['diverged','diverged']]),n('dirty','report.write',[['success','done']],{fileName:'dirty.md',content:'dirty branch'}),n('diverged','report.write',[['success','done']],{fileName:'diverged.md',content:'diverged branch'}),n('done','end.success')];
 if(mode==='cleanup')actions.push(n('cleanup','worktree.remove',[['success','done']]));
 const task=await call('task',{name:'验收1023 S-Git-'+mode,entry:'create',actions:actions.filter(a=>mode!=='unhandled'||a.id!=='dirty').map((a,i)=>({...a,position:{x:i*280,y:0}}))});
 const started=await call('run',{taskId:task.id},{directory:dir});
 let mutated=false, final;
 for(let i=0;i<180;i++){
  const r=(await call('state')).runs.find(r=>r.id===started.id);
  if(r.worktree&&fs.existsSync(r.worktree.directory)&&!mutated){
   const wd=r.worktree.directory;
   if(mode==='dirty'||mode==='unhandled'){fs.writeFileSync(wd+'/dirty.txt','UNCOMMITTED\n')}
   else{fs.writeFileSync(wd+'/fixture.txt','WORKTREE\n');git(wd,'add','fixture.txt');git(wd,'commit','-m','test: Worktree fixture');if(mode==='diverged'){fs.writeFileSync(dir+'/fixture.txt','SOURCE\n');git(dir,'add','fixture.txt');git(dir,'commit','-m','test: Source fixture')}}
   mutated=true;
  }
  if(!['running','queued'].includes(r.status)){final=r;break;}
  await new Promise(r=>setTimeout(r,200));
 }
 if(!final)throw Error('timeout '+started.id);
 fs.writeFileSync(base+'run-'+final.id+'.json',JSON.stringify(final,null,2));
 const branch=final.actions.find(a=>a.id==='merge')?.branch;
 const expected=['success','cleanup'].includes(mode)?'success':mode==='unhandled'?'dirty':mode;
 const row={id:'S-Git-'+mode,pass:mutated&&branch===expected&&final.status===(mode==='unhandled'?'error':'completed')&&(mode!=='cleanup'||!fs.existsSync(final.worktree.directory)),runID:final.id,status:final.status,branch,error:final.error,path:final.actions.map(a=>a.id+':'+a.status),directory:dir,worktree:final.worktree};
 results.push(row);fs.writeFileSync(base+'static-results.json',JSON.stringify(results,null,2));console.log(JSON.stringify(row));
}
