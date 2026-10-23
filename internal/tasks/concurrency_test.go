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
