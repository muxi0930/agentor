import { isValidElement, lazy, memo, Suspense, type ReactNode } from "react";
import Markdown, { defaultUrlTransform } from "react-markdown";
import remarkGfm from "remark-gfm";
import * as Data from "../../../bindings/agentor/internal/appdata/service";
import { CodeView } from "./CodeView";
import { HTMLView } from "./HTMLView";

const MermaidView = lazy(() => import("./MermaidView").then(module => ({ default: module.MermaidView })));

export const MarkdownView = memo(function MarkdownView({ text, streaming = false, onError }: { text: string; streaming?: boolean; onError: (message: string) => void }) {
  return <div className="markdown"><Markdown remarkPlugins={[remarkGfm]} urlTransform={url => /^wails:\/\/localhost\//i.test(url) ? url : defaultUrlTransform(url)} components={{
    a: ({ href, children }) => <a href={href} onClick={event => {
      event.preventDefault();
      if (!href) return;

      void (async () => {
        const path = href.replace(/^wails:\/\/localhost(?=\/)/i, "");
        if (/^(\/|[A-Za-z]:[\\/])/.test(path)) {
          await Data.OpenFile(decodeURIComponent(path.split("#")[0]).replace(/:\d+(?::\d+)?$/, ""));
        } else {
          await Data.OpenURL(href);
        }
      })().catch(error => onError(String(error)));
    }}>{children}</a>,
    pre: ({ children }) => {
      if (!isValidElement<{ children?: ReactNode; className?: string }>(children)) return <pre>{children}</pre>;
      const code = String(children.props.children || "").replace(/\n$/, "");
      const language = children.props.className?.replace(/^language-/, "") || "text";
      if (!streaming && language.toLowerCase() === "html") return <HTMLView source={code} />;
      return language === "mermaid" && !streaming ? <Suspense fallback={<CodeView code={code} language="mermaid" />}><MermaidView code={code} /></Suspense> : <CodeView code={code} language={language} />;
    },
  }}>{text}</Markdown></div>;
});
