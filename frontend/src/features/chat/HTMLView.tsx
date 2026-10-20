import { createContext, useContext, useEffect, useRef, useState } from "react";
import { Code, Eye, RefreshCw } from "lucide-react";
import * as Files from "../../../bindings/agentor/internal/artifact/service";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { Skeleton } from "@/components/ui/skeleton";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { CodeView } from "./CodeView";

export const PreviewContext = createContext<{ threadId: string; followup: (text: string) => void } | null>(null);

export function HTMLView({ source }: { source: string }) {
  const context = useContext(PreviewContext);
  const [preview, setPreview] = useState(false);
  const [url, setURL] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [suggestion, setSuggestion] = useState("");
  const [height, setHeight] = useState(400);
  const [width, setWidth] = useState("100%");
  const [revision, setRevision] = useState(0);
  const frame = useRef<HTMLIFrameElement>(null);
  const threadId = context?.threadId;

  useEffect(() => {
    if (!preview || !threadId) return;
    let disposed = false;
    let resource = "";
    setLoading(true); setError(""); setURL(""); setSuggestion("");
    void Files.CreateHTMLPreview(threadId, source).then(value => {
      resource = value;
      if (disposed) void Files.ReleaseHTMLPreview(value);
      else setURL(value);
    }).catch(error => { if (!disposed) { setError(String(error)); setLoading(false); } });
    return () => { disposed = true; if (resource) void Files.ReleaseHTMLPreview(resource); };
  }, [preview, threadId, source, revision]);

  useEffect(() => {
    function receive(event: MessageEvent) {
      if (!frame.current || event.source !== frame.current.contentWindow || event.data?.channel !== "agentor-preview") return;
      if (event.data.type === "resize" && Number.isFinite(event.data.height)) setHeight(Math.min(1000, Math.max(180, event.data.height + 8)));
      if (event.data.type === "error" && typeof event.data.text === "string") setError(event.data.text);
      if (event.data.type === "followup" && typeof event.data.text === "string") setSuggestion(event.data.text);
    }
    window.addEventListener("message", receive);
    return () => window.removeEventListener("message", receive);
  }, []);

  if (!context) return <CodeView code={source} language="html" />;
  return <Card className="my-3 gap-3 overflow-hidden py-3">
    <CardHeader className="flex flex-row items-center gap-2 px-3"><CardTitle className="mr-auto text-sm">HTML 可视化</CardTitle>
      <Button variant="ghost" size="sm" onClick={() => setPreview(!preview)}>{preview ? <Code /> : <Eye />}{preview ? "查看源码" : "预览 HTML"}</Button>
      {preview && <><NativeSelect aria-label="预览宽度" value={width} onChange={event => setWidth(event.target.value)} className="h-8 w-28 text-xs"><NativeSelectOption value="100%">适应宽度</NativeSelectOption><NativeSelectOption value="640px">640 px</NativeSelectOption><NativeSelectOption value="960px">960 px</NativeSelectOption></NativeSelect><Button variant="ghost" size="icon-sm" aria-label="重新加载预览" onClick={() => setRevision(value => value + 1)}><RefreshCw /></Button></>}
    </CardHeader>
    <CardContent className="space-y-3 px-3">
      {!preview ? <CodeView code={source} language="html" /> : <>
        {loading && <Skeleton className="h-40 w-full" aria-label="正在加载预览" />}
        {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
        {url && <div className="mx-auto max-w-full overflow-hidden rounded-md border bg-white" style={{ width, height, minHeight:180, maxHeight:1000, resize:"vertical" }}><iframe ref={frame} src={url} title="HTML 可视化预览" sandbox="allow-scripts" referrerPolicy="no-referrer" className="size-full border-0" onLoad={() => setLoading(false)} onError={() => { setLoading(false); setError("预览资源加载失败，请重试。"); }} /></div>}
        {suggestion && <div className="space-y-2 rounded-md bg-muted p-3 text-sm"><p className="whitespace-pre-wrap">{suggestion}</p><Button size="sm" onClick={() => { context.followup(suggestion); setSuggestion(""); }}>将此提问填入输入框</Button><Button size="sm" variant="ghost" onClick={() => setSuggestion("")}>忽略</Button></div>}
      </>}
    </CardContent>
  </Card>;
}
