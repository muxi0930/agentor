import { useState } from "react";
import * as Work from "../../../bindings/agentor/internal/workspace/service";
import * as Data from "../../../bindings/agentor/internal/appdata/service";
import type { Project, State } from "../../../bindings/agentor/internal/workspace/models";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { PermissionSelect } from "@/features/accounts/PermissionSelect";
import { errorText } from "@/app/errors";
import type { NodeKind, Selection } from "./selection";

export type EntityDraft = { kind: NodeKind; id:string; projectId:string; versionId:string; featureId:string; title:string; description:string; directory:string; context:string; prompt:string; structure:string; tagIds:string[]; permissionMode:string };
export const nodeLabels = {project:"项目",version:"版本",feature:"功能",step:"步骤"};

export function makeDraft(kind: NodeKind, selection: Selection | undefined, directory: string, edit = false): EntityDraft {
  const draft: EntityDraft = {kind,id:"",projectId:selection?.project.id || "",versionId:selection?.version?.id || "",featureId:selection?.feature?.id || "",title:"",description:"",directory,context:"",prompt:"",structure:"",tagIds:[],permissionMode:""};
  if (!edit || !selection) return draft;
  if (kind === "project") Object.assign(draft,{id:selection.project.id,title:selection.project.name,directory:selection.project.directory,context:selection.project.sharedContext,prompt:selection.project.prompt,structure:selection.project.structure});
  if (kind === "version") Object.assign(draft,{id:selection.version!.id,title:selection.version!.name,description:selection.version!.description,tagIds:selection.version!.tagIds || []});
  if (kind === "feature") Object.assign(draft,{id:selection.feature!.id,title:selection.feature!.title,description:selection.feature!.description,context:selection.feature!.context,prompt:selection.feature!.prompt});
  if (kind === "step") Object.assign(draft,{id:selection.step!.id,title:selection.step!.title,description:selection.step!.description,prompt:selection.step!.prompt,permissionMode:selection.step!.permissionMode});
  return draft;
}

