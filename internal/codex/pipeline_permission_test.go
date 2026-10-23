package codex

import (
	"encoding/json"
	"io"
	"testing"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
)

// TestPipelineNodePermissions 验证执行节点保留命令审批入口并传递显式权限
func TestPipelineNodePermissions(t *testing.T) {
	for _, permission := range []string{constant.AppDataPermissionWorkspaceWrite, constant.AppDataPermissionDangerFullAccess} {
		t.Run(permission, func(t *testing.T) {
			s := accountFixture(t)
			cwd := t.TempDir()
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			c := &client{stdin: writer, done: make(chan struct{}), pending: map[string]chan packet{}}
			s.client, s.status.Phase = c, "ready"
			defer func() { s.client = nil }()
			requests := make(chan map[string]any, 2)
			go func() {
				decoder := json.NewDecoder(reader)
				for {
					var request packet
					if decoder.Decode(&request) != nil {
						return
					}
					var params map[string]any
					json.Unmarshal(request.Params, &params)
					requests <- params
					result, _ := json.Marshal(map[string]any{"thread": codexmodel.Thread{ID: "node", Cwd: cwd}, "turn": map[string]string{"id": "turn"}})
					c.receive(packet{ID: request.ID, Result: result})
				}
			}()
			if _, err := s.CreateToolThread(cwd, "", permission, constant.ToolRouterRolePipelineNode, "", nil); err != nil {
				t.Fatal(err)
			}
			if params := <-requests; params["approvalPolicy"] != "on-request" || params["sandbox"] != permission {
				t.Fatal("执行节点错误沿用了管理会话的审批策略", params)
			}
			if _, err := s.StartManagedTurn("node", "execute", "", "", permission, cwd, "", nil, ""); err != nil {
				t.Fatal(err)
			}
			params := <-requests
			want := "workspaceWrite"
			if permission == constant.AppDataPermissionDangerFullAccess {
				want = "dangerFullAccess"
			}
			if params["approvalPolicy"] != "on-request" || params["sandboxPolicy"].(map[string]any)["type"] != want {
				t.Fatal("轮次未采用节点显式配置的权限", params)
			}
		})
	}
}
