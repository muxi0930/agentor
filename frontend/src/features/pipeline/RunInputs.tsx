import { SlidersHorizontal } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { useEffect, useId, useState } from "react";
import { typeNames } from "./labels";
import type { Task } from "../../../bindings/agentor/internal/pipeline/model/models";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Choice } from "./GraphControls";

type Props = {
  task: Task;
  popup?: boolean;
  values: Record<string, unknown>;
  onChange: (values: Record<string, unknown>) => void;
  onValidityChange?: (valid: boolean) => void;
};

export function RunInputs({ task, values, onChange, onValidityChange, popup = false }: Props) {
  const prefix = useId();
  const [open, setOpen] = useState(false);
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [issues, setIssues] = useState<Record<string, boolean>>({});
  const valid = !(task.inputs || []).some(field => issues[field.name]);
  useEffect(() => { onValidityChange?.(valid); }, [valid, onValidityChange]);

  function update(name: string, value: unknown) {
    const next = { ...values };
    if (value === undefined) delete next[name]; else next[name] = value;
    onChange(next);
  }

  if (!task.inputs?.length) return <p className="text-sm text-muted-foreground">此流水线无需填写运行参数。</p>;
  const fields = <div className={popup ? "max-h-[50dvh] space-y-3 overflow-y-auto" : "grid gap-3 rounded-lg border bg-muted/15 p-3 @md/pipeline-form:grid-cols-2"}>
    {task.inputs.map(field => {
      const id = `${prefix}-${field.name}`;
      return <Field key={field.name}>
        <FieldLabel htmlFor={id}>{field.name} · {typeNames[field.type]}{field.required ? "（必填）" : "（可选）"}</FieldLabel>
        {field.type === "boolean" ? <Choice label={`运行输入 ${field.name}`} value={values[field.name] === undefined ? "" : String(values[field.name])} items={[{ value: "", label: "未填写" }, { value: "true", label: "是" }, { value: "false", label: "否" }]} onChange={value => update(field.name, value === "" ? undefined : value === "true")} />
          : field.type === "any" ? <>
            <Textarea id={id} aria-invalid={!!issues[field.name]} aria-describedby={issues[field.name] ? `${id}-error` : undefined} placeholder={'填写 JSON，例如 {"message":"开始"} 或 "一段文字"'} value={drafts[field.name] ?? (values[field.name] === undefined ? "" : JSON.stringify(values[field.name], null, 2))} onChange={event => {
              const text = event.target.value;
              setDrafts(current => ({ ...current, [field.name]: text }));
              try {
                const value = text.trim() ? JSON.parse(text) : undefined;
                setIssues(current => ({ ...current, [field.name]: false }));
                update(field.name, value);
              } catch { setIssues(current => ({ ...current, [field.name]: true })); }
            }} />
            {issues[field.name] && <p id={`${id}-error`} role="alert" className="text-xs text-destructive">请输入有效 JSON，文字请用双引号包裹。</p>}
          </> : <Input id={id} type={field.type === "number" ? "number" : "text"} value={values[field.name] === undefined ? "" : String(values[field.name])} onChange={event => update(field.name, event.target.value === "" ? undefined : field.type === "number" ? Number(event.target.value) : event.target.value)} />}
      </Field>;
    })}
  </div>;
  if (!popup) return fields;
  return <Dialog open={open} onOpenChange={setOpen}><DialogTrigger render={<Button type="button" variant="outline" />}><SlidersHorizontal />运行输入 · {task.inputs.length}</DialogTrigger><DialogContent className="sm:max-w-[540px]"><DialogHeader><DialogTitle>运行输入</DialogTitle><DialogDescription>填写本次流水线运行需要的参数。</DialogDescription></DialogHeader>{fields}<DialogFooter><Button type="button" onClick={() => setOpen(false)}>完成</Button></DialogFooter></DialogContent></Dialog>;
}
