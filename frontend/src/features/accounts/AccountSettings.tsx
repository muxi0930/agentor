import { HelpText } from "@/components/ui/help-text";
import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import * as Codex from "../../../bindings/agentor/internal/codex/service";
import * as Data from "../../../bindings/agentor/internal/appdata/service";
import type { Settings } from "../../../bindings/agentor/internal/appdata/models";
import type { AccountProfile, AccountsView, BalanceConfig, BalanceResult, ProfileInput, RateLimits, RateSnapshot, Status } from "../../../bindings/agentor/internal/codex/models";
import { Button } from "@/components/ui/button";
import { ButtonGroup } from "@/components/ui/button-group";
import { Badge } from "@/components/ui/badge";
import { SettingsField, SettingsSection } from "@/components/ui/settings-section";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Progress } from "@/components/ui/progress";
import { errorText } from "@/app/errors";

const emptyBalance: BalanceConfig = {url:"",method:"GET",headerName:"Authorization",headerPrefix:"Bearer ",body:"",balancePath:"",usedPath:"",totalPath:"",unitPath:"",unit:""};
function draftFor(profile?: AccountProfile, type="chatgpt"): ProfileInput {
  return {id:profile?.id||"",name:profile?.name||"",type:profile?.type||type,provider:profile?.provider||"",baseURL:profile?.baseURL||"https://api.openai.com/v1",apiKey:"",model:profile?.model||"",balance:profile?.balance.url?profile.balance:{...emptyBalance}};
}

