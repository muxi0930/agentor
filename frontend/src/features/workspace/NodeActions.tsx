import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";

export function NodeActions({busy,onDelete,create,project=false}:{busy:boolean;onDelete:()=>void;create?:{label:string;onClick:()=>void};project?:boolean}){
  return <div className={`absolute right-1 top-1/2 hidden -translate-y-1/2 items-center rounded-md group-hover/nav-row:flex group-focus-within/nav-row:flex ${project?"bg-foreground text-background":"bg-accent"}`}>
    {create&&<Tooltip><TooltipTrigger render={<Button variant="ghost" size="icon-xs" aria-label={create.label} disabled={busy} onClick={create.onClick}><Plus/></Button>} /><TooltipContent>{create.label}</TooltipContent></Tooltip>}
    <Tooltip><TooltipTrigger render={<Button variant="ghost" size="icon-xs" aria-label="删除当前对象" disabled={busy} onClick={onDelete}><Trash2/></Button>} /><TooltipContent>删除</TooltipContent></Tooltip>
  </div>;
}
