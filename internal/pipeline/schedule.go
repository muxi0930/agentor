package pipeline

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"
	_ "time/tzdata"

	"agentor/internal/tasks"
)

func instant(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }
func utc(value time.Time) string              { return value.UTC().Format(time.RFC3339Nano) }
func (s *Service) ResolveScheduleTime(local, timezone string) (string, error) {
	zone, err := time.LoadLocation(timezone)
	if err != nil {
		return "", err
	}
	at, err := time.ParseInLocation("2006-01-02T15:04:05", local, zone)
	if err != nil {
		return "", err
	}
	return utc(at), nil
}
func nextInterval(anchor time.Time, seconds int64, after time.Time) time.Time {
	if after.Before(anchor) {
		return anchor
	}
	interval := time.Duration(seconds) * time.Second
	return anchor.Add((after.Sub(anchor)/interval + 1) * interval)
}
func (s *Service) SaveSchedule(input Schedule) (Schedule, error) {
	input = clone(input)
	if input.Name == "" {
		return Schedule{}, errors.New("请填写调度名称")
	}
	if input.Timezone == "" {
		input.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return Schedule{}, errors.New("无效的 IANA 时区")
	}
	if input.MisfirePolicy != "skip" && input.MisfirePolicy != "fire_once" {
		return Schedule{}, errors.New("错过策略必须为 skip 或 fire_once")
	}
	var next time.Time
	switch input.Kind {
	case "once":
		at, err := instant(input.At)
		if err != nil {
			return Schedule{}, fmt.Errorf("一次性时间需包含时区：%w", err)
		}
		input.At = utc(at)
		next = at
	case "interval":
		if input.IntervalSeconds < 1 || input.IntervalSeconds > int64((1<<63-1)/int64(time.Second)) {
			return Schedule{}, errors.New("间隔秒数无效")
		}
		anchor, err := instant(input.Anchor)
		if err != nil {
			return Schedule{}, fmt.Errorf("锚点需包含时区：%w", err)
		}
		input.Anchor = utc(anchor)
		next = anchor
	default:
		return Schedule{}, errors.New("调度类型仅支持 once / interval")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var task Task
	for _, item := range s.state.Tasks {
		if item.ID == input.TaskID && item.ProjectID == input.ProjectID {
			task = item
			break
		}
	}
	if task.ID == "" {
		return Schedule{}, errors.New("任务不属于此项目")
	}
	if err := validateInputs(task, input.Inputs); err != nil {
		return Schedule{}, err
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
				return Schedule{}, errors.New("调度不属于此项目")
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
		return Schedule{}, err
	}
	s.changed()
	return clone(input), nil
}
func (s *Service) writeState(state State) error {
	old := s.state
	s.state = state
	if err := s.persistLocked(); err != nil {
		s.state = old
		return err
	}
	return nil
}
func (s *Service) SetScheduleEnabled(projectID, id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.state)
	for i := range next.Schedules {
		schedule := &next.Schedules[i]
		if schedule.ID == id && schedule.ProjectID == projectID {
			schedule.Enabled = enabled
			if enabled && schedule.Kind == "interval" {
				anchor, _ := instant(schedule.Anchor)
				schedule.NextAt = utc(nextInterval(anchor, schedule.IntervalSeconds, time.Now().UTC()))
			}
			if enabled && schedule.Kind == "once" {
				schedule.NextAt = schedule.At
			}
			err := s.writeState(next)
			s.changed()
			return err
		}
	}
	return errors.New("调度不存在")
}
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
	return errors.New("调度不存在")
}
func (s *Service) TriggerSchedule(projectID, id string) (Run, error) {
	return s.TriggerScheduleFor(projectID, id, RunInput{Source: tasks.Source{Type: "schedule", MessageID: id}})
}

//wails:ignore
func (s *Service) TriggerScheduleFor(projectID, id string, input RunInput) (Run, error) {
	s.mu.Lock()
	var schedule Schedule
	for _, item := range s.state.Schedules {
		if item.ID == id && item.ProjectID == projectID {
			schedule = item
			break
		}
	}
	s.mu.Unlock()
	if schedule.ID == "" {
		return Run{}, errors.New("调度不存在")
	}
	input.TaskID, input.Inputs = schedule.TaskID, schedule.Inputs
	return s.startRun(input, schedule.ID)
}
func (s *Service) tick(at time.Time, startup bool) {
	s.mu.Lock()
	if s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	due := []Schedule{}
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
		if schedule.Kind == "once" {
			schedule.Enabled = false
			schedule.NextAt = ""
		} else {
			anchor, _ := instant(schedule.Anchor)
			schedule.NextAt = utc(nextInterval(anchor, schedule.IntervalSeconds, at))
		}
		if (startup || at.Sub(next) >= time.Second) && schedule.MisfirePolicy == "skip" {
			schedule.LastOutcome = "skipped: 应用关闭期间错过触发"
			continue
		}
		schedule.LastOutcome = "triggering"
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
		run, err := s.startRun(RunInput{TaskID: schedule.TaskID, Inputs: schedule.Inputs, Source: tasks.Source{Type: "schedule", MessageID: schedule.ID + ":" + utc(at)}}, schedule.ID)
		s.mu.Lock()
		if s.ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		for i := range s.state.Schedules {
			current := &s.state.Schedules[i]
			if current.ID == schedule.ID {
				current.LastOutcome = "started"
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
