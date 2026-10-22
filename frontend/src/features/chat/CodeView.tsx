import { useState } from "react";
import { WrapText } from "lucide-react";
import { CodeBlock, CodeBlockActions, CodeBlockContent, CodeBlockCopyButton, CodeBlockFooter, CodeBlockHeader, CodeBlockTitle } from "@/components/ui/code-block";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

export function CodeView({ code, language = "text", maxHeight = "max-h-96" }: { code: string; language?: string; maxHeight?: string }) {
  const [expanded, setExpanded] = useState(false);
  const [wrap, setWrap] = useState(true);
  const truncated = !expanded && code.length > 16000;
  return <CodeBlock className="my-3"><CodeBlockHeader><CodeBlockTitle>{language}</CodeBlockTitle><CodeBlockActions><Tooltip><TooltipTrigger render={<Button type="button" variant="ghost" size="icon-xs" aria-label="自动换行" aria-pressed={wrap} className="aria-pressed:bg-muted aria-pressed:text-foreground" onClick={() => setWrap(value => !value)} />}><WrapText aria-hidden="true" /></TooltipTrigger><TooltipContent>{wrap ? "关闭自动换行" : "开启自动换行"}</TooltipContent></Tooltip><CodeBlockCopyButton value={code} aria-label="复制代码" /></CodeBlockActions></CodeBlockHeader><CodeBlockContent className={maxHeight} wrap={wrap} lineNumbers>{truncated ? code.slice(0, 16000) : code}</CodeBlockContent>{truncated && <CodeBlockFooter><Button variant="ghost" size="sm" onClick={() => setExpanded(true)}>展开全部（{code.length.toLocaleString()} 字符）</Button></CodeBlockFooter>}</CodeBlock>;
}
