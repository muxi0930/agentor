import type { ComponentProps } from "react";
import type { LucideIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

export function HeaderActionButton({ icon: Icon, label, count = 0, attention = false, animationKey = 0, onAttentionEnd, ...props }: Omit<ComponentProps<typeof Button>, "children" | "variant" | "size"> & {
  icon: LucideIcon;
  label: string;
  count?: number;
  attention?: boolean;
  animationKey?: number;
  onAttentionEnd?: () => void;
}) {
  return <Button variant="ghost" size="sm" {...props}>
    <Icon key={animationKey} aria-hidden="true" className={attention ? "notification-bell-bounce" : undefined} onAnimationEnd={onAttentionEnd} />
    <span key={`label-${animationKey}`} className="inline-flex w-[2em] shrink-0 justify-center overflow-visible">
      <span className={`shrink-0 ${attention ? "notification-label-blink" : ""}`}>{attention ? `新${label}` : label}</span>
    </span>
    {count > 0 && <Badge>{count}</Badge>}
  </Button>;
}
