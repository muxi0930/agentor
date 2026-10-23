package pipeline

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"path/filepath"

	"agentor/internal/appdata"
	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
)

// SaveTemplate 保存可重复应用且不绑定项目的模板
func (s *Service) SaveTemplate(text string) (pipelinemodel.Task, error) {
	task, err := s.ValidateTemplate(text)
	if err != nil {
		return task, err
	}
	if task.ID == "" {
		task.ID = rand.Text()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.state)
	found := false
	for i, item := range next.Templates {
		if item.ID == task.ID {
			next.Templates[i] = task
			found = true
			break
		}
	}
	if !found {
		next.Templates = append(next.Templates, task)
	}
	if err = appdata.WriteJSON(filepath.Join(s.root, constant.PipelineStateFilename), next); err != nil {
		return pipelinemodel.Task{}, err
	}
	s.state = next
	s.changed()
	return clone(task), nil
}

// DeleteTemplate 仅删除模板定义，不修改已复制的实例
func (s *Service) DeleteTemplate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.state)
	for i, item := range next.Templates {
		if item.ID == id {
			next.Templates = append(next.Templates[:i], next.Templates[i+1:]...)
			if err := appdata.WriteJSON(filepath.Join(s.root, constant.PipelineStateFilename), next); err != nil {
				return err
			}
			s.state = next
			s.changed()
			return nil
		}
	}
	return fmt.Errorf("模板 %s 不存在", id)
}

// Instantiate 复制模板并一次性校验所有步骤插槽
func (s *Service) Instantiate(input pipelinemodel.InstantiateInput) (pipelinemodel.Task, error) {
	var task pipelinemodel.Task
	for _, item := range s.GetState().Templates {
		if item.ID == input.TemplateID {
			task = item
			break
		}
	}
	if task.ID == "" {
		return task, fmt.Errorf("模板 %s 不存在", input.TemplateID)
	}
	task.TemplateID, task.ID, task.ProjectID, task.Directory = input.TemplateID, "", input.ProjectID, input.Directory
	if input.Name != "" {
		task.Name = input.Name
	}
	used := map[string]bool{}
	for i := range task.Actions {
		node := &task.Actions[i]
		if node.StepSlot != "" {
			node.StepID = input.Bindings[node.StepSlot]
			used[node.StepSlot] = true
		}
	}
	for slot := range input.Bindings {
		if !used[slot] {
			return pipelinemodel.Task{}, fmt.Errorf("模板中没有步骤插槽 %s", slot)
		}
	}
	raw, err := json.Marshal(task)
	if err != nil {
		return pipelinemodel.Task{}, err
	}
	return s.SaveTask(input.ProjectID, string(raw))
}
