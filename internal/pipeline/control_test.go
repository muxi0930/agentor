package pipeline

import (
	"context"
	"testing"
	"time"

	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
)

// TestPauseResumeFrozenRun 验证暂停跨重启保留且继续使用原定义
func TestPauseResumeFrozenRun(t *testing.T) {
	s, projectID := fixture(t)
	task, err := s.SaveTask(projectID, textOf(snapshotTask()))
	if err != nil {
		t.Fatal(err)
	}
	run := pipelinemodel.Run{ID: "paused-test", ProjectID: projectID, TaskID: task.ID, Task: task, Status: "queued", PauseRequested: true}
	s.mu.Lock()
	s.state.Runs = append(s.state.Runs, run)
	ctx, cancel := context.WithCancel(s.ctx)
	s.cancellations[run.ID] = cancel
	s.workerGroup.Add(1)
	s.mu.Unlock()
	s.execute(ctx, run)
	if got := s.GetState().Runs[0]; got.Status != "paused" || len(got.Actions) != 0 {
		t.Fatal(got)
	}
	if _, err = s.RunTask(pipelinemodel.RunInput{TaskID: task.ID}); err == nil {
		t.Fatal("paused task reentered")
	}
	if err = s.DeleteTask(projectID, task.ID); err == nil {
		t.Fatal("paused task deleted")
	}
	if _, err = s.ControlRun("other", run.ID, "resume"); err == nil {
		t.Fatal("foreign run resumed")
	}
	if _, err = s.ControlRun(projectID, run.ID, "completed"); err == nil {
		t.Fatal("arbitrary transition accepted")
	}
	task.Actions[1].Params["fileName"] = "new.md"
	if _, err = s.SaveTask(projectID, textOf(task)); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService(s.data, s.work, s.tasks, s.files, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.ServiceShutdown()
	if _, err = reopened.ControlRun(projectID, run.ID, "resume"); err != nil {
		t.Fatal(err)
	}
	completed := waitRun(t, reopened, run.ID)
	if completed.Status != "completed" || len(completed.Actions) != 2 || completed.Task.Actions[1].Params["fileName"] != "result.md" {
		t.Fatal(completed)
	}
	if _, err = reopened.ControlRun(projectID, run.ID, "retry"); err == nil {
		t.Fatal("successful run retried")
	}
}

// TestRetryPreservesAttemptsAndSuccessfulPrefix 验证重试保留成功节点和失败尝试
func TestRetryPreservesAttemptsAndSuccessfulPrefix(t *testing.T) {
	s, projectID := fixture(t)
	task := snapshotTask()
	task.Actions[1].Params["fileName"] = map[string]any{"input": "file"}
	task.Inputs = []pipelinemodel.InputField{{Name: "file", Type: "string", Required: true}}
	task, err := s.SaveTask(projectID, textOf(task))
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.RunTask(pipelinemodel.RunInput{TaskID: task.ID, Inputs: map[string]any{"file": "../bad.md"}})
	if err != nil {
		t.Fatal(err)
	}
	failed := waitRun(t, s, run.ID)
	if failed.Status != "error" {
		t.Fatal(failed)
	}
	// 等待结果通知完成后，执行器才释放重试入口
	deadline := time.Now().Add(time.Second)
	for {
		_, err = s.ControlRun(projectID, run.ID, "retry")
		if err != constant.ErrPipelineWorkerActive || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	retried := waitRun(t, s, run.ID)
	if retried.Status != "error" || len(retried.Actions) != 3 || retried.Actions[0].Attempt != 1 || retried.Actions[2].Attempt != 2 || retried.Inputs["file"] != "../bad.md" {
		t.Fatal(retried)
	}
}
