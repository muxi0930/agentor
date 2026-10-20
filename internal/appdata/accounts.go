package appdata

type AccountPreferences struct {
	Model      string `json:"model"`      // 当前模型
	Effort     string `json:"effort"`     // 推理强度
	Permission string `json:"permission"` // 账号默认权限
}

// SetAccountModel 保存账号模型，在更换模型时清除旧推理强度
//
//wails:ignore
func (s *Service) SetAccountModel(id, model string) (Settings, error) {
	return s.update(func(settings *Settings) {
		next := make(map[string]AccountPreferences, len(settings.AccountPreferences)+1)
		for key, preferences := range settings.AccountPreferences {
			next[key] = preferences
		}
		preferences := next[id]
		if preferences.Model != model {
			preferences.Effort = ""
		}
		preferences.Model = model
		if preferences.Permission == "" {
			preferences.Permission = "read-only"
		}
		next[id] = preferences
		settings.AccountPreferences = next
		if !settings.ReuseCLI && settings.ActiveAccountID == id {
			settings.Model, settings.Effort = preferences.Model, preferences.Effort
		}
	})
}

// SelectAccount 切换独立账号或 CLI 接入并恢复各自偏好
//
//wails:ignore
func (s *Service) SelectAccount(id string, reuseCLI bool) (Settings, error) {
	return s.update(func(settings *Settings) {
		rememberPreferences(settings)
		settings.ActiveAccountID, settings.ReuseCLI = id, reuseCLI
		key := id
		if reuseCLI {
			key = "cli"
		}
		preferences, exists := settings.AccountPreferences[key]
		if !exists {
			preferences.Permission = "read-only"
		}
		settings.Model, settings.Effort, settings.PermissionMode = preferences.Model, preferences.Effort, preferences.Permission
	})
}

// rememberPreferences 保存当前身份的偏好，不改变项目与会话的覆盖设置
func rememberPreferences(settings *Settings) {
	key := settings.ActiveAccountID
	if settings.ReuseCLI {
		key = "cli"
	}
	if key == "" {
		return
	}

	next := make(map[string]AccountPreferences, len(settings.AccountPreferences)+1)
	for id, preferences := range settings.AccountPreferences {
		next[id] = preferences
	}
	next[key] = AccountPreferences{Model: settings.Model, Effort: settings.Effort, Permission: settings.PermissionMode}
	settings.AccountPreferences = next
}
