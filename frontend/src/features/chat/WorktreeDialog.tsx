import { useState } from "react";
import { GitBranch } from "lucide-react";
import * as Data from "../../../bindings/agentor/internal/appdata/service";
import type { Settings } from "../../../bindings/agentor/internal/appdata/models";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { errorText } from "@/app/errors";

export function WorktreeDialog({ settings, onSettings }: { settings: Settings; onSettings: (settings: Settings) => void }) {
  const [open, setOpen] = useState(false);
  const [branch, setBranch] = useState("");
  const [directory, setDirectory] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  return <><Button variant="ghost" size="icon" aria-label="创建 Git worktree" disabled={!settings.projectDirectory} onClick={() => setOpen(true)}><GitBranch /></Button>
    <Dialog open={open} onOpenChange={value => { if (!busy) setOpen(value); }}><DialogContent>
      <DialogHeader><DialogTitle>创建 Git worktree</DialogTitle><DialogDescription>从当前 HEAD 新建分支和工作目录。成功后将该目录设为当前项目。</DialogDescription></DialogHeader>
      <form className="space-y-5" onSubmit={event => { event.preventDefault(); setBusy(true); setError(""); void Data.CreateWorktree(settings.projectDirectory, branch.trim(), directory.trim()).then(value => { onSettings(value); setOpen(false); setBranch(""); setDirectory(""); }).catch(error => setError(errorText(error))).finally(() => setBusy(false)); }}>
        <p className="break-all text-xs text-muted-foreground">源项目：{settings.projectDirectory}</p>
        {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
        <FieldGroup><Field><FieldLabel htmlFor="worktree-branch">新分支</FieldLabel><Input id="worktree-branch" required value={branch} disabled={busy} onChange={event => setBranch(event.target.value)} placeholder="feature/my-task" /></Field><Field><FieldLabel htmlFor="worktree-directory">新目录完整路径</FieldLabel><Input id="worktree-directory" required value={directory} disabled={busy} onChange={event => setDirectory(event.target.value)} /></Field></FieldGroup>
        <DialogFooter><Button type="button" variant="outline" disabled={busy} onClick={() => setOpen(false)}>取消</Button><Button type="submit" disabled={busy || !branch.trim() || !directory.trim()}>{busy ? "正在创建…" : "创建"}</Button></DialogFooter>
      </form>
    </DialogContent></Dialog>
  </>;
}
