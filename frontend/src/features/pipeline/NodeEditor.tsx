import { useEffect, useRef, useState } from "react";
import { Plus, Trash2, X } from "lucide-react";
import type { ActionResult, ActionSpec } from "../../../bindings/agentor/internal/pipeline/model/models";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { actionName, parameterHelp, runLabels } from "./labels";
import { canConnect, type Graph, type GraphNode } from "./graph";
import { Choice, FormField, JSONField, TextField } from "./GraphControls";

export type StepOption = { value: string; label: string };
export function NodeEditor({ node, task, specs, template, steps, result, currentStatus, readOnly, onChange, onEntry, onDelete, onClose, onIssue }: {
  node: GraphNode; task: Graph; specs: ActionSpec[]; template?: boolean; steps: StepOption[]; result?: ActionResult; currentStatus?: string; readOnly?: boolean;
  onChange: (node: GraphNode) => void; onEntry: () => void; onDelete: () => void; onClose: () => void; onIssue: (key: string, error: string) => void;
}) {
  const panel = useRef<HTMLDivElement>(null);
  useEffect(() => { const previous = document.activeElement as HTMLElement | null; panel.current?.focus(); return () => previous?.focus(); }, []);
  const spec = specs.find(spec => spec.name === node.type);
  const targets = [{ value: "", label: "未连接 · 到达时终止" }, ...task.actions.filter(item => canConnect(task, node.id, item.id)).map(item => ({ value: item.id, label: `${item.name || actionName(item.type)} (${item.id})` }))];
  const patch = (change: Partial<GraphNode>) => onChange({ ...node, ...change });
  return <div ref={panel} tabIndex={-1} role="dialog" aria-modal="false" aria-label={`${actionName(node.type)} 配置`} className="pipeline-node-editor pipeline-form nowheel nodrag nopan" onKeyDown={event => { event.stopPropagation(); if (event.key === "Escape") { event.preventDefault(); onClose(); } }}>
    <header className="flex items-start justify-between gap-2 border-b p-3"><div className="min-w-0"><p className="break-words text-sm font-semibold">{actionName(node.type)}</p><p className="mt-1 truncate text-xs text-muted-foreground">{node.id}</p></div><Button size="icon-sm" variant="ghost" aria-label="关闭节点配置" onClick={onClose}><X /></Button></header>
    <div className="min-h-0 space-y-4 overflow-y-auto p-3">
      {readOnly ? <><p className="text-sm">{node.name}</p><p className="text-xs">状态：{runLabels[result?.status || currentStatus || "pending"]} {result?.branch && `· 分支 ${result.branch}`}</p>{result?.error && <p className="text-sm text-destructive">{result.error}</p>}{node.kind === "agent" && <><FormField label="执行要求"><p className="whitespace-pre-wrap text-sm">{node.prompt}</p></FormField><FormField label="验收标准"><p className="whitespace-pre-wrap text-sm">{node.criteria}</p></FormField></>}<FormField label="输入与输出"><pre className="overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted p-3 text-xs">{JSON.stringify({ input: result?.inputs || node.params, output: result?.output, report: result?.report }, null, 2)}</pre></FormField></> : <>
        <TextField label="节点备注名称（可选）" value={node.name} onChange={name => patch({ name })} />
        <Button size="sm" variant="outline" disabled={task.entry === node.id} onClick={onEntry}>{task.entry === node.id ? "当前入口节点" : "设为入口节点"}</Button>
        {node.kind === "static" && <><p className="text-xs text-muted-foreground">{spec?.description}</p><ParameterFields node={node} task={task} spec={spec} onChange={params => patch({ params })} onIssue={onIssue} /><p className="text-xs text-muted-foreground">静态节点不重试。预期结果沿对应分支继续，异常立即终止。</p></>}
        {node.kind === "agent" && <>
          {node.type === "agent.resolve-conflicts" && <p className="text-xs text-muted-foreground">将静态合并节点的 diverged 分支连接到此节点。运行时自动注入 worktree（仓库、工作区及分支信息），无需手填路径。成功后已完成合并，可连接清理节点；无法解决时走“重试耗尽后”连线。输出包含 summary、resolvedFiles、unresolvedFiles、mergeCommit。</p>}
          <TextField label="执行要求" value={node.prompt} multiline onChange={prompt => patch({ prompt })} />
          <TextField label="验收标准" value={node.criteria} multiline onChange={criteria => patch({ criteria })} help="Agent 按此标准自检，并通过工具汇报结果。" />
          <AgentInputs node={node} task={task} onChange={params => patch({ params })} onIssue={onIssue} />
          <FormField label="执行权限"><Choice label="执行权限" value={node.permission} items={[{ value: "workspace-write", label: "允许修改工作目录" }, { value: "read-only", label: "只读" }, { value: "danger-full-access", label: "完全访问（无沙箱）" }]} onChange={permission => patch({ permission })} /></FormField>
          <div className="grid grid-cols-2 gap-3"><TextField label="失败重试次数" type="number" min={0} max={10} value={node.maxRetries} onChange={value => patch({ maxRetries: Number(value) })} /><TextField label="单次超时（秒）" type="number" min={1} value={node.timeoutSeconds} onChange={value => patch({ timeoutSeconds: Number(value) })} /></div>
          <p className="text-xs text-muted-foreground">最多执行 {node.maxRetries + 1} 次；业务分支不触发重试。</p>
          {template ? <FormField label="项目步骤"><label className="flex items-center gap-2 text-xs"><Switch checked={!!node.stepSlot} onCheckedChange={required => patch({ stepSlot: required ? node.stepSlot || node.id : "", stepId: "" })} />需要绑定项目步骤</label><p className="text-xs text-muted-foreground">开启后，根据模板创建流水线时，必须为此节点选择当前项目下的一个步骤。</p></FormField> : task.projectId ? <FormField label={node.stepSlot ? "绑定项目步骤（必填）" : "绑定项目步骤（可选）"}><Choice label="绑定项目步骤" value={node.stepId} items={[...(node.stepSlot ? [] : [{ value: "", label: "不绑定步骤" }]), ...steps]} onChange={stepId => patch({ stepId })} /></FormField> : <p className="text-xs text-muted-foreground">独立 Agent 节点，无需绑定项目步骤。</p>}
        </>}
        {node.kind !== "end" && <section className="space-y-3 border-t pt-3"><div className="flex items-center justify-between"><h4 className="text-xs font-semibold">结果分支</h4>{node.kind === "agent" && <Button size="xs" variant="outline" onClick={() => patch({ branches: [...node.branches, { name: `branch_${node.branches.length + 1}`, description: "", target: "" }] })}><Plus />添加分支</Button>}</div>
          {node.branches.map((branch, index) => <div key={index} className="space-y-2 rounded-lg border p-2">
            {node.kind === "agent" ? <div className="flex items-center gap-2"><Input aria-label={`分支 ${index + 1} 标识`} value={branch.name} onChange={event => patch({ branches: node.branches.map((value, i) => i === index ? { ...value, name: event.target.value } : value) })} /><Button size="icon-xs" variant="ghost" aria-label={`移除分支 ${branch.name}`} onClick={() => patch({ branches: node.branches.filter((_, i) => i !== index) })}><Trash2 /></Button></div> : <p className="text-xs font-medium">{branch.name} · {branch.description}</p>}
            {node.kind === "agent" && <TextField label="选择此分支的条件" value={branch.description} onChange={description => patch({ branches: node.branches.map((value, i) => i === index ? { ...value, description } : value) })} />}
            <Choice label={`${branch.name} 的后续节点`} value={branch.target} items={targets} onChange={target => patch({ branches: node.branches.map((value, i) => i === index ? { ...value, target } : value) })} />
          </div>)}
          {node.kind === "agent" && <FormField label="重试耗尽后"><Choice label="重试耗尽后" value={node.failureTarget} items={targets} onChange={failureTarget => patch({ failureTarget })} /></FormField>}
        </section>}
        {node.kind === "end" && <p className="text-sm text-muted-foreground">到达此终点后，整条流水线以{node.type === "end.success" ? "成功" : "失败"}状态结束。</p>}
      </>}
    </div>
    <footer className="flex shrink-0 items-center justify-between border-t p-3">{!readOnly && <Button variant="ghost" size="sm" onClick={onDelete}><Trash2 />移除节点</Button>}<Button size="sm" className="ml-auto" onClick={onClose}>完成</Button></footer>
  </div>;
}

