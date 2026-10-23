import { useEffect, useState, type ComponentProps } from "react";
import { AnimatePresence, motion } from "motion/react";
import { Check, Copy } from "lucide-react";
import { Button } from "@/components/ui/button";

// 复制成功后切换图标，并保留调用方的错误反馈
export function CopyButton({ value, onCopyError, onClick, ...props }: Omit<ComponentProps<typeof Button>, "children"> & {
  value: string;
  onCopyError?: (error: unknown) => void;
}) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timeout = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timeout);
  }, [copied]);

  return <>
    <Button type="button" variant="ghost" size="icon-xs" data-copied={copied || undefined} aria-label={copied ? "已复制" : "复制"} {...props} onClick={event => {
      onClick?.(event);
      if (event.defaultPrevented) return;
      void navigator.clipboard.writeText(value).then(() => setCopied(true)).catch(error => onCopyError?.(error));
    }}>
      <AnimatePresence initial={false} mode="wait">
        <motion.span key={copied ? "copied" : "copy"} className="inline-flex" initial={{ opacity: 0, scale: 0.6, rotate: -45 }} animate={{ opacity: 1, scale: 1, rotate: 0 }} exit={{ opacity: 0, scale: 0.6, rotate: 45 }} transition={{ duration: 0.12 }}>
          {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
        </motion.span>
      </AnimatePresence>
    </Button>
    <span aria-live="polite" className="sr-only">{copied ? "已复制" : ""}</span>
  </>;
}
