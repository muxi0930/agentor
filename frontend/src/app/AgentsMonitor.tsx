import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { Copy, Square } from "lucide-react";
import * as Tasks from "../../bindings/agentor/internal/tasks/service";
import type { Execution } from "../../bindings/agentor/internal/tasks/models";
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

  return <SettingsSection value="agents" title="Agents 监控" description="实时查看工作队列中的功能任务，终止操作会中断对应 session 并取消任务。">
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
    </div>
  </SettingsSection>;
}

function elapsed(startedAt: string, now: number) {
  const started = Date.parse(startedAt);
  if (!Number.isFinite(started)) return "—";
  const seconds = Math.max(0, Math.floor((now - started) / 1000));
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor(seconds % 3600 / 60);
  return `${hours > 0 ? `${hours}小时 ` : ""}${minutes}分 ${seconds % 60}秒`;
}
