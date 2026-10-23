package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
	workspacemodel "agentor/internal/workspace/model"
)

// TestGraphExclusiveBranchesAndAgentRetries 验证多出口单选、自动重试和失败处理终点
func TestGraphExclusiveBranchesAndAgentRetries(t *testing.T) {
	for _, test := range []struct {
		name, branch        string // 场景与业务分支
		failures, retries   int    // 失败次数与重试上限
		wantStatus, wantEnd string // 期望终态与终点
		wantAttempts        int    // 期望尝试次数
	}{
		{"third branch", "review", 0, 0, "completed", "review", 1},
		{"eventual success", "ready", 2, 2, "completed", "done", 3},
		{"exhausted handler succeeds", "", 4, 2, "completed", "handled", 3},
		{"business rejection", "rejected", 0, 3, "error", "rejected", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, _ := fixture(t)
			var calls atomic.Int32
			s.runAgent = func(ctx context.Context, run pipelinemodel.Run, node pipelinemodel.Action, params map[string]any) (pipelinemodel.NodeReport, error) {
				n := int(calls.Add(1))
				if n <= test.failures {
					return pipelinemodel.NodeReport{}, errors.New("not yet")
				}
				return pipelinemodel.NodeReport{Outcome: "success", Branch: test.branch, Output: map[string]any{"attempt": n}}, nil
			}
			task := agentGraph()
			task.Actions[0].MaxRetries = test.retries
			saved, err := s.SaveTask("", textOf(task))
			if err != nil {
				t.Fatal(err)
			}
			run, err := s.RunTask(pipelinemodel.RunInput{TaskID: saved.ID})
			if err != nil {
				t.Fatal(err)
			}
			completed := waitRun(t, s, run.ID)
			if completed.Status != test.wantStatus || completed.CurrentNode != test.wantEnd || len(completed.Actions) != test.wantAttempts {
				t.Fatalf("unexpected path: %+v", completed)
			}
			if int(calls.Load()) != test.wantAttempts {
				t.Fatal("wrong retry count", calls.Load())
			}
			for i, attempt := range completed.Actions {
				if attempt.Attempt != i+1 {
					t.Fatal("attempt history lost", completed.Actions)
				}
			}
		})
	}
}

// TestStaticUnhandledOutcomeAndExceptionStop 验证静态分支未捕获和非预期异常都终止
func TestStaticUnhandledOutcomeAndExceptionStop(t *testing.T) {
	for _, branch := range []string{"", "done"} {
		t.Run("target="+branch, func(t *testing.T) {
			s, _ := fixture(t)
			task := pipelinemodel.Task{Name: "static", Entry: "report", Inputs: []pipelinemodel.InputField{{Name: "file", Type: "string", Required: true}}, Actions: []pipelinemodel.Action{
				{ID: "report", Kind: "static", Type: "report.write", Params: map[string]any{"fileName": map[string]any{"input": "file"}, "content": "hello"}, Branches: []pipelinemodel.Branch{{Name: "success", Target: branch}}},
			}}
			if branch != "" {
				task.Actions = append(task.Actions, pipelinemodel.Action{ID: "done", Kind: "end", Type: "end.success"})
			}
			saved, err := s.SaveTask("", textOf(task))
			if err != nil {
				t.Fatal(err)
			}
			name := "report.md"
			if branch != "" {
				name = "../invalid.md"
			}
			run, err := s.RunTask(pipelinemodel.RunInput{TaskID: saved.ID, Inputs: map[string]any{"file": name}})
			if err != nil {
				t.Fatal(err)
			}
			result := waitRun(t, s, run.ID)
			if result.Status != "error" || len(result.Actions) != 1 {
				t.Fatal(result)
			}
			if _, err = s.ControlRun("", run.ID, "retry"); err == nil {
				t.Fatal("static action replayed")
			}
		})
	}
}

