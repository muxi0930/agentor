package tasks

import (
	"context"
	"testing"
	"time"

	tasksmodel "agentor/internal/tasks/model"
)

// TestConcurrencyLimitChanges 验证满额排队、动态增减上限及完成后释放名额
func TestConcurrencyLimitChanges(t *testing.T) {
	data := testData(t)
	if _, err := data.SetMaxConcurrentAgents(1); err != nil {
		t.Fatal(err)
	}
	s, err := NewService(data, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.ServiceShutdown() })
	started := make(chan string, 3)
	release := map[string]chan struct{}{"a": make(chan struct{}), "b": make(chan struct{}), "c": make(chan struct{})}
	s.runExecution = func(ctx context.Context, execution tasksmodel.Execution) (string, string, error) {
		started <- execution.ID
		select {
		case <-release[execution.ID]:
			return "completed", "", nil
		case <-ctx.Done():
			return "interrupted", "", ctx.Err()
		}
	}
	for _, id := range []string{"a", "b", "c"} {
		s.state.Executions = append(s.state.Executions, tasksmodel.Execution{ID: id, FeatureID: id, Status: "queued"})
	}
	s.kick("a")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("首个任务未启动")
	}
	s.kick("b")
	s.kick("c")
	select {
	case <-started:
		t.Fatal("超过并发上限")
	case <-time.After(200 * time.Millisecond):
	}
	if _, err := data.SetMaxConcurrentAgents(2); err != nil {
		t.Fatal(err)
	}
	var second string
	select {
	case second = <-started:
	case <-time.After(time.Second):
		t.Fatal("提高上限后未启动排队任务")
	}
	if _, err := data.SetMaxConcurrentAgents(1); err != nil {
		t.Fatal(err)
	}
	close(release["a"])
	select {
	case <-started:
		t.Fatal("降低上限后提前启动任务")
	case <-time.After(200 * time.Millisecond):
	}
	if s.activeExecutions.Load() != 1 {
		t.Fatal("降低上限中断了已有任务或未释放完成任务的名额")
	}
	close(release[second])
	select {
	case third := <-started:
		close(release[third])
	case <-time.After(time.Second):
		t.Fatal("任务完成后未释放并发名额")
	}
}

// TestPipelineExecutionBlocksFeatureQueueAndNotifiesWaiters 验证绑定步骤与普通队列互斥且终态唤醒等待者
func TestPipelineExecutionBlocksFeatureQueueAndNotifiesWaiters(t *testing.T) {
	s, err := NewService(testData(t), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ServiceShutdown()
	execution := tasksmodel.Execution{ID: "pipeline-attempt", StepID: "step", FeatureID: "feature", PipelineRunID: "run", Status: "running"}
	if err = s.RecordPipelineExecution(execution, func() {}); err != nil {
		t.Fatal(err)
	}
	another := execution
	another.ID = "second-attempt"
	if err = s.RecordPipelineExecution(another, func() {}); err == nil {
		t.Fatal("同一功能下两个流水线节点同时执行")
	}
	started := make(chan struct{}, 1)
	s.runExecution = func(ctx context.Context, execution tasksmodel.Execution) (string, string, error) {
		started <- struct{}{}
		return "completed", "", nil
	}
	s.mu.Lock()
	s.state.Executions = append(s.state.Executions, tasksmodel.Execution{ID: "regular", StepID: "other-step", FeatureID: "feature", Status: "queued"})
	listener := make(chan tasksmodel.Execution, 1)
	s.completed[execution.ID] = []chan tasksmodel.Execution{listener}
	s.mu.Unlock()
	s.kick("feature")
	select {
	case <-started:
		t.Fatal("流水线步骤未结束时普通步骤提前执行")
	case <-time.After(150 * time.Millisecond):
	}
	execution.Status = "completed"
	if err = s.RecordPipelineExecution(execution, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case finished := <-listener:
		if finished.Status != "completed" {
			t.Fatal(finished)
		}
	case <-time.After(time.Second):
		t.Fatal("流水线终态未通知步骤等待者")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("流水线步骤结束后未继续普通队列")
	}
}

// TestManagedTurnWaitsForSharedConcurrencySlot 验证节点 Agent 在普通步骤占满名额时可取消等待
func TestManagedTurnWaitsForSharedConcurrencySlot(t *testing.T) {
	data := testData(t)
	data.SetMaxConcurrentAgents(1)
	s, err := NewService(data, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ServiceShutdown()
	s.activeExecutions.Store(1)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	status, _, err := s.RunManagedTurn(ctx, "thread", "", "", "", "read-only", "", "", func(string) { t.Error("超出全局并发上限") })
	if err != context.DeadlineExceeded || status != "interrupted" || s.activeExecutions.Load() != 1 {
		t.Fatal(status, err, s.activeExecutions.Load())
	}
}
