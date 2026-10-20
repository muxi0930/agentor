import { useState } from "react";
import { FolderOpen, FileSearch, ExternalLink, Save, Monitor } from "lucide-react";
import * as Data from "../../bindings/agentor/internal/appdata/service";
import type { Info, Settings } from "../../bindings/agentor/internal/appdata/models";
import type { Status } from "../../bindings/agentor/internal/codex/models";
import { RuntimeConnection } from "@/features/accounts/RuntimeConnection";
import { PermissionSelect } from "@/features/accounts/PermissionSelect";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Field, FieldLabel, FieldDescription, FieldGroup } from "@/components/ui/field";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { errorText } from "./errors";

export function SettingsPage({ info, settings, onSettings, status, onStatus }: {
  info: Info; settings: Settings; onSettings: (settings: Settings) => void; status?: Status; onStatus: (status: Status) => void;
}) {
  const [path, setPath] = useState(settings.codexPath);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  async function run(action: () => Promise<void>) {
    setBusy(true); setError(""); setMessage("");
    try { await action(); } catch (error) { setError(errorText(error)); }
    finally { setBusy(false); }
  }

  return <div className="mx-auto w-full max-w-3xl space-y-6 p-6 lg:p-10">
    <div><h1 className="text-2xl font-semibold tracking-tight">设置</h1>
      <p className="mt-2 text-sm text-muted-foreground">连接您的工具，安排舒适的工作环境。</p></div>
    {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
    {message && <p role="status" className="text-sm text-muted-foreground">{message}</p>}
    <Card>
      <CardHeader><CardTitle>工作环境</CardTitle><CardDescription>偏好保存在 agentor 的独立数据目录中。</CardDescription></CardHeader>
      <CardContent><FieldGroup>
        <Field><FieldLabel htmlFor="theme">外观</FieldLabel>
          <NativeSelect id="theme" value={settings.theme} disabled={busy} onChange={event => void run(async () => {
            onSettings(await Data.SetTheme(event.target.value));
          })}><NativeSelectOption value="light">浅色</NativeSelectOption><NativeSelectOption value="dark">深色</NativeSelectOption></NativeSelect>
        </Field>
        <Field><FieldLabel htmlFor="default-permission">默认执行权限</FieldLabel><PermissionSelect id="default-permission" disabled={busy} value={settings.permissionMode} onChange={mode => void run(async () => onSettings(await Data.SetPermission("default", "", mode)))} /><FieldDescription>会话设置优先于项目设置，再继承此默认值。只读可查看文件；项目内读写可修改当前项目；完全访问允许项目外文件与网络操作。</FieldDescription></Field>
        <Field><FieldLabel>当前项目目录</FieldLabel>
          <p className="break-all rounded-lg border bg-muted/40 p-3 text-sm">{settings.projectDirectory || "尚未选择"}</p>
          <div className="flex gap-2"><Button variant="outline" disabled={busy} onClick={() => void run(async () => {
            const directory = await Data.ChooseDirectory();
            if (directory) onSettings(await Data.SetProjectDirectory(directory));
          })}><FolderOpen />选择目录</Button>
            {settings.projectDirectory && <Button variant="ghost" disabled={busy} onClick={() => void run(() => Data.OpenDirectory(settings.projectDirectory))}>打开目录<ExternalLink /></Button>}
          </div>
        </Field>
      </FieldGroup></CardContent>
    </Card>
    <Card>
      <CardHeader><CardTitle>Codex 原生程序</CardTitle><CardDescription>指定原生可执行文件；留空时从 PATH 查找。</CardDescription></CardHeader>
      <CardContent><form onSubmit={event => { event.preventDefault(); void run(async () => {
        onSettings(await Data.SetCodexPath(path.trim())); setMessage("Codex 路径已保存");
      }); }}><FieldGroup>
        <Field><FieldLabel htmlFor="codex-path">可执行文件路径</FieldLabel>
          <div className="flex gap-2"><Input id="codex-path" value={path} placeholder="Codex 原生二进制路径" onChange={event => setPath(event.target.value)} />
            <Button variant="outline" type="button" aria-label="选择 Codex 程序" disabled={busy} onClick={() => void run(async () => {
              const file = await Data.ChooseFile(); if (file) setPath(file);
            })}><FileSearch /></Button></div>
          <FieldDescription>{settings.reuseCLI ? "当前沿用 CLI 的登录状态与连接配置。" : "使用 agentor 独立的 Codex 数据与登录信息。"}</FieldDescription>
        </Field>
        <div><Button type="submit" disabled={busy}><Save />保存路径</Button></div>
      </FieldGroup></form><div className="mt-5"><RuntimeConnection status={status} onStatus={onStatus} /></div></CardContent>
    </Card>
    <Card>
      <CardHeader><CardTitle className="flex items-center gap-2"><Monitor className="size-4" />应用信息</CardTitle></CardHeader>
      <CardContent className="space-y-4 text-sm">
        <dl className="grid grid-cols-[7rem_1fr] gap-x-4 gap-y-3">
          <dt className="text-muted-foreground">版本</dt><dd>{info.version}</dd>
          <dt className="text-muted-foreground">运行平台</dt><dd>{info.os} / {info.arch} · {info.goVersion}</dd>
          <dt className="text-muted-foreground">数据目录</dt><dd className="min-w-0 break-all">{info.dataDirectory}</dd>
          <dt className="text-muted-foreground">CODEX_HOME</dt><dd className="min-w-0 break-all">{info.codexHome}</dd>
        </dl>
        <Button variant="outline" disabled={busy} onClick={() => void run(() => Data.OpenDirectory(info.dataDirectory))}><FolderOpen />打开数据目录</Button>
      </CardContent>
    </Card>
  </div>;
}
