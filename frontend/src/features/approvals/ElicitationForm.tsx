import { useState } from "react";
import type { Approval } from "../../../bindings/agentor/internal/codex/models";
import * as Codex from "../../../bindings/agentor/internal/codex/service";
import * as Data from "../../../bindings/agentor/internal/appdata/service";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { CodeView } from "@/features/chat/CodeView";

export function ElicitationForm({ approval, busy, respond }: { approval: Approval; busy: boolean; respond: (action: () => Promise<void>) => Promise<void> }) {
  const [values, setValues] = useState<Record<string, unknown>>(() => Object.fromEntries((approval.fields || []).flatMap(field => {
    if (!field.defaultJSON || field.defaultJSON === "null") return [];
    try { return [[field.name, JSON.parse(field.defaultJSON)]]; } catch { return []; }
  })));
  const [raw, setRaw] = useState("{}");
  const verification = approval.mode === "openai/userVerification";
  const form = approval.mode !== "url" && !verification;
  const set = (key: string, value: unknown) => setValues(old => ({ ...old, [key]:value }));
  return <form className="space-y-4" onSubmit={event => { event.preventDefault(); void respond(() => Codex.AnswerElicitation(approval.id, "accept", approval.fields?.length ? JSON.stringify(values) : raw)); }}>
    <p className="text-xs text-muted-foreground">MCP 服务：{approval.serverName}</p><p className="whitespace-pre-wrap">{approval.message}</p>
    {approval.mode === "url" && <><p className="break-all text-xs">{approval.url}</p><Button type="button" variant="outline" disabled={busy} onClick={() => void respond(() => Data.OpenURL(approval.url))}>在浏览器继续</Button></>}
    {verification && <p>该请求需要设备身份验证。请在原服务完成验证，或取消此请求。</p>}
    {form && (approval.fields?.length ? approval.fields.map(field => {
      const id = `${approval.generation}-${approval.id}-${field.name}`;
      const value = values[field.name];
      return <Field key={field.name}><FieldLabel htmlFor={id}>{field.title}{field.required ? " *" : ""}</FieldLabel>
        {field.type === "array" ? <div className="flex flex-wrap gap-2">{(field.options || []).map(option => {
          const selected = Array.isArray(value) ? value as string[] : [];
          return <Button key={option.value} type="button" disabled={busy} variant={selected.includes(option.value) ? "default" : "outline"} aria-pressed={selected.includes(option.value)} onClick={() => set(field.name, selected.includes(option.value) ? selected.filter(item => item !== option.value) : [...selected, option.value])}>{option.label}</Button>;
        })}</div> : field.type === "boolean" || field.options?.length ? <Select required={field.required} disabled={busy} value={value == null ? "" : String(value)} onValueChange={selectedValue => { if (selectedValue === null) return; set(field.name, selectedValue === "" ? undefined : field.type === "boolean" ? selectedValue === "true" : selectedValue); }} items={[{ value: "", label: "请选择" }, ...(field.type === "boolean" ? [({ value: "true", label: "是" }), ({ value: "false", label: "否" })] : (field.options || []).map(option => ({ value: option.value, label: option.label })))]}><SelectTrigger id={id}><SelectValue /></SelectTrigger><SelectContent>
          <SelectItem value="">请选择</SelectItem>{field.type === "boolean" ? <><SelectItem value="true">是</SelectItem><SelectItem value="false">否</SelectItem></> : (field.options || []).map(option => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}
        </SelectContent></Select> : <Input id={id} disabled={busy} required={field.required} autoComplete="off" type={field.type === "number" || field.type === "integer" ? "number" : field.format === "email" ? "email" : field.format === "uri" ? "url" : field.format === "date" ? "date" : "text"} step={field.type === "integer" ? 1 : "any"} min={field.minimum ?? undefined} max={field.maximum ?? undefined} minLength={field.minLength ?? undefined} maxLength={field.maxLength ?? undefined} value={value == null ? "" : String(value)} onChange={event => set(field.name, event.target.value === "" ? undefined : field.type === "number" || field.type === "integer" ? Number(event.target.value) : event.target.value)} />}
        {field.description && <FieldDescription>{field.description}</FieldDescription>}
      </Field>;
    }) : <><CodeView code={approval.schemaJSON || "{}"} language="请求表单定义" /><Field><FieldLabel htmlFor="elicitation-json">JSON 回答</FieldLabel><Textarea id="elicitation-json" disabled={busy} value={raw} onChange={event => setRaw(event.target.value)} /></Field></>)}
    <div className="flex flex-wrap gap-2">{!verification && <Button type="submit" size="sm" disabled={busy}>{approval.mode === "url" ? "已完成，继续" : "提交回答"}</Button>}<Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void respond(() => Codex.AnswerElicitation(approval.id, "decline", ""))}>拒绝</Button><Button type="button" size="sm" variant="ghost" disabled={busy} onClick={() => void respond(() => Codex.AnswerElicitation(approval.id, "cancel", ""))}>取消</Button></div>
  </form>;
}
