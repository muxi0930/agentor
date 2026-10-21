package codex

import (
	"encoding/json"
	"io"
	"testing"
	"time"
)

// TestPrepareShutdownInterruptsActiveTurnsAndRejectsNewTurns 验证退出发送真实中断请求且阻止新轮次
func TestPrepareShutdownInterruptsActiveTurnsAndRejectsNewTurns(t *testing.T) {
	s := NewService(nil, nil)
	defer s.cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	c := &client{stdin: writer, done: make(chan struct{}), pending: make(map[string]chan packet)}
	s.client = c
	s.status.Phase = "ready"
	s.active["thread"] = "turn"
	s.activeText["thread"] = "检查退出行为"
	request := make(chan packet, 1)
	go func() {
		var message packet
		if json.NewDecoder(reader).Decode(&message) == nil {
			request <- message
			c.receive(packet{ID: message.ID, Result: json.RawMessage(`{}`)})
		}
	}()

	active := s.GetActiveTurns()
	if len(active) != 1 || active[0].Summary != "检查退出行为" {
		t.Fatalf("活动对话简述丢失：%+v", active)
	}
	s.PrepareShutdown()
	select {
	case message := <-request:
		var params struct {
			ThreadID string `json:"threadId"` // 中断目标会话
			TurnID   string `json:"turnId"`   // 中断目标轮次
		}
		if err := json.Unmarshal(message.Params, &params); err != nil {
			t.Fatal(err)
		}
		if message.Method != "turn/interrupt" || params.ThreadID != "thread" || params.TurnID != "turn" {
			t.Fatalf("退出未发送对应轮次的中断请求：%+v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("退出未及时中断普通对话")
	}
	s.shutdownRequests.Wait()
	if _, err := s.startTurn("another", "新任务", "", "", "", nil, "read-only", ""); err == nil {
		t.Fatal("退出过程中仍能开始新轮次")
	}
	if s.ctx.Err() != nil {
		t.Fatal("中断请求完成前不应关闭原生连接上下文")
	}
}

// TestShutdownWaitsForStartingChatBeforeClosingConnection 验证普通对话启动在途时先中断再关闭连接
func TestShutdownWaitsForStartingChatBeforeClosingConnection(t *testing.T) {
	s := accountFixture(t)
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	c := &client{stdin: writer, done: make(chan struct{}), pending: make(map[string]chan packet)}
	s.client = c
	s.status.Phase = "ready"
	started, release := make(chan struct{}), make(chan struct{})
	interrupted := make(chan bool, 1)
	go func() {
		defer c.finish(io.EOF)
		decoder := json.NewDecoder(reader)
		for {
			var message packet
			if decoder.Decode(&message) != nil {
				return
			}
			if message.Method == "turn/start" {
				close(started)
				<-release
				c.receive(packet{ID: message.ID, Result: json.RawMessage(`{"turn":{"id":"turn","status":"inProgress","items":[]}}`)})
			} else if message.Method == "turn/interrupt" {
				interrupted <- s.ctx.Err() == nil
				c.receive(packet{ID: message.ID, Result: json.RawMessage(`{}`)})
			}
		}
	}()
	startDone := make(chan error, 1)
	go func() {
		_, err := s.startTurn("thread", "尚未返回的启动请求", "", "", "", nil, "read-only", s.data.GetInfo().DataDirectory)
		startDone <- err
	}()
	<-started
	s.PrepareShutdown()
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- s.ServiceShutdown() }()
	select {
	case <-s.ctx.Done():
		t.Error("启动请求尚在途时已关闭原生连接上下文")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)

	select {
	case err := <-startDone:
		if err == nil {
			t.Error("退出过程中启动的对话仍成功返回")
		}
	case <-time.After(time.Second):
		t.Fatal("在途启动未结束")
	}
	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("中断后连接未关闭")
	}
	select {
	case live := <-interrupted:
		if !live {
			t.Error("中断请求发出前连接已被取消")
		}
	default:
		t.Error("在途启动结束后没有发送中断请求")
	}
}
