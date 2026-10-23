import { Page, PageContent, PageHeader } from "@/components/ui/page";
import { useEffect, useState } from "react";
import { FolderOpen, FileSearch, Save } from "lucide-react";
import * as Data from "../../bindings/agentor/internal/appdata/service";
import type { Info, Settings } from "../../bindings/agentor/internal/appdata/model/models";
import * as Codex from "../../bindings/agentor/internal/codex/service";
import type { Model, Status } from "../../bindings/agentor/internal/codex/model/models";
import { RuntimeConnection } from "@/features/accounts/RuntimeConnection";
import { AccountSettings } from "@/features/accounts/AccountSettings";
import { PermissionSelect } from "@/features/accounts/PermissionSelect";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel, FieldGroup } from "@/components/ui/field";
import { Accordion } from "@/components/ui/accordion";
import { SettingsField, SettingsSection } from "@/components/ui/settings-section";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { HelpText } from "@/components/ui/help-text";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { errorText } from "./errors";
import { ControllerConfiguration } from "@/features/controller/ControllerConfiguration";
import { ExitApplication } from "./ExitApplication";
import { AgentsMonitor } from "./AgentsMonitor";

export function SettingsPage({ info, settings, onSettings, status, onStatus, onOpenThread, initialTab = "system" }: {
  info: Info; settings: Settings; onSettings: (settings: Settings) => void; status?: Status; onStatus: (status: Status) => void; onOpenThread: (threadId: string) => void; initialTab?: string;
}) {
  const [tab, setTab] = useState(initialTab);
  const [models, setModels] = useState<Model[]>([]);
  const [systemSections, setSystemSections] = useState<string[]>(["information"]);
  const [agentSections, setAgentSections] = useState<string[]>(["queue", "timeout"]);
  const [path, setPath] = useState(settings.codexPath);
  const [maxConcurrentAgents, setMaxConcurrentAgents] = useState(String(settings.maxConcurrentAgents));
  const [agentTimeoutMinutes, setAgentTimeoutMinutes] = useState(String(settings.agentTimeoutSeconds / 60));
  const [dataDirectory, setDataDirectory] = useState(info.dataDirectory);
  const [homeMode, setHomeMode] = useState(settings.codexHomeMode);
  const [homePath, setHomePath] = useState(settings.codexHomePath);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    if (tab !== "controller" || status?.phase !== "ready") return;
    let cancelled = false;
    void Codex.GetModels().then(value => { if (!cancelled) setModels(value || []); }).catch(error => { if (!cancelled) setError(errorText(error)); });
    return () => { cancelled = true; };
  }, [tab, status?.generation, status?.phase]);

  async function run(action: () => Promise<void>) {
    setBusy(true); setError(""); setMessage("");
    try { await action(); } catch (error) { setError(errorText(error)); }
    finally { setBusy(false); }
  }

  return <Page>
    <Tabs value={tab} onValueChange={value => {
      setTab(String(value));
      if (value === "system") setSystemSections(["information"]);
      else if (value === "agents") setAgentSections(["queue", "timeout"]);
    }} className="gap-6">
      <PageHeader title="设置" description="连接您的工具，安排舒适的工作环境。"><TabsList aria-label="设置分组">
        <TabsTrigger value="system">系统信息</TabsTrigger>
        <TabsTrigger value="codex">Codex配置</TabsTrigger>
        <TabsTrigger value="session">会话配置</TabsTrigger>
        <TabsTrigger value="agents">子代理配置</TabsTrigger>
        <TabsTrigger value="controller">控制中心配置</TabsTrigger>
      </TabsList></PageHeader>
      <PageContent>
    {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
    {message && <p role="status" className="text-sm text-muted-foreground">{message}</p>}

      <TabsContent value="system" keepMounted>
        <Accordion multiple value={systemSections} onValueChange={setSystemSections} className="gap-4">
    <SettingsSection value="information" title="应用信息" description="查看应用版本、运行平台与目录。">
      <div className="space-y-4 text-sm">
        <dl className="grid grid-cols-[7rem_1fr] gap-x-4 gap-y-3">
          <dt className="text-muted-foreground">版本</dt><dd>{info.version}</dd>
          <dt className="text-muted-foreground">运行平台</dt><dd>{info.os} / {info.arch} · {info.goVersion}</dd>
          <dt className="text-muted-foreground">全局启动配置</dt><dd className="min-w-0 break-all">{info.globalConfigPath}</dd>
          <dt className="text-muted-foreground">软件配置目录</dt><dd className="min-w-0 break-all">{info.dataDirectory}</dd>
          <dt className="text-muted-foreground">CODEX_HOME</dt><dd className="min-w-0 break-all">{status?.home || info.codexHome}</dd>
        </dl>
        <div className="flex justify-end"><Button variant="outline" disabled={busy} onClick={() => void run(() => Data.OpenDirectory(info.dataDirectory))}><FolderOpen />打开软件配置目录</Button></div>
      </div>
    </SettingsSection>
    <SettingsSection value="data-directory" title="应用数据根目录" description="保存设置、账号、任务和附件的位置。切换目录需要确认并完整重启，不会自动迁移现有数据。">
      <div className="space-y-4">
        <Field><FieldLabel htmlFor="data-directory">数据目录</FieldLabel><div className="flex gap-2">
          <Input id="data-directory" value={dataDirectory} disabled={busy || info.dataDirectoryFromEnv} onChange={event => setDataDirectory(event.target.value)} />
          <Button variant="outline" disabled={busy || info.dataDirectoryFromEnv} onClick={() => void run(async () => { const directory = await Data.ChooseDirectory(); if (directory) setDataDirectory(directory); })}>选择目录</Button>
        </div></Field>
        {info.dataDirectoryFromEnv && <p className="text-sm text-muted-foreground">当前目录由 AGENTOR_DATA_DIR 环境变量指定，优先于全局配置。请移除该变量并重启后再切换目录。</p>}
        {!info.dataDirectoryFromEnv && dataDirectory.trim() !== info.dataDirectory && <ExitApplication dataDirectory={{ current: info.dataDirectory, target: dataDirectory.trim() }} />}
      </div>
    </SettingsSection>
    <SettingsSection value="runtime" title="app-server" description="查看 Codex 运行时状态与连接信息。">
      <RuntimeConnection status={status} onStatus={onStatus} />
    </SettingsSection>
    {tab === "system" && <AgentsMonitor limit={settings.maxConcurrentAgents} onOpenThread={onOpenThread} />}
        </Accordion>
      </TabsContent>
      <TabsContent value="agents" keepMounted>
        <Accordion multiple value={agentSections} onValueChange={setAgentSections} className="gap-4">
    <SettingsSection value="queue" title="工作队列" description="仅限制功能任务的并发数。保存后立即生效，已运行任务继续执行。">
      <form onSubmit={event => { event.preventDefault(); void run(async () => {
        onSettings(await Data.SetMaxConcurrentAgents(Number(maxConcurrentAgents))); setMessage("工作队列并发上限已保存");
      }); }}><FieldGroup>
        <SettingsField><FieldLabel htmlFor="max-concurrent-agents">最大并发 agent 数</FieldLabel>
          <div className="flex flex-wrap items-center gap-2"><Input id="max-concurrent-agents" className="w-32" type="number" min={1} max={50} step={1} required value={maxConcurrentAgents} onChange={event => setMaxConcurrentAgents(event.target.value)} />
            <Button type="submit" variant="outline" disabled={busy}><Save />保存</Button>
          </div>
        </SettingsField>
      </FieldGroup></form>
    </SettingsSection>
    <SettingsSection value="timeout" title="最大执行时间" description="仅影响保存后新启动的功能任务，超时后中断对应 session 并取消任务。0 表示不限时。">
      <form onSubmit={event => { event.preventDefault(); void run(async () => {
        onSettings(await Data.SetAgentTimeoutSeconds(Number(agentTimeoutMinutes) * 60)); setMessage("子代理最大执行时间已保存");
      }); }}><FieldGroup>
        <SettingsField><FieldLabel htmlFor="agent-timeout-minutes">执行时间（分钟）</FieldLabel>
          <div className="flex flex-wrap items-center gap-2"><Input id="agent-timeout-minutes" className="w-32" type="number" min={0} step={1} required value={agentTimeoutMinutes} onChange={event => setAgentTimeoutMinutes(event.target.value)} />
            <Button type="submit" variant="outline" disabled={busy}><Save />保存</Button>
          </div>
        </SettingsField>
      </FieldGroup></form>
    </SettingsSection>
        </Accordion>
      </TabsContent>
      <TabsContent value="session" keepMounted>
        <Accordion key="session-settings" multiple defaultValue={["layout", "permissions"]} className="gap-4">
          <SettingsSection value="layout" title="聊天页宽度" description="更宽布局将聊天正文区域加宽 60%，输入框宽度保持不变。">
            <FieldGroup>
              <SettingsField><FieldLabel htmlFor="chat-width">消息区域宽度</FieldLabel><Select disabled={busy} value={settings.chatWidth || "standard"} onValueChange={value => { if (value) void run(async () => onSettings(await Data.SetChatWidth(value))); }} items={[{ value: "standard", label: "标准" }, { value: "wide", label: "更宽（+60%）" }]}><SelectTrigger id="chat-width"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="standard">标准</SelectItem><SelectItem value="wide">更宽（+60%）</SelectItem></SelectContent></Select></SettingsField>
            </FieldGroup>
          </SettingsSection>
          <SettingsSection value="permissions" title="执行权限" description="设置会话默认权限，项目和会话可分别覆盖。">
            <FieldGroup>
              <SettingsField><FieldLabel htmlFor="default-permission"><HelpText help="会话设置优先于项目设置，再继承此默认值。只读可查看文件；项目内读写可修改当前项目；完全访问允许项目外文件与网络操作。">默认执行权限</HelpText></FieldLabel><PermissionSelect id="default-permission" className="text-sm" disabled={busy} value={settings.permissionMode} onChange={mode => void run(async () => onSettings(await Data.SetPermission("default", "", mode)))} /></SettingsField>
            </FieldGroup>
          </SettingsSection>
        </Accordion>
      </TabsContent>
      <TabsContent value="codex" keepMounted>
        <Accordion key="codex-settings" multiple defaultValue={["executable", "home", "accounts"]} className="gap-4">
    <SettingsSection value="executable" title="可执行路径" description="指定原生可执行文件；留空时从 PATH 查找。">
      <div><form onSubmit={event => { event.preventDefault(); void run(async () => {
        onSettings(await Data.SetCodexPath(path.trim())); setMessage("Codex 路径已保存");
      }); }}><FieldGroup>
        <SettingsField><FieldLabel htmlFor="codex-path">可执行文件路径</FieldLabel>
          <div className="flex flex-wrap items-center gap-2"><Input className="min-w-32 flex-1" id="codex-path" value={path} placeholder="留空使用默认路径" onChange={event => setPath(event.target.value)} />
            <Button variant="outline" type="button" aria-label="选择 Codex 程序" disabled={busy} onClick={() => void run(async () => {
              const file = await Data.ChooseFile(); if (file) setPath(file);
            })}><FileSearch /></Button>
            <Button type="button" variant="outline" disabled={busy} onClick={() => void run(async () => {
              onSettings(await Data.SetCodexPath("")); setPath(""); setMessage("已使用默认路径");
            })}>使用默认路径</Button>
            <Button type="submit" variant="outline" disabled={busy}><Save />保存路径</Button>
          </div>
        </SettingsField>
      </FieldGroup></form></div>
    </SettingsSection>
    <SettingsSection value="home" title="数据目录" description="目录与账号分别选择。切换目录需要确认并完全重启应用。">
      <div><FieldGroup>
        <SettingsField><FieldLabel htmlFor="codex-home-mode">Home 路径</FieldLabel>
          <Select value={homeMode} disabled={busy} onValueChange={selectedValue => { if (selectedValue === null) return; setHomeMode(selectedValue); }} items={[{ value: "system", label: "共用系统默认 CODEX_HOME" }, { value: "custom", label: "自定义 CODEX_HOME" }]}><SelectTrigger id="codex-home-mode"><SelectValue /></SelectTrigger><SelectContent>
            <SelectItem value="system">共用系统默认 CODEX_HOME</SelectItem>
            <SelectItem value="custom">自定义 CODEX_HOME</SelectItem>
          </SelectContent></Select>
        </SettingsField>
        {homeMode === "custom" && <SettingsField><FieldLabel htmlFor="codex-home-path">自定义目录</FieldLabel>
          <div className="flex gap-2"><Input id="codex-home-path" value={homePath} placeholder="选择或输入绝对路径" onChange={event => setHomePath(event.target.value)} />
            <Button type="button" variant="outline" aria-label="选择 CODEX_HOME 目录" disabled={busy} onClick={() => void run(async () => {
              const directory = await Data.ChooseDirectory(); if (directory) setHomePath(directory);
            })}><FolderOpen /></Button></div>
        </SettingsField>}
        <Field orientation="horizontal">
          <Checkbox id="read-external-threads" checked={settings.readExternalThreads} disabled={busy} onCheckedChange={enabled => void run(async () => {
            onSettings(await Data.SetReadExternalThreads(enabled));
          })} />
          <div className="space-y-1"><FieldLabel htmlFor="read-external-threads"><HelpText help="包含所选 CODEX_HOME 中其他应用创建的会话，保存后立即生效，无需重启。">读取非本应用创建的会话</HelpText></FieldLabel></div>
        </Field>
        {(homeMode !== settings.codexHomeMode || homeMode === "custom" && homePath.trim() !== settings.codexHomePath) && <ExitApplication restart={{ mode: homeMode, directory: homePath.trim(), currentHome: status?.home || info.codexHome, targetHome: homeMode === "system" ? info.systemCodexHome : homePath.trim() }} />}
      </FieldGroup></div>
    </SettingsSection>
          <AccountSettings status={status} onStatus={onStatus} onSettings={onSettings}/>
        </Accordion>
      </TabsContent>
      <TabsContent value="controller" keepMounted>
        <ControllerConfiguration settings={settings} models={models} busy={busy} run={run} onSettings={onSettings} />
      </TabsContent>
      </PageContent>
    </Tabs>
    <PageContent><ExitApplication />
    <p className="text-right text-xs text-muted-foreground">version {info.version}</p></PageContent>
  </Page>;
}
