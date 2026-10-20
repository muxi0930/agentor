import { useEffect, useId, useState } from "react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { CodeView } from "./CodeView";
import { errorText } from "@/app/errors";

export function MermaidView({ code }: { code: string }) {
  const id = useId().replace(/[^a-zA-Z0-9]/g, "");
  const [svg, setSvg] = useState("");
  const [error, setError] = useState("");
  const [tab, setTab] = useState("diagram");
  const [dark, setDark] = useState(document.documentElement.classList.contains("dark"));
  useEffect(() => {
    const observer = new MutationObserver(() => setDark(document.documentElement.classList.contains("dark")));
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    let cancelled = false;
    setError("");
    void import("mermaid").then(async ({ default: mermaid }) => {
      if (cancelled) return;
      mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: dark ? "dark" : "default", flowchart: { htmlLabels: false } });
      const valid = await mermaid.parse(code, { suppressErrors: true });
      if (cancelled) return;
      if (!valid) throw new Error("图表语法尚未完整或无法解析，请查看源码。");
      const result = await mermaid.render(`agentor-mermaid-${id}`, code);
      if (!cancelled) setSvg(result.svg);
    }).catch(error => { if (!cancelled) { setError(errorText(error)); setTab("source"); } });
    return () => { cancelled = true; };
  }, [code, id, dark]);
  return <Tabs value={tab} onValueChange={value => setTab(String(value))} className="my-3 min-w-0 rounded-lg border p-3">
    <TabsList><TabsTrigger value="diagram">图表</TabsTrigger><TabsTrigger value="source">源码</TabsTrigger></TabsList>
    {error && <Alert><AlertDescription>{error}</AlertDescription></Alert>}
    <TabsContent value="diagram">{svg ? <div className="overflow-auto [&_svg]:mx-auto [&_svg]:h-auto [&_svg]:max-w-full" role="img" aria-label="Mermaid 图表" dangerouslySetInnerHTML={{ __html: svg }} /> : <Skeleton className="h-40 w-full" />}</TabsContent>
    <TabsContent value="source"><CodeView code={code} language="mermaid" /></TabsContent>
  </Tabs>;
}
