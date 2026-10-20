import { useEffect, useState } from "react";
import { ShieldCheck } from "lucide-react";
import { Events } from "@wailsio/runtime";
import * as Codex from "../../../bindings/agentor/internal/codex/service";
import type { Approval, Status } from "../../../bindings/agentor/internal/codex/models";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Questionnaire, QuestionnaireActions, QuestionnaireChoice, QuestionnaireChoices, QuestionnaireDescription, QuestionnaireError, QuestionnaireInput, QuestionnaireItem, QuestionnaireNext, QuestionnairePrevious, QuestionnaireProgress, QuestionnaireSubmit, QuestionnaireTitle } from "@/components/ui/questionnaire";
import { errorText } from "@/app/errors";

const labels: Record<string, string> = { accept: "允许本次", acceptForSession: "本会话允许", decline: "拒绝", cancel: "拒绝并停止" };

function ApprovalCard({ approval, refresh }: { approval: Approval; refresh: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function respond(action: () => Promise<void>) {
    setBusy(true); setError("");
    try { await action(); refresh(); } catch (error) { setError(errorText(error)); }
    finally { setBusy(false); }
  }
  const pending = approval.status === "pending";
  return <Card>
    <CardHeader className="gap-2"><CardTitle className="flex items-center justify-between gap-2 text-sm">{approval.method === "item/tool/requestUserInput" ? "Codex 需要您的回答" : approval.method === "item/fileChange/requestApproval" ? "文件修改审批" : "命令执行审批"}<Badge variant="outline">{pending ? "待处理" : approval.status === "resolved" ? "已处理" : "已失效"}</Badge></CardTitle>
      <p className="break-all text-xs text-muted-foreground">会话 {approval.threadId}<br />轮次 {approval.turnId}</p>
    </CardHeader>
    <CardContent className="space-y-3 text-sm">
      {approval.reason && <p>{approval.reason}</p>}
      {approval.command && <pre className="max-h-60 overflow-auto whitespace-pre-wrap rounded-lg bg-muted p-3 text-xs">{approval.command}</pre>}
      {approval.cwd && <p className="break-all text-xs text-muted-foreground">工作目录：{approval.cwd}</p>}
      {approval.grantRoot && <p className="break-all text-xs">请求写入：{approval.grantRoot}</p>}
      {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
      {pending && approval.method === "item/tool/requestUserInput" ? <Questionnaire key={approval.id} items={(approval.questions || []).map(question => ({ name: question.id, required: true }))} onSubmit={event => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        const answers = Object.fromEntries((approval.questions || []).map(question => [question.id, { answers: data.getAll(question.id).map(String).filter(Boolean) }]));
        void respond(() => Codex.AnswerQuestions(approval.id, answers));
      }}>
        <QuestionnaireProgress />
        {(approval.questions || []).map(question => <QuestionnaireItem key={question.id} name={question.id} required disabled={busy}>
          <QuestionnaireTitle>{question.header}</QuestionnaireTitle><QuestionnaireDescription>{question.question}</QuestionnaireDescription>
          {!!question.options?.length && <QuestionnaireChoices>{question.options.map(option => <QuestionnaireChoice key={option.label} value={option.label}><span>{option.label}<span className="mt-1 block text-xs text-muted-foreground">{option.description}</span></span></QuestionnaireChoice>)}</QuestionnaireChoices>}
          {(question.isOther || !question.options?.length) && <QuestionnaireInput type={question.isSecret ? "password" : "text"} aria-label={question.question} placeholder={question.isOther ? "或填写您的答案" : "填写答案"} autoComplete="off" />}
          <QuestionnaireError>请填写答案。</QuestionnaireError>
        </QuestionnaireItem>)}
        <QuestionnaireActions><QuestionnairePrevious disabled={busy}>上一步</QuestionnairePrevious><QuestionnaireNext disabled={busy}>下一步</QuestionnaireNext><QuestionnaireSubmit disabled={busy}>提交回答</QuestionnaireSubmit></QuestionnaireActions>
      </Questionnaire> : pending && <div className="flex flex-wrap gap-2">{(approval.availableDecisions || []).filter(decision => labels[decision]).map(decision => <Button key={decision} disabled={busy} size="sm" variant={decision === "accept" ? "default" : "outline"} onClick={() => void respond(() => Codex.RespondApproval(approval.id, decision))}>{labels[decision]}</Button>)}</div>}
    </CardContent>
  </Card>;
}

export function ApprovalCenter({ status }: { status?: Status }) {
  const [open, setOpen] = useState(false);
  const [approvals, setApprovals] = useState<Approval[]>([]);
  const [error, setError] = useState("");
  async function refresh() {
    try { setApprovals(await Codex.GetApprovals() || []); setError(""); }
    catch (error) { setError(errorText(error)); }
  }
  useEffect(() => { void refresh(); }, [status?.phase, status?.generation]);
  useEffect(() => Events.On("codex:event", event => {
    if (event.data.generation === status?.generation && ["host/approvalsUpdated", "turn/completed", "serverRequest/resolved"].includes(event.data.method)) void refresh();
  }), [status?.generation]);
  const pending = approvals.filter(approval => approval.status === "pending").length;
  return <>
    <Button variant="ghost" size="sm" onClick={() => setOpen(true)}><ShieldCheck />审批{pending > 0 && <Badge>{pending}</Badge>}</Button>
    <Sheet open={open} onOpenChange={setOpen}><SheetContent className="flex w-full flex-col sm:max-w-xl">
      <SheetHeader><SheetTitle>审批与问答</SheetTitle><SheetDescription>每个回答仅返回对应的 Codex 请求。任务结束或连接断开后，请求会失效。</SheetDescription></SheetHeader>
      <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 pb-6">
        {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
        {!approvals.length && <p className="py-10 text-center text-sm text-muted-foreground">暂时没有审批或问答。</p>}
        {[...approvals].sort((a, b) => Number(b.status === "pending") - Number(a.status === "pending")).map(approval => <ApprovalCard key={`${approval.generation}:${approval.id}`} approval={approval} refresh={() => void refresh()} />)}
      </div>
    </SheetContent></Sheet>
  </>;
}
