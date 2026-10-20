package tasks

import "errors"

func (s *Service) resolveTarget(input ControllerInput) (ControllerInput, bool, error) {
	needsTarget, inferred := false, false
	s.mu.Lock()
	if input.StepID == "" && input.Source.ReplyToMessageID != "" {
		matches := map[string]bool{}
		for _, execution := range s.state.Executions {
			if execution.Source.AppID == input.Source.AppID && execution.Source.ChatID == input.Source.ChatID && execution.Source.MessageID == input.Source.ReplyToMessageID {
				matches[execution.StepID] = true
			}
		}
		if len(matches) == 1 {
			for id := range matches {
				input.StepID = id
			}
			inferred = true
		} else {
			needsTarget = true
		}
	} else if input.StepID == "" && isContinue(input.Text) {
		input.StepID = s.state.LastSteps[sourceKey(input.Source)]
		inferred = true
	}
	s.mu.Unlock()
	if input.StepID != "" {
		context, err := s.work.GetStepContext(input.StepID)
		if err != nil {
			if !inferred {
				return input, false, err
			}
			input.StepID = ""
			needsTarget = true
		} else {
			if input.ProjectID != "" && input.ProjectID != context.ProjectID {
				if !inferred {
					return input, false, errors.New("目标步骤不属于当前请求项目")
				}
				input.StepID = ""
				needsTarget = true
			} else {
				input.ProjectID = context.ProjectID
			}
		}
	}
	if input.StepID == "" && isContinue(input.Text) {
		needsTarget = true
	}
	if input.ProjectID != "" {
		found := false
		for _, project := range s.work.GetState().Projects {
			if project.ID == input.ProjectID {
				found = true
				break
			}
		}
		if !found {
			return input, false, errors.New("请求项目不存在")
		}
	}
	return input, needsTarget, nil
}
