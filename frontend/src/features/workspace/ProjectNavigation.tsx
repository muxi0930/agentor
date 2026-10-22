import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { useEffect, useState, type HTMLAttributes, type ReactNode } from "react";
import { ChevronRight, Check, GitBranch, ListChecks, Puzzle, type LucideIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import type { Selection } from "./selection";

type ProjectNavigationProps = {
  selected: Selection;
  busy: boolean;
  actions: (id: string) => ReactNode;
  dragRow: (id: string) => HTMLAttributes<HTMLDivElement>;
  onSelect: (id: string) => void;
};

export function ProjectNavigation({selected,busy,actions,dragRow,onSelect}:ProjectNavigationProps){
  const [expanded,setExpanded]=useState<Record<string,boolean>>({});
  const {project,version,feature}=selected;
  useEffect(()=>{
    setExpanded(old=>({...old,...(version&&selected.kind!=="version"?{[version.id]:true}:{}),...(feature&&selected.kind==="step"?{[feature.id]:true}:{})}));
  },[version?.id,feature?.id,selected.kind]);
  const open=(id:string,value:boolean)=>setExpanded(old=>({...old,[id]:value}));
  const nav=(id:string,title:string,icon:LucideIcon,children?:ReactNode,expandable=true,completed=false)=><NavNode key={id} id={id} title={title} icon={icon} completed={completed} expandable={expandable} selected={selected.step?.id||feature?.id||version?.id||project.id} open={expanded[id]!==false} onOpen={open} onSelect={onSelect} disabled={busy} actions={actions(id)} dragProps={dragRow(id)}>{children}</NavNode>;

  return <nav aria-label={project.name+" 项目导航"} className="space-y-2 px-1 py-2">
    {!project.versions?.length&&<p className="px-2 text-xs text-muted-foreground">无版本</p>}
    {project.versions?.map(item=>nav(item.id,item.name,GitBranch,item.features?.length?item.features.map(feature=>nav(feature.id,feature.title,Puzzle,feature.steps?.length?feature.steps.map(step=>nav(step.id,step.title,ListChecks,undefined,false,step.completed)):undefined,true,feature.completed)):undefined))}
  </nav>;
}

function NavNode({id,title,icon:Icon,completed,expandable,selected,open,onOpen,onSelect,disabled,actions,dragProps,children}:{id:string;title:string;icon:LucideIcon;completed:boolean;expandable:boolean;selected:string;open:boolean;onOpen:(id:string,open:boolean)=>void;onSelect:(id:string)=>void;disabled:boolean;actions:ReactNode;dragProps:HTMLAttributes<HTMLDivElement>;children?:ReactNode}){
  return <Collapsible open={open} onOpenChange={value=>onOpen(id,value)}><div {...dragProps} className="group/nav-row relative flex cursor-grab active:cursor-grabbing items-center rounded-md">
    {expandable?<CollapsibleTrigger disabled={disabled} aria-label={`展开或折叠 ${title}`} className="flex size-5 shrink-0 items-center justify-center"><ChevronRight className={`size-3 transition-transform ${open?"rotate-90":""}`}/></CollapsibleTrigger>:<span className="w-5 shrink-0"/>}
    <Tooltip><TooltipTrigger render={<Button disabled={disabled} variant="ghost" className={`h-8 min-w-0 flex-1 justify-start rounded-md px-1 text-xs group-hover/nav-row:pr-14 group-focus-within/nav-row:pr-14 ${selected===id?"font-semibold":"font-normal"}`} aria-expanded={expandable?open:undefined} onClick={()=>{if(expandable)onOpen(id,!open);onSelect(id);}}>{!completed&&<Icon aria-hidden="true" className="size-4 shrink-0"/>}{completed&&<span role="img" aria-label="已完成" className="flex size-3 shrink-0 items-center justify-center rounded-full bg-green-600 text-white"><Check aria-hidden="true" className="size-2.5" strokeWidth={3}/></span>}<span className="truncate">{title}</span></Button>} /><TooltipContent>{title}</TooltipContent></Tooltip>{actions}
  </div>{expandable&&<CollapsibleContent className="ml-2 border-l pl-1">{children}</CollapsibleContent>}</Collapsible>;
}

