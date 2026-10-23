package codex

import (
	"agentor/internal/constant"
)

// sandboxPolicy 将权限模式转换为原生沙盒策略
func sandboxPolicy(mode, cwd string) map[string]any {
	switch mode {
	case constant.AppDataPermissionWorkspaceWrite:
		return map[string]any{"type": "workspaceWrite", "writableRoots": []string{cwd}, "networkAccess": false}
	case constant.AppDataPermissionDangerFullAccess:
		return map[string]any{"type": "dangerFullAccess"}
	default:
		return map[string]any{"type": "readOnly", "networkAccess": false}
	}
}
