package tasks

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"agentor/internal/appdata"
	"agentor/internal/codex"
)

// SelectExecutor 检查控制中心所选执行配置是否在请求范围内且被模型支持
//
//wails:ignore
func (s *Service) SelectExecutor(request Request, model, effort string) (string, string, error) {
	models, err := s.runtime.GetModels()
	if err != nil {
		return "", "", err
	}
	return selectExecutor(request, model, effort, models)
}

// executorSettings 冻结控制中心配置并解析跟随默认会话的选项
func (s *Service) executorSettings(model, effort string) appdata.ControllerSettings {
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
func controllerPrompt(config appdata.ControllerSettings) string {
	prompt := config.Prompt + "\n\n" + fmt.Sprintf("你可以在 [ %s ] 模型中选择，并在 [ %s ] 中决定 effort；", strings.Join(config.Models, ","), strings.Join(config.Efforts, ",")) + "\n" + config.DecisionPrompt + "\n派发步骤、运行流水线或请求专用会话时，必须通过工具的 model 和 effort 参数明确选择。空字符串表示跟随本次默认会话，但只有解析后的值在允许范围内时才可使用。仅选择模型支持的 effort；工具返回不兼容或超出范围时，重新选择允许的组合，不要重复提交相同组合。"
	prompt += "\n创建项目版本时使用 create_version，显式传入本次请求范围内的 projectId；无法唯一确定目标项目时先询问用户。"
	securityPrompt := appdata.DefaultControllerSecurityPrompt
	if config.SecurityPrompt != nil {
		securityPrompt = *config.SecurityPrompt
	}
	if securityPrompt != "" {
		prompt += "\n\n" + securityPrompt
	}
	return prompt
}

// selectExecutor 解析默认选择并拒绝越界或不兼容的执行配置
func selectExecutor(request Request, model, effort string, models []codex.Model) (string, string, error) {
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
		return "", "", errors.New("所选模型或 effort 不在本次请求允许范围内，请重新选择")
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
