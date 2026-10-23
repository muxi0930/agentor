import { HelpText } from "@/components/ui/help-text";
import { useEffect, useId, useState, type ReactNode } from "react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

export function Choice({ value, items, label, onChange }: { value: string; items: { value: string; label: string }[]; label: string; onChange: (value: string) => void }) {
  return <Select value={value} onValueChange={next => { if (next !== null) onChange(next); }} items={items}><SelectTrigger aria-label={label} className="w-full"><SelectValue /></SelectTrigger><SelectContent>{items.map(item => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent></Select>;
}
export function TextField({ label, value, onChange, multiline = false, type = "text", help, labelHelp, min, max }: { label: string; value: string | number; onChange: (value: string) => void; multiline?: boolean; type?: string; help?: string; labelHelp?: string; min?: number; max?: number }) {
  const id = useId();
  return <div className="grid gap-1.5 text-xs font-medium"><label htmlFor={id}>{labelHelp ? <HelpText help={labelHelp}>{label}</HelpText> : label}</label>{multiline ? <Textarea id={id} aria-describedby={help ? `${id}-help` : undefined} value={value} onChange={event => onChange(event.target.value)} /> : <Input id={id} aria-describedby={help ? `${id}-help` : undefined} type={type} min={min} max={max} value={value} onChange={event => onChange(event.target.value)} />}{help && <span id={`${id}-help`} className="font-normal text-muted-foreground">{help}</span>}</div>;
}
export function FormField({ label, children }: { label: string; children: ReactNode }) { return <div className="grid min-w-0 gap-1.5"><span className="text-xs font-medium">{label}</span>{children}</div>; }
export function JSONField({ label, value, onChange, onIssue, validate }: { label: string; value: unknown; onChange: (value: unknown) => void; onIssue: (error: string) => void; validate?: (value: unknown) => string }) {
  const [text, setText] = useState(() => JSON.stringify(value ?? {}, null, 2)), [error, setError] = useState("");
  useEffect(() => () => onIssue(""), []);
  return <div><TextField label={label} value={text} multiline onChange={text => { setText(text); try { const next = JSON.parse(text); const issue = validate?.(next) || ""; setError(issue); onIssue(issue); if (!issue) onChange(next); } catch { setError("请输入有效 JSON"); onIssue("请输入有效 JSON"); } }} />{error && <p role="alert" className="mt-1 text-xs text-destructive">{error}</p>}</div>;
}
