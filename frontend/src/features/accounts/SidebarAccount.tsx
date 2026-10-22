import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { useEffect, useState } from "react";
import { ChevronsUpDown, LoaderCircle } from "lucide-react";
import { Events } from "@wailsio/runtime";
import * as Codex from "../../../bindings/agentor/internal/codex/service";
import * as Data from "../../../bindings/agentor/internal/appdata/service";
import type { Settings } from "../../../bindings/agentor/internal/appdata/models";
import type { AccountInfo, AccountsView, RateLimits, RateSnapshot, RateWindow, Status } from "../../../bindings/agentor/internal/codex/models";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuLabel, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem } from "@/components/ui/sidebar";
import { errorText } from "@/app/errors";

function quotaSnapshot(rates?: RateLimits): RateSnapshot | undefined {
  const entries = Object.entries(rates?.rateLimitsByLimitId || {}).filter((entry): entry is [string, RateSnapshot] => !!entry[1]);
  const codex = entries.find(([key, value]) => key.toLowerCase() === "codex" || value.limitId?.toLowerCase() === "codex");
  if (codex) return codex[1];
  if (entries.length === 1) return entries[0][1];
  if (entries.length > 1) return undefined;
  return rates?.rateLimits || undefined;
}

function quotaText(window: RateWindow): string {
  const minutes = window.windowDurationMins!;
  const duration = minutes === 10080 ? "周" : minutes % 10080 === 0 ? `${minutes / 10080}周` : minutes % 1440 === 0 ? `${minutes / 1440}天` : minutes % 60 === 0 ? `${minutes / 60}小时` : `${minutes}分钟`;
  return `${duration} ${Number(window.usedPercent.toFixed(1))}%`;
}

