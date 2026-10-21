package tasks

import (
	"context"
	"testing"
	"time"
)

// TestAgentTimeoutAtStart 验证启动时应用最新时限并保留更短的任务时限
func TestAgentTimeoutAtStart(t *testing.T) {
	for _, item := range []struct {
		name  string // 场景名称
		limit int    // 全局时限
		task  int    // 任务时限
		want  int    // 生效时限
	}{
		{"全局限制", 1, 0, 1},
		{"任务更短", 60, 3, 3},
		{"全局更短", 2, 60, 2},
		{"不限时", 0, 0, 0},
		{"保留任务限制", 0, 3, 3},
	} {
		t.Run(item.name, func(t *testing.T) {
			data := testData(t)
			s, err := NewService(data, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { s.ServiceShutdown() })
			s.state.Executions = []Execution{{ID: "task", FeatureID: "feature", Status: "queued", TimeoutSeconds: item.task}}
			if _, err := data.SetAgentTimeoutSeconds(item.limit); err != nil {
				t.Fatal(err)
			}
			started := make(chan context.Context, 1)
			release := make(chan struct{})
			s.runExecution = func(ctx context.Context, execution Execution) (string, string, error) {
				started <- ctx
				select {
				case <-ctx.Done():
					return "interrupted", "", ctx.Err()
				case <-release:
					return "completed", "", nil
				}
			}
			s.kick("feature")
			var ctx context.Context
			select {
			case ctx = <-started:
			case <-time.After(time.Second):
				t.Fatal("任务未启动")
			}
			deadline, limited := ctx.Deadline()
			if limited != (item.want > 0) {
				t.Fatal("时限启用状态不正确")
			}
			if limited && (time.Until(deadline) <= 0 || time.Until(deadline) > time.Duration(item.want)*time.Second) {
				t.Fatal("任务时限不正确", deadline)
			}
			if _, err := data.SetAgentTimeoutSeconds(120); err != nil {
				t.Fatal(err)
			}
			if current, _ := ctx.Deadline(); current != deadline {
				t.Fatal("修改配置影响了已运行任务")
			}
			if item.limit == 1 {
				select {
				case <-ctx.Done():
					if ctx.Err() != context.DeadlineExceeded {
						t.Fatal(ctx.Err())
					}
				case <-time.After(2 * time.Second):
					t.Fatal("超时后没有取消任务")
				}
			}
			close(release)
		})
	}
}
