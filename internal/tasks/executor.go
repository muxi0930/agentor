package tasks

import (
	"fmt"
	"slices"
	"strings"

	appdatamodel "agentor/internal/appdata/model"
	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	tasksmodel "agentor/internal/tasks/model"
)

// SelectExecutor 检查控制中心所选执行配置是否在请求范围内且被模型支持
//
//wails:ignore
func (s *Service) SelectExecutor(request tasksmodel.Request, model, effort string) (string, string, error) {
	models, err := s.runtime.GetModels()
	if err != nil {
		return "", "", err
	}
	return selectExecutor(request, model, effort, models)
}

// executorSettings 冻结控制中心配置并解析跟随默认会话的选项
func (s *Service) executorSettings(model, effort string) appdatamodel.ControllerSettings {
	config := s.data.GetSettings().Controller
	config.Models = append([]string{}, config.Models...)
	config.Efforts = append([]string{}, config.Efforts...)
	for i, value := range config.Models {
		if value == "" {
			config.Models[i] = model
		}
	}
	for i, value := range config.Efforts {
		if value == "" {
			config.Efforts[i] = effort
		}
	}
	return config
}

// controllerPrompt 组合控制中心指令、派发提示词与末尾安全提示词
func controllerPrompt(config appdatamodel.ControllerSettings) string {
	prompt := config.Prompt + "\n\n" + fmt.Sprintf(constant.TasksExecutorChoiceTemplate, strings.Join(config.Models, ","), strings.Join(config.Efforts, ",")) + "\n" + config.DecisionPrompt + constant.TasksExecutorSelectionInstructions
	prompt += constant.TasksVersionCreationInstructions + constant.TasksVersionWorktreeInstructions
	securityPrompt := constant.AppDataDefaultControllerSecurityPrompt
	if config.SecurityPrompt != nil {
		securityPrompt = *config.SecurityPrompt
	}
	if securityPrompt != "" {
		prompt += "\n\n" + securityPrompt
	}
	return prompt
}

// selectExecutor 解析默认选择并拒绝越界或不兼容的执行配置
func selectExecutor(request tasksmodel.Request, model, effort string, models []codexmodel.Model) (string, string, error) {
	if model == "" {
		model = request.Model
	}
	if effort == "" {
		effort = request.Effort
	}
	allowedModels, allowedEfforts := request.Executor.Models, request.Executor.Efforts
	// 旧版排队请求只允许沿用其原始执行配置
	if len(allowedModels) == 0 {
		allowedModels = []string{request.Model}
	}
	if len(allowedEfforts) == 0 {
		allowedEfforts = []string{request.Effort}
	}
	if !slices.Contains(allowedModels, model) || !slices.Contains(allowedEfforts, effort) {
		return "", "", constant.ErrTasksExecutorChoiceOutsideScope
	}
	for _, option := range models {
		if option.Model != model {
			continue
		}
		if effort == "" && len(option.SupportedReasoningEfforts) == 0 {
			return model, effort, nil
		}
		for _, supported := range option.SupportedReasoningEfforts {
			if supported.ReasoningEffort == effort {
				return model, effort, nil
			}
		}
		return "", "", fmt.Errorf("模型 %s 不支持 effort %s，请从允许范围内重新选择兼容组合", model, effort)
	}
	return "", "", fmt.Errorf("模型 %s 当前不可用，请重新选择", model)
}
