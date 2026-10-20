import { useCallback, useEffect, useState } from "react";
import { Command, FolderOpen, Layers3, MessageSquare, Moon, Settings2, Sun, Unplug } from "lucide-react";
import * as Data from "../bindings/agentor/internal/appdata/service";
import type { Info, Settings } from "../bindings/agentor/internal/appdata/models";
import * as Codex from "../bindings/agentor/internal/codex/service";
import type { Status } from "../bindings/agentor/internal/codex/models";
import { Events } from "@wailsio/runtime";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { Sidebar, SidebarContent, SidebarFooter, SidebarGroup, SidebarGroupLabel, SidebarHeader, SidebarInset, SidebarMenu, SidebarMenuButton, SidebarMenuItem, SidebarProvider, SidebarSeparator, SidebarTrigger } from "@/components/ui/sidebar";
import { SettingsPage } from "@/app/SettingsPage";
import { ChatPage } from "@/features/chat/ChatPage";
import { ApprovalCenter } from "@/features/approvals/ApprovalCenter";
import { errorText } from "@/app/errors";

export default function App() {
  const [info, setInfo] = useState<Info>();
  const [settings, setSettings] = useState<Settings>();
  const [status, setStatus] = useState<Status>();
  const acceptStatus = useCallback((next: Status) => setStatus(current => current && (next.generation < current.generation || (next.generation === current.generation && next.revision < current.revision)) ? current : next), []);
  const [page, setPage] = useState("home");
  const [error, setError] = useState("");

  async function load() {
    setError("");
    try {
      const [nextInfo, nextSettings, nextStatus] = await Promise.all([Data.GetInfo(), Data.GetSettings(), Codex.GetStatus()]);
      setInfo(nextInfo); setSettings(nextSettings); acceptStatus(nextStatus);
    } catch (error) { setError(errorText(error)); }
  }
  useEffect(() => { void load(); }, []);
  useEffect(() => Events.On("codex:status", event => acceptStatus(event.data)), [acceptStatus]);
  useEffect(() => {
    if (!settings) return;
    document.documentElement.classList.toggle("dark", settings.theme === "dark");
    document.documentElement.style.colorScheme = settings.theme;
  }, [settings?.theme]);

  async function changeMode(mode: string) {
    try { setSettings(await Data.SetMode(mode)); setPage("home"); }
    catch (error) { setError(errorText(error)); }
  }
  async function chooseDirectory() {
    try {
      const directory = await Data.ChooseDirectory();
      if (directory) setSettings(await Data.SetProjectDirectory(directory));
    } catch (error) { setError(errorText(error)); }
  }

  if (!settings || !info) return <main className="flex min-h-dvh items-center justify-center p-8">
    {error ? <Alert variant="destructive" className="max-w-lg"><AlertDescription>{error}<Button variant="outline" onClick={() => void load()}>重试</Button></AlertDescription></Alert>
      : <div role="status" aria-label="正在加载应用" className="w-72 space-y-4"><Skeleton className="h-8 w-32" /><Skeleton className="h-4" /><Skeleton className="h-4 w-3/4" /></div>}
  </main>;

  return <SidebarProvider className="h-dvh min-h-0">
    <Sidebar collapsible="icon">
      <SidebarHeader className="px-4 py-5"><div className="flex items-center gap-3 overflow-hidden">
        <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground"><Command className="size-4" /></div>
        <div className="group-data-[collapsible=icon]:hidden"><p className="text-sm font-semibold tracking-tight">agentor</p><p className="text-xs text-muted-foreground">让工作有序发生</p></div>
      </div></SidebarHeader>
      <SidebarContent>
        <SidebarGroup><SidebarGroupLabel>工作模式</SidebarGroupLabel><SidebarMenu>
          <SidebarMenuItem><SidebarMenuButton tooltip="Codex 对话" isActive={settings.mode === "chat" && page === "home"} onClick={() => void changeMode("chat")}><MessageSquare /><span>Codex 对话</span></SidebarMenuButton></SidebarMenuItem>
          <SidebarMenuItem><SidebarMenuButton tooltip="工作区" isActive={settings.mode === "workspace" && page === "home"} onClick={() => void changeMode("workspace")}><Layers3 /><span>工作区</span></SidebarMenuButton></SidebarMenuItem>
        </SidebarMenu></SidebarGroup>
        <SidebarSeparator />
        <SidebarGroup><SidebarGroupLabel>项目</SidebarGroupLabel><SidebarMenu>
          <SidebarMenuItem><SidebarMenuButton tooltip="选择项目目录" onClick={() => void chooseDirectory()}><FolderOpen /><span className="truncate">{settings.projectAliases?.[settings.projectDirectory] || settings.projectDirectory.split(/[\\/]/).filter(Boolean).slice(-1)[0] || "选择项目目录"}</span></SidebarMenuButton></SidebarMenuItem>
        </SidebarMenu></SidebarGroup>
      </SidebarContent>
      <SidebarFooter className="p-3"><SidebarMenu>
        <SidebarMenuItem><SidebarMenuButton tooltip="设置" isActive={page === "settings"} onClick={() => setPage("settings")}><Settings2 /><span>设置</span></SidebarMenuButton></SidebarMenuItem>
        <SidebarMenuItem><SidebarMenuButton tooltip={settings.theme === "dark" ? "切换浅色主题" : "切换深色主题"} onClick={() => void Data.SetTheme(settings.theme === "dark" ? "light" : "dark").then(setSettings).catch(error => setError(errorText(error)))}>
          {settings.theme === "dark" ? <Sun /> : <Moon />}<span>{settings.theme === "dark" ? "浅色主题" : "深色主题"}</span>
        </SidebarMenuButton></SidebarMenuItem>
      </SidebarMenu><p className="px-2 pt-3 text-[11px] text-muted-foreground group-data-[collapsible=icon]:hidden">本地工作空间 · v{info.version}</p></SidebarFooter>
    </Sidebar>
    <SidebarInset className="min-w-0 overflow-hidden">
      <header className="flex h-16 shrink-0 items-center gap-3 border-b px-5">
        <SidebarTrigger /><span className="text-sm font-medium">{page === "settings" ? "设置" : settings.mode === "chat" ? "Codex 对话" : "工作区"}</span>
        <div className="ml-auto flex items-center gap-3"><ApprovalCenter status={status} /><Badge variant="outline" className="gap-1.5 text-muted-foreground"><Unplug className="size-3" />{status?.phase === "ready" ? "Codex 已连接" : status?.phase === "starting" ? "正在连接" : status?.phase === "error" ? "连接失败" : "尚未连接"}</Badge></div>
      </header>
      {error && <div className="p-4"><Alert variant="destructive"><AlertDescription className="flex justify-between">{error}<Button variant="ghost" size="sm" onClick={() => setError("")}>关闭</Button></AlertDescription></Alert></div>}
      <main className="relative flex min-h-0 flex-1 overflow-hidden">
        {page === "settings" && <div className="size-full overflow-y-auto"><SettingsPage info={info} settings={settings} onSettings={setSettings} status={status} onStatus={acceptStatus} /></div>}
        <div className={page === "home" && settings.mode === "chat" ? "flex size-full min-h-0" : "hidden"}>
          <ChatPage settings={settings} onSettings={setSettings} status={status} openSettings={() => setPage("settings")} />
        </div>
        {page === "home" && settings.mode === "workspace" && <div className="m-auto w-full max-w-2xl p-10"><Empty><EmptyHeader><EmptyMedia variant="icon"><Layers3 /></EmptyMedia><EmptyTitle>工作区尚未建立</EmptyTitle><EmptyDescription>项目、版本、功能和步骤将在工作区阶段接入。您现在可以选择项目目录。</EmptyDescription></EmptyHeader><EmptyContent><Button variant="outline" onClick={() => void chooseDirectory()}><FolderOpen />选择项目目录</Button></EmptyContent></Empty></div>}
      </main>
    </SidebarInset>
  </SidebarProvider>;
}
