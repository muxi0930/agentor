package codex

import (
	"encoding/json"
	"io"
	"testing"
)

// TestManagedThreadNames 验证业务会话创建后设置固定标题前缀且普通会话不改名
func TestManagedThreadNames(t *testing.T) {
	for _, test := range []struct {
		role  string // 会话角色
		title string // 上游初始标题
		want  string // 预期标题
	}{
		{"step", "任务", "[AgentorWoker] 任务"},
		{"controller", "", "[AgentorCtl]"},
		{"pipeline_manager", "管理", "[AgentorPipe] 管理"},
		{"pipeline_config", "配置", "[AgentorPipe] 配置"},
		{"", "普通", "普通"},
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
						response.Result, _ = json.Marshal(map[string]any{"thread": Thread{ID: "new-thread", Cwd: cwd, Name: test.title}})
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
				if test.role == "" || params["threadId"] != thread.ID || params["name"] != test.want {
					t.Fatalf("重命名请求错误: %+v", params)
				}
			default:
				if test.role != "" {
					t.Fatal("业务会话未发送重命名请求")
				}
			}
		})
	}
}
