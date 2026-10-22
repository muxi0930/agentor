import { useCallback, useEffect, useRef, useState } from "react";
import { ListTodo, Moon, Radio, Settings2, Sun } from "lucide-react";
import { move, visit, type NavigationHistory } from "@/app/navigation";
import * as Data from "../bindings/agentor/internal/appdata/service";
import type { Info, Settings } from "../bindings/agentor/internal/appdata/models";
import * as Codex from "../bindings/agentor/internal/codex/service";
import type { Status } from "../bindings/agentor/internal/codex/models";
import { Events } from "@wailsio/runtime";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import { Sidebar, SidebarContent, SidebarFooter, SidebarHeader, SidebarInset, SidebarMenu, SidebarMenuButton, SidebarMenuItem, SidebarProvider, SidebarSeparator, SidebarTrigger } from "@/components/ui/sidebar";
import { SettingsPage } from "@/app/SettingsPage";
import { ChatPage } from "@/features/chat/ChatPage";
import { ApprovalCenter } from "@/features/approvals/ApprovalCenter";
import { WorkspacePage } from "@/features/workspace/WorkspacePage";
import { ControllerPage } from "@/features/controller/ControllerPage";
import * as Pipeline from "../bindings/agentor/internal/pipeline/service";
import { ModeSwitcher } from "@/app/ModeSwitcher";
import { LarkPage } from "@/features/lark/LarkPage";
import { SidebarAccount } from "@/features/accounts/SidebarAccount";
import { NotificationCenter, type NotificationTarget } from "@/features/notifications/NotificationCenter";
import * as Work from "../bindings/agentor/internal/workspace/service";
import { errorText } from "@/app/errors";

