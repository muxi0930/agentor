import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";

export const permissionLabels: Record<string, string> = { "read-only":"只读", "workspace-write":"项目内读写", "danger-full-access":"完全访问" };

export function PermissionSelect({ value, onChange, inherit, disabled, id, label = "执行权限" }: { value: string; onChange: (mode: string) => void; inherit?: string; disabled?: boolean; id?: string; label?: string }) {
  return <NativeSelect id={id} aria-label={label} disabled={disabled} value={value} onChange={event => onChange(event.target.value)} className="text-xs">
    {inherit && <NativeSelectOption value="">继承（{inherit}）</NativeSelectOption>}
    {Object.entries(permissionLabels).map(([value, label]) => <NativeSelectOption key={value} value={value}>{label}</NativeSelectOption>)}
  </NativeSelect>;
}
