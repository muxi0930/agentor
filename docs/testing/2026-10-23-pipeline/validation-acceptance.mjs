import fs from 'node:fs';
const base=new URL('.',import.meta.url).pathname,projectID='OZK6T6UTSA72SJGPJTK7ONAE7N';
const rpc=async(op,raw)=>fetch('http://127.0.0.1:9278',{method:'POST',body:JSON.stringify({op,raw,projectID})}).then(r=>r.json());
const state=(await rpc('state')).data,original=state.tasks.find(t=>t.name==='验收1023 A01 三分支实例'),results=[];
for(const [name,change] of [
 ['循环连线',t=>t.actions[0].branches[0].target='choose'],
 ['不存在的目标节点',t=>t.actions[0].branches[0].target='missing'],
 ['空提示词',t=>t.actions[0].prompt=''],
 ['必填步骤清空',t=>t.actions[0].stepId=''],
 ['未来节点输出引用',t=>t.actions[0].params={bad:{result:'ready-report'}}],
 ['重复分支名称',t=>t.actions[0].branches[1].name='ready']
]){
 const t=structuredClone(original);t.id='';change(t);const response=await rpc('validate',t);
 results.push({id:'V-'+name,pass:!!response.error,response});
 fs.writeFileSync(base+'invalid-'+results.length+'.json',JSON.stringify(t,null,2));
}
const wrong=await rpc('run',{taskId:original.id,inputs:{route:123,payload:{}}});
results.push({id:'V-运行输入类型错误',pass:!!wrong.error,response:wrong});
fs.writeFileSync(base+'validation-results.json',JSON.stringify(results,null,2));console.log(JSON.stringify(results,null,2));
