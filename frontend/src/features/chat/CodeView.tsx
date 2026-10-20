import { useState } from "react";
import { CodeBlock, CodeBlockActions, CodeBlockContent, CodeBlockCopyButton, CodeBlockFooter, CodeBlockHeader, CodeBlockTitle } from "@/components/ui/code-block";
import { Button } from "@/components/ui/button";

export function CodeView({ code, language = "text", maxHeight = "max-h-96" }: { code: string; language?: string; maxHeight?: string }) {
  const [expanded, setExpanded] = useState(false);
  const truncated = !expanded && code.length > 16000;
  return <CodeBlock className="my-3"><CodeBlockHeader><CodeBlockTitle>{language}</CodeBlockTitle><CodeBlockActions><CodeBlockCopyButton value={code} aria-label="复制代码" /></CodeBlockActions></CodeBlockHeader><CodeBlockContent className={maxHeight} lineNumbers>{truncated ? code.slice(0, 16000) : code}</CodeBlockContent>{truncated && <CodeBlockFooter><Button variant="ghost" size="sm" onClick={() => setExpanded(true)}>展开全部（{code.length.toLocaleString()} 字符）</Button></CodeBlockFooter>}</CodeBlock>;
}
