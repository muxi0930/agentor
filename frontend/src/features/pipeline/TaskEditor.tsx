import { useEffect, useState } from "react";
import * as Workspace from "../../../bindings/agentor/internal/workspace/service";
import type { Action, ActionSpec, InputField, Task } from "../../../bindings/agentor/internal/pipeline/model/models";
import * as Pipeline from "../../../bindings/agentor/internal/pipeline/service";
import { Button } from "@/components/ui/button";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { errorText } from "@/app/errors";
import { actionName, parameterHelp, typeNames } from "./labels";

type Draft = Omit<Task, "inputs" | "actions"> & { inputs: InputField[]; actions: (Omit<Action, "params"> & { params: Record<string, unknown> })[] };
function normalized(task: Task): Draft { return { ...task, inputs: task.inputs || [], actions: (task.actions || []).map(action => ({ ...action, params: action.params || {} })) }; }
function Choice({ value, items, label, onChange }: { value: string; items: { value: string; label: string }[]; label: string; onChange: (value: string) => void }) {
  return <Select value={value} onValueChange={value => { if (value !== null) onChange(value); }} items={items}><SelectTrigger aria-label={label} className="w-full"><SelectValue /></SelectTrigger><SelectContent>{items.map(item => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent></Select>;
}
export function TaskEditor({ projectId, task, actions, onSaved, onError }: { projectId: string; task: Task | null; actions: ActionSpec[]; onSaved: (task: Task) => void; onError: (text: string) => void }) {
  const [draft, setDraft] = useState<Draft>(() => normalized(task || { id: "", projectId, name: "", inputs: [], actions: [{ id: "snapshot", type: "workspace.snapshot", params: {} }, { id: "report", type: "report.write", params: { fileName: "report.md", content: { result: "snapshot" } } }] }));
  const [mode, setMode] = useState("form"), [json, setJSON] = useState(""), [busy, setBusy] = useState(false), [valid, setValid] = useState(""), [error, setError] = useState("");
  const [targets, setTargets] = useState<Record<string, { value: string; label: string }[]>>({});
  useEffect(() => {
    let cancelled = false;
    Workspace.GetState().then(state => {
      const project = state.projects?.find(project => project.id === projectId);
      const versions = project?.versions || [];
      if (!cancelled) setTargets({
        versionId: versions.map(version => ({ value: version.id, label: version.name })),
        featureId: versions.flatMap(version => (version.features || []).map(feature => ({ value: feature.id, label: `${version.name} / ${feature.title}` }))),
        stepId: versions.flatMap(version => (version.features || []).flatMap(feature => (feature.steps || []).map(step => ({ value: step.id, label: `${version.name} / ${feature.title} / ${step.title}` })))),
      });
    }).catch(error => { if (!cancelled) setError(errorText(error)); });
    return () => { cancelled = true; };
  }, [projectId]);
  function update(next: Draft) { setDraft(next); setValid(""); setError(""); }
  function actionAt(index: number, change: Partial<Draft["actions"][number]>) { update({ ...draft, actions: draft.actions.map((action, i) => i === index ? { ...action, ...change } : action) }); }
  function changeMode(next: string) { try { if (next === "json") setJSON(JSON.stringify(draft, null, 2)); else setDraft(normalized(JSON.parse(json))); setMode(next); setValid(""); } catch (error) { setError(errorText(error)); } }
  async function submit(save: boolean) { setBusy(true); setValid(""); setError(""); try { const text = mode === "json" ? json : JSON.stringify(draft); if (save) onSaved(await Pipeline.SaveTask(projectId, text)); else { await Pipeline.ValidateTask(projectId, text); setValid("配置验证通过，尚未执行节点。"); } } catch (error) { const message = errorText(error); setError(message); onError(message); } finally { setBusy(false); } }
  return <div className="pipeline-form space-y-3"><p className="text-sm text-muted-foreground">先定义运行时需要填写的输入，再按执行顺序配置节点。保存后的修改只影响新运行。</p>
    <Tabs value={mode} onValueChange={changeMode}><TabsList><TabsTrigger value="form">可视化配置</TabsTrigger><TabsTrigger value="json">JSON 配置</TabsTrigger></TabsList>
      <TabsContent value="json"><Textarea aria-label="流水线任务 JSON" className="min-h-[28rem] font-mono text-xs" value={json} onChange={event => { setJSON(event.target.value); setValid(""); }} /></TabsContent>
      <TabsContent value="form" className="space-y-4">
        {!task && <div className="rounded-xl border bg-muted/15 p-3"><p className="mb-3 text-sm font-medium">从常用流程开始</p><Button variant="outline" size="sm" onClick={() => update({ ...draft, name: draft.name || "完成步骤并合并", actions: [
          { id: "create", type: "worktree.create", params: {} },
          { id: "develop", type: "step.run", params: { stepId: "", instruction: "完成选定步骤，运行验证并提交变更，汇报业务结果。", completionCriteria: "验证通过、变更已提交且工作区干净。" } },
          { id: "merge", type: "worktree.merge", params: {} },
          { id: "cleanup", type: "worktree.remove", params: {} },
        ] })}>使用「开发并合并」四节点模板</Button><p className="mt-2 text-xs text-muted-foreground">替换当前节点列表，然后在执行步骤中选择目标步骤。</p></div>}
        <Field><FieldLabel htmlFor="pipeline-task-name">任务名称 <span className="text-muted-foreground">必填</span></FieldLabel><Input id="pipeline-task-name" placeholder="例如：完成步骤并合并到主分支" value={draft.name} onChange={event => update({ ...draft, name: event.target.value })} /></Field>
        <section className="space-y-4 rounded-xl border bg-muted/15 p-3"><div className="flex items-center justify-between gap-3"><div><h3 className="font-semibold">1. 运行时输入</h3><p className="mt-1 text-xs text-muted-foreground">每次启动任务时填写的变量。节点可通过“任务输入”使用这些值。</p></div><Button variant="outline" size="sm" onClick={() => update({ ...draft, inputs: [...draft.inputs, { name: "", type: "string", required: true }] })}>添加输入</Button></div>
          {!draft.inputs.length && <p className="rounded-lg border border-dashed p-3 text-sm text-muted-foreground">没有运行时输入。所有参数都可以直接填写在节点中。</p>}
          {draft.inputs.map((field, index) => <fieldset key={index} className="rounded-lg border bg-card p-3"><legend className="px-2 text-sm font-medium">输入 {index + 1}</legend><div className="grid gap-3 @md/pipeline-form:grid-cols-2"><Field><FieldLabel htmlFor={`input-name-${index}`}>输入名称</FieldLabel><Input id={`input-name-${index}`} placeholder="例如：目标步骤" value={field.name} onChange={event => update({ ...draft, inputs: draft.inputs.map((value, i) => i === index ? { ...value, name: event.target.value } : value) })} /><p className="text-xs text-muted-foreground">名称须唯一，用于运行表单和节点引用。</p></Field><Field><FieldLabel>值的类型</FieldLabel><Choice label={`输入 ${index + 1} 的值类型`} value={field.type} items={["string", "number", "boolean"].map(value => ({ value, label: typeNames[value] }))} onChange={type => update({ ...draft, inputs: draft.inputs.map((value, i) => i === index ? { ...value, type } : value) })} /></Field></div><div className="mt-3 flex items-center justify-between"><label className="flex items-center gap-2 text-sm"><Checkbox checked={field.required} onCheckedChange={required => update({ ...draft, inputs: draft.inputs.map((value, i) => i === index ? { ...value, required: !!required } : value) })} />每次运行必须填写</label><Button variant="ghost" size="sm" onClick={() => update({ ...draft, inputs: draft.inputs.filter((_, i) => i !== index) })}>移除输入</Button></div></fieldset>)}
        </section>
        <section className="space-y-4"><div><h3 className="font-semibold">2. 执行节点</h3><p className="mt-1 text-xs text-muted-foreground">节点成功后自动执行下一个；失败则停止。引用结果只能来自前面的节点。</p></div>
          {draft.actions.map((action, index) => { const spec = actions.find(option => option.name === action.type); return <fieldset id={`pipeline-node-${index}`} key={index} className="min-w-0 space-y-3 rounded-xl border bg-card p-3"><legend className="px-2 text-sm font-semibold">节点 {index + 1} · {actionName(action.type)}</legend>
            <div className="grid gap-3 @md/pipeline-form:grid-cols-2"><Field><FieldLabel htmlFor={`node-id-${index}`}>节点标识</FieldLabel><Input id={`node-id-${index}`} value={action.id} onChange={event => actionAt(index, { id: event.target.value })} /><p className="text-xs text-muted-foreground">在此任务内唯一，后续节点用它引用输出。</p></Field><Field><FieldLabel>执行动作</FieldLabel><Choice label={`节点 ${index + 1} 的执行动作`} value={action.type} items={actions.map(option => ({ value: option.name, label: actionName(option.name) }))} onChange={type => actionAt(index, { type, params: {} })} /><p className="text-xs text-muted-foreground">{spec?.description}</p></Field></div>
            <div className="space-y-4 border-t pt-3">{!spec?.parameters?.length && <p className="text-sm text-muted-foreground">无需填写参数，自动使用当前项目与本次工作区。</p>}{spec?.parameters?.map(param => {
              const value = action.params[param.name], object = value && typeof value === "object" ? value as Record<string, string> : undefined;
              const kind = object?.input !== undefined ? "input" : object?.result !== undefined ? "result" : "literal";
              const info = parameterHelp[param.name] || { label: param.name, help: "", placeholder: "" };
              const options = kind === "input" ? draft.inputs.filter(field => field.name && (param.type === "any" || field.type === param.type)).map(field => ({ value: field.name, label: field.name })) : draft.actions.slice(0, index).filter(item => item.id).map(item => ({ value: item.id, label: `${item.id} · ${actionName(item.type)}` }));
              const fieldID = `node-${index}-${param.name}`;
              return <Field key={param.name}><FieldLabel htmlFor={kind === "literal" ? fieldID : undefined}>{info.label}<span className="ml-2 text-xs text-muted-foreground">{param.required ? "必填" : "可选"}</span></FieldLabel><p id={`${fieldID}-help`} className="text-xs text-muted-foreground">{info.help}</p><div className="grid items-start gap-2 @md/pipeline-form:grid-cols-[140px_minmax(0,1fr)]"><Choice label={`${info.label}的参数来源`} value={kind} items={[{ value: "literal", label: "直接填写" }, { value: "input", label: "使用任务输入" }, ...(param.type === "any" ? [{ value: "result", label: "使用节点输出" }] : [])]} onChange={type => { const params = { ...action.params }; delete params[param.name]; if (type !== "literal") params[param.name] = { [type]: "" }; actionAt(index, { params }); }} />{kind === "literal" ? targets[param.name] ? <Choice label={info.label} value={typeof value === "string" ? value : ""} items={[{ value: "", label: param.required ? "请选择" : "全部 / 不限定" }, ...targets[param.name], ...(typeof value === "string" && value && !targets[param.name].some(item => item.value === value) ? [{ value, label: `已失效：${value}` }] : [])]} onChange={selected => { const params = { ...action.params }; if (!selected && !param.required) delete params[param.name]; else params[param.name] = selected; actionAt(index, { params }); }} /> : ["instruction", "completionCriteria", "content"].includes(param.name) ? <Textarea id={fieldID} aria-describedby={`${fieldID}-help`} value={value === undefined ? "" : String(value)} placeholder={info.placeholder} onChange={event => actionAt(index, { params: { ...action.params, [param.name]: event.target.value } })} /> : <Input id={fieldID} aria-describedby={`${fieldID}-help`} value={value === undefined ? "" : String(value)} placeholder={info.placeholder} onChange={event => { const params = { ...action.params }; if (!event.target.value && !param.required) delete params[param.name]; else params[param.name] = event.target.value; actionAt(index, { params }); }} /> : <div className="space-y-1"><Choice label={`${info.label}引用目标`} value={object?.[kind] || ""} items={[{ value: "", label: "请选择" }, ...options]} onChange={selected => actionAt(index, { params: { ...action.params, [param.name]: { [kind]: selected } } })} />{!options.length && <p className="text-xs text-muted-foreground">{kind === "input" ? "请先添加相同值类型的运行时输入。" : "前面还没有可引用的节点。"}</p>}</div>}</div></Field>;
            })}</div>
            <div className="flex justify-end border-t pt-3"><Button variant="ghost" size="sm" onClick={() => update({ ...draft, actions: draft.actions.filter((_, i) => i !== index) })}>移除此节点</Button></div>
          </fieldset>; })}
          <Button variant="outline" className="w-full border-dashed" onClick={() => { let id = 1; while (draft.actions.some(action => action.id === `node-${id}`)) id++; update({ ...draft, actions: [...draft.actions, { id: `node-${id}`, type: "workspace.snapshot", params: {} }] }); }}>添加下一个节点</Button>
        </section>
      </TabsContent>
    </Tabs>
    {error && <p role="alert" className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive">{error}</p>}{valid && <p role="status" className="text-sm text-muted-foreground">{valid}</p>}
    <div className="sticky bottom-0 flex gap-2 border-t bg-background py-3"><Button disabled={busy} onClick={() => void submit(true)}>保存任务</Button><Button variant="outline" disabled={busy} onClick={() => void submit(false)}>验证配置</Button></div>
  </div>;
}
