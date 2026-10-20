import { isValidElement, lazy, Suspense, type ReactNode } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import * as Data from "../../../bindings/agentor/internal/appdata/service";
import { CodeView } from "./CodeView";

const MermaidView = lazy(() => import("./MermaidView").then(module => ({ default: module.MermaidView })));

export function MarkdownView({ text, streaming = false, onError }: { text: string; streaming?: boolean; onError: (message: string) => void }) {
  return <div className="markdown"><Markdown remarkPlugins={[remarkGfm]} components={{
    a: ({ href, children }) => <a href={href} onClick={event => {
      event.preventDefault(); if (href) void Data.OpenURL(href).catch(error => onError(String(error)));
    }}>{children}</a>,
    pre: ({ children }) => {
      if (!isValidElement<{ children?: ReactNode; className?: string }>(children)) return <pre>{children}</pre>;
      const code = String(children.props.children || "").replace(/\n$/, "");
      const language = children.props.className?.replace(/^language-/, "") || "text";
      return language === "mermaid" && !streaming ? <Suspense fallback={<CodeView code={code} language="mermaid" />}><MermaidView code={code} /></Suspense> : <CodeView code={code} language={language} />;
    },
  }}>{text}</Markdown></div>;
}