export function SidebarAccount({ settings, status, onSettings, onStatus, onError }: {
  settings: Settings;
  status?: Status;
  onSettings: (settings: Settings) => void;
  onStatus: (status: Status) => void;
  onError: (error: string) => void;
}) {
  const [view, setView] = useState<AccountsView>();
  const [account, setAccount] = useState<AccountInfo>();
  const [rates, setRates] = useState<RateLimits>();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [quotaError, setQuotaError] = useState("");

  useEffect(() => {
    setAccount(undefined);
    setRates(undefined);
    setQuotaError("");
  }, [settings.activeAccountId, settings.reuseCLI, status?.phase, status?.generation]);

  useEffect(() => {
    let cancelled = false;
    let revision = 0;
    async function refresh() {
      const current = ++revision;
      try {
        const nextView = await Codex.GetAccounts();
        if (cancelled || current !== revision) return;
        setView(nextView);
        setLoadError("");
        if (status?.phase !== "ready") return;
        const nextAccount = await Codex.GetAccount();
        if (cancelled || current !== revision) return;
        setAccount(nextAccount);
        if (nextAccount.account?.type !== "chatgpt") { setRates(undefined); return; }
        try {
          const nextRates = await Codex.GetRateLimits();
          if (!cancelled && current === revision) { setRates(nextRates); setQuotaError(""); }
        } catch (error) {
          if (!cancelled && current === revision) { setRates(undefined); setQuotaError(errorText(error)); }
        }
      } catch (error) {
        if (!cancelled && current === revision) setLoadError(errorText(error));
      }
    }
    void refresh();
    const removeListener = Events.On("codex:event", event => {
      if (event.data.generation === status?.generation && (event.data.method.startsWith("account/") || event.data.method === "host/loginUpdated")) void refresh();
    });
    return () => { cancelled = true; removeListener(); };
  }, [settings.activeAccountId, settings.reuseCLI, status?.phase, status?.generation, open]);

  async function switchAccount(id: string) {
    if (busy || (settings.reuseCLI ? id === "" : id === settings.activeAccountId)) return;
    setBusy(true);
    try {
      onStatus(await Codex.UseAccount(id));
      onSettings(await Data.GetSettings());
      setOpen(false);
    } catch (error) { onError(errorText(error)); }
    finally { setBusy(false); }
  }

  const active = settings.reuseCLI ? undefined : view?.profiles?.find(profile => profile.id === settings.activeAccountId);
  const identity = account?.account?.email || (active ? [active.name, active.type === "api" ? active.provider : ""].filter(Boolean).join(" · ") : "目录现有配置");
  const snapshot = quotaSnapshot(rates);
  const windows = [snapshot?.primary, snapshot?.secondary].filter((window): window is RateWindow => !!window && Number.isFinite(window.usedPercent) && window.windowDurationMins != null && Number.isFinite(window.windowDurationMins) && window.windowDurationMins > 0).sort((a, b) => a.windowDurationMins! - b.windowDurationMins!);
  const summary = windows.map(quotaText).join(" · ") || "额度未知";
  const connectionDetail = status?.phase === "starting" ? "正在连接…" : status?.phase === "error" ? status.message || "连接失败" : "";

  return <SidebarMenu><SidebarMenuItem>
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger render={<SidebarMenuButton size="lg" tooltip={`${identity} · ${summary}`} aria-label={`切换账号：${identity}，${summary}`} className="h-auto min-h-12 group-data-[collapsible=icon]:min-h-8 data-open:bg-sidebar-accent data-open:text-sidebar-accent-foreground" />}>
        <div className="flex w-full min-w-0 items-center gap-2 text-left">
          <Avatar className="rounded-lg"><AvatarFallback className="rounded-lg">{Array.from(identity)[0]?.toUpperCase() || "A"}</AvatarFallback></Avatar>
          <div className="grid min-w-0 flex-1 gap-1 leading-tight group-data-[collapsible=icon]:hidden">
            <Tooltip><TooltipTrigger render={<span className="truncate text-sm font-medium">{identity}</span>} /><TooltipContent>{identity}</TooltipContent></Tooltip>
            <Tooltip><TooltipTrigger render={<span className="truncate text-xs text-muted-foreground">{summary}</span>} /><TooltipContent>{summary}</TooltipContent></Tooltip>
          </div>
          {busy || status?.phase === "starting" ? <LoaderCircle aria-label="正在切换账号" className="size-4 animate-spin group-data-[collapsible=icon]:hidden" /> : <ChevronsUpDown aria-hidden="true" className="size-4 group-data-[collapsible=icon]:hidden" />}
        </div>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="top" align="end" sideOffset={8} className="w-72 max-w-[calc(100vw-1rem)]">
        <DropdownMenuGroup>
          <DropdownMenuLabel className="space-y-1 px-2 py-2 font-normal">
            <Tooltip><TooltipTrigger render={<span className="block truncate text-sm font-medium text-foreground">{identity}</span>} /><TooltipContent>{identity}</TooltipContent></Tooltip>
            <span className="block text-xs">{summary}</span>
            {connectionDetail && <span className="block break-words text-xs">{connectionDetail}</span>}
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuLabel>切换账号</DropdownMenuLabel>
          <DropdownMenuRadioGroup value={settings.reuseCLI ? "" : settings.activeAccountId} onValueChange={id => void switchAccount(String(id))} disabled={busy || status?.phase === "starting"}>
            <DropdownMenuRadioItem value=""><span className="min-w-0"><span className="block">目录现有配置</span><span className="block text-xs text-muted-foreground">沿用现有登录和连接配置</span></span></DropdownMenuRadioItem>
            {view?.profiles?.map(profile => <DropdownMenuRadioItem key={profile.id} value={profile.id} label={profile.name}>
              <Avatar className="size-7 rounded-md"><AvatarFallback className="rounded-md text-xs">{Array.from(profile.name)[0]?.toUpperCase() || "A"}</AvatarFallback></Avatar>
              <span className="min-w-0 flex-1"><Tooltip><TooltipTrigger render={<span className="block truncate">{profile.name}</span>} /><TooltipContent>{profile.name}</TooltipContent></Tooltip><span className="block truncate text-xs text-muted-foreground">{profile.type === "chatgpt" ? "ChatGPT" : profile.provider || "API"}</span></span>
            </DropdownMenuRadioItem>)}
          </DropdownMenuRadioGroup>
        </DropdownMenuGroup>
        {(loadError || quotaError) && <><DropdownMenuSeparator /><p role="status" className="break-words px-2 py-1 text-xs text-destructive">{loadError || `额度读取失败：${quotaError}`}</p></>}
      </DropdownMenuContent>
    </DropdownMenu>
  </SidebarMenuItem></SidebarMenu>;
}