export default function App() {
  const [info, setInfo] = useState<Info>();
  const [settings, setSettings] = useState<Settings>();
  const [status, setStatus] = useState<Status>();
  const acceptStatus = useCallback((next: Status) => setStatus(current => current && (next.generation < current.generation || (next.generation === current.generation && next.revision < current.revision)) ? current : next), []);
  const [history, setHistory] = useState<NavigationHistory>({ entries: [], index: -1 });
  const navigating = useRef(false);
  const page = history.entries[history.index]?.page || "home";
  const [settingsTab, setSettingsTab] = useState("system");
  const [sidebarBody,setSidebarBody]=useState<HTMLDivElement|null>(null);
  const [headerBody,setHeaderBody]=useState<HTMLDivElement|null>(null);
  const [workspaceKey,setWorkspaceKey]=useState(0);
  const [focusExecution,setFocusExecution]=useState("");
  const [focusRequest,setFocusRequest]=useState("");
  const [focusThread, setFocusThread] = useState<{ threadId: string }>();
  const [focusPipeline,setFocusPipeline]=useState<NotificationTarget>();
  const [error, setError] = useState("");

  async function load() {
    setError("");
    try {
      const [nextInfo, nextSettings, nextStatus] = await Promise.all([Data.GetInfo(), Data.GetSettings(), Codex.GetStatus()]);
      setInfo(nextInfo); setSettings(nextSettings); acceptStatus(nextStatus);
      setHistory(current => current.entries.length ? current : visit(current, { page: "home", mode: nextSettings.mode }));
    } catch (error) { setError(errorText(error)); }
  }
  useEffect(() => { void load(); }, []);
  useEffect(() => Events.On("codex:status", event => acceptStatus(event.data)), [acceptStatus]);
  useEffect(() => {
    if (!settings) return;
    document.documentElement.classList.toggle("dark", settings.theme === "dark");
    document.documentElement.style.colorScheme = settings.theme;
  }, [settings?.theme]);

  async function navigate(targetPage: string, mode = settings?.mode, offset?: number) {
    if (!settings || !mode || navigating.current) return;
    const next = offset === undefined ? visit(history, { page: targetPage, mode }) : move(history, offset);
    if (next === history) return;
    navigating.current = true;
    try {
      const destination = next.entries[next.index];
      if (destination.mode !== settings.mode) setSettings(await Data.SetMode(destination.mode));
      setHistory(next);
    } catch (error) { setError(errorText(error)); }
    finally { navigating.current = false; }
  }

  function setPage(targetPage: string) { void navigate(targetPage); }

  useEffect(() => {
    function keydown(event: KeyboardEvent) {
      if (event.defaultPrevented || event.repeat || event.isComposing) return;
      const mac = /Mac|iPhone|iPad/.test(navigator.platform);
      let offset = 0;
      if (mac && event.metaKey && !event.ctrlKey && !event.altKey && !event.shiftKey) {
        if (event.code === "BracketLeft") offset = -1;
        if (event.code === "BracketRight") offset = 1;
      } else if (!mac && event.altKey && !event.ctrlKey && !event.metaKey && !event.shiftKey) {
        if (event.key === "ArrowLeft") offset = -1;
        if (event.key === "ArrowRight") offset = 1;
      }
      if (event.key === "BrowserBack") offset = -1;
      if (event.key === "BrowserForward") offset = 1;
      if (offset) { event.preventDefault(); void navigate("", undefined, offset); }
    }
    function mouseNavigation(event: MouseEvent) {
      if (event.button !== 3 && event.button !== 4) return;
      event.preventDefault();
      if (event.type === "mouseup") void navigate("", undefined, event.button === 3 ? -1 : 1);
    }
    window.addEventListener("keydown", keydown);
    window.addEventListener("mousedown", mouseNavigation);
    window.addEventListener("mouseup", mouseNavigation);
    window.addEventListener("auxclick", mouseNavigation);
    return () => {
      window.removeEventListener("keydown", keydown);
      window.removeEventListener("mousedown", mouseNavigation);
      window.removeEventListener("mouseup", mouseNavigation);
      window.removeEventListener("auxclick", mouseNavigation);
    };
  }, [history, settings]);

  function openSettings(tab = "system") {
    setSettingsTab(tab);
    setPage("settings");
  }

  async function changeMode(mode: string) {
    await navigate("home", mode);
  }
  async function openSource(target:NotificationTarget){
    if(target.pipelineRunId||target.pipelineSessionId){
      let projectId=target.projectId;
      if(!projectId){
        const pipelines=await Pipeline.GetState();
        projectId=pipelines.runs?.find(run=>run.id===target.pipelineRunId)?.projectId||pipelines.requests?.find(request=>request.id===target.pipelineSessionId)?.input.projectId;
      }
      if(projectId)await Work.Select(projectId);
      setFocusPipeline({...target,projectId});setWorkspaceKey(key=>key+1);await navigate("home", "workspace");
    }
    else if(target.stepId){setFocusPipeline(undefined);await Work.GetStepContext(target.stepId);await Work.Select(target.stepId);setFocusExecution(target.executionId||"");setWorkspaceKey(key=>key+1);await navigate("home", "workspace");}
    else if(target.requestId){setFocusRequest(target.requestId);await navigate("controller", "workspace");}
  }

  if (!settings || !info) return <main className="flex min-h-dvh items-center justify-center p-8">
    {error ? <Alert variant="destructive" className="max-w-lg"><AlertDescription>{error}<Button variant="outline" onClick={() => void load()}>重试</Button></AlertDescription></Alert>
      : <div role="status" aria-label="正在加载应用" className="w-72 space-y-4"><Skeleton className="h-8 w-32" /><Skeleton className="h-4" /><Skeleton className="h-4 w-3/4" /></div>}
  </main>;

  return <SidebarProvider className="h-dvh min-h-0">
    <Sidebar collapsible="icon" resizable>
      <SidebarHeader className="h-16 shrink-0 justify-center border-b border-sidebar-border px-3 py-0 group-data-[collapsible=icon]:px-2"><ModeSwitcher mode={settings.mode} onChange={mode=>void changeMode(mode)}/></SidebarHeader>
      <SidebarContent><div ref={setSidebarBody} className="flex min-h-0 flex-1 flex-col overflow-hidden group-data-[collapsible=icon]:hidden"/></SidebarContent>
      <SidebarFooter className="p-3 group-data-[collapsible=icon]:p-2"><SidebarMenu>
        <SidebarMenuItem><SidebarMenuButton tooltip="控制中心" isActive={page==="controller"} onClick={()=>setPage("controller")}><ListTodo/><span>控制中心</span></SidebarMenuButton></SidebarMenuItem>
        <SidebarMenuItem><SidebarMenuButton tooltip="飞书连接" isActive={page==="lark"} onClick={()=>setPage("lark")}><Radio/><span>飞书连接</span></SidebarMenuButton></SidebarMenuItem>
        <SidebarMenuItem><SidebarMenuButton tooltip="设置" isActive={page === "settings"} onClick={() => openSettings()}><Settings2 /><span>设置</span></SidebarMenuButton></SidebarMenuItem>

      </SidebarMenu><SidebarSeparator className="mx-0"/><SidebarAccount settings={settings} status={status} onSettings={setSettings} onStatus={acceptStatus} onError={setError} /></SidebarFooter>
    </Sidebar>
    <SidebarInset className="min-w-0 overflow-hidden">
      <header className="flex h-16 shrink-0 items-center gap-3 border-b px-5">
        <div className="flex shrink-0 items-center gap-1">
        </div>
        <SidebarTrigger /><span className="shrink-0 text-sm font-medium">{page === "settings" ? "设置" : page==="controller"?"控制中心":page==="lark"?"飞书连接":settings.mode === "chat" ? "会话" : "工作区"}</span>
        <div ref={setHeaderBody} className={page==="home"&&settings.mode==="workspace"?"min-w-0 flex-1 overflow-x-auto":"hidden"}/>
        <div className="ml-auto flex shrink-0 items-center gap-3"><span role="status" aria-label={status?.phase === "ready" ? "app-server 已连接" : "app-server 未连接"} className="inline-flex items-center gap-2 whitespace-nowrap text-xs text-muted-foreground"><span aria-hidden="true" className={`size-2 shrink-0 rounded-full ${status?.phase === "ready" ? "bg-emerald-500" : "bg-orange-500"}`}/>{status?.phase === "ready" ? "app-server" : "app-server 未连接"}</span><Button variant="ghost" size="sm" aria-label={settings.theme === "dark" ? "切换到日间模式" : "切换到夜间模式"} onClick={() => void Data.SetTheme(settings.theme === "dark" ? "light" : "dark").then(setSettings).catch(error => setError(errorText(error)))}>{settings.theme === "dark" ? <Moon aria-hidden="true"/> : <Sun aria-hidden="true"/>}{settings.theme === "dark" ? "夜间" : "日间"}</Button><ApprovalCenter status={status} /><NotificationCenter onOpenSource={openSource}/></div>
      </header>
      {error && <div className="p-4"><Alert variant="destructive"><AlertDescription className="flex justify-between">{error}<Button variant="ghost" size="sm" onClick={() => setError("")}>关闭</Button></AlertDescription></Alert></div>}
      <main className="relative flex min-h-0 flex-1 overflow-hidden">
        {page === "settings" && <div className="size-full overflow-y-auto"><SettingsPage initialTab={settingsTab} info={info} settings={settings} onSettings={setSettings} status={status} onStatus={acceptStatus} onOpenThread={threadId => { void navigate("home", "chat").then(() => setFocusThread({ threadId })); }} /></div>}
        {page==="controller"&&<div className="size-full overflow-y-auto"><ControllerPage openSettings={() => openSettings("controller")} settings={settings} onSettings={setSettings} status={status} focusRequestId={focusRequest} onOpenThread={threadId => { void navigate("home", "chat").then(() => setFocusThread({ threadId })); }}/></div>}
        {page==="lark"&&<div className="size-full overflow-y-auto"><LarkPage onOpenRequest={requestId=>void openSource({requestId}).catch(error=>setError(errorText(error)))}/></div>}
        <div className={page === "home" && settings.mode === "chat" ? "flex size-full min-h-0" : "hidden"}>
          <ChatPage settings={settings} onSettings={setSettings} status={status} openSettings={() => openSettings()} sidebarContainer={sidebarBody} onActivate={()=>setPage("home")} focusThread={focusThread} />
        </div>
        <div className={page === "home" && settings.mode === "workspace" ? "flex size-full min-h-0" : "hidden"}>{settings.mode==="workspace"&&<WorkspacePage key={workspaceKey} active={page==="home"} settings={settings} status={status} focusExecutionId={focusExecution} focusPipeline={focusPipeline} headerContainer={headerBody} sidebarContainer={sidebarBody} onActivate={()=>setPage("home")} onOpenThread={threadId => { void navigate("home", "chat").then(() => setFocusThread({ threadId })); }}/>}</div>
      </main>
    </SidebarInset>
  </SidebarProvider>;
}
