package appdata

import (
	appdatamodel "agentor/internal/appdata/model"
	"agentor/internal/constant"
)

// SetPermission 保存账号默认权限或项目会话覆盖值
func (s *Service) SetPermission(scope, id, mode string) (appdatamodel.Settings, error) {
	if mode != constant.AppDataPermissionReadOnly && mode != constant.AppDataPermissionWorkspaceWrite && mode != constant.AppDataPermissionDangerFullAccess && mode != "" {
		return appdatamodel.Settings{}, constant.ErrAppDataPermissionModeUnsupported
	}
	if scope == constant.AppDataPermissionScopeDefault && mode == "" {
		return appdatamodel.Settings{}, constant.ErrAppDataDefaultPermissionRequired
	}
	if scope != constant.AppDataPermissionScopeDefault && scope != constant.WorkspaceKindProject && scope != constant.AppDataPermissionScopeThread {
		return appdatamodel.Settings{}, constant.ErrAppDataPermissionScopeUnknown
	}
	if scope != constant.AppDataPermissionScopeDefault && id == "" {
		return appdatamodel.Settings{}, constant.ErrAppDataPermissionTargetRequired
	}

	return s.update(func(settings *appdatamodel.Settings) {
		if scope == constant.AppDataPermissionScopeDefault {
			settings.PermissionMode = mode
			rememberPreferences(settings)
			return
		}
		next := map[string]string{}
		source := settings.ProjectPermissions
		if scope == constant.AppDataPermissionScopeThread {
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
		if scope == constant.AppDataPermissionScopeThread {
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
