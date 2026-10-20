import { CodeBlock, CodeBlockActions, CodeBlockContent, CodeBlockCopyButton, CodeBlockHeader, CodeBlockTitle } from "@/components/ui/code-block";

export function CodeView({ code, language = "text", maxHeight = "max-h-96" }: { code: string; language?: string; maxHeight?: string }) {
  return <CodeBlock className="my-3"><CodeBlockHeader><CodeBlockTitle>{language}</CodeBlockTitle><CodeBlockActions><CodeBlockCopyButton value={code} aria-label="复制代码" /></CodeBlockActions></CodeBlockHeader><CodeBlockContent className={maxHeight} lineNumbers>{code}</CodeBlockContent></CodeBlock>;
}
