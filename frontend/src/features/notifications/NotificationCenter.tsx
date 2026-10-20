import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { Bell } from "lucide-react";
import * as Tasks from "../../../bindings/agentor/internal/tasks/service";
import type { State } from "../../../bindings/agentor/internal/tasks/models";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { MarkdownView } from "@/features/chat/MarkdownView";
import { errorText } from "@/app/errors";
import { DeliveryPart } from "./DeliveryPart";

export type NotificationTarget={stepId?:string;executionId?:string;requestId?:string;pipelineRunId?:string;pipelineSessionId?:string;projectId?:string};

export function NotificationCenter({onOpenSource}:{onOpenSource:(target:NotificationTarget)=>Promise<void>}){
  const [open,setOpen]=useState(false),[state,setState]=useState<State>(),[error,setError]=useState("");
  async function refresh(){try{setState(await Tasks.GetState());}catch(error){setError(errorText(error));}}
  useEffect(()=>{void refresh();return Events.On("tasks:changed",()=>void refresh());},[]);
  const notifications=[...(state?.notifications||[])].reverse();
  const unread=notifications.filter(notification=>!notification.read).length;
  return <><Button variant="ghost" size="sm" onClick={()=>setOpen(true)}><Bell/>通知{unread>0&&<Badge>{unread}</Badge>}</Button><Sheet open={open} onOpenChange={setOpen}><SheetContent className="flex w-full flex-col sm:max-w-xl"><SheetHeader><SheetTitle>通知与结果</SheetTitle><SheetDescription>业务结果与送达状态分别记录，历史在重开后仍可查看。</SheetDescription></SheetHeader><div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 pb-6">{error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}{!notifications.length&&<p className="py-8 text-center text-sm text-muted-foreground">暂时没有通知。</p>}{notifications.map(notification=><Card key={notification.id}><CardHeader><CardTitle className="flex items-center gap-2 text-sm">{notification.title}{!notification.read&&<Badge variant="secondary">未读</Badge>}</CardTitle><p className="text-xs text-muted-foreground">{new Date(notification.createdAt).toLocaleString()}</p></CardHeader><CardContent className="space-y-3"><MarkdownView text={notification.body} onError={setError}/><div className="flex flex-wrap gap-2">{notification.parts?.map(part=><DeliveryPart key={part.id} notificationId={notification.id} part={part} refresh={refresh} onError={setError}/>)}</div><div className="flex gap-2">{(notification.executionId||notification.requestId||notification.pipelineRunId||notification.pipelineSessionId)&&<Button size="sm" variant="outline" onClick={()=>void (async()=>{try{const execution=state?.executions?.find(execution=>execution.id===notification.executionId);await Tasks.MarkNotificationRead(notification.id);await onOpenSource(notification.pipelineRunId||notification.pipelineSessionId?{pipelineRunId:notification.pipelineRunId,pipelineSessionId:notification.pipelineSessionId,projectId:notification.projectId}:execution?{executionId:execution.id,stepId:execution.stepId}:{requestId:notification.requestId});await refresh();setOpen(false);}catch(error){setError(errorText(error));}})()}>查看来源</Button>}{!notification.read&&<Button size="sm" variant="ghost" onClick={()=>void Tasks.MarkNotificationRead(notification.id).then(refresh).catch(error=>setError(errorText(error)))}>标为已读</Button>}</div></CardContent></Card>)}</div></SheetContent></Sheet></>;
}
