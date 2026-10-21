import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";

export function NodeActions({busy,onDelete,create,project=false}:{busy:boolean;onDelete:()=>void;create?:{label:string;onClick:()=>void};project?:boolean}){
  return <div className={`absolute right-1 top-1/2 hidden -translate-y-1/2 items-center rounded-md group-hover/nav-row:flex group-focus-within/nav-row:flex ${project?"bg-foreground text-background":"bg-accent"}`}>
    {create&&<Button variant="ghost" size="icon-xs" aria-label={create.label} title={create.label} disabled={busy} onClick={create.onClick}><Plus/></Button>}
    <Button variant="ghost" size="icon-xs" aria-label="删除当前对象" title="删除" disabled={busy} onClick={onDelete}><Trash2/></Button>
  </div>;
}