export function EntityEditor({ initial, project, onSaved, close }: {initial:EntityDraft; project?:Project; onSaved:(state:State)=>void; close:()=>void}) {
  const [draft,setDraft] = useState(initial);
  const [busy,setBusy] = useState(false);
  const [error,setError] = useState("");
  const set = (key: keyof EntityDraft,value: string) => setDraft(old=>({...old,[key]:value}));
  async function save() {
    setBusy(true);setError("");
    try {
      let result:State;
      if (draft.kind === "project") result = await Work.SaveProject({id:draft.id,name:draft.title,directory:draft.directory,sharedContext:draft.context,prompt:draft.prompt,structure:draft.structure,tags:[],versions:[],documentIds:[]});
      else if (draft.kind === "version") result = await Work.SaveVersion(draft.projectId,{id:draft.id,name:draft.title,description:draft.description,tagIds:draft.tagIds,documentIds:[],features:[]});
      else if (draft.kind === "feature") {
        result = await Work.SaveFeature(draft.projectId,draft.id?initial.versionId:draft.versionId,{id:draft.id,title:draft.title,description:draft.description,context:draft.context,prompt:draft.prompt,documentIds:[],steps:[],completed:false,completedAt:""});
        if (draft.id && draft.versionId !== initial.versionId) result = await Work.MoveFeature(draft.projectId,draft.id,draft.versionId);
      }
      else result = await Work.SaveStep(draft.projectId,draft.featureId,{id:draft.id,title:draft.title,description:draft.description,prompt:draft.prompt,permissionMode:draft.permissionMode,threadId:"",completed:false,completedAt:""});
      onSaved(result);close();
    } catch(error) {setError(errorText(error));} finally {setBusy(false);}
  }
  return <Dialog open onOpenChange={open=>{if(!open && !busy)close();}}><DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
    <DialogHeader><DialogTitle>{draft.id ? "编辑" : "新建"}{nodeLabels[draft.kind]}</DialogTitle><DialogDescription>{draft.kind === "project" ? "关联本地目录，填写项目共享资料。" : "保存后将定位到此对象。"}</DialogDescription></DialogHeader>
    <form onSubmit={event=>{event.preventDefault();void save();}} className="space-y-5">
      {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
      <fieldset disabled={busy}><FieldGroup>
        <Field><FieldLabel htmlFor="entity-title">{draft.kind === "project" || draft.kind === "version" ? "名称" : "标题"}</FieldLabel><Input id="entity-title" autoFocus required value={draft.title} onChange={event=>set("title",event.target.value)} /></Field>
        {draft.kind === "project" ? <Field><FieldLabel htmlFor="entity-directory">项目目录</FieldLabel><div className="flex gap-2"><Input id="entity-directory" required value={draft.directory} onChange={event=>set("directory",event.target.value)} /><Button variant="outline" type="button" onClick={()=>void Data.ChooseDirectory().then(value=>{if(value)set("directory",value);}).catch(error=>setError(errorText(error)))}>选择目录</Button></div></Field> : <Field><FieldLabel htmlFor="entity-description">说明</FieldLabel><Textarea id="entity-description" value={draft.description} onChange={event=>set("description",event.target.value)} /></Field>}
        {draft.kind === "feature" && <Field><FieldLabel htmlFor="entity-version">所属版本</FieldLabel><Select value={draft.versionId} disabled={busy} onValueChange={value=>{if(value!==null)set("versionId",value);}} items={project?.versions?.map(version=>({value:version.id,label:version.name}))??[]}><SelectTrigger id="entity-version"><SelectValue/></SelectTrigger><SelectContent>{project?.versions?.map(version=><SelectItem key={version.id} value={version.id}>{version.name}</SelectItem>)}</SelectContent></Select></Field>}
        {draft.kind === "version" && !!project?.tags?.length && <Field><FieldLabel>版本标签</FieldLabel><div className="flex flex-wrap gap-2">{project.tags.map(tag=><Button key={tag.id} variant={draft.tagIds.includes(tag.id)?"default":"outline"} aria-pressed={draft.tagIds.includes(tag.id)} type="button" size="sm" onClick={()=>setDraft(old=>({...old,tagIds:old.tagIds.includes(tag.id)?old.tagIds.filter(id=>id!==tag.id):[...old.tagIds,tag.id]}))}><span className="size-2 rounded-full" style={{background:tag.color}} />{tag.name}</Button>)}</div></Field>}
        {(draft.kind === "project" || draft.kind === "feature") && <Field><FieldLabel htmlFor="entity-context">{draft.kind === "project" ? "共享上下文" : "功能上下文"}</FieldLabel><Textarea id="entity-context" className="min-h-28" placeholder={draft.kind === "project" ? "告诉模型“项目的背景和已知事实”，如：\n业务目标、技术栈、领域术语、已有设计决策，例如“这是基于 Wails 的桌面应用，数据保存在本地”。" : undefined} value={draft.context} onChange={event=>set("context",event.target.value)} /></Field>}
        {draft.kind !== "version" && <Field><FieldLabel htmlFor="entity-prompt">提示词</FieldLabel><Textarea id="entity-prompt" className="min-h-28" placeholder={draft.kind === "project" ? "告诉模型“在这个项目里应该怎样工作”，如：\n编码规范、工作约束、输出要求，例如“使用中文回答、遵循最小变更原则、修改前检查可复用工具”。" : undefined} value={draft.prompt} onChange={event=>set("prompt",event.target.value)} /></Field>}
        {draft.kind === "project" && <Field><FieldLabel htmlFor="entity-structure">项目结构说明</FieldLabel><Textarea id="entity-structure" className="min-h-28 font-mono text-xs" placeholder={"告诉模型“代码在哪里，各目录负责什么”，如：\n目录结构、模块职责、关键入口，例如“internal/workspace 管理项目数据，frontend/src/features 存放功能页面”。"} value={draft.structure} onChange={event=>set("structure",event.target.value)} /></Field>}
        {draft.kind === "step" && <Field><FieldLabel htmlFor="step-permission">步骤执行权限</FieldLabel><PermissionSelect id="step-permission" value={draft.permissionMode} inherit="项目与账号偏好" onChange={value=>set("permissionMode",value)} /></Field>}
      </FieldGroup></fieldset>
      <DialogFooter><Button type="button" variant="outline" disabled={busy} onClick={close}>取消</Button><Button type="submit" disabled={busy || !draft.title.trim()}>{busy?"保存中…":"保存"}</Button></DialogFooter>
    </form>
  </DialogContent></Dialog>;
}
