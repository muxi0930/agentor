import type { Task } from "../../../bindings/agentor/internal/pipeline/models";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";

export function RunInputs({task,values,onChange}:{task:Task;values:Record<string,unknown>;onChange:(values:Record<string,unknown>)=>void}){
  return <div className="grid gap-3 sm:grid-cols-2">{task.inputs?.map(field=><Field key={field.name}><FieldLabel>{field.name}{field.required?" *":""}</FieldLabel>{field.type==="boolean"?<NativeSelect aria-label={`任务输入 ${field.name}`} value={values[field.name]===undefined?"":String(values[field.name])} onChange={event=>{const next={...values};if(event.target.value==="")delete next[field.name];else next[field.name]=event.target.value==="true";onChange(next);}}><NativeSelectOption value="">未填写</NativeSelectOption><NativeSelectOption value="true">true</NativeSelectOption><NativeSelectOption value="false">false</NativeSelectOption></NativeSelect>:<Input aria-label={`任务输入 ${field.name}`} type={field.type==="number"?"number":"text"} value={values[field.name]===undefined?"":String(values[field.name])} onChange={event=>{const next={...values};if(event.target.value==="")delete next[field.name];else next[field.name]=field.type==="number"?Number(event.target.value):event.target.value;onChange(next);}}/>}</Field>)}</div>;
}
