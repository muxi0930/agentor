import { useEffect, useRef, useState } from "react";
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
import { ElicitationForm } from "./ElicitationForm";
import { CodeView } from "@/features/chat/CodeView";
import { DiffView } from "@/features/chat/DiffView";

const titles: Record<string, string> = { "item/tool/requestUserInput":"Codex 需要您的回答", "item/fileChange/requestApproval":"文件修改审批", "item/commandExecution/requestApproval":"命令执行审批", "item/permissions/requestApproval":"额外权限审批", "mcpServer/elicitation/request":"MCP 请求" };

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
    <CardHeader className="gap-2"><CardTitle className="flex items-center justify-between gap-2 text-sm">{titles[approval.method] || approval.method}<Badge variant="outline">{pending ? "待处理" : approval.status === "resolved" ? "已处理" : "已失效"}</Badge></CardTitle>
      <p className="break-all text-xs text-muted-foreground">会话 {approval.threadId}<br />轮次 {approval.turnId}</p>
    </CardHeader>
    <CardContent className="space-y-3 text-sm">
      {approval.reason && <p>{approval.reason}</p>}
      {approval.command && <pre className="max-h-60 overflow-auto whitespace-pre-wrap rounded-lg bg-muted p-3 text-xs">{approval.command}</pre>}
      {approval.cwd && <p className="break-all text-xs text-muted-foreground">工作目录：{approval.cwd}</p>}
      {approval.grantRoot && <p className="break-all text-xs">请求写入：{approval.grantRoot}</p>}
      {approval.details && <CodeView code={approval.details} language="请求详情" />}
      {approval.permissionsJSON && <CodeView code={approval.permissionsJSON} language="请求的权限" />}
      {!!approval.changes?.length && <DiffView changes={approval.changes} />}
      {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
      {pending && approval.method === "mcpServer/elicitation/request" ? <ElicitationForm approval={approval} busy={busy} respond={respond} /> : pending && approval.method === "item/permissions/requestApproval" ? <div className="flex flex-wrap gap-2"><Button size="sm" disabled={busy} onClick={() => void respond(() => Codex.RespondPermissions(approval.id, true, "turn"))}>允许本轮</Button><Button size="sm" variant="outline" disabled={busy} onClick={() => void respond(() => Codex.RespondPermissions(approval.id, true, "session"))}>本会话允许</Button><Button size="sm" variant="outline" disabled={busy} onClick={() => void respond(() => Codex.RespondPermissions(approval.id, false, "turn"))}>拒绝</Button></div> : pending && approval.method === "item/tool/requestUserInput" ? <Questionnaire key={approval.id} items={(approval.questions || []).map(question => ({ name: question.id, required: true }))} onSubmit={event => {
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
      </Questionnaire> : pending && <div className="space-y-3">
        {(approval.decisionOptions || []).filter(option => option.details).map(option => <div key={option.value}><p className="mb-1 text-xs text-muted-foreground">{option.label}</p><CodeView code={option.details} language="将保存的规则" /></div>)}
        <div className="flex flex-wrap gap-2">{(approval.decisionOptions || []).map(option => <Button key={option.value} disabled={busy} size="sm" variant={option.value === "accept" ? "default" : "outline"} onClick={() => void respond(() => Codex.RespondApproval(approval.id, option.value))}>{option.label}</Button>)}</div>
      </div>}
    </CardContent>
  </Card>;
}

export function ApprovalCenter({ status }: { status?: Status }) {
  const [open, setOpen] = useState(false);
  const [approvals, setApprovals] = useState<Approval[]>([]);
  const [error, setError] = useState("");
  const [approvalAnimation, setApprovalAnimation] = useState(0);
  const previousApprovals = useRef<Set<string> | null>(null);
  async function refresh() {
    try {
      const next = await Codex.GetApprovals() || [];
      const previous = previousApprovals.current;
      if (previous && next.some(approval => approval.status === "pending" && !previous.has(`${approval.generation}:${approval.id}`))) setApprovalAnimation(value => value + 1);
      previousApprovals.current = new Set(next.map(approval => `${approval.generation}:${approval.id}`));
      setApprovals(next); setError("");
    }
    catch (error) { setError(errorText(error)); }
  }
  useEffect(() => { previousApprovals.current = null; void refresh(); }, [status?.phase, status?.generation]);
  useEffect(() => Events.On("codex:event", event => {
    if (event.data.generation === status?.generation && ["host/approvalsUpdated", "turn/completed", "serverRequest/resolved"].includes(event.data.method)) void refresh();
  }), [status?.generation]);
  const pending = approvals.filter(approval => approval.status === "pending").length;
  return <>
    <Button variant="ghost" size="sm" onClick={() => setOpen(true)}><ShieldCheck key={approvalAnimation} aria-hidden="true" className={approvalAnimation ? "notification-bell-bounce" : undefined} />审批{pending > 0 && <Badge>{pending}</Badge>}</Button>
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
