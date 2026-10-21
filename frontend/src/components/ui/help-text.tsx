import type { ReactNode } from "react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

export function HelpText({ children, help }: { children: ReactNode; help: string }) {
  return <Tooltip>
    <TooltipTrigger render={<span tabIndex={0} className="relative cursor-help rounded-sm pr-2 outline-none focus-visible:ring-2 focus-visible:ring-ring" />}>
      {children}<span aria-hidden="true" className="absolute -top-1 right-0 text-[10px] leading-none text-muted-foreground">?</span>
    </TooltipTrigger>
    <TooltipContent>{help}</TooltipContent>
  </Tooltip>;
}
