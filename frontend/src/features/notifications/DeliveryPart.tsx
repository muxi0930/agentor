import { useState } from "react";
import * as Lark from "../../../bindings/agentor/internal/lark/service";
import type { NotificationPart } from "../../../bindings/agentor/internal/tasks/models";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { errorText } from "@/app/errors";

const labels:Record<string,string>={pending:"待发送",sent:"已发送",failed:"明确失败",unknown:"发送中或结果待确认",ignored:"已忽略"};
export function DeliveryPart({notificationId,part,refresh,onError}:{notificationId:string;part:NotificationPart;refresh:()=>Promise<void>;onError:(text:string)=>void}){
  const [busy,setBusy]=useState(false),[confirm,setConfirm]=useState(""),[reason,setReason]=useState("用户选择忽略此发送项");
  async function run(action:()=>Promise<void>){setBusy(true);try{await action();await refresh();}catch(error){onError(errorText(error));}finally{setBusy(false);}}
  if(part.channel==="desktop")return <Badge variant="outline">桌面已记录</Badge>;
  return <div className="w-full space-y-2 rounded-md border p-3"><div className="flex flex-wrap items-center gap-2"><Badge variant={part.status==="failed"?"destructive":"outline"}>飞书 · {part.kind==="file"?"产物":part.kind==="approval"?"审批卡片":"文字"} · {labels[part.status]||part.status}</Badge><span className="break-all text-xs text-muted-foreground">{part.target}</span></div>{part.error&&<p className="whitespace-pre-wrap text-xs text-muted-foreground">{part.error}</p>}<div className="flex flex-wrap gap-2">{part.status==="failed"&&<Button variant="outline" size="xs" disabled={busy} onClick={()=>void run(()=>Lark.RetryNotificationPart(notificationId,part.id))}>重试此项</Button>}{part.status==="unknown"&&<Button variant="outline" size="xs" disabled={busy} onClick={()=>setConfirm("retry")}>确认未送达后重试</Button>}{part.status!=="sent"&&part.status!=="ignored"&&<Button variant="ghost" size="xs" disabled={busy} onClick={()=>setConfirm("ignore")}>忽略此项</Button>}</div><AlertDialog open={!!confirm} onOpenChange={open=>{if(!open)setConfirm("");}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{confirm==="retry"?"确认原消息没有送达？":"忽略此发送项？"}</AlertDialogTitle><AlertDialogDescription>{confirm==="retry"?"仅在您已确认接收端没有这条消息时重试。服务端未确认的发送可能已经送达。":"保存忽略原因，执行结果仍保留。"}</AlertDialogDescription></AlertDialogHeader>{confirm==="ignore"&&<Input aria-label="忽略发送项的原因" value={reason} onChange={event=>setReason(event.target.value)}/>}<AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction disabled={busy||confirm==="ignore"&&!reason.trim()} onClick={()=>{const action=confirm;setConfirm("");void run(()=>action==="retry"?Lark.ConfirmNotDelivered(notificationId,part.id):Lark.IgnoreNotificationPart(notificationId,part.id,reason));}}>{confirm==="retry"?"确认未送达并重试":"保存忽略原因"}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog></div>;
}
