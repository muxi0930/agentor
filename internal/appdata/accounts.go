package appdata

import (
	appdatamodel "agentor/internal/appdata/model"
	"agentor/internal/constant"
)

// SetAccountModel 保存账号模型，在更换模型时清除旧推理强度
//
//wails:ignore
func (s *Service) SetAccountModel(id, model string) (appdatamodel.Settings, error) {
	return s.update(func(settings *appdatamodel.Settings) {
		next := make(map[string]appdatamodel.AccountPreferences, len(settings.AccountPreferences)+1)
		for key, preferences := range settings.AccountPreferences {
			next[key] = preferences
		}
		preferences := next[id]
		if preferences.Model != model {
			preferences.Effort = ""
		}
		preferences.Model = model
		if preferences.Permission == "" {
			preferences.Permission = constant.AppDataPermissionWorkspaceWrite
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
func (s *Service) SelectAccount(id string, reuseCLI bool) (appdatamodel.Settings, error) {
	return s.update(func(settings *appdatamodel.Settings) {
		rememberPreferences(settings)
		settings.ActiveAccountID, settings.ReuseCLI = id, reuseCLI
		key := id
		if reuseCLI {
			key = "cli"
		}
		preferences, exists := settings.AccountPreferences[key]
		if !exists {
			preferences.Permission = constant.AppDataPermissionWorkspaceWrite
		}
		settings.Model, settings.Effort, settings.PermissionMode = preferences.Model, preferences.Effort, preferences.Permission
		settings.FastMode = preferences.FastMode
	})
}

// rememberPreferences 保存当前身份的偏好，不改变项目与会话的覆盖设置
func rememberPreferences(settings *appdatamodel.Settings) {
	key := settings.ActiveAccountID
	if settings.ReuseCLI {
		key = "cli"
	}
	if key == "" {
		return
	}

	next := make(map[string]appdatamodel.AccountPreferences, len(settings.AccountPreferences)+1)
	for id, preferences := range settings.AccountPreferences {
		next[id] = preferences
	}
	next[key] = appdatamodel.AccountPreferences{Model: settings.Model, Effort: settings.Effort, FastMode: settings.FastMode, Permission: settings.PermissionMode}
	settings.AccountPreferences = next
}
