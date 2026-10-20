import { useEffect, useState } from "react";
import { Plug, RefreshCw } from "lucide-react";
import * as Codex from "../../../bindings/agentor/internal/codex/service";
import type { AccountInfo, Status } from "../../../bindings/agentor/internal/codex/models";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { errorText } from "@/app/errors";

export function RuntimeConnection({ status, onStatus }: { status?: Status; onStatus: (status: Status) => void }) {
  const [account, setAccount] = useState<AccountInfo>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    if (status?.phase !== "ready") { setAccount(undefined); return; }
    let cancelled = false;
    void Codex.GetAccount().then(value => { if (!cancelled) { setAccount(value); setError(""); } }).catch(error => { if (!cancelled) setError(errorText(error)); });
    return () => { cancelled = true; };
  }, [status?.phase, status?.generation]);
  async function connect() {
    setBusy(true); setError(""); setAccount(undefined);
    try { onStatus(await Codex.Connect()); }
    catch (error) { setError(errorText(error)); }
    finally { setBusy(false); }
  }
  return <div className="space-y-3 border-t pt-4">
    <p role="status" className="text-sm">{status?.message || "尚未连接 Codex"}</p>
    {status?.path && <dl className="grid grid-cols-[5rem_1fr] gap-x-3 gap-y-2 text-xs">
      <dt className="text-muted-foreground">程序</dt><dd className="min-w-0 break-all">{status.path}</dd>
      <dt className="text-muted-foreground">版本</dt><dd>{status.version}</dd>
      <dt className="text-muted-foreground">数据目录</dt><dd className="min-w-0 break-all">{status.home}</dd>
      <dt className="text-muted-foreground">配置来源</dt><dd>{status.reuseCLI ? "当前 CLI 的登录状态与连接配置" : "agentor 独立配置"}</dd>
    </dl>}
    {account && <p className="text-sm text-muted-foreground">{account.account
      ? `当前身份：${account.account.email || account.account.type}${account.account.planType ? ` · ${account.account.planType}` : ""}`
      : account.requiresOpenaiAuth ? "当前 CLI 尚未登录" : "已使用 CLI 配置的 API 服务"}</p>}
    {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
    <Button variant="outline" onClick={() => void connect()} disabled={busy || status?.phase === "starting"}>
      {status?.phase === "ready" ? <RefreshCw /> : <Plug />}{busy ? "正在连接…" : status?.phase === "ready" ? "重新连接" : "连接 Codex"}
    </Button>
  </div>;
}
