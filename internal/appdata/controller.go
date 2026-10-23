package appdata

import (
	appdatamodel "agentor/internal/appdata/model"
	"agentor/internal/constant"
)

// SetControllerSettings 保存控制中心提示词与执行者选择范围
func (s *Service) SetControllerSettings(input appdatamodel.ControllerSettings) (appdatamodel.Settings, error) {
	if len(input.Models) == 0 || len(input.Efforts) == 0 {
		return appdatamodel.Settings{}, constant.ErrAppDataExecutorChoicesRequired
	}
	input.Models = append([]string{}, input.Models...)
	input.Efforts = append([]string{}, input.Efforts...)
	return s.update(func(settings *appdatamodel.Settings) { settings.Controller = input })
}
