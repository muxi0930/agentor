import { Children, isValidElement, type ComponentProps, type ReactNode } from "react";
import { Highlight, HighlightItem } from "@animate-ui/react/primitives/effects/highlight";
import { AnimatePresence, motion } from "motion/react";
import { Check } from "lucide-react";
import { cn } from "cn";
import { Button } from "@/components/ui/button";

export function SelectionList({ className, children, ...props }: ComponentProps<"div">) {
  const items = Children.toArray(children);
  const selected = items.findIndex(child => isValidElement<{ selected?: boolean }>(child) && child.props.selected);

  return <div role="group" className={cn("max-h-80 overflow-y-auto", className)} {...props}>
    <Highlight controlledItems value={selected < 0 ? null : String(selected)} click={false} className="inset-0 rounded-2xl bg-accent">
      {items.map((child, index) => isValidElement(child) ? <HighlightItem key={child.key ?? index} value={String(index)}>{child}</HighlightItem> : child)}
    </Highlight>
  </div>;
}

export function SelectionListItem({ selected, description, children, className, ...props }: ComponentProps<typeof Button> & { selected: boolean; description?: ReactNode }) {
  return <Button type="button" variant="ghost" aria-pressed={selected} className={cn("w-full justify-between gap-3 rounded-2xl px-3 text-left text-[22px] font-normal", description ? "h-auto py-2.5" : "h-10", className)} {...props}>
    <span className="min-w-0"><span className={description ? "block truncate font-medium" : "block truncate"}>{children}</span>{description && <span className="block text-[18px] text-muted-foreground">{description}</span>}</span>
    <AnimatePresence initial={false}>
      {selected && <motion.span className="inline-flex shrink-0" initial={{ opacity: 0, scale: 0.6 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.6 }} transition={{ duration: 0.14 }}><Check aria-hidden="true" /></motion.span>}
    </AnimatePresence>
  </Button>;
}
