import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { Bell, Check, CheckCheck } from "lucide-react";
import * as Tasks from "../../../bindings/agentor/internal/tasks/service";
import type { State } from "../../../bindings/agentor/internal/tasks/models";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { MarkdownView } from "@/features/chat/MarkdownView";
import { errorText } from "@/app/errors";
import { DeliveryPart } from "./DeliveryPart";

export type NotificationTarget={stepId?:string;executionId?:string;requestId?:string;pipelineRunId?:string;pipelineSessionId?:string;projectId?:string};

export function NotificationCenter({onOpenSource}:{onOpenSource:(target:NotificationTarget)=>Promise<void>}){
  const [open,setOpen]=useState(false),[state,setState]=useState<State>(),[error,setError]=useState("");
  const [markingAll,setMarkingAll]=useState(false);
  async function refresh(){try{setState(await Tasks.GetState());}catch(error){setError(errorText(error));}}
  useEffect(()=>{void refresh();return Events.On("tasks:changed",()=>void refresh());},[]);
  const notifications=[...(state?.notifications||[])].reverse();
  const unread=notifications.filter(notification=>!notification.read).length;
  async function markAllRead(){
    setMarkingAll(true);
    try{
      for(const notification of notifications.filter(notification=>!notification.read)){
        await Tasks.MarkNotificationRead(notification.id);
      }
    }catch(error){setError(errorText(error));}
    finally{await refresh();setMarkingAll(false);}
  }
  return <><Button variant="ghost" size="sm" onClick={()=>setOpen(true)}><Bell/>通知{unread>0&&<Badge>{unread}</Badge>}</Button><Sheet open={open} onOpenChange={setOpen}><SheetContent className="flex w-full flex-col sm:max-w-xl"><SheetHeader><SheetTitle className="pr-16">通知与结果</SheetTitle><SheetDescription>业务结果与送达状态分别记录，历史在重开后仍可查看。</SheetDescription></SheetHeader><Button variant="ghost" size="icon-sm" className="absolute top-3 right-12" aria-label="全部已读" title="全部已读" disabled={!unread||markingAll} onClick={()=>void markAllRead()}><CheckCheck aria-hidden="true"/></Button><Tabs defaultValue="unread" className="min-h-0 flex-1 px-4 pb-6"><TabsList aria-label="通知状态"><TabsTrigger value="unread">未读{unread>0&&<Badge variant="secondary">{unread}</Badge>}</TabsTrigger><TabsTrigger value="read">已读</TabsTrigger></TabsList>{error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}{(["unread","read"] as const).map(tab=>{const items=notifications.filter(notification=>notification.read===(tab==="read"));return <TabsContent key={tab} value={tab} className="min-h-0 space-y-4 overflow-y-auto p-1">{!items.length&&<p className="py-8 text-center text-sm text-muted-foreground">{tab==="unread"?"暂无未读通知。":"暂无已读通知。"}</p>}{items.map(notification=><Card key={notification.id}><CardHeader><CardTitle className="flex items-center gap-2 text-sm">{notification.title}</CardTitle><p className="text-xs text-muted-foreground">{new Date(notification.createdAt).toLocaleString()}</p><CardAction>{notification.read?<span className="flex size-7 items-center justify-center" role="img" aria-label="已读" title="已读"><span className="flex size-5 items-center justify-center rounded-full bg-emerald-500 text-white"><Check className="size-3.5" aria-hidden="true"/></span></span>:<Button size="sm" variant="ghost" aria-label="设为已读" onClick={()=>void Tasks.MarkNotificationRead(notification.id).then(refresh).catch(error=>setError(errorText(error)))}>已读</Button>}</CardAction></CardHeader><CardContent className="space-y-3"><MarkdownView text={notification.body} onError={setError}/><div className="flex flex-wrap gap-2">{notification.parts?.map(part=><DeliveryPart key={part.id} notificationId={notification.id} part={part} refresh={refresh} onError={setError}/>)}</div><div className="flex gap-2">{(notification.executionId||notification.requestId||notification.pipelineRunId||notification.pipelineSessionId)&&<Button size="sm" variant="outline" onClick={()=>void (async()=>{try{const execution=state?.executions?.find(execution=>execution.id===notification.executionId);await Tasks.MarkNotificationRead(notification.id);await onOpenSource(notification.pipelineRunId||notification.pipelineSessionId?{pipelineRunId:notification.pipelineRunId,pipelineSessionId:notification.pipelineSessionId,projectId:notification.projectId}:execution?{executionId:execution.id,stepId:execution.stepId}:{requestId:notification.requestId});await refresh();setOpen(false);}catch(error){setError(errorText(error));}})()}>查看来源</Button>}</div></CardContent></Card>)}</TabsContent>;})}</Tabs></SheetContent></Sheet></>;
}
