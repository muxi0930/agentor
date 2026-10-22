import type { ComponentProps, ReactNode } from "react";
import { Check } from "lucide-react";
import { cn } from "cn";
import { Button } from "@/components/ui/button";

export function SelectionList({ className, ...props }: ComponentProps<"div">) {
  return <div role="group" className={cn("max-h-80 overflow-y-auto", className)} {...props} />;
}

export function SelectionListItem({ selected, description, children, className, ...props }: ComponentProps<typeof Button> & { selected: boolean; description?: ReactNode }) {
  return <Button type="button" variant="ghost" aria-pressed={selected} className={cn("w-full justify-between gap-3 rounded-2xl px-3 text-left text-[22px] font-normal", description ? "h-auto py-2.5" : "h-10", className)} {...props}>
    <span className="min-w-0"><span className={description ? "block truncate font-medium" : "block truncate"}>{children}</span>{description && <span className="block text-[18px] text-muted-foreground">{description}</span>}</span>
    {selected && <Check aria-hidden="true" />}
  </Button>;
}
