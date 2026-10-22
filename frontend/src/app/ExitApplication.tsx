import { useEffect, useState } from "react";
import { LoaderCircle, Power, RotateCw } from "lucide-react";
import { Events } from "@wailsio/runtime";
import * as Lifecycle from "../../bindings/agentor/internal/lifecycle/service";
import type { ExitState } from "../../bindings/agentor/internal/lifecycle/models";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { errorText } from "./errors";

export function ExitApplication({ restart, dataDirectory }: { restart?: { mode: string; directory: string; currentHome: string; targetHome: string }; dataDirectory?: { current: string; target: string } }) {
  const switching = !!restart || !!dataDirectory;
  const [restartApplication, setRestartApplication] = useState(false);
  const restarting = switching || restartApplication;
  const [open, setOpen] = useState(false);
  const [snapshot, setSnapshot] = useState<ExitState>();
  const [error, setError] = useState("");
  const [quitting, setQuitting] = useState(false);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    let revision = 0;
    setSnapshot(undefined);
    setError("");
    async function refresh() {
      const current = ++revision;
      try {
        const next = await Lifecycle.GetExitState();
        if (!cancelled && current === revision) { setSnapshot(next); setError(""); }
      } catch (error) {
        if (!cancelled && current === revision) setError(errorText(error));
      }
    }
    void refresh();
    const removeTasks = Events.On("tasks:changed", () => void refresh());
    const removePipeline = Events.On("pipeline:changed", () => void refresh());
    const removeCodex = Events.On("codex:event", event => {
      if (["turn/started", "turn/completed"].includes(event.data.method)) void refresh();
    });
    return () => { cancelled = true; removeTasks(); removePipeline(); removeCodex(); };
  }, [open]);

  async function quit() {
    setQuitting(true);
    setError("");
    try {
      if (dataDirectory) await Lifecycle.RestartDataDirectory(dataDirectory.target);
      else if (restart) await Lifecycle.RestartCodexHome(restart.mode, restart.directory);
      else if (restartApplication) await Lifecycle.Restart();
      else await Lifecycle.Quit();
    }
    catch (error) { setError(errorText(error)); setQuitting(false); }
  }

  const tasks = snapshot?.tasks || [];

  return <div className={switching ? "flex justify-end" : "flex justify-end gap-2 border-t pt-6"}>
    {!switching && <Button variant="outline" disabled={quitting} onClick={() => { setRestartApplication(true); setOpen(true); }}><RotateCw />重启</Button>}
    <Button variant={switching ? "default" : "destructive"} disabled={quitting || !!dataDirectory && !dataDirectory.target.trim() || !!restart && restart.mode === "custom" && !restart.directory.trim()} onClick={() => { setRestartApplication(false); setOpen(true); }}>{quitting ? <LoaderCircle className="animate-spin" /> : <Power />}{switching ? "应用并重启" : "完全退出"}</Button>
    <AlertDialog open={open} onOpenChange={next => { if (!quitting) setOpen(next); }}>
      <AlertDialogContent className="sm:max-w-lg">
        <AlertDialogHeader>
          <AlertDialogTitle>{dataDirectory ? "是否切换应用数据根目录并完全重启？" : restart ? "是否切换 CODEX_HOME 并完全重启？" : restartApplication ? "是否立刻中断所有任务并重启应用？" : "是否立刻中断所有任务并退出？"}</AlertDialogTitle>
          <AlertDialogDescription>{dataDirectory ? "确认后会中断运行中的任务、取消全部排队任务，并完整重启应用。现有数据可能丢失：新目录不会自动包含当前设置、账号、任务和附件。本应用不提供迁移功能；如需保留，请取消切换，退出应用后自行拷贝数据到目标目录。原目录中的文件不会被删除。" : restart ? "确认后会中断运行中的任务、取消全部排队任务，完成关闭后重新启动 agentor。保留当前账号选择，不迁移历史会话；当前账号的配置与凭证会应用到目标目录。" : restartApplication ? "确认后会中断运行中的任务、取消全部排队任务，并在后台完成收尾后重新启动整个应用。当前账号和数据目录保持不变。" : "确认后会中断运行中的任务、取消全部排队任务，并在后台完成收尾后关闭程序。"}</AlertDialogDescription>
        </AlertDialogHeader>
        {dataDirectory && <dl className="space-y-2 text-sm"><dt className="text-muted-foreground">当前目录</dt><dd className="break-all">{dataDirectory.current}</dd><dt className="text-muted-foreground">目标目录</dt><dd className="break-all">{dataDirectory.target}</dd></dl>}
        {restart && <dl className="space-y-2 text-sm"><dt className="text-muted-foreground">当前目录</dt><dd className="break-all">{restart.currentHome}</dd><dt className="text-muted-foreground">目标目录</dt><dd className="break-all">{restart.targetHome}</dd></dl>}
        {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
        {snapshot ? <div className="space-y-3">
          <p className="rounded-lg bg-muted p-3 text-sm" role="status">正在运行 {snapshot.running} 项 · 排队 {snapshot.queued} 项</p>
          {tasks.length > 0 ? <ul aria-label="退出会中断或取消的任务" className="max-h-64 space-y-2 overflow-y-auto">
            {tasks.map(task => <li key={`${task.kind}:${task.id}`} className="rounded-lg border p-3 text-sm">
              <p className="mb-1 text-xs text-muted-foreground">{task.kind} · {task.status === "running" ? "运行中" : "排队中"}</p>
              <p className="break-words">{task.summary}</p>
            </li>)}
          </ul> : <p className="text-sm text-muted-foreground">当前没有运行或排队任务。</p>}
        </div> : <p className="flex items-center gap-2 text-sm text-muted-foreground"><LoaderCircle className="size-4 animate-spin" />正在读取任务…</p>}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={quitting}>取消</AlertDialogCancel>
          <Button variant="destructive" disabled={quitting || !snapshot || !!error} aria-busy={quitting} onClick={() => void quit()}>
            {quitting && <LoaderCircle aria-hidden="true" className="animate-spin" />}{quitting ? restarting ? "正在关闭并重启…" : "正在退出…" : switching ? "确认切换并重启" : restartApplication ? "确认重启" : "确认退出"}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>;
}
