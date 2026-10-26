import type { Project } from "../../bindings/agentor/internal/workspace/model/models";
import { ProjectNavigation } from "../../src/features/workspace/ProjectNavigation";
import { TooltipProvider } from "../../src/components/ui/tooltip";

const project = {id:"p",name:"示例项目",directory:"/projects/demo",versions:[
 {id:"v1",name:"V1 · Worktree 开发",features:[],worktree:{directory:"/projects/worktrees/version-v1",repository:"/projects/demo",sourceBranch:"main",sourceCommit:"base",branch:"codex/version-v1",status:"ready",runId:"prepare-v1",mergedCommit:""}},
 {id:"v2",name:"V2 · 项目目录开发",features:[]}
]} as Project;
export function VersionPreview() {
 return <TooltipProvider><main className="m-8 w-80 rounded-xl border bg-card p-4"><h1 className="mb-3 text-sm font-medium">版本工作区</h1><ProjectNavigation selected={{kind:"project",project}} busy={false} actions={()=>null} dragRow={()=>({})} onSelect={()=>{}} /></main></TooltipProvider>;
}
