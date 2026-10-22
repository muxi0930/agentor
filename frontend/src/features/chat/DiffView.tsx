import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import type { FileChange } from "../../../bindings/agentor/internal/codex/models";
import { CodeBlock, CodeBlockActions, CodeBlockContent, CodeBlockCopyButton, CodeBlockHeader, CodeBlockLine, CodeBlockTitle } from "@/components/ui/code-block";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { parseDiff, type DiffRow } from "./diff";

function DiffLines({ rows }: { rows: DiffRow[] }) {
  return rows.map((row, index) => <CodeBlockLine key={index} variant={row.kind === "added" ? "added" : row.kind === "removed" ? "removed" : "default"}>
    <span className="mr-3 inline-block w-9 select-none text-right text-muted-foreground" aria-label={row.oldLine !== null ? `原行号 ${row.oldLine}` : undefined}>{row.oldLine}</span>
    <span className="mr-3 inline-block w-9 select-none text-right text-muted-foreground" aria-label={row.newLine !== null ? `新行号 ${row.newLine}` : undefined}>{row.newLine}</span>
    {row.text}
  </CodeBlockLine>);
}

export function DiffView({ changes, diff }: { changes?: FileChange[] | null; diff?: string }) {
  const files = changes?.length ? changes : [{ path: "本轮改动", kind: { type: "update" }, diff: diff || "" }];
  return <Tabs defaultValue={files[0].path} className="min-w-0 rounded-lg border p-3">
    <TabsList className="max-w-full justify-start overflow-x-auto">{files.map(file => <Tooltip key={file.path}><TooltipTrigger render={<TabsTrigger value={file.path} className="max-w-64 truncate">{file.path.split(/[\\/]/).slice(-1)[0]}</TabsTrigger>} /><TooltipContent>{file.path}</TooltipContent></Tooltip>)}</TabsList>
    {files.map(file => {
      const rows = parseDiff(file.diff, file.kind.type);
      const additions = rows.filter(row => row.kind === "added").length;
      const deletions = rows.filter(row => row.kind === "removed").length;
      return <TabsContent key={file.path} value={file.path}><CodeBlock size="sm"><CodeBlockHeader><CodeBlockTitle>{file.path}</CodeBlockTitle><Badge variant="outline">+{additions} −{deletions}</Badge><CodeBlockActions><CodeBlockCopyButton value={file.diff} aria-label="复制 diff" /></CodeBlockActions></CodeBlockHeader><CodeBlockContent className="max-h-96"><DiffLines rows={rows} /></CodeBlockContent></CodeBlock></TabsContent>;
    })}
  </Tabs>;
}
