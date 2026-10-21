import type { ComponentProps, ReactNode } from "react";
import { Field } from "@/components/ui/field";
import { cn } from "cn";
import { AccordionItem, AccordionTrigger, AccordionContent } from "@/components/ui/accordion";

export function SettingsSection({ value, title, description, children }: {
  value: string; title: ReactNode; description?: string; children: ReactNode;
}) {
  return <AccordionItem value={value} className="rounded-xl bg-card text-card-foreground ring-1 ring-foreground/10 not-last:border-b-0">
    <AccordionTrigger className="items-center gap-4 p-4 hover:no-underline">
      <span className="min-w-0 space-y-1">
        <span className="block text-base font-medium">{title}</span>
        {description && <span className="block text-sm font-normal text-muted-foreground">{description}</span>}
      </span>
    </AccordionTrigger>
    <AccordionContent keepMounted className="px-4 pb-4">{children}</AccordionContent>
  </AccordionItem>;
}

export function SettingsField({ className, ...props }: ComponentProps<typeof Field>) {
  return <Field className={cn("grid grid-cols-1 items-center gap-x-4 gap-y-2 sm:grid-cols-[8rem_minmax(0,1fr)] [&>[data-slot=field-label]]:w-auto [&>[data-slot=field-label]]:self-center [&>[data-slot=field-description]]:col-span-full [&>[data-slot=field-description]]:mt-0 [&>div]:min-w-0", className)} {...props}/>;
}
