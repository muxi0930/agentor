package appdata

import "errors"

// SetPermission 保存账号默认权限或项目会话覆盖值
func (s *Service) SetPermission(scope, id, mode string) (Settings, error) {
	if mode != "read-only" && mode != "workspace-write" && mode != "danger-full-access" && mode != "" {
		return Settings{}, errors.New("不支持的权限模式")
	}
	if scope == "default" && mode == "" {
		return Settings{}, errors.New("请选择默认权限")
	}
	if scope != "default" && scope != "project" && scope != "thread" {
		return Settings{}, errors.New("未知权限范围")
	}
	if scope != "default" && id == "" {
		return Settings{}, errors.New("请选择项目或会话")
	}

	return s.update(func(settings *Settings) {
		if scope == "default" {
			settings.PermissionMode = mode
			rememberPreferences(settings)
			return
		}
		next := map[string]string{}
		source := settings.ProjectPermissions
		if scope == "thread" {
			source = settings.ThreadPermissions
		}
		for key, value := range source {
			next[key] = value
		}
		if mode == "" {
			delete(next, id)
		} else {
			next[id] = mode
		}
		if scope == "thread" {
			settings.ThreadPermissions = next
		} else {
			settings.ProjectPermissions = next
		}
	})
}

// EffectivePermission 按会话项目账号的顺序取得最终权限
//
//wails:ignore
func (s *Service) EffectivePermission(threadID, cwd string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if value := s.settings.ThreadPermissions[threadID]; value != "" {
		return value
	}
	if value := s.settings.ProjectPermissions[cwd]; value != "" {
		return value
	}
	return s.settings.PermissionMode
}
