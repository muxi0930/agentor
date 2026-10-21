import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

export const permissionLabels: Record<string, string> = { "read-only":"只读", "workspace-write":"项目内读写", "danger-full-access":"完全访问" };

export function PermissionSelect({ value, onChange, inherit, disabled, id, label = "执行权限", className, size }: { value: string; onChange: (mode: string) => void; inherit?: string; disabled?: boolean; id?: string; label?: string; className?: string; size?: "sm" | "default" }) {
  return <Select disabled={disabled} value={value} onValueChange={selectedValue => { if (selectedValue === null) return; onChange(selectedValue); }} items={[...(inherit ? [({ value: "", label: "继承（" + inherit + "）" })] : []), ...(Object.entries(permissionLabels).map(([value, label]) => ({ value: value, label: label })))]}><SelectTrigger id={id} aria-label={label} size={size} className={className || "text-xs"}><SelectValue /></SelectTrigger><SelectContent>
    {inherit && <SelectItem value="">继承（{inherit}）</SelectItem>}
    {Object.entries(permissionLabels).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}
  </SelectContent></Select>;
}