function ParameterFields({ node, task, spec, onChange, onIssue }: { node: GraphNode; task: Graph; spec?: ActionSpec; onChange: (params: Record<string, unknown>) => void; onIssue: (key: string, error: string) => void }) {
  return <div className="space-y-3">{!spec?.parameters?.length && <p className="text-xs text-muted-foreground">无需参数，使用本次运行的工作目录。</p>}{spec?.parameters?.map(param => <ParameterValue key={param.name} name={param.name} label={`${parameterHelp[param.name]?.label || param.name}${param.required ? "（必填）" : "（可选）"}`} type={param.type} value={node.params?.[param.name]} task={task} node={node} onChange={value => { const next = { ...node.params }; if (value === undefined) delete next[param.name]; else next[param.name] = value; onChange(next); }} onIssue={error => onIssue(`${node.id}:${param.name}`, error)} />)}</div>;
}
function ParameterValue({ name, label, type, value, task, node, onChange, onIssue }: { name: string; label: string; type: string; value: unknown; task: Graph; node: GraphNode; onChange: (value: unknown) => void; onIssue: (error: string) => void }) {
  const ref = value && typeof value === "object" ? value as Record<string, string> : undefined;
  const [mode, setMode] = useState(ref?.input !== undefined ? "input" : ref?.result !== undefined ? "result" : value !== undefined && typeof value !== "string" ? "json" : "text");
  return <FormField label={label}><Choice label={`${label}来源`} value={mode} items={[{ value: "text", label: "固定文本" }, ...(type === "any" ? [{ value: "json", label: "固定 JSON" }] : []), { value: "input", label: "运行输入" }, { value: "result", label: "前序节点输出" }]} onChange={next => { setMode(next); onIssue(""); onChange(next === "text" ? "" : next === "json" ? {} : { [next]: "" }); }} />
    {mode === "text" ? <TextField label={name} value={typeof value === "string" ? value : ""} multiline={type === "any"} onChange={text => onChange(text === "" ? undefined : text)} /> : mode === "json" ? <JSONField label={name} value={value} onChange={onChange} onIssue={onIssue} /> : <><Choice label={`${label}引用`} value={ref?.[mode] || ""} items={[{ value: "", label: "请选择" }, ...(mode === "input" ? task.inputs.map(input => ({ value: input.name, label: input.name })) : task.actions.filter(item => item.id !== node.id && item.kind !== "end").map(item => ({ value: item.id, label: `${item.id} · ${item.name || actionName(item.type)}` })))]} onChange={id => onChange({ [mode]: id, ...(ref?.path ? { path: ref.path } : {}) })} /><TextField label="输出字段路径（可选）" value={ref?.path || ""} onChange={path => onChange({ [mode]: ref?.[mode] || "", ...(path ? { path } : {}) })} /></>}
  </FormField>;
}

