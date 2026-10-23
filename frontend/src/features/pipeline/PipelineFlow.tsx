import { useState } from "react";
import { Check, Circle, Pause, X } from "lucide-react";
import type { Action, ActionResult } from "../../../bindings/agentor/internal/pipeline/model/models";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { actionName, parameterHelp, runLabels } from "./labels";

// 参考 demo 的 diagram-ascii-flow 节点、连接和选中详情结构，改用适合中文的卡片布局
export function PipelineFlow({ actions, results = [], onSelect }: { actions: Action[]; results?: ActionResult[]; onSelect?: (index: number) => void }) {
  const [selected, setSelected] = useState(0);
  const index = Math.min(selected, Math.max(0, actions.length - 1));
  const current = actions[index];
  const latest = (id: string) => { const matches = results.filter(result => result.id === id); return matches[matches.length - 1]; };
  if (!actions.length) return <p className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">添加第一个节点后，这里会展示完整执行流程。</p>;
  return <section aria-label="流水线节点流程图" className="pipeline-flow min-w-0 overflow-hidden rounded-xl border bg-muted/20">
    <div className="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3"><h3 className="text-sm font-semibold">节点流程 · {actions.length} 个节点</h3><p className="text-xs text-muted-foreground">按编号顺序执行 · 成功后进入下一节点</p></div>
    <div className="p-3">
      <ol className="pipeline-flow-grid">
        {actions.map((action, i) => { const result = latest(action.id); const status = result?.status || "pending"; const Icon = status === "completed" ? Check : status === "error" || status === "interrupted" ? X : status === "paused" ? Pause : Circle;
          return <li key={`${i}:${action.id}`} className="min-w-0">
            
            <Button variant="outline" aria-pressed={index === i} className={`h-full w-full min-w-0 flex-col items-start gap-2 whitespace-normal p-3 text-left ${index === i ? "border-primary bg-primary/5 ring-1 ring-primary" : "bg-card"}`} onClick={() => { setSelected(i); onSelect?.(i); }}>
              <span className="flex w-full items-center justify-between text-xs text-muted-foreground"><span>节点 {String(i + 1).padStart(2, "0")}</span><Icon aria-hidden="true" className="size-4" /></span>
              <span className="break-words text-sm font-semibold">{actionName(action.type)}</span>
              <span className="max-w-full truncate text-xs text-muted-foreground">{action.id || "尚未命名"}</span>
              <Badge variant={status === "error" ? "destructive" : "outline"}>{runLabels[status] || status}</Badge>
            </Button>
          </li>;
        })}
      </ol>
    </div>
    {current && <div className="space-y-2 border-t bg-card px-4 py-3"><p className="text-sm font-medium">节点 {index + 1} · {actionName(current.type)}</p><dl className="grid min-w-0 gap-2 text-xs">{Object.entries(current.params || {}).map(([key, value]) => <div key={key} className="min-w-0"><dt className="text-muted-foreground">{parameterHelp[key]?.label || key}</dt><dd className="mt-1 break-words whitespace-pre-wrap">{typeof value === "object" && value !== null ? ("result" in value ? `使用节点「${String(value.result)}」的输出` : "input" in value ? `使用任务输入「${String(value.input)}」` : JSON.stringify(value)) : String(value)}</dd></div>)}</dl>{!Object.keys(current.params || {}).length && <p className="text-xs text-muted-foreground">此节点无需填写参数，自动使用当前项目与本次工作区。</p>}{latest(current.id)?.error && <p className="text-xs text-destructive">{latest(current.id)?.error}</p>}</div>}
  </section>;
}
