package codex

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

// TestSubscriptionsEvictOldestIdle 验证重复查看不改变顺序且淘汰时跳过活动会话
func TestSubscriptionsEvictOldestIdle(t *testing.T) {
	s, requests := subscriptionFixture(t)
	for i := range ThreadSubscriptionLimit {
		if err := s.EnsureOpen(fmt.Sprintf("thread-%d", i)); err != nil {
			t.Fatal(err)
		}
		<-requests
	}
	s.active["thread-0"] = "turn"
	if err := s.EnsureOpen("thread-1"); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureOpen("next"); err != nil {
		t.Fatal(err)
	}
	eviction, resume := <-requests, <-requests
	if eviction.Method != "thread/unsubscribe" || string(eviction.Params) != `{"threadId":"thread-1"}` || resume.Method != "thread/resume" {
		t.Fatalf("淘汰请求错误: %+v, %+v", eviction, resume)
	}
	if s.opened["thread-1"] || !s.opened["thread-0"] || !s.opened["next"] || len(s.subscriptions) != ThreadSubscriptionLimit {
		t.Fatalf("订阅状态错误: %v", s.subscriptions)
	}
	if err := s.EnsureOpen("thread-1"); err != nil {
		t.Fatal(err)
	}
	eviction, resume = <-requests, <-requests
	if string(eviction.Params) != `{"threadId":"thread-2"}` || resume.Method != "thread/resume" || s.subscriptions[len(s.subscriptions)-1] != "thread-1" {
		t.Fatal("淘汰后重新打开未恢复订阅或顺序错误")
	}
}

// TestSubscriptionsRejectWhenAllBusy 验证全忙时拒绝普通会话但业务线程不受限制
func TestSubscriptionsRejectWhenAllBusy(t *testing.T) {
	s, requests := subscriptionFixture(t)
	for i := range ThreadSubscriptionLimit {
		id := fmt.Sprintf("thread-%d", i)
		if err := s.EnsureOpen(id); err != nil {
			t.Fatal(err)
		}
		<-requests
		s.active[id] = "turn"
	}
	for _, open := range []func() error{
		func() error { return s.EnsureOpen("next") },
		func() error { _, err := s.OpenThread("next"); return err },
		func() error { _, err := s.CreateThread(t.TempDir(), ""); return err },
		func() error { _, err := s.StartTurn("next", "消息", "", "", "", nil); return err },
	} {
		if err := open(); err == nil || !strings.Contains(err.Error(), "全部正在运行") {
			t.Fatalf("全忙时未拒绝新增订阅: %v", err)
		}
	}
	if s.active["next"] != "" {
		t.Fatal("失败的启动保留了活动状态")
	}
	for i := range ThreadSubscriptionLimit + 1 {
		id := fmt.Sprintf("managed-%d", i)
		if err := s.data.RememberManagedThread(id, t.TempDir(), "step"); err != nil {
			t.Fatal(err)
		}
		if err := s.EnsureOpen(id); err != nil {
			t.Fatal(err)
		}
		if request := <-requests; request.Method != "thread/resume" {
			t.Fatalf("业务线程淘汰了普通会话: %s", request.Method)
		}
	}
	if len(s.subscriptions) != ThreadSubscriptionLimit || len(s.opened) != 2*ThreadSubscriptionLimit+1 {
		t.Fatal("业务线程被计入普通订阅限额")
	}
}

// TestCreateThreadUsesSubscriptionLimit 验证新建普通会话也执行淘汰且完成轮次不自动取消订阅
func TestCreateThreadUsesSubscriptionLimit(t *testing.T) {
	s, requests := subscriptionFixture(t)
	for i := range ThreadSubscriptionLimit {
		if err := s.EnsureOpen(fmt.Sprintf("thread-%d", i)); err != nil {
			t.Fatal(err)
		}
		<-requests
	}
	thread, err := s.CreateThread(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if eviction, start := <-requests, <-requests; eviction.Method != "thread/unsubscribe" || start.Method != "thread/start" {
		t.Fatalf("新建会话未执行淘汰: %s, %s", eviction.Method, start.Method)
	}
	s.active[thread.ID] = "turn"
	params, _ := json.Marshal(map[string]any{"threadId": thread.ID, "turn": map[string]string{"id": "turn", "status": "completed"}})
	s.trackNotification(s.status.Generation, packet{Method: "turn/completed", Params: params})
	if !s.opened[thread.ID] || s.active[thread.ID] != "" {
		t.Fatal("完成轮次后不应释放限额内的订阅")
	}
	s.trackNotification(s.status.Generation, packet{Method: "thread/closed", Params: json.RawMessage(`{"threadId":"created"}`)})
	if s.opened[thread.ID] || len(s.subscriptions) != ThreadSubscriptionLimit-1 {
		t.Fatal("线程卸载后未清理本地订阅")
	}
}

// TestConcurrentSubscriptionsRespectLimit 验证并发恢复不会重复订阅或突破普通会话上限
func TestConcurrentSubscriptionsRespectLimit(t *testing.T) {
	s, requests := subscriptionFixture(t)
	var workers sync.WaitGroup
	for i := range ThreadSubscriptionLimit + 10 {
		workers.Go(func() {
			if err := s.EnsureOpen(fmt.Sprintf("thread-%d", i)); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	if len(s.subscriptions) != ThreadSubscriptionLimit || len(s.opened) != ThreadSubscriptionLimit {
		t.Fatalf("并发恢复突破限额: %d, %d", len(s.subscriptions), len(s.opened))
	}
	resumes, unsubscribes := 0, 0
	for len(requests) > 0 {
		switch request := <-requests; request.Method {
		case "thread/resume":
			resumes++
		case "thread/unsubscribe":
			unsubscribes++
		}
	}
	if resumes != ThreadSubscriptionLimit+10 || unsubscribes != 10 {
		t.Fatalf("请求数量错误: resume=%d, unsubscribe=%d", resumes, unsubscribes)
	}
}

// subscriptionFixture 使用真实协议客户端模拟会话订阅响应并收集请求
func subscriptionFixture(t *testing.T) (*Service, <-chan packet) {
	t.Helper()
	s := accountFixture(t)
	reader, writer := io.Pipe()
	c := &client{stdin: writer, done: make(chan struct{}), pending: map[string]chan packet{}}
	s.client, s.status.Phase = c, "ready"
	requests := make(chan packet, 200)
	t.Cleanup(func() {
		reader.Close()
		writer.Close()
		s.client = nil
		s.active = make(map[string]string)
	})
	go func() {
		decoder := json.NewDecoder(reader)
		for {
			var request packet
			if decoder.Decode(&request) != nil {
				return
			}
			requests <- request
			result := json.RawMessage(`{}`)
			if request.Method == "thread/start" {
				result = json.RawMessage(`{"thread":{"id":"created","cwd":"/tmp"}}`)
			}
			c.receive(packet{ID: request.ID, Result: result})
		}
	}()
	return s, requests
}
