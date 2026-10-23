import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { useState } from "react";
import { Slider } from "@animate-ui/react/primitives/base-ui/slider";
import { ArrowLeft, ChevronDown, ChevronRight, Zap } from "lucide-react";
import type { Settings } from "../../../bindings/agentor/internal/appdata/model/models";
import type { Model } from "../../../bindings/agentor/internal/codex/model/models";
import * as Data from "../../../bindings/agentor/internal/appdata/service";
import { Button } from "@/components/ui/button";
import { SelectionList, SelectionListItem } from "@/components/ui/selection-list";
import { Popover, PopoverContent, PopoverTitle, PopoverTrigger } from "@/components/ui/popover";
import { errorText } from "@/app/errors";

const effortLabels: Record<string, string> = { none: "无", minimal: "最少", low: "轻度", medium: "标准", high: "深度", xhigh: "超高", max: "极高", ultra: "最高" };

export function ModelEffortSelect({ models, settings, onSettings, onError, disabled = false }: {
  models: Model[]; settings: Settings; onSettings: (settings: Settings) => void; onError: (message: string) => void; disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [view, setView] = useState<"effort" | "models">("effort");
  const [saving, setSaving] = useState(false);
  const [draftEffort, setDraftEffort] = useState<string | null>(null);
  const model = settings.model ? models.find(model => model.model === settings.model) : models.find(model => model.isDefault);
  const name = model?.displayName || settings.model || "默认";
  const options = model?.supportedReasoningEfforts || [];
  const effort = draftEffort ?? (settings.effort || model?.defaultReasoningEffort || "");
  const label = effortLabels[effort] || effort || "默认强度";
  const index = Math.max(0, options.findIndex(option => option.reasoningEffort === effort));
  const supportsFast = !!model?.serviceTiers?.some(tier => tier.id === "fast" || tier.id === "priority");
  const locked = disabled || saving;

  async function save(action: () => Promise<Settings>) {
    setSaving(true);
    try {
      onSettings(await action());
      return true;
    } catch (error) {
      setDraftEffort(null);
      onError(errorText(error));
      return false;
    } finally {
      setSaving(false);
    }
  }

  async function selectModel(value: string) {
    const selected = value ? models.find(model => model.model === value) : models.find(model => model.isDefault);
    const saved = await save(async () => {
      if (settings.fastMode && !selected?.serviceTiers?.some(tier => tier.id === "fast" || tier.id === "priority")) {
        onSettings(await Data.SetFastMode(false));
      }
      return Data.SetModelSelection(value, selected?.defaultReasoningEffort || "");
    });
    if (saved) {
      setDraftEffort(null);
      setView("effort");
    }
  }

  return <Popover open={open} onOpenChange={value => { setOpen(value); setView("effort"); setDraftEffort(null); }}>
    <PopoverTrigger render={<Button type="button" variant="ghost" size="sm" disabled={locked} className="h-8 max-w-full gap-1.5 rounded-xl px-2.5 font-normal" aria-label={`模型与思考强度：${name}，${label}${settings.fastMode ? "，Fast 已开启" : ""}`} />}>
      {settings.fastMode && <Zap aria-hidden="true" className="fill-current text-primary" />}
      <span className="truncate">{name}</span><span className="shrink-0 text-muted-foreground">{label}</span><ChevronDown aria-hidden="true" className="size-3 text-muted-foreground" />
    </PopoverTrigger>
    <PopoverContent side="top" align="start" sideOffset={6.5} style={{ zoom: 0.65 }} className="w-80 max-w-[calc(100vw-2rem)] gap-0 rounded-3xl p-3 shadow-lg">
      {view === "effort" ? <>
        <div className="grid grid-cols-[2rem_minmax(0,1fr)_2rem] items-start gap-2">
          <Tooltip><TooltipTrigger render={<Button type="button" variant="ghost" size="icon" disabled={locked || !supportsFast} aria-label="Fast 模式" aria-pressed={settings.fastMode} className={`rounded-full ${settings.fastMode ? "text-primary" : "text-muted-foreground"}`} onClick={() => void save(() => Data.SetFastMode(!settings.fastMode))}><Zap aria-hidden="true" className={settings.fastMode ? "fill-current" : undefined} /></Button>} /><TooltipContent>{supportsFast ? (settings.fastMode ? "关闭 Fast 模式" : "开启 Fast 模式") : "当前模型未提供 Fast 模式"}</TooltipContent></Tooltip>
          <div className="min-w-0 text-center">
            <PopoverTitle className="py-0.5 text-lg font-semibold text-primary" aria-live="polite">{label}</PopoverTitle>
            <Button type="button" variant="ghost" size="sm" disabled={locked} className="max-w-full gap-0.5 font-normal text-muted-foreground" onClick={() => setView("models")} aria-label="选择模型"><span className="truncate">{name}</span><ChevronRight aria-hidden="true" className="size-3.5" /></Button>
          </div>
        </div>
        {options.length > 0 ? <Slider.Root value={index} min={0} max={Math.max(1, options.length - 1)} step={1} largeStep={1} thumbAlignment="edge" disabled={locked || options.length < 2} className="mx-1 mt-3 mb-1" onValueChange={value => setDraftEffort(options[value].reasoningEffort)} onValueCommitted={value => {
          const next = options[value].reasoningEffort;
          if (next !== (settings.effort || model?.defaultReasoningEffort)) void save(() => Data.SetModelSelection(settings.model, next));
        }}>
          <Slider.Control className="relative flex h-10 w-full touch-none items-center">
            <Slider.Track className="relative h-7 w-full rounded-full border border-border bg-muted">
              <div className="pointer-events-none absolute inset-x-4 inset-y-0 flex items-center justify-between" aria-hidden="true">{options.map(option => <span key={option.reasoningEffort} className="size-1.5 rounded-full bg-muted-foreground/40" />)}</div>
            </Slider.Track>
            <Slider.Thumb aria-label="思考强度" getAriaValueText={(_, value) => effortLabels[options[value].reasoningEffort] || options[value].reasoningEffort} className="size-9 rounded-full border border-border bg-background shadow-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50" />
          </Slider.Control>
        </Slider.Root> : <p className="px-2 pt-4 pb-2 text-center text-xs text-muted-foreground">当前模型未提供可调整的思考强度</p>}
      </> : <>
        <div className="mb-1 flex items-center gap-2 px-1">
          <Button type="button" variant="ghost" size="icon-sm" aria-label="返回思考强度" onClick={() => setView("effort")}><ArrowLeft aria-hidden="true" /></Button>
          <PopoverTitle className="text-[20px] text-muted-foreground">选择模型</PopoverTitle>
        </div>
        <SelectionList aria-label="模型列表">
          <SelectionListItem disabled={locked} selected={!settings.model} description={settings.reuseCLI ? "CLI 默认模型" : "账号默认模型"} onClick={() => void selectModel("")}>默认</SelectionListItem>
          {models.map(option => <SelectionListItem key={option.id} disabled={locked} selected={settings.model === option.model} onClick={() => void selectModel(option.model)}>{option.displayName}</SelectionListItem>)}
        </SelectionList>
      </>}
    </PopoverContent>
  </Popover>;
}
