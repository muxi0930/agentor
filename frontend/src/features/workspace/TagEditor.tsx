import { useState } from "react";
import { ArrowDown, ArrowUp, Pencil, Plus, Trash2 } from "lucide-react";
import * as Work from "../../../bindings/agentor/internal/workspace/service";
import type { Project, State, Tag } from "../../../bindings/agentor/internal/workspace/models";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Input } from "@/components/ui/input";
import { Field, FieldLabel } from "@/components/ui/field";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { errorText } from "@/app/errors";

export function TagEditor({project,onSaved,close}:{project:Project;onSaved:(state:State)=>void;close:()=>void}){
  const [draft,setDraft]=useState<Tag>({id:"",name:"",color:"#64748b"});
  const [removing,setRemoving]=useState<Tag>();
  const [busy,setBusy]=useState(false),[error,setError]=useState("");
  async function run(action:()=>Promise<State>){setBusy(true);setError("");try{onSaved(await action());}catch(error){setError(errorText(error));}finally{setBusy(false);}}
  return <><Dialog open onOpenChange={open=>{if(!open)close();}}><DialogContent><DialogHeader><DialogTitle>版本标签</DialogTitle><DialogDescription>项目内共用标签；调整顺序会保存。</DialogDescription></DialogHeader>
    {error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
    <div className="max-h-72 space-y-2 overflow-y-auto">{(project.tags||[]).map(tag=><div key={tag.id} className="flex items-center gap-2 rounded-lg border p-2"><span className="size-3 rounded-full" style={{background:tag.color}}/><span className="mr-auto text-sm">{tag.name}</span><Button disabled={busy} variant="ghost" size="icon-xs" aria-label={`上移标签 ${tag.name}`} onClick={()=>void run(()=>Work.MoveNode("tag",tag.id,-1))}><ArrowUp/></Button><Button disabled={busy} variant="ghost" size="icon-xs" aria-label={`下移标签 ${tag.name}`} onClick={()=>void run(()=>Work.MoveNode("tag",tag.id,1))}><ArrowDown/></Button><Button variant="ghost" size="icon-xs" aria-label={`编辑标签 ${tag.name}`} onClick={()=>setDraft(tag)}><Pencil/></Button><Button variant="ghost" size="icon-xs" aria-label={`删除标签 ${tag.name}`} onClick={()=>setRemoving(tag)}><Trash2/></Button></div>)}</div>
    <form className="space-y-3" onSubmit={event=>{event.preventDefault();void run(async()=>{const next=await Work.SaveTag(project.id,draft);setDraft({id:"",name:"",color:"#64748b"});return next;});}}><Field><FieldLabel htmlFor="tag-name">标签名称</FieldLabel><div className="flex gap-2"><Input id="tag-name" required value={draft.name} onChange={event=>setDraft(old=>({...old,name:event.target.value}))}/><Input aria-label="标签颜色" type="color" className="w-14 p-1" value={draft.color} onChange={event=>setDraft(old=>({...old,color:event.target.value}))}/><Button type="submit" disabled={busy||!draft.name.trim()}>{draft.id?<Pencil/>:<Plus/>}{draft.id?"保存":"添加"}</Button></div></Field></form>
  </DialogContent></Dialog><AlertDialog open={!!removing} onOpenChange={open=>{if(!open)setRemoving(undefined);}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>删除标签“{removing?.name}”？</AlertDialogTitle><AlertDialogDescription>将同时移除版本上的此标签引用。</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction onClick={()=>{if(removing)void run(()=>Work.DeleteNode("tag",removing.id));setRemoving(undefined);}}>删除</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog></>;
}
