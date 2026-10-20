package codex

func sandboxPolicy(mode, cwd string) map[string]any {
	switch mode {
	case "workspace-write":
		return map[string]any{"type": "workspaceWrite", "writableRoots": []string{cwd}, "networkAccess": false}
	case "danger-full-access":
		return map[string]any{"type": "dangerFullAccess"}
	default:
		return map[string]any{"type": "readOnly", "networkAccess": false}
	}
}
