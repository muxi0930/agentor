package pipeline

import (
	"crypto/rand"
	"fmt"
	"time"
	_ "time/tzdata"

	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
	tasksmodel "agentor/internal/tasks/model"
)

// ResolveScheduleTime 将指定时区的本地时间转换为 UTC
func (s *Service) ResolveScheduleTime(local, timezone string) (string, error) {
	zone, err := time.LoadLocation(timezone)
	if err != nil {
		return "", err
	}

	at, err := time.ParseInLocation(constant.PipelineLocalTimeLayout, local, zone)
	if err != nil {
		return "", err
	}

	return utc(at), nil
}

// SaveSchedule 校验并保存任务调度
func (s *Service) SaveSchedule(input pipelinemodel.Schedule) (pipelinemodel.Schedule, error) {
	input = clone(input)
	if input.Name == "" {
		return pipelinemodel.Schedule{}, constant.ErrPipelineScheduleNameRequired
	}

	if input.Timezone == "" {
		input.Timezone = "UTC"
	}

	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return pipelinemodel.Schedule{}, constant.ErrPipelineTimezoneInvalid
	}

	if input.MisfirePolicy != constant.PipelineMisfireSkip && input.MisfirePolicy != constant.PipelineMisfireFireOnce {
		return pipelinemodel.Schedule{}, constant.ErrPipelineMisfirePolicyInvalid
	}

	var next time.Time
	switch input.Kind {
	case constant.PipelineScheduleKindOnce:
		at, err := instant(input.At)
		if err != nil {
			return pipelinemodel.Schedule{}, fmt.Errorf("一次性时间需包含时区：%w", err)
		}

		input.At = utc(at)
		next = at
	case constant.PipelineScheduleKindInterval:
		if input.IntervalSeconds < 1 || input.IntervalSeconds > int64((1<<63-1)/int64(time.Second)) {
			return pipelinemodel.Schedule{}, constant.ErrPipelineIntervalInvalid
		}

		anchor, err := instant(input.Anchor)
		if err != nil {
			return pipelinemodel.Schedule{}, fmt.Errorf("锚点需包含时区：%w", err)
		}

		input.Anchor = utc(anchor)
		next = anchor
	default:
		return pipelinemodel.Schedule{}, constant.ErrPipelineScheduleKindInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var task pipelinemodel.Task
	for _, item := range s.state.Tasks {
		if item.ID == input.TaskID && item.ProjectID == input.ProjectID {
			task = item
			break
		}
	}

	if task.ID == "" {
		return pipelinemodel.Schedule{}, constant.ErrPipelineTaskOutsideProject
	}

	if err := validateInputs(task, input.Inputs); err != nil {
		return pipelinemodel.Schedule{}, err
	}

	copy := clone(s.state)
	if input.ID == "" {
		input.ID = rand.Text()
	}

	input.NextAt = utc(next)
	input.LastAt, input.LastOutcome, input.LastRunID = "", "", ""
	found := false
	for i, existing := range copy.Schedules {
		if existing.ID == input.ID {
			if existing.ProjectID != input.ProjectID {
				return pipelinemodel.Schedule{}, constant.ErrPipelineScheduleOutsideProject
			}

			input.LastAt, input.LastOutcome, input.LastRunID = existing.LastAt, existing.LastOutcome, existing.LastRunID
			copy.Schedules[i] = input
			found = true
			break
		}
	}

	if !found {
		copy.Schedules = append(copy.Schedules, input)
	}

	if err := s.writeState(copy); err != nil {
		return pipelinemodel.Schedule{}, err
	}

	s.changed()
	return clone(input), nil
}

// SetScheduleEnabled 切换调度状态并恢复下次触发时间
func (s *Service) SetScheduleEnabled(projectID, id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := clone(s.state)
	for i := range next.Schedules {
		schedule := &next.Schedules[i]
		if schedule.ID == id && schedule.ProjectID == projectID {
			schedule.Enabled = enabled
			if enabled && schedule.Kind == constant.PipelineScheduleKindInterval {
				anchor, _ := instant(schedule.Anchor)
				schedule.NextAt = utc(nextInterval(anchor, schedule.IntervalSeconds, time.Now().UTC()))
			}

			if enabled && schedule.Kind == constant.PipelineScheduleKindOnce {
				schedule.NextAt = schedule.At
			}

			err := s.writeState(next)
			s.changed()
			return err
		}
	}

	return constant.ErrPipelineScheduleNotFound
}

