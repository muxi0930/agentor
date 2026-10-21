import { PageContent, PageHeader } from "@/components/ui/page";
import type { ReactNode } from "react";
import { Plus } from "lucide-react";
import type { Feature, Project, State, Version } from "../../../bindings/agentor/internal/workspace/models";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { permissionLabels } from "@/features/accounts/PermissionSelect";
import { MarkdownView } from "@/features/chat/MarkdownView";
import { KnowledgePanel } from "@/features/knowledge/KnowledgePanel";

import { formatCreatedAt } from "./createdAt";

type PageProps = {
  header: { type?: string; typeIcon?: ReactNode; title: ReactNode; description?: ReactNode; actions?: ReactNode };
  project: Project;
  busy: boolean;
  onSelect: (id: string) => void;
  onSaved: (state: State) => void;
  onError: (message: string) => void;
};

export function TextCard({title,text,onError}:{title:string;text:string;onError:(text:string)=>void}){
  return <Card><CardHeader><CardTitle className="text-sm">{title}</CardTitle></CardHeader><CardContent>{text?<MarkdownView text={text} onError={onError}/>:<p className="text-sm text-muted-foreground">尚未填写。</p>}</CardContent></Card>;
}

export function ProjectDetails({header,project,busy,documents,onSelect,onSaved,onError,onCreateVersion,tab,onTabChange,pipeline}:PageProps & {documents:State["documents"];onCreateVersion:()=>void;tab:string;onTabChange:(tab:string)=>void;pipeline:ReactNode}){
  return <Tabs value={tab} onValueChange={onTabChange} className="gap-6">
    <PageHeader {...header}><TabsList aria-label="项目页面内容"><TabsTrigger value="versions">版本列表</TabsTrigger><TabsTrigger value="knowledge">知识库</TabsTrigger><TabsTrigger value="pipeline">流水线</TabsTrigger></TabsList></PageHeader>
    <PageContent>
    <TabsContent value="versions" className="space-y-5">
      <Button disabled={busy} onClick={onCreateVersion}><Plus/>新建版本</Button>
      <Table aria-label="版本列表">
        <TableHeader><TableRow><TableHead>名称</TableHead><TableHead>说明</TableHead><TableHead>创建时间</TableHead><TableHead>标签</TableHead><TableHead className="text-right">功能数</TableHead></TableRow></TableHeader>
        <TableBody>{project.versions?.map(version=><TableRow key={version.id}><TableCell><Button variant="link" className="h-auto max-w-56 justify-start whitespace-normal break-words p-0 text-left" onClick={()=>onSelect(version.id)}>{version.name}</Button></TableCell><TableCell className="min-w-40 max-w-72 whitespace-normal break-words text-muted-foreground">{version.description||"暂无说明"}</TableCell><TableCell className="text-muted-foreground">{formatCreatedAt(version.createdAt)}</TableCell><TableCell><div className="flex flex-wrap gap-2">{version.tagIds?.map(id=>{const tag=project.tags?.find(tag=>tag.id===id);return tag&&<Badge key={id} variant="outline"><span className="mr-1 size-2 rounded-full" style={{background:tag.color}}/>{tag.name}</Badge>;})}</div></TableCell><TableCell className="text-right">{version.features?.length||0}</TableCell></TableRow>)}{!project.versions?.length&&<TableRow><TableCell colSpan={5} className="py-8 text-center text-muted-foreground">此项目暂无版本。</TableCell></TableRow>}</TableBody>
      </Table>
    </TabsContent>
    <TabsContent value="knowledge"><KnowledgePanel project={project} documents={documents||[]} onSaved={onSaved} onError={onError}/></TabsContent>
    <TabsContent value="pipeline">{pipeline}</TabsContent>
    </PageContent>
  </Tabs>;
}