export function AccountSettings({status,onStatus,onSettings}:{status?:Status;onStatus:(status:Status)=>void;onSettings:(settings:Settings)=>void}) {
  const [view,setView]=useState<AccountsView>(),[draft,setDraft]=useState<ProfileInput>(),[removing,setRemoving]=useState<AccountProfile>();
  const [busy,setBusy]=useState(false),[error,setError]=useState(""),[message,setMessage]=useState("");
  const [rates,setRates]=useState<RateLimits>(),[balances,setBalances]=useState<Record<string,BalanceResult>>({});
  async function refresh(){setView(await Codex.GetAccounts());}
  useEffect(()=>{void refresh().catch(error=>setError(errorText(error)));setRates(undefined);},[status?.generation]);
  useEffect(()=>Events.On("codex:event",event=>{if(event.data.method.startsWith("account/")||event.data.method==="host/loginUpdated")void refresh().catch(error=>setError(errorText(error)));}),[]);
  async function run(action:()=>Promise<void>){
    setBusy(true);setError("");setMessage("");
    try {await action();await refresh();} catch(error){setError(errorText(error));}
    finally {try{const [settings,current]=await Promise.all([Data.GetSettings(),Codex.GetStatus()]);onSettings(settings);onStatus(current);}catch(error){setError(errorText(error));}setBusy(false);}
  }
  const active=view?.reuseCLI?undefined:view?.profiles?.find(profile=>profile.id===view.activeId);
  function balanceField(key:keyof BalanceConfig,value:string){if(draft)setDraft({...draft,balance:{...draft.balance,[key]:value}});}
  return <SettingsSection value="accounts" title="账号与额度" description="账号共用所选 CODEX_HOME；切换账号不改变目录，模型与默认权限分别保存。"><div className="space-y-5">
    {error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}{message&&<p role="status" className="text-sm text-muted-foreground">{message}</p>}
    <div className="space-y-3 rounded-lg border p-3"><div className="flex flex-wrap items-center gap-3"><div className="mr-auto"><p className="text-sm font-medium"><HelpText help="沿用所选 CODEX_HOME 中的登录和连接配置">目录现有配置</HelpText></p></div>{view?.reuseCLI?<Badge>使用中</Badge>:<Button variant="outline" size="sm" disabled={busy} onClick={()=>void run(async()=>{onStatus(await Codex.UseAccount(""));setRates(undefined);})}>沿用目录配置</Button>}</div><div className="flex justify-end"><Button variant="outline" size="sm" disabled={busy||!view?.reuseCLI||status?.phase!=="ready"} onClick={()=>void run(async()=>setRates(await Codex.GetRateLimits()))}>刷新配额</Button></div><AccountQuota rates={view?.reuseCLI?rates:undefined}/></div>
    {view?.profiles?.map(profile=>{
      const login=view.login.profileId===profile.id?view.login:undefined;
      return <div key={profile.id} className="space-y-3 rounded-lg border p-3"><div className="flex items-start gap-3"><div className="mr-auto min-w-0"><p className="flex flex-wrap items-center gap-2 text-sm font-medium">{profile.name} <Badge variant="outline">{profile.type==="chatgpt"?"ChatGPT":"API"}</Badge></p><p className="mt-1 break-all text-xs text-muted-foreground">{profile.type==="chatgpt"?login?.status==="pending"?"正在等待浏览器登录…":login?.status==="completed"?"登录成功":login?.status==="cancelled"?"登录已取消，可重新发起":profile.authenticated?"已保存本应用登录状态":"等待登录":`${profile.provider} · ${profile.baseURL}`}{login?.status==="pending"&&<Button variant="link" size="xs" disabled={busy} onClick={()=>void run(async()=>{await Codex.CancelLogin();})}>取消登录</Button>}</p></div>{!view.reuseCLI&&view.activeId===profile.id?<Badge>使用中</Badge>:<Button size="sm" variant="outline" disabled={busy||status?.phase==="starting"} onClick={()=>void run(async()=>{onStatus(await Codex.UseAccount(profile.id));setRates(undefined);})}>启用</Button>}</div>
      <div className="flex justify-end overflow-x-auto"><ButtonGroup aria-label={`${profile.name}账号操作`} orientation="horizontal">
        {profile.type==="chatgpt"&&<Button size="sm" variant="outline" disabled={busy||status?.phase==="starting"||active?.id===profile.id&&status?.phase!=="ready"} onClick={()=>void run(async()=>{
          if(active?.id!==profile.id){onStatus(await Codex.UseAccount(profile.id));setRates(undefined);}
          await Codex.StartLogin();
          await Codex.ContinueLogin();
        })}>{login?.status==="pending"?"继续登录":profile.authenticated?"重新登录":"登录"}</Button>}
        <Button size="sm" variant="outline" disabled={busy} onClick={()=>setDraft(draftFor(profile))}>编辑</Button>
        <Button size="sm" variant="outline" disabled={busy||!view.reuseCLI&&view.activeId===profile.id} onClick={()=>setRemoving(profile)}>删除</Button>
        <Button variant="outline" size="sm" disabled={busy||profile.type==="chatgpt"&&active?.id!==profile.id} onClick={()=>void run(async()=>setMessage(await Codex.CheckAccountConnection(profile.id)))}>检查连接</Button>
        {profile.type==="chatgpt"?<Button variant="outline" size="sm" disabled={busy||active?.id!==profile.id||status?.phase!=="ready"} onClick={()=>void run(async()=>setRates(await Codex.GetRateLimits()))}>刷新配额</Button>:<Button variant="outline" size="sm" disabled={busy||!profile.balance.url} onClick={()=>void run(async()=>{const result=await Codex.GetBalance(profile.id,true);setBalances(old=>({...old,[profile.id]:result}));})}>刷新余额</Button>}
      </ButtonGroup></div>
      {login?.error&&<p className="text-xs text-destructive">{login.error}</p>}
      {profile.type==="chatgpt"?<AccountQuota rates={active?.id===profile.id?rates:undefined}/>:balances[profile.id]?<BalanceView result={balances[profile.id]}/>:<p className="text-sm text-muted-foreground">尚未读取余额。</p>}</div>;
    })}
    <div className="flex flex-wrap justify-end gap-2"><Button variant="outline" disabled={busy} onClick={()=>setDraft(draftFor())}>新增 ChatGPT 账号</Button><Button variant="outline" disabled={busy} onClick={()=>setDraft(draftFor(undefined,"api"))}>新增 API 账号</Button></div>

    <Dialog open={!!draft} onOpenChange={open=>{if(!open&&!busy)setDraft(undefined);}}><DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-xl"><DialogHeader><DialogTitle>{draft?.id?"编辑账号":draft?.type==="api"?"新增 API 账号":"新增 ChatGPT 账号"}</DialogTitle><DialogDescription>{draft?.type==="api"?"使用支持 Responses API 的服务；密钥留空可保留已保存值。":"登录由原生 Codex 发起，凭据保存在 agentor 的独立目录。"}</DialogDescription></DialogHeader>{error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}{draft&&<form className="space-y-4" onSubmit={event=>{event.preventDefault();void run(async()=>{await Codex.SaveProfile(draft);setBalances({});setDraft(undefined);});}}><SettingsField><FieldLabel htmlFor="account-name">账号名称</FieldLabel><Input id="account-name" value={draft.name} required onChange={event=>setDraft({...draft,name:event.target.value})}/></SettingsField>{draft.type==="api"&&<><SettingsField><FieldLabel htmlFor="api-provider">提供方名称</FieldLabel><Input id="api-provider" value={draft.provider} onChange={event=>setDraft({...draft,provider:event.target.value})}/></SettingsField><SettingsField><FieldLabel htmlFor="api-base">API 基础地址</FieldLabel><Input id="api-base" type="url" required value={draft.baseURL} onChange={event=>setDraft({...draft,baseURL:event.target.value})}/><FieldDescription>例如 https://api.openai.com/v1</FieldDescription></SettingsField><SettingsField><FieldLabel htmlFor="api-model">账号模型</FieldLabel><Input id="api-model" value={draft.model} placeholder="服务支持的模型标识，可稍后选择" onChange={event=>setDraft({...draft,model:event.target.value})}/></SettingsField><SettingsField><FieldLabel htmlFor="api-key">API 密钥</FieldLabel><Input id="api-key" type="password" autoComplete="off" value={draft.apiKey} placeholder={draft.id?"留空保留已保存密钥":"仅保存在应用私有配置"} onChange={event=>setDraft({...draft,apiKey:event.target.value})}/></SettingsField><div className="space-y-4 border-t pt-4"><p className="text-sm font-medium">可选余额接口</p><SettingsField><FieldLabel htmlFor="balance-url">余额接口地址</FieldLabel><Input id="balance-url" type="url" value={draft.balance.url} onChange={event=>balanceField("url",event.target.value)}/></SettingsField>{draft.balance.url&&<><div className="grid gap-3"><SettingsField><FieldLabel htmlFor="balance-method">HTTP 方法</FieldLabel><Select value={draft.balance.method} onValueChange={selectedValue => { if (selectedValue === null) return; balanceField("method",selectedValue); }} items={[{ value: "GET", label: "GET" }, { value: "POST", label: "POST" }]}><SelectTrigger id="balance-method"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="GET">GET</SelectItem><SelectItem value="POST">POST</SelectItem></SelectContent></Select></SettingsField><SettingsField><FieldLabel htmlFor="balance-header">密钥请求头</FieldLabel><Input id="balance-header" value={draft.balance.headerName} onChange={event=>balanceField("headerName",event.target.value)}/></SettingsField></div><SettingsField><FieldLabel htmlFor="balance-prefix">密钥前缀</FieldLabel><Input id="balance-prefix" value={draft.balance.headerPrefix} onChange={event=>balanceField("headerPrefix",event.target.value)}/><FieldDescription>当前账号密钥会自动接在前缀后；Bearer 后需一个空格。</FieldDescription></SettingsField>{draft.balance.method==="POST"&&<Field><FieldLabel htmlFor="balance-body">JSON 请求正文</FieldLabel><Textarea id="balance-body" className="font-mono text-xs" value={draft.balance.body} onChange={event=>balanceField("body",event.target.value)} placeholder={'{"key":"$API_KEY"}'}/><FieldDescription>完整字符串值 $API_KEY 引用当前账号密钥。</FieldDescription></Field>}<div className="grid gap-3">{([['balancePath','余额字段'],['usedPath','已用字段'],['totalPath','总量字段'],['unitPath','单位字段']] as const).map(([key,label])=><SettingsField key={key}><FieldLabel htmlFor={key}>{label}</FieldLabel><Input id={key} className="font-mono text-xs" value={draft.balance[key]} placeholder="/data/balance" onChange={event=>balanceField(key,event.target.value)}/></SettingsField>)}</div><SettingsField><FieldLabel htmlFor="balance-unit">固定单位</FieldLabel><Input id="balance-unit" value={draft.balance.unit} onChange={event=>balanceField("unit",event.target.value)}/><FieldDescription>使用 JSON Pointer 映射；未配置字段保持未知，不自动计算。</FieldDescription></SettingsField></>}</div></>}<DialogFooter><Button type="button" variant="outline" disabled={busy} onClick={()=>setDraft(undefined)}>取消</Button><Button type="submit" disabled={busy}>保存账号</Button></DialogFooter></form>}</DialogContent></Dialog>
    <AlertDialog open={!!removing} onOpenChange={open=>{if(!open)setRemoving(undefined);}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>删除 {removing?.name}？</AlertDialogTitle><AlertDialogDescription>移除此账号及其托管凭据，保留共享会话历史。当前使用中的账号需要先切换。</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction variant="destructive" onClick={()=>{const id=removing?.id;if(id)void run(async()=>{await Codex.DeleteProfile(id);setRemoving(undefined);});}}>删除账号</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </div></SettingsSection>;
}


function AccountQuota({rates}:{rates?:RateLimits}){
  const fallback=rates?.rateLimits;
  const buckets:Array<[string,RateSnapshot]>=Object.keys(rates?.rateLimitsByLimitId||{}).length?Object.entries(rates?.rateLimitsByLimitId||{}).filter((entry):entry is [string,RateSnapshot]=>entry[1]!==undefined):fallback?[["codex",fallback]]:[];

  return <div className="space-y-3">{!rates&&<p className="text-sm text-muted-foreground">尚未读取配额。</p>}{rates&&!buckets.length&&<p className="text-sm text-muted-foreground">上游未提供可显示的窗口。</p>}{buckets.map(([key,snapshot])=><QuotaView key={key} label={snapshot.limitName||snapshot.limitId||key} snapshot={snapshot}/>)}{rates&&<p className="text-xs text-muted-foreground">读取于 {new Date(rates.checkedAt).toLocaleString()}</p>}</div>;
}

function QuotaView({label,snapshot}:{label:string;snapshot:RateSnapshot}){
  return <div className="space-y-3 rounded-lg border p-3"><p className="text-sm font-medium">{label}</p>{([['主窗口',snapshot.primary],['次窗口',snapshot.secondary]] as const).map(([name,window])=>window&&<div key={name} className="space-y-2"><p className="text-xs">{name} · 已用 {window.usedPercent}% · 剩余 {Math.max(0,Math.min(100,100-window.usedPercent))}%{window.windowDurationMins!=null&&` · ${window.windowDurationMins} 分钟`}</p><Progress aria-label={`${label} ${name}已用配额`} value={Math.max(0,Math.min(100,window.usedPercent))}/><p className="text-xs text-muted-foreground">{window.resetsAt!=null?`重置于 ${new Date(window.resetsAt*1000).toLocaleString()}`:"重置时间未知"}</p></div>)}{!snapshot.primary&&!snapshot.secondary&&<p className="text-xs text-muted-foreground">窗口数据未知。</p>}{snapshot.credits&&<p className="text-xs text-muted-foreground">附加额度：{snapshot.credits.unlimited?"无限":snapshot.credits.balance??"未知"}</p>}</div>;
}

function BalanceView({result}:{result:BalanceResult}){
  return <div className="space-y-2 text-xs">{result.error?<p className="text-destructive">{result.error}</p>:<p>余额 {result.balance??"未知"} · 已用 {result.used??"未知"} · 总量 {result.total??"未知"}{result.unit&&` · ${result.unit}`}</p>}<p className="text-muted-foreground">查询于 {new Date(result.checkedAt).toLocaleString()}</p></div>;
}
