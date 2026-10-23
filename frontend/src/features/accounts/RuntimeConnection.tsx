import { useState } from "react";
import { Plug, RefreshCw } from "lucide-react";
import * as Codex from "../../../bindings/agentor/internal/codex/service";
import type { Status } from "../../../bindings/agentor/internal/codex/model/models";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { errorText } from "@/app/errors";

export function RuntimeConnection({ status, onStatus }: { status?: Status; onStatus: (status: Status) => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function connect() {
    setBusy(true); setError("");
    try { onStatus(await Codex.Connect()); }
    catch (error) { setError(errorText(error)); }
    finally { setBusy(false); }
  }
  return <div className="space-y-3">
    <dl className="grid grid-cols-[5rem_1fr] gap-x-3 gap-y-2 text-xs">
      <dt className="text-muted-foreground">状态</dt><dd role="status">{status?.message || "尚未连接"}</dd>
      {status?.path && <>
        <dt className="text-muted-foreground">程序</dt><dd className="min-w-0 break-all">{status.path}</dd>
        <dt className="text-muted-foreground">版本</dt><dd>{status.version}</dd>
        <dt className="text-muted-foreground">数据目录</dt><dd className="min-w-0 break-all">{status.home}</dd>
        <dt className="text-muted-foreground">配置来源</dt><dd>{status.reuseCLI ? "所选目录的现有登录状态与连接配置" : "agentor 独立配置"}</dd>
      </>}
    </dl>
    {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
    <div className="flex justify-end"><Button variant="outline" onClick={() => void connect()} disabled={busy || status?.phase === "starting"}>
      {status?.phase === "ready" ? <RefreshCw /> : <Plug />}{busy ? "正在连接…" : status?.phase === "ready" ? "重新连接" : "连接 Codex"}
    </Button></div>
  </div>;
}
