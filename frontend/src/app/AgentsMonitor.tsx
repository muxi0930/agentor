import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { Copy, Square } from "lucide-react";
import * as Tasks from "../../bindings/agentor/internal/tasks/service";
import * as Pipeline from "../../bindings/agentor/internal/pipeline/service";
import type { Run } from "../../bindings/agentor/internal/pipeline/model/models";
import type { Execution } from "../../bindings/agentor/internal/tasks/model/models";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { SettingsSection } from "@/components/ui/settings-section";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { errorText } from "./errors";

export function AgentsMonitor({ limit, onOpenThread }: { limit: number; onOpenThread: (threadId: string) => void }) {
  const [agents, setAgents] = useState<Execution[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [now, setNow] = useState(Date.now());
  const [stopping, setStopping] = useState<string[]>([]);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [runs, setRuns] = useState<Run[]>([]);
  const [pipelineLoaded, setPipelineLoaded] = useState(false);
  const [pipelineError, setPipelineError] = useState("");

  useEffect(() => {
    let active = true;
    let revision = 0;
    async function refresh() {
      const request = ++revision;
      try {
        const state = await Pipeline.GetState();
        if (!active || request !== revision) return;
        setRuns((state.runs || []).filter(run => run.status === "running"));
        setPipelineLoaded(true);
        setPipelineError("");
      } catch (error) {
        if (active && request === revision) setPipelineError(errorText(error));
      }
    }
    const unsubscribe = Events.On("pipeline:changed", () => void refresh());
    void refresh();
    return () => { active = false; unsubscribe(); };
  }, []);

  const pipelineAgents = runs.flatMap(run => (run.actions || [])
    .filter(node => node.kind === "agent" && node.status === "running")
    .map(node => ({ run, node })));

  useEffect(() => {
    let active = true;
    let revision = 0;
    async function refresh() {
      const request = ++revision;
      try {
        const state = await Tasks.GetState();
        if (!active || request !== revision) return;
        setAgents((state.executions || []).filter(execution => execution.status === "running"));
        setLoaded(true);
      } catch (error) {
        if (active && request === revision) setError(errorText(error));
      }
    }
    const unsubscribe = Events.On("tasks:changed", () => void refresh());
    void refresh();
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => { active = false; unsubscribe(); window.clearInterval(timer); };
  }, []);

  async function copySession(agent: Execution) {
    setError(""); setMessage("");
    try {
      await navigator.clipboard.writeText(agent.threadId);
      setMessage(`已复制 ${agent.context.featureTitle} 的 session ID`);
    } catch (error) { setError(errorText(error)); }
  }

  async function stop(agent: Execution) {
    setStopping(ids => [...ids, agent.id]);
    setError(""); setMessage("");
    try {
      await Tasks.StopExecution(agent.id);
      setMessage(`已请求终止 ${agent.context.featureTitle} 的任务`);
    } catch (error) { setError(errorText(error)); }
    finally { setStopping(ids => ids.filter(id => id !== agent.id)); }
  }

  return <><SettingsSection value="agents" title="Agents 监控" description="实时查看功能任务与流水线 Agent，功能任务的终止操作会中断对应 session 并取消任务。">
    <div className="space-y-3">
      <p role="status" aria-atomic="true" className="text-sm">{loaded ? <>当前运行 <strong>{agents.length}</strong> 个 agent · 并发上限 {limit}</> : "正在读取 agent 状态…"}</p>
      {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
      {message && <p role="status" className="text-sm text-muted-foreground">{message}</p>}
      <Table aria-label="运行中的功能 agents">
        <TableHeader><TableRow><TableHead>所属功能</TableHead><TableHead>已运行时间</TableHead><TableHead className="text-right">操作</TableHead></TableRow></TableHeader>
        <TableBody>
          {agents.map(agent => <TableRow key={agent.id}>
            <TableCell className="whitespace-normal"><button type="button" disabled={!agent.threadId} onClick={() => onOpenThread(agent.threadId)} aria-label={`查看 ${agent.context.featureTitle} 的会话`} className="w-full cursor-pointer rounded-sm text-left hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-default disabled:no-underline"><span className="block font-medium">{agent.context.featureTitle}</span><span className="block text-xs text-muted-foreground">{agent.context.projectName} · {agent.context.step.title}</span></button></TableCell>
            <TableCell className="tabular-nums">{elapsed(agent.startedAt, now)}</TableCell>
            <TableCell><div className="flex justify-end gap-2">
              <Button size="sm" variant="outline" disabled={!agent.threadId} onClick={() => void copySession(agent)}><Copy />复制session</Button>
              <Button size="sm" variant="destructive" disabled={agent.stopRequested || stopping.includes(agent.id)} onClick={() => void stop(agent)}><Square />{agent.stopRequested || stopping.includes(agent.id) ? "正在终止…" : "终止"}</Button>
            </div></TableCell>
          </TableRow>)}
          {agents.length === 0 && <TableRow><TableCell colSpan={3} className="py-6 text-center text-muted-foreground">{loaded ? "当前没有运行中的功能 agent" : "正在加载…"}</TableCell></TableRow>}
        </TableBody>
      </Table>
      <div className="space-y-3 border-t pt-4">
        <h3 className="text-sm font-medium">流水线 Agents</h3>
        <p role="status" aria-atomic="true" className="text-sm">{pipelineLoaded ? `当前运行 ${pipelineAgents.length} 个流水线 agent` : "正在读取流水线 agent 状态…"}</p>
        {pipelineError && <Alert variant="destructive"><AlertDescription>{pipelineError}</AlertDescription></Alert>}
        <Table aria-label="运行中的流水线 agents">
          <TableHeader><TableRow><TableHead>所属流水线 / 节点</TableHead><TableHead>模型 / 推理强度</TableHead><TableHead>尝试</TableHead><TableHead>已运行时间</TableHead><TableHead>Session</TableHead></TableRow></TableHeader>
          <TableBody>
            {pipelineAgents.map(({ run, node }) => <TableRow key={`${run.id}:${node.id}:${node.attempt}`}>
              <TableCell className="whitespace-normal"><span className="block font-medium">{run.task.name}</span><span className="block text-xs text-muted-foreground">{run.task.actions?.find(action => action.id === node.id)?.name || node.id}</span></TableCell>
              <TableCell>{run.model || "默认模型"} · {run.effort || "默认"}</TableCell>
              <TableCell>第 {node.attempt} 次</TableCell>
              <TableCell className="tabular-nums">{elapsed(node.startedAt, now)}</TableCell>
              <TableCell className="max-w-64 whitespace-normal break-all text-xs">{node.threadId || "正在创建会话…"}</TableCell>
            </TableRow>)}
            {!pipelineAgents.length && <TableRow><TableCell colSpan={5} className="py-6 text-center text-muted-foreground">{pipelineLoaded ? "当前没有运行中的流水线 agent" : "正在加载…"}</TableCell></TableRow>}
          </TableBody>
        </Table>
      </div>
    </div>
  </SettingsSection>
  <SettingsSection value="pipelines" title="流水线监控" description="实时查看所有项目及独立实例中正在运行的流水线。">
    <div className="space-y-3">
      <p role="status" aria-atomic="true" className="text-sm">{pipelineLoaded ? `当前运行 ${runs.length} 条流水线` : "正在读取流水线状态…"}</p>
      {pipelineError && <Alert variant="destructive"><AlertDescription>{pipelineError}</AlertDescription></Alert>}
      <Table aria-label="运行中的流水线">
        <TableHeader><TableRow><TableHead>流水线</TableHead><TableHead>当前节点</TableHead><TableHead>状态</TableHead><TableHead>已运行时间</TableHead><TableHead>执行目录</TableHead></TableRow></TableHeader>
        <TableBody>
          {runs.map(run => <TableRow key={run.id}>
            <TableCell className="whitespace-normal"><span className="block font-medium">{run.task.name}</span><span className="block break-all text-xs text-muted-foreground">{run.id}</span></TableCell>
            <TableCell className="whitespace-normal">{run.task.actions?.find(node => node.id === run.currentNode)?.name || run.currentNode || "—"}</TableCell>
            <TableCell>{run.pauseRequested ? "等待节点结束后暂停" : "运行中"}</TableCell>
            <TableCell className="tabular-nums">{elapsed(run.createdAt, now)}</TableCell>
            <TableCell className="max-w-64 whitespace-normal break-all text-xs">{run.worktree?.directory || run.directory || "—"}</TableCell>
          </TableRow>)}
          {!runs.length && <TableRow><TableCell colSpan={5} className="py-6 text-center text-muted-foreground">{pipelineLoaded ? "当前没有运行中的流水线" : "正在加载…"}</TableCell></TableRow>}
        </TableBody>
      </Table>
    </div>
  </SettingsSection></>;
}

function elapsed(startedAt: string, now: number) {
  const started = Date.parse(startedAt);
  if (!Number.isFinite(started)) return "—";
  const seconds = Math.max(0, Math.floor((now - started) / 1000));
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor(seconds % 3600 / 60);
  return `${hours > 0 ? `${hours}小时 ` : ""}${minutes}分 ${seconds % 60}秒`;
}
