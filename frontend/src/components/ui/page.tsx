import type { ComponentProps, ReactNode } from "react";
import { cn } from "cn";

export function Page({ className, ...props }: ComponentProps<"div">) {
  return <div data-slot="page" className={cn("mx-auto w-full min-w-0 max-w-3xl space-y-6 p-6 lg:p-10", className)} {...props} />;
}

export function PageHeader({ type, typeIcon, title, description, actions, children }: { type?: string; typeIcon?: ReactNode; title: ReactNode; description?: ReactNode; actions?: ReactNode; children?: ReactNode }) {
  return <header data-slot="page-header" className="space-y-6 rounded-lg border p-4">
    {type && <p className="flex items-center gap-2 text-sm font-medium text-muted-foreground">{typeIcon}{type}</p>}
    <div className="flex flex-wrap items-center gap-3">
      <div className="mr-auto min-w-0"><h1 className="break-words text-2xl font-semibold tracking-tight">{title}</h1>{description && <div className="mt-2 text-sm text-muted-foreground">{description}</div>}</div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
    {children}
  </header>;
}

export function PageContent({ className, ...props }: ComponentProps<"div">) {
  return <div data-slot="page-content" className={cn("min-w-0 space-y-6", className)} {...props} />;
}
