import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import * as Codex from "../../../bindings/agentor/internal/codex/service";
import type { Status } from "../../../bindings/agentor/internal/codex/models";
import { MessageScroller, MessageScrollerButton, MessageScrollerContent, MessageScrollerItem, MessageScrollerProvider, MessageScrollerViewport } from "@/components/ui/message-scroller";
import { MessageView } from "@/features/chat/MessageView";
import { PreviewContext } from "@/features/chat/HTMLView";
import { applyNotification, type ConversationState, type Notification } from "@/features/chat/state";
import { errorText } from "@/app/errors";

export function TaskConversation({threadId,turnId,running,status,onError,onFollowup}:{threadId:string;turnId:string;running:boolean;status?:Status;onError:(message:string)=>void;onFollowup:(text:string)=>void}){
  const [conversation,setConversation]=useState<ConversationState>();
  useEffect(()=>{
    if(!threadId||!turnId||status?.phase!=="ready")return;
    let cancelled=false,loaded=false,frame=0;setConversation(undefined);
    const pending:{method:string;params:Notification}[]=[];
    const flush=()=>{if(frame)cancelAnimationFrame(frame);frame=0;if(!loaded)return;const batch=pending.splice(0);setConversation(old=>old?batch.reduce((next,event)=>applyNotification(next,event.method,event.params),old):old);};
    void (async()=>{
      let value;
      try{value=await Codex.OpenThread(threadId);}catch(error){
        // A newly started thread may not have persisted history yet. Continue
        // displaying live events, then read the final history on completion.
        if(!running)throw error;
        value={thread:{id:threadId,name:"",preview:"",cwd:"",model:"",originator:"",createdAt:0,updatedAt:0,status:{type:"active"}},turns:{data:[],nextCursor:null},activeTurnId:turnId};
      }
      let turns=value.turns.data||[],cursor=value.turns.nextCursor;
      while(!cancelled&&turnId&&!turns.some(turn=>turn.id===turnId)&&cursor){const earlier=await Codex.ListTurns(threadId,cursor);turns=[...turns,...(earlier.data||[])];cursor=earlier.nextCursor;}
      if(cancelled)return;
      let next:ConversationState={thread:value.thread,turns:[...turns].reverse(),cursor,activeTurnId:value.activeTurnId};
      for(const event of pending.splice(0))next=applyNotification(next,event.method,event.params);
      loaded=true;setConversation(next);
    })().catch(error=>{if(!cancelled)onError(errorText(error));});
    const off=Events.On("codex:event",event=>{
      if(event.data.generation!==status.generation)return;
      let params:Notification;try{params=JSON.parse(event.data.params);}catch{return;}
      if(params.threadId!==threadId)return;
      pending.push({method:event.data.method,params});
      if(params.delta!==undefined){if(!frame)frame=requestAnimationFrame(flush);}else flush();
    });
    return()=>{cancelled=true;off();if(frame)cancelAnimationFrame(frame);};
  },[threadId,turnId,running,status?.generation,status?.phase]);
  const turns=turnId?conversation?.turns.filter(turn=>turn.id===turnId):[];
  return <PreviewContext.Provider value={{threadId,followup:onFollowup}}><div className="h-[32rem] min-h-48 rounded-lg border">
    <MessageScrollerProvider key={`${threadId}:${turnId}`}><MessageScroller><MessageScrollerViewport><MessageScrollerContent className="space-y-5 p-4">
      {!turns?.length&&<p className="text-sm text-muted-foreground">{threadId?"正在读取此次执行的消息…":"任务开始后将在这里显示对话。"}</p>}
      {turns?.map(turn=><MessageScrollerItem key={turn.id}>{turn.items?.map(item=><MessageView key={item.id} item={item} streaming={turn.status==="inProgress"} onError={onError}/>)}</MessageScrollerItem>)}
    </MessageScrollerContent></MessageScrollerViewport><MessageScrollerButton/></MessageScroller></MessageScrollerProvider>
  </div></PreviewContext.Provider>;
}
