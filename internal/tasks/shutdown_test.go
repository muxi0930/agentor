package tasks

import (
	"context"
	"testing"
	"time"
)

// TestShutdownWaitsForWorkerAndCancelsQueuedRequests 验证退出等待运行收尾并持久化取消排队项
func TestShutdownWaitsForWorkerAndCancelsQueuedRequests(t *testing.T) {
	data := testData(t)
	s, err := NewService(data, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	started, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	s.runRequest = func(ctx context.Context, request Request) (string, string, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-release
		return "interrupted", "", ctx.Err()
	}
	s.state.Requests = []Request{{ID: "running", Status: "queued"}, {ID: "queued", Status: "queued"}}
	s.state.Executions = []Execution{{ID: "queued-step", Status: "queued"}}
	s.kickController()
	<-started

	stopped := make(chan error, 1)
	go func() { stopped <- s.ServiceShutdown() }()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("退出未中断运行请求")
	}
	select {
	case err := <-stopped:
		t.Fatalf("工作线程尚未收尾，退出已经返回：%v", err)
	default:
	}
	close(release)
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("工作线程结束后仍未退出")
	}

	reopened, err := NewService(data, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.ServiceShutdown()
	state := reopened.GetState()
	if state.Paused {
		t.Error("正常退出不应把新任务永久留在暂停队列")
	}
	if state.Requests[0].Status != "interrupted" || state.Requests[1].Status != "cancelled" || state.Executions[0].Status != "cancelled" {
		t.Fatalf("重新启动后仍有未取消任务：%+v", state)
	}
}
