import { useState } from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "cn";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { SelectionList, SelectionListItem } from "@/components/ui/selection-list";

export const permissionLabels: Record<string, string> = { "read-only":"只读", "workspace-write":"项目内读写", "danger-full-access":"完全访问" };

export function PermissionSelect({ value, onChange, inherit, disabled, id, label = "执行权限", className, size }: { value: string; onChange: (mode: string) => void; inherit?: string; disabled?: boolean; id?: string; label?: string; className?: string; size?: "sm" | "default" }) {
  const [open, setOpen] = useState(false);
  const options = [...(inherit ? [{ value: "", label: `继承（${inherit}）` }] : []), ...Object.entries(permissionLabels).map(([value, label]) => ({ value, label }))];
  return <Popover open={open} onOpenChange={setOpen}>
    <PopoverTrigger render={<Button type="button" variant="outline" size={size} disabled={disabled} id={id} aria-label={label} className={cn("w-full justify-between font-normal", className || "text-xs")} />}><span className="truncate">{options.find(option => option.value === value)?.label || "选择权限"}</span><ChevronDown aria-hidden="true" className="size-3.5 text-muted-foreground" /></PopoverTrigger>
    <PopoverContent side="top" align="start" sideOffset={6.5} style={{ zoom: 0.65 }} className="w-80 max-w-[calc(100vw-2rem)] gap-0 rounded-3xl p-3 shadow-lg" aria-label={label}>
      <SelectionList aria-label={label}>{options.map(option => <SelectionListItem key={option.value} selected={value === option.value} disabled={disabled} onClick={() => { onChange(option.value); setOpen(false); }}>{option.label}</SelectionListItem>)}</SelectionList>
    </PopoverContent>
  </Popover>;
}