// TestTemplateCopiesBindingsAndDefinition 验证项目应用模板与独立修改
func TestTemplateCopiesBindingsAndDefinition(t *testing.T) {
	s, projectID := fixture(t)
	state, err := s.work.SaveVersion(projectID, workspacemodel.Version{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = s.work.SaveFeature(projectID, state.SelectedID, workspacemodel.Feature{Title: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = s.work.SaveStep(projectID, state.SelectedID, workspacemodel.Step{Title: "step"})
	if err != nil {
		t.Fatal(err)
	}
	template := agentGraph()
	template.Actions[0].StepSlot = "implementation"
	template, err = s.SaveTemplate(textOf(template))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Instantiate(pipelinemodel.InstantiateInput{TemplateID: template.ID}); err == nil {
		t.Fatal("missing step binding accepted")
	}
	instance, err := s.Instantiate(pipelinemodel.InstantiateInput{TemplateID: template.ID, ProjectID: projectID, Bindings: map[string]string{"implementation": state.SelectedID}})
	if err != nil {
		t.Fatal(err)
	}
	if instance.ID == template.ID || instance.Actions[0].StepID != state.SelectedID {
		t.Fatal(instance)
	}
	template.Actions[0].Prompt = "changed template"
	if _, err = s.SaveTemplate(textOf(template)); err != nil {
		t.Fatal(err)
	}
	if s.GetState().Tasks[0].Actions[0].Prompt == "changed template" {
		t.Fatal("template edit changed instance")
	}
	instance.Actions[0].Prompt = "instance only"
	if _, err = s.SaveTask(projectID, textOf(instance)); err != nil {
		t.Fatal(err)
	}
	if s.GetState().Templates[0].Actions[0].Prompt != "changed template" {
		t.Fatal("instance edit changed template")
	}
}

// TestGraphValidationAndReferences 验证图必须无环且数据在所有到达路径上可用
func TestGraphValidationAndReferences(t *testing.T) {
	s, _ := fixture(t)
	graph := agentGraph()
	graph.Actions[0].Branches[0].Target = "agent"
	if _, err := s.SaveTask("", textOf(graph)); err == nil {
		t.Fatal("cycle accepted")
	}
	graph = agentGraph()
	graph.Actions[0].Kind = "static"
	graph.Actions[0].Type = "report.write"
	if _, err := s.SaveTask("", textOf(graph)); err == nil {
		t.Fatal("invalid static configuration accepted")
	}
	graph = agentGraph()
	graph.Actions[0].Branches = append(graph.Actions[0].Branches, graph.Actions[0].Branches[0])
	if _, err := s.SaveTask("", textOf(graph)); err == nil {
		t.Fatal("duplicate business outcome accepted")
	}
	out, err := resolveParams(map[string]any{"nested": []any{map[string]any{"result": "agent", "path": "person.name"}}, "whole": map[string]any{"input": "payload"}}, map[string]any{"payload": []any{1.0, "x"}}, map[string]any{"agent": map[string]any{"person": map[string]any{"name": "Ada"}}})
	if err != nil || out["nested"].([]any)[0] != "Ada" {
		t.Fatal(out, err)
	}
	if _, err = resolveParams(map[string]any{"missing": map[string]any{"result": "agent", "path": "bad"}}, nil, map[string]any{"agent": map[string]any{}}); err == nil {
		t.Fatal("missing output field accepted")
	}
	// 分支 A 产生的值不能在另一条分支的节点中读取
	graph = agentGraph()
	graph.Actions[1] = pipelinemodel.Action{ID: "done", Kind: "static", Type: "report.write", Params: map[string]any{"fileName": "result.md", "content": map[string]any{"result": "review"}}, Branches: []pipelinemodel.Branch{{Name: "success", Target: "handled"}}}
	if _, err = s.SaveTask("", textOf(graph)); err == nil {
		t.Fatal("branch-local result escaped its path")
	}
}

// TestNodeReportOwnershipIdempotencyAndCancellation 验证汇报归属、不可覆盖与取消后拒绝
func TestNodeReportOwnershipIdempotencyAndCancellation(t *testing.T) {
	s, _ := fixture(t)
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	s.nodeContexts["thread"] = ctx
	graph := agentGraph()
	s.state.Runs = []pipelinemodel.Run{{ID: "run", Status: "running", CurrentNode: "agent", Task: graph, Actions: []pipelinemodel.ActionResult{{ID: "agent", Kind: "agent", Status: "running", ThreadID: "thread", TurnID: "turn"}}}}
	raw, _ := json.Marshal(pipelinemodel.NodeReport{Outcome: "success", Branch: "ready", Summary: "checks passed", Output: map[string]any{"text": "ok"}})
	call := codexmodel.ToolCall{ThreadID: "thread", TurnID: "wrong", Arguments: raw}
	if _, err := s.CompleteNode(call); err == nil {
		t.Fatal("wrong turn accepted")
	}
	call.TurnID = "turn"
	bad := call
	bad.Arguments = json.RawMessage(`{"outcome":"success","branch":"arbitrary","summary":"ok"}`)
	if _, err := s.CompleteNode(bad); err == nil {
		t.Fatal("undeclared branch accepted")
	}
	for range 2 {
		if _, err := s.CompleteNode(call); err != nil {
			t.Fatal("idempotent report rejected", err)
		}
	}
	if run := s.GetState().Runs[0]; run.CurrentNode != "agent" || run.Status != "running" {
		t.Fatal("report advanced before turn ended", run)
	}
	bad.Arguments = json.RawMessage(`{"outcome":"failure","summary":"changed"}`)
	if _, err := s.CompleteNode(bad); err == nil {
		t.Fatal("report overwritten")
	}
	cancel()
	if _, err := s.CompleteNode(call); err == nil {
		t.Fatal("cancelled report accepted")
	}
}

// TestAgentCancellationDoesNotRetryOrFollowFailureEdge 验证取消优先于重试和错误分支
func TestAgentCancellationDoesNotRetryOrFollowFailureEdge(t *testing.T) {
	s, _ := fixture(t)
	started := make(chan struct{})
	var calls atomic.Int32
	s.runAgent = func(ctx context.Context, run pipelinemodel.Run, node pipelinemodel.Action, params map[string]any) (pipelinemodel.NodeReport, error) {
		calls.Add(1)
		close(started)
		<-ctx.Done()
		return pipelinemodel.NodeReport{}, ctx.Err()
	}
	graph := agentGraph()
	graph.Actions[0].MaxRetries = 3
	task, err := s.SaveTask("", textOf(graph))
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.RunTask(pipelinemodel.RunInput{TaskID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("agent not started")
	}
	if err = s.StopRun(run.ID); err != nil {
		t.Fatal(err)
	}
	result := waitRun(t, s, run.ID)
	if calls.Load() != 1 || result.Status != "cancelled" || result.CurrentNode != "agent" {
		t.Fatal(result, calls.Load())
	}
}

// TestRunRecoveryNeverReplaysSideEffects 验证退出后未完成节点标记中断而不重放
func TestRunRecoveryNeverReplaysSideEffects(t *testing.T) {
	s, _ := fixture(t)
	s.state.Runs = []pipelinemodel.Run{{ID: "old", Status: "running", CurrentNode: "merge", Actions: []pipelinemodel.ActionResult{{ID: "merge", Status: "running"}}}}
	if err := s.persistLocked(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService(s.data, s.work, s.tasks, s.files, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.ServiceShutdown()
	result := reopened.GetState().Runs[0]
	if result.Status != "interrupted" || !strings.Contains(result.Error, "未自动重放") {
		t.Fatal(result)
	}
}

// TestResumePreservesFailureOutput 验证失败处理前暂停恢复仍能引用失败节点的输出
func TestResumePreservesFailureOutput(t *testing.T) {
	s, _ := fixture(t)
	s.runAgent = func(ctx context.Context, run pipelinemodel.Run, node pipelinemodel.Action, params map[string]any) (pipelinemodel.NodeReport, error) {
		if _, err := s.ControlRun("", run.ID, constant.PipelineControlPause); err != nil {
			return pipelinemodel.NodeReport{}, err
		}
		return pipelinemodel.NodeReport{Outcome: "failure", Summary: "needs handling", Output: "failure evidence"}, nil
	}
	graph := agentGraph()
	graph.Actions[4] = pipelinemodel.Action{ID: "handled", Kind: "static", Type: "report.write", Params: map[string]any{"fileName": "failure.md", "content": map[string]any{"result": "agent"}}, Branches: []pipelinemodel.Branch{{Name: "success", Target: "done"}}}
	task, err := s.SaveTask("", textOf(graph))
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.RunTask(pipelinemodel.RunInput{TaskID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	paused := waitRun(t, s, run.ID)
	if paused.Status != constant.PipelineStatusPaused || paused.CurrentNode != "handled" {
		t.Fatal(paused)
	}
	s.workerGroup.Wait()
	if _, err = s.ControlRun("", run.ID, constant.PipelineControlResume); err != nil {
		t.Fatal(err)
	}
	result := waitRun(t, s, run.ID)
	if result.Status != constant.TasksStatusCompleted || len(result.Actions) != 2 || result.Actions[1].Inputs["content"] != "failure evidence" {
		t.Fatal(result)
	}
}

// agentGraph 提供多出口与显式终点的测试定义
func agentGraph() pipelinemodel.Task {
	return pipelinemodel.Task{Name: "agent graph", Entry: "agent", Actions: []pipelinemodel.Action{
		{ID: "agent", Kind: "agent", Type: "agent.run", Params: map[string]any{}, Prompt: "work", Criteria: "checks pass", Permission: constant.AppDataPermissionReadOnly, TimeoutSeconds: 60, FailureTarget: "handled", Branches: []pipelinemodel.Branch{{Name: "ready", Target: "done"}, {Name: "review", Target: "review"}, {Name: "rejected", Target: "rejected"}}},
		{ID: "done", Kind: "end", Type: "end.success"}, {ID: "review", Kind: "end", Type: "end.success"}, {ID: "rejected", Kind: "end", Type: "end.failure"}, {ID: "handled", Kind: "end", Type: "end.success"},
	}}
}
