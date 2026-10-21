package codex

import (
	"encoding/json"
	"io"
	"testing"

	"agentor/internal/appdata"
)

// TestThreadVisibilityFollowsSetting 验证默认和自定义目录都由勾选项控制外部会话及查询范围
func TestThreadVisibilityFollowsSetting(t *testing.T) {
	for _, mode := range []string{"system", "custom"} {
		for _, enabled := range []bool{false, true} {
			name := mode + "/disabled"
			if enabled {
				name = mode + "/enabled"
			}
			t.Run(name, func(t *testing.T) {
				t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
				t.Setenv("CODEX_HOME", t.TempDir())
				data, err := appdata.NewService("test")
				if err != nil {
					t.Fatal(err)
				}
				if err := data.SetCodexHome(mode, t.TempDir()); err != nil {
					t.Fatal(err)
				}
				if !enabled {
					if _, err := data.RememberThread("own", "/own-project"); err != nil {
						t.Fatal(err)
					}
				}
				if err := data.RememberExternalThread("opened-external", "/external-project"); err != nil {
					t.Fatal(err)
				}
				if _, err := data.SetReadExternalThreads(enabled); err != nil {
					t.Fatal(err)
				}
				s := NewService(data, nil)
				t.Cleanup(s.cancel)
				reader, writer := io.Pipe()
				t.Cleanup(func() { reader.Close(); writer.Close() })
				c := &client{stdin: writer, done: make(chan struct{}), pending: make(map[string]chan packet)}
				s.client, s.status.Phase = c, "ready"
				requests := make(chan packet, 1)
				go func() {
					var request packet
					if json.NewDecoder(reader).Decode(&request) != nil {
						return
					}
					requests <- request
					c.receive(packet{ID: request.ID, Result: json.RawMessage(`{"data":[{"id":"own","cwd":"/own-project"},{"id":"opened-external","cwd":"/external-project"},{"id":"new-external","cwd":"/new-project"}],"nextCursor":"next-page"}`)})
				}()
				page, err := s.ListThreads("current-page")
				if err != nil {
					t.Fatal(err)
				}
				want := 1
				if enabled {
					want = 3
				}
				if len(page.Data) != want || page.Data[0].ID != "own" {
					t.Fatalf("会话可见范围错误：%+v", page)
				}
				request := <-requests
				var params map[string]any
				if err := json.Unmarshal(request.Params, &params); err != nil {
					t.Fatal(err)
				}
				_, filtered := params["cwd"]
				if request.Method != "thread/list" || filtered == enabled || params["cursor"] != "current-page" {
					t.Fatalf("查询范围或分页错误：%s", request.Params)
				}
				if filtered {
					paths := params["cwd"].([]any)
					if len(paths) != 1 || paths[0] != "/own-project" {
						t.Fatalf("外部目录混入本应用范围：%v", paths)
					}
				}
			})
		}
	}
}
