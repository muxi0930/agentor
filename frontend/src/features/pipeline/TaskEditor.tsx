import { useCallback, useEffect, useState } from "react";
import { FolderOpen, Plus, SlidersHorizontal, Trash2 } from "lucide-react";
import * as Workspace from "../../../bindings/agentor/internal/workspace/service";
import type { ActionSpec, Task } from "../../../bindings/agentor/internal/pipeline/model/models";
import * as Pipeline from "../../../bindings/agentor/internal/pipeline/service";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { errorText } from "@/app/errors";
import { typeNames } from "./labels";
import { PipelineCanvas } from "./PipelineCanvas";
import { newTask, normalizeTask } from "./graph";
import { Choice, TextField } from "./GraphControls";
import type { StepOption } from "./NodeEditor";

export function TaskEditor({ projectId, task, actions, template = false, onSaved, onError }: { projectId: string; task: Task | null; actions: ActionSpec[]; template?: boolean; onSaved: (task: Task) => void; onError: (text: string) => void }) {
  const [inputsOpen, setInputsOpen] = useState(false);
  const [directoryOpen, setDirectoryOpen] = useState(false);
  const [draft, setDraft] = useState(() => normalizeTask(task || newTask(projectId)));
  const [busy, setBusy] = useState(false), [message, setMessage] = useState(""), [error, setError] = useState("");
  const [issues, setIssues] = useState<Record<string, string>>({}), [steps, setSteps] = useState<StepOption[]>([]);
  useEffect(() => { let live = true; Workspace.GetState().then(state => { const project = state.projects?.find(project => project.id === projectId); if (live) setSteps((project?.versions || []).flatMap(version => (version.features || []).flatMap(feature => (feature.steps || []).map(step => ({ value: step.id, label: `${version.name} / ${feature.title} / ${step.title}` }))))); }).catch(error => { if (live) setError(errorText(error)); }); return () => { live = false; }; }, [projectId]);
  const onIssue = useCallback((key: string, error: string) => setIssues(current => { if (current[key] === error) return current; return { ...current, [key]: error }; }), []);
  const invalid = Object.entries(issues).some(([key, issue]) => issue && draft.actions.some(node => key.startsWith(node.id + ":")));
  async function submit(save: boolean) {
    if (invalid) return;
    setBusy(true); setError(""); setMessage("");
    try {
      const text = JSON.stringify(draft);
      const next = template ? await (save ? Pipeline.SaveTemplate(text) : Pipeline.ValidateTemplate(text)) : await (save ? Pipeline.SaveTask(projectId, text) : Pipeline.ValidateTask(projectId, text));
      if (save) onSaved(next); else setMessage("节点、分支连线与输入引用校验通过。");
    } catch (error) { const text = errorText(error); setError(text); onError(text); } finally { setBusy(false); }
  }
  return <div className="pipeline-task-editor pipeline-form">
    <div className="flex shrink-0 flex-wrap items-end gap-3"><div className="w-[20%] min-w-0"><Input aria-label={template ? "模板名称" : "流水线名称"} value={draft.name} onChange={event => setDraft({ ...draft, name: event.target.value })} /></div><Dialog open={inputsOpen} onOpenChange={setInputsOpen}><DialogTrigger render={<Button type="button" variant="outline" />}><SlidersHorizontal />运行输入 · {draft.inputs.length}</DialogTrigger><DialogContent className="sm:max-w-[540px]"><DialogHeader><DialogTitle>运行输入</DialogTitle><DialogDescription>配置运行时需要填写的字段。</DialogDescription></DialogHeader><div className="max-h-[50dvh] space-y-3 overflow-y-auto">{draft.inputs.map((field, index) => <div key={index} className="grid grid-cols-[minmax(0,1fr)_7rem] sm:grid-cols-[minmax(0,1fr)_7rem_auto_auto] items-center gap-2"><Input aria-label={`输入 ${index + 1} 名称`} placeholder="输入名称" value={field.name} onChange={event => setDraft({ ...draft, inputs: draft.inputs.map((item, i) => i === index ? { ...item, name: event.target.value } : item) })} /><Choice label={`输入 ${index + 1} 类型`} value={field.type} items={["string", "number", "boolean", "any"].map(value => ({ value, label: typeNames[value] }))} onChange={type => setDraft({ ...draft, inputs: draft.inputs.map((item, i) => i === index ? { ...item, type } : item) })} /><label className="flex items-center gap-1 text-xs"><Switch checked={field.required} onCheckedChange={required => setDraft({ ...draft, inputs: draft.inputs.map((item, i) => i === index ? { ...item, required } : item) })} />必填</label><Button size="icon-xs" variant="ghost" aria-label={`移除输入 ${index + 1}`} onClick={() => setDraft({ ...draft, inputs: draft.inputs.filter((_, i) => i !== index) })}><Trash2 /></Button></div>)}<Button size="xs" variant="outline" onClick={() => setDraft({ ...draft, inputs: [...draft.inputs, { name: `input_${draft.inputs.length + 1}`, type: "string", required: true }] })}><Plus />添加输入</Button></div><DialogFooter><Button type="button" onClick={() => setInputsOpen(false)}>完成</Button></DialogFooter></DialogContent></Dialog>{!template && !projectId ? <Dialog open={directoryOpen} onOpenChange={setDirectoryOpen}><DialogTrigger render={<Button type="button" variant="outline" />}><FolderOpen />执行目录</DialogTrigger><DialogContent className="sm:max-w-[540px]"><DialogHeader><DialogTitle>执行目录</DialogTitle><DialogDescription>配置流水线运行时使用的工作目录。</DialogDescription></DialogHeader><TextField label="执行目录（可选）" value={draft.directory} onChange={directory => setDraft({ ...draft, directory })} help="留空则为每次运行创建独立目录。" labelHelp="请输入绝对路径，不支持相对路径。如果流水线要处理现有代码，就填代码仓库路径；如果只是执行不依赖现有项目文件的任务，可以留空，每次运行会自动创建独立目录。" /><DialogFooter><Button type="button" onClick={() => setDirectoryOpen(false)}>完成</Button></DialogFooter></DialogContent></Dialog> : !template && <p className="self-center text-xs text-muted-foreground">执行目录由当前项目提供，创建 worktree 后自动切换到独立工作区。</p>}</div>

    <PipelineCanvas task={draft} specs={actions} template={template} steps={steps} onIssue={onIssue} onChange={next => { setDraft(next); setError(""); setMessage(""); }} />
    <footer className="flex shrink-0 flex-wrap items-center gap-3 border-t pt-3"><Button disabled={busy || invalid} onClick={() => void submit(true)}>{template ? "保存模板" : "保存流水线"}</Button><Button variant="outline" disabled={busy || invalid} onClick={() => void submit(false)}>验证配置</Button><span role={error ? "alert" : "status"} className={`min-w-0 flex-1 text-xs ${error || invalid ? "text-destructive" : "text-muted-foreground"}`}>{error || (invalid ? "请修正节点中的 JSON 输入" : message || "点击节点配置；分支未连接时，执行到该出口会终止。")}</span></footer>
  </div>;
}