export function VersionDetails({header,version,busy,onSelect,onCreateFeature}:PageProps & {version:Version;onCreateFeature:()=>void}){
  return <>
    <PageHeader {...header}/>
    <PageContent>
      <Button disabled={busy} onClick={onCreateFeature}><Plus/>新建功能</Button>
      <Table aria-label="功能列表">
        <TableHeader><TableRow><TableHead>名称</TableHead><TableHead>说明</TableHead><TableHead>创建时间</TableHead><TableHead>状态</TableHead><TableHead>完成时间</TableHead><TableHead className="text-right">步骤数</TableHead></TableRow></TableHeader>
        <TableBody>{version.features?.map(feature=><TableRow key={feature.id}><TableCell><Button variant="link" className="h-auto max-w-56 justify-start whitespace-normal break-words p-0 text-left" onClick={()=>onSelect(feature.id)}>{feature.title}</Button></TableCell><TableCell className="min-w-40 max-w-72 whitespace-normal break-words text-muted-foreground">{feature.description||"暂无说明"}</TableCell><TableCell className="text-muted-foreground">{formatCreatedAt(feature.createdAt)}</TableCell><TableCell><Badge variant="outline" className={feature.completed?"border-transparent bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200":undefined}>{feature.completed?"已完成":"未完成"}</Badge></TableCell><TableCell className="text-muted-foreground">{feature.completedAt?formatCreatedAt(feature.completedAt):"—"}</TableCell><TableCell className="text-right">{feature.steps?.length||0}</TableCell></TableRow>)}{!version.features?.length&&<TableRow><TableCell colSpan={6} className="py-8 text-center text-muted-foreground">此版本暂无功能。</TableCell></TableRow>}</TableBody>
      </Table>
    </PageContent>
  </>;
}

export function FeatureDetails({header,project,version,feature,busy,documents,onSelect,onSaved,onError,onCreateStep}:PageProps & {version:Version;feature:Feature;documents:State["documents"];onCreateStep:()=>void}){
  return <Tabs defaultValue="steps" className="gap-6">
    <PageHeader {...header}><TabsList aria-label="功能页面内容" className="h-auto max-w-full flex-wrap justify-start"><TabsTrigger value="steps">步骤列表</TabsTrigger><TabsTrigger value="knowledge">知识库</TabsTrigger></TabsList></PageHeader>
    <PageContent>
    <TabsContent value="steps" className="space-y-5">
      <Button disabled={busy} onClick={onCreateStep}><Plus/>新建步骤</Button>
      <Table aria-label="步骤列表">
        <TableHeader><TableRow><TableHead>名称</TableHead><TableHead>说明</TableHead><TableHead>创建时间</TableHead><TableHead>状态</TableHead><TableHead>执行权限</TableHead><TableHead>会话</TableHead></TableRow></TableHeader>
        <TableBody>{feature.steps?.map(step=><TableRow key={step.id}><TableCell><Button variant="link" className="h-auto max-w-56 justify-start whitespace-normal break-words p-0 text-left" onClick={()=>onSelect(step.id)}>{step.title}</Button></TableCell><TableCell className="min-w-40 max-w-72 whitespace-normal break-words text-muted-foreground">{step.description||"暂无说明"}</TableCell><TableCell className="text-muted-foreground">{formatCreatedAt(step.createdAt)}</TableCell><TableCell><Badge variant="outline" className={step.completed?"border-transparent bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200":undefined}>{step.completed?"已完成":"未完成"}</Badge></TableCell><TableCell>{step.permissionMode?permissionLabels[step.permissionMode]:"继承项目与账号"}</TableCell><TableCell>{step.threadId?"已绑定":"未绑定"}</TableCell></TableRow>)}{!feature.steps?.length&&<TableRow><TableCell colSpan={6} className="py-8 text-center text-muted-foreground">此功能暂无步骤。</TableCell></TableRow>}</TableBody>
      </Table>
    </TabsContent>
    <TabsContent value="knowledge"><KnowledgePanel project={project} version={version} feature={feature} documents={documents||[]} onSaved={onSaved} onError={onError}/></TabsContent>
    </PageContent>
  </Tabs>;
}