// DeleteSchedule 移除当前项目的指定调度
func (s *Service) DeleteSchedule(projectID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := clone(s.state)
	for i, item := range next.Schedules {
		if item.ID == id && item.ProjectID == projectID {
			next.Schedules = append(next.Schedules[:i], next.Schedules[i+1:]...)
			err := s.writeState(next)
			s.changed()
			return err
		}
	}

	return constant.ErrPipelineScheduleNotFound
}

// TriggerSchedule 手动触发调度并记录调度来源
func (s *Service) TriggerSchedule(projectID, id string) (pipelinemodel.Run, error) {
	return s.TriggerScheduleFor(projectID, id, pipelinemodel.RunInput{Source: tasksmodel.Source{Type: "schedule", MessageID: id}})
}

// TriggerScheduleFor 按指定来源触发调度对应的任务
//
//wails:ignore
func (s *Service) TriggerScheduleFor(projectID, id string, input pipelinemodel.RunInput) (pipelinemodel.Run, error) {
	s.mu.Lock()
	var schedule pipelinemodel.Schedule
	for _, item := range s.state.Schedules {
		if item.ID == id && item.ProjectID == projectID {
			schedule = item
			break
		}
	}

	s.mu.Unlock()
	if schedule.ID == "" {
		return pipelinemodel.Run{}, constant.ErrPipelineScheduleNotFound
	}

	input.TaskID, input.Inputs = schedule.TaskID, schedule.Inputs
	return s.startRun(input, schedule.ID)
}

// instant 解析包含时区的时间字符串
func instant(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

// utc 将时间格式化为 UTC 字符串
func utc(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

// nextInterval 按固定锚点计算下一次间隔触发时间
func nextInterval(anchor time.Time, seconds int64, after time.Time) time.Time {
	if after.Before(anchor) {
		return anchor
	}

	interval := time.Duration(seconds) * time.Second
	return anchor.Add((after.Sub(anchor)/interval + 1) * interval)
}

// writeState 保存新状态并在写入失败时恢复内存状态
func (s *Service) writeState(state pipelinemodel.State) error {
	old := s.state
	s.state = state
	if err := s.persistLocked(); err != nil {
		s.state = old
		return err
	}

	return nil
}

// tick 检查到期调度并启动符合策略的任务
func (s *Service) tick(at time.Time, startup bool) {
	s.mu.Lock()
	if s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}

	due := []pipelinemodel.Schedule{}
	changed := false
	for i := range s.state.Schedules {
		schedule := &s.state.Schedules[i]
		if !schedule.Enabled {
			continue
		}

		next, err := instant(schedule.NextAt)
		if err != nil || next.After(at) {
			continue
		}

		changed = true
		schedule.LastAt = utc(at)
		if schedule.Kind == constant.PipelineScheduleKindOnce {
			schedule.Enabled = false
			schedule.NextAt = ""
		} else {
			anchor, _ := instant(schedule.Anchor)
			schedule.NextAt = utc(nextInterval(anchor, schedule.IntervalSeconds, at))
		}

		if (startup || at.Sub(next) >= time.Second) && schedule.MisfirePolicy == constant.PipelineMisfireSkip {
			schedule.LastOutcome = constant.PipelineScheduleMisfireMessage
			continue
		}

		schedule.LastOutcome = constant.PipelineScheduleOutcomeTriggering
		due = append(due, clone(*schedule))
	}

	if changed {
		if err := s.persistLocked(); err != nil {
			s.mu.Unlock()
			return
		}
	}

	s.mu.Unlock()
	if changed {
		s.changed()
	}

	for _, schedule := range due {
		run, err := s.startRun(pipelinemodel.RunInput{
			TaskID: schedule.TaskID,
			Inputs: schedule.Inputs,
			Source: tasksmodel.Source{Type: "schedule", MessageID: schedule.ID + ":" + utc(at)},
		}, schedule.ID)
		s.mu.Lock()
		if s.ctx.Err() != nil {
			s.mu.Unlock()
			return
		}

		for i := range s.state.Schedules {
			current := &s.state.Schedules[i]
			if current.ID == schedule.ID {
				current.LastOutcome = constant.PipelineScheduleOutcomeStarted
				current.LastRunID = run.ID
				if err != nil {
					current.LastOutcome = "skipped: " + err.Error()
				}

				break
			}
		}

		s.persistLocked()
		s.mu.Unlock()
		s.changed()
	}
}
