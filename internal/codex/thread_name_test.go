package codex

import (
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	codexmodel "agentor/internal/codex/model"
)

// TestManagedThreadNames 验证业务会话创建后设置固定标题前缀且普通会话不改名
func TestManagedThreadNames(t *testing.T) {
	for _, test := range []struct {
		role    string // 会话角色
		title   string // 上游初始标题
		preview string // 上游初始预览
		want    string // 预期标题
		rename  bool   // 是否应发送重命名请求
	}{
		{"step", "任务", "预览", "[AgentorWoker] 任务", true},
		{"controller", "", "", "", false},
		{"step", "", "", "", false},
		{"pipeline_manager", "", "", "", false},
		{"pipeline_config", "", "", "", false},
		{"controller", "控制", "", "[AgentorCtl] 控制", true},
		{"pipeline_manager", "管理", "", "[AgentorPipe] 管理", true},
		{"pipeline_config", "配置", "", "[AgentorPipe] 配置", true},
		{"step", "[AgentorWoker] 任务", "预览", "[AgentorWoker] 任务", false},
		{"controller", "[AgentorCtl] 控制", "", "[AgentorCtl] 控制", false},
		{"pipeline_manager", "[AgentorPipe] 管理", "", "[AgentorPipe] 管理", false},
		{"pipeline_config", "[AgentorPipe] 配置", "", "[AgentorPipe] 配置", false},
		{"step", "", "预览", "[AgentorWoker] 预览", true},
		{"controller", "", "预览", "[AgentorCtl] 预览", true},
		{"pipeline_manager", "", "预览", "[AgentorPipe] 预览", true},
		{"pipeline_config", "", "预览", "[AgentorPipe] 预览", true},
		{"", "普通", "预览", "普通", false},
		{"", "", "预览", "", false},
	} {
		t.Run(test.role, func(t *testing.T) {
			s := accountFixture(t)
			cwd := t.TempDir()
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			c := &client{stdin: writer, done: make(chan struct{}), pending: map[string]chan packet{}}
			s.client, s.status.Phase = c, "ready"
			defer func() { s.client = nil }()
			names := make(chan map[string]string, 1)
			go func() {
				decoder := json.NewDecoder(reader)
				for {
					var request packet
					if decoder.Decode(&request) != nil {
						return
					}
					response := packet{ID: request.ID}
					switch request.Method {
					case "thread/start":
						response.Result, _ = json.Marshal(map[string]any{"thread": codexmodel.Thread{ID: "new-thread", Cwd: cwd, Name: test.title, Preview: test.preview}})
					case "thread/name/set":
						var params map[string]string
						json.Unmarshal(request.Params, &params)
						names <- params
					default:
						response.Error = &rpcError{Code: -32601, Message: "unexpected method: " + request.Method}
					}
					c.receive(response)
				}
			}()

			thread, err := s.CreateToolThread(cwd, "", "read-only", test.role, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if thread.Name != test.want {
				t.Fatalf("标题为 %q，预期 %q", thread.Name, test.want)
			}
			select {
			case params := <-names:
				if !test.rename || params["threadId"] != thread.ID || params["name"] != test.want {
					t.Fatalf("重命名请求错误: %+v", params)
				}
			default:
				if test.rename {
					t.Fatal("业务会话未发送重命名请求")
				}
			}
		})
	}
}

// TestManagedThreadNameUpdates 验证新建业务会话的延迟标题、更新通知和重复事件
func TestManagedThreadNameUpdates(t *testing.T) {
	for role, prefix := range map[string]string{
		"step":             "[AgentorWoker]",
		"controller":       "[AgentorCtl]",
		"pipeline_manager": "[AgentorPipe]",
		"pipeline_config":  "[AgentorPipe]",
	} {
		t.Run(role, func(t *testing.T) {
			s := accountFixture(t)
			var upstreamMu sync.Mutex
			upstream := codexmodel.Thread{ID: "new-thread", Cwd: t.TempDir()}
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			c := &client{stdin: writer, done: make(chan struct{}), pending: map[string]chan packet{}}
			c.onMessage = func(message packet) { s.trackNotification(0, message) }
			s.client, s.status.Phase = c, "ready"
			defer func() {
				s.mu.Lock()
				s.client = nil
				s.mu.Unlock()
			}()
			names := make(chan string, 10)
			reads := make(chan struct{}, 10)
			go func() {
				decoder := json.NewDecoder(reader)
				for {
					var request packet
					if decoder.Decode(&request) != nil {
						return
					}
					response := packet{ID: request.ID}
					upstreamMu.Lock()
					switch request.Method {
					case "thread/start", "thread/read":
						response.Result, _ = json.Marshal(map[string]any{"thread": upstream})
					case "thread/name/set":
						var params map[string]string
						json.Unmarshal(request.Params, &params)
						upstream.Name = params["name"]
						names <- upstream.Name
					default:
						response.Error = &rpcError{Code: -32601, Message: "unexpected method: " + request.Method}
					}
					upstreamMu.Unlock()
					c.receive(response)
					if request.Method == "thread/read" {
						reads <- struct{}{}
					}
				}
			}()

			thread, err := s.CreateToolThread(upstream.Cwd, "", "read-only", role, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if thread.Name != "" {
				t.Fatalf("初始空标题被覆盖为 %q", thread.Name)
			}

			// 等待本次事件触发的读取和串行更新完成，无需依赖定时休眠
			emit := func(method, name, preview, want string) {
				t.Helper()
				upstreamMu.Lock()
				upstream.Name, upstream.Preview = name, preview
				upstreamMu.Unlock()
				c.receive(packet{Method: method, Params: json.RawMessage(`{"threadId":"new-thread","item":{"type":"userMessage"},"turn":{"id":"turn"}}`)})
				select {
				case <-reads:
				case <-time.After(2 * time.Second):
					t.Fatal("未重新读取新会话标题")
				}
				n := s.threadNames[thread.ID]
				n.mu.Lock()
				n.mu.Unlock()
				select {
				case got := <-names:
					if want == "" || got != want {
						t.Fatalf("%s 改名为 %q，预期 %q", method, got, want)
					}
				default:
					if want != "" {
						t.Fatalf("%s 未改名为 %q", method, want)
					}
				}
			}
			emit("item/completed", "", "", "")
			emit("item/completed", "", "首条输入生成的预览", prefix+" 首条输入生成的预览")
			emit("turn/completed", "原生有效标题", "首条输入生成的预览", prefix+" 原生有效标题")
			emit("thread/name/updated", "后续生成的标题", "首条输入生成的预览", prefix+" 后续生成的标题")
			for range 3 {
				emit("thread/name/updated", prefix+" 后续生成的标题", "首条输入生成的预览", "")
				emit("turn/completed", prefix+" 后续生成的标题", "首条输入生成的预览", "")
			}

			if err := s.data.RememberManagedThread("old-thread", upstream.Cwd, role); err != nil {
				t.Fatal(err)
			}
			for _, method := range []string{"item/completed", "turn/completed", "thread/name/updated"} {
				c.receive(packet{Method: method, Params: json.RawMessage(`{"threadId":"old-thread","item":{"type":"userMessage"},"turn":{"id":"turn"}}`)})
				s.trackNotification(1, packet{Method: method, Params: json.RawMessage(`{"threadId":"new-thread","item":{"type":"userMessage"},"turn":{"id":"turn"}}`)})
			}
			select {
			case <-reads:
				t.Fatal("不应处理旧会话或过期连接的标题")
			default:
			}
		})
	}
}