// Agent 输入逐项编辑，保留现有参数值与引用结构
function AgentInputs({ node, task, onChange, onIssue }: { node: GraphNode; task: Graph; onChange: (params: Record<string, unknown>) => void; onIssue: (key: string, error: string) => void }) {
  const upstream = new Set<string>();
  const collect = (target: string) => {
    for (const item of task.actions) {
      if (upstream.has(item.id) || item.id === node.id) continue;
      if (item.failureTarget === target || item.branches.some(branch => branch.target === target)) {
        upstream.add(item.id);
        collect(item.id);
      }
    }
  };
  collect(node.id);
  const sources = { ...task, actions: task.actions.filter(item => upstream.has(item.id)) };
  return <section className="space-y-3"><div className="flex items-center justify-between"><h4 className="text-xs font-semibold">输入数据</h4><Button size="xs" variant="outline" onClick={() => {
    let index = 1;
    while (Object.prototype.hasOwnProperty.call(node.params, "param_" + index)) index++;
    onChange({ ...node.params, ["param_" + index]: "" });
  }}><Plus />添加参数</Button></div>
    {node.type === "agent.resolve-conflicts" && <p className="text-xs text-muted-foreground">worktree 由运行时自动提供，无需添加。</p>}
    {!Object.keys(node.params).length && <p className="text-xs text-muted-foreground">暂无自定义参数，可添加固定值或选择运行输入、前序节点输出。</p>}
    {Object.entries(node.params).map(([name, value]) => <AgentInput key={name} name={name} value={value} node={node} task={sources} onChange={onChange} onIssue={onIssue} />)}
  </section>;
}

// 参数重命名在失焦时提交，避免输入过程中丢失焦点与原有值
function AgentInput({ name, value, node, task, onChange, onIssue }: { name: string; value: unknown; node: GraphNode; task: Graph; onChange: (params: Record<string, unknown>) => void; onIssue: (key: string, error: string) => void }) {
  const [draftName, setDraftName] = useState(name);
  const [error, setError] = useState("");
  const issueKey = node.id + ":param-name:" + name;
  const nameError = (next: string) => !next.trim() ? "参数名不能为空" : next !== name && Object.prototype.hasOwnProperty.call(node.params, next) ? "参数名已存在" : node.type === "agent.resolve-conflicts" && next === "worktree" ? "worktree 由运行时自动提供" : "";
  return <div className="space-y-2 rounded-lg border p-2">
    <div className="flex items-center gap-2"><Input aria-label={"参数 " + name + " 名称"} value={draftName} onChange={event => {
      const next = event.target.value;
      setDraftName(next);
      const message = nameError(next);
      setError(message);
      onIssue(issueKey, message);
    }} onBlur={() => {
      if (nameError(draftName) || draftName === name) return;
      onIssue(issueKey, "");
      onIssue(node.id + ":" + name, "");
      onChange(Object.fromEntries(Object.entries(node.params).map(([key, item]) => [key === name ? draftName : key, item])));
    }} /><Button size="icon-xs" variant="ghost" aria-label={"移除参数 " + name} onClick={() => {
      onIssue(issueKey, "");
      onIssue(node.id + ":" + name, "");
      onChange(Object.fromEntries(Object.entries(node.params).filter(([key]) => key !== name)));
    }}><Trash2 /></Button></div>
    {error && <p role="alert" className="text-xs text-destructive">{error}</p>}
    <ParameterValue name={name} label="参数值" type="any" value={value} node={node} task={task} onChange={next => onChange({ ...node.params, [name]: next === undefined ? "" : next })} onIssue={message => onIssue(node.id + ":" + name, message)} />
  </div>;
}
