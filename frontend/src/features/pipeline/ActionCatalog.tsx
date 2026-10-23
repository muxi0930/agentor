import { useState } from "react";
import type { ActionSpec } from "../../../bindings/agentor/internal/pipeline/model/models";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { actionName, actionNames, parameterHelp, typeNames } from "./labels";

export function ActionCatalog({ actions }: { actions: ActionSpec[] }) {
  const [selected, setSelected] = useState<ActionSpec>();
  return <>
    <div className="overflow-hidden rounded-lg border">
      <Table aria-label="流水线动作目录" className="table-fixed">
        <TableHeader><TableRow>
          <TableHead className="w-[30%]" aria-sort="ascending">英文名</TableHead>
          <TableHead className="w-[25%]">中文名</TableHead>
          <TableHead className="w-[45%]">说明</TableHead>
        </TableRow></TableHeader>
        <TableBody>{[...actions].sort((a, b) => a.name.localeCompare(b.name, "en")).map(action => <TableRow
          key={action.name}
          tabIndex={0}
          aria-label={`查看 ${actionName(action.name)} 的详情`}
          className="cursor-pointer focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring"
          onClick={() => setSelected(action)}
          onKeyDown={event => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); setSelected(action); } }}
        >
          <TableCell className="truncate font-medium">{action.name}</TableCell>
          <TableCell className="truncate">{actionNames[action.name] || "—"}</TableCell>
          <TableCell className="truncate text-muted-foreground">{action.description}</TableCell>
        </TableRow>)}</TableBody>
      </Table>
    </div>
    <Sheet open={!!selected} onOpenChange={open => { if (!open) setSelected(undefined); }}>
      <SheetContent side="right" className="w-full sm:max-w-xl">
        <SheetHeader><SheetTitle className="break-words pr-8">{selected && actionName(selected.name)}</SheetTitle><SheetDescription>动作详情</SheetDescription></SheetHeader>
        {selected && <div className="min-h-0 flex-1 space-y-6 overflow-y-auto px-4 pb-6 text-sm">
          <dl className="space-y-3">
            <div><dt className="text-muted-foreground">英文名</dt><dd className="mt-1 break-words">{selected.name}</dd></div>
            <div><dt className="text-muted-foreground">中文名</dt><dd className="mt-1">{actionNames[selected.name] || "—"}</dd></div>
            <div><dt className="text-muted-foreground">说明</dt><dd className="mt-1 whitespace-pre-wrap break-words">{selected.description}</dd></div>
          </dl>
          <section className="space-y-3"><h3 className="font-semibold">输入参数</h3>
            {!selected.parameters?.length && <p className="text-muted-foreground">无需参数</p>}
            {selected.parameters?.map(param => <div key={param.name} className="space-y-2 rounded-lg border p-3">
              <p className="break-words font-medium">{param.name} · {parameterHelp[param.name]?.label || param.name}</p>
              <p className="text-muted-foreground">{typeNames[param.type] || param.type} · {param.required ? "必填" : "可选"}</p>
              {parameterHelp[param.name]?.help && <p>{parameterHelp[param.name].help}</p>}
            </div>)}
          </section>
          <section className="space-y-2"><h3 className="font-semibold">输出</h3><p className="whitespace-pre-wrap break-words">{selected.result}</p></section>
 <section className="space-y-2"><h3 className="font-semibold">固定结果分支</h3>{selected.branches?.map(branch=><p key={branch.name}>{branch.name} · {branch.description}</p>)}<p className="text-muted-foreground">静态动作不重试；未处理的结果与非预期异常终止流水线。</p></section>
        </div>}
      </SheetContent>
    </Sheet>
  </>;
}
