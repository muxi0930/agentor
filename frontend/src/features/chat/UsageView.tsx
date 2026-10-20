import type { TokenUsage } from "../../../bindings/agentor/internal/codex/models";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

export function UsageView({ usage }: { usage?: TokenUsage }) {
  return <Tooltip><TooltipTrigger className="text-[11px] text-muted-foreground underline decoration-dotted underline-offset-4">
    {usage ? `最近 ${usage.last.totalTokens.toLocaleString()} tokens` : "用量未知"}
  </TooltipTrigger><TooltipContent><dl className="grid grid-cols-2 gap-x-5 gap-y-1 text-xs">
    <dt>最近输入</dt><dd>{usage?.last.inputTokens.toLocaleString() ?? "未知"}</dd>
    <dt>最近输出</dt><dd>{usage?.last.outputTokens.toLocaleString() ?? "未知"}</dd>
    <dt>其中思考</dt><dd>{usage?.last.reasoningOutputTokens.toLocaleString() ?? "未知"}</dd>
    <dt>缓存输入</dt><dd>{usage?.last.cachedInputTokens.toLocaleString() ?? "未知"}</dd>
    <dt>会话累计</dt><dd>{usage?.total.totalTokens.toLocaleString() ?? "未知"}</dd>
    <dt>上下文上限</dt><dd>{usage?.modelContextWindow?.toLocaleString() ?? "未知"}</dd>
  </dl></TooltipContent></Tooltip>;
}
