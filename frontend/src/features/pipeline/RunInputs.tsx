import { typeNames } from "./labels";
import type { Task } from "../../../bindings/agentor/internal/pipeline/model/models";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

export function RunInputs({task,values,onChange}:{task:Task;values:Record<string,unknown>;onChange:(values:Record<string,unknown>)=>void}){
  if (!task.inputs?.length) return <p className="text-sm text-muted-foreground">此任务无需填写运行参数。</p>;
  return <div className="grid gap-3 rounded-lg border bg-muted/15 p-3 @md/pipeline-form:grid-cols-2">{task.inputs?.map(field=><Field key={field.name}><FieldLabel>{field.name} · {typeNames[field.type]}{field.required?"（必填）":"（可选）"}</FieldLabel>{field.type==="boolean"?<Select value={values[field.name]===undefined?"":String(values[field.name])} onValueChange={selectedValue => { if (selectedValue === null) return; const next={...values};if(selectedValue==="")delete next[field.name];else next[field.name]=selectedValue==="true";onChange(next); }} items={[{ value: "", label: "未填写" }, { value: "true", label: "是" }, { value: "false", label: "否" }]}><SelectTrigger aria-label={`任务输入 ${field.name}`}><SelectValue /></SelectTrigger><SelectContent><SelectItem value="">未填写</SelectItem><SelectItem value="true">是</SelectItem><SelectItem value="false">否</SelectItem></SelectContent></Select>:<Input aria-label={`任务输入 ${field.name}`} type={field.type==="number"?"number":"text"} value={values[field.name]===undefined?"":String(values[field.name])} onChange={event=>{const next={...values};if(event.target.value==="")delete next[field.name];else next[field.name]=field.type==="number"?Number(event.target.value):event.target.value;onChange(next);}}/>}</Field>)}</div>;
}
