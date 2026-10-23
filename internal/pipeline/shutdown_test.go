package pipeline

import (
	"testing"
	"time"

	pipelinemodel "agentor/internal/pipeline/model"
)

// TestShutdownWaitsForWorkAndCancelsEveryQueue 验证流水线退出等待后台结束并取消所有排队项
func TestShutdownWaitsForWorkAndCancelsEveryQueue(t *testing.T) {
	s, _ := fixture(t)
	s.state.Runs = []pipelinemodel.Run{{ID: "running", Status: "running", Actions: []pipelinemodel.ActionResult{{ID: "action", Status: "running"}}}, {ID: "queued", Status: "queued"}}
	s.state.Requests = []pipelinemodel.SessionRequest{{ID: "request", Status: "queued"}}
	s.workerGroup.Add(1)
	release := make(chan struct{})
	go func() {
		defer s.workerGroup.Done()
		<-s.ctx.Done()
		<-release
	}()

	stopped := make(chan error, 1)
	go func() { stopped <- s.ServiceShutdown() }()
	<-s.ctx.Done()
	select {
	case err := <-stopped:
		t.Fatalf("后台尚未收尾，退出已经返回：%v", err)
	default:
	}
	close(release)
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("后台结束后仍未退出")
	}

	reopened, err := NewService(s.data, s.work, s.tasks, s.files, s.runtime)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.ServiceShutdown()
	state := reopened.GetState()
	if state.Paused {
		t.Error("正常退出不应把新会话永久留在暂停队列")
	}
	if state.Runs[0].Status != "interrupted" || state.Runs[0].Actions[0].Status != "interrupted" || state.Runs[1].Status != "cancelled" || state.Requests[0].Status != "cancelled" {
		t.Fatalf("重新启动后仍有未取消流水线任务：%+v", state)
	}
}
