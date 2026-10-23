package pipeline

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	pipelinemodel "agentor/internal/pipeline/model"
	"agentor/internal/tasks"
	tasksmodel "agentor/internal/tasks/model"
	"agentor/internal/workspace"
	workspacemodel "agentor/internal/workspace/model"
)

// TestTaskReferencesSnapshotAndReport 验证任务快照、结果引用与报告产物
func TestTaskReferencesSnapshotAndReport(t *testing.T) {
	s, projectID := fixture(t)
	task, err := s.SaveTask(projectID, textOf(snapshotTask()))
	if err != nil {
		t.Fatal(err)
	}

	run, err := s.RunTask(pipelinemodel.RunInput{TaskID: task.ID})
	if err != nil {
		t.Fatal(err)
	}

	// 编辑已保存任务时，当前运行仍使用创建时冻结的任务定义
	task.Name = "edited"
	task.Actions[1].Params["fileName"] = "edited.md"
	if _, err = s.SaveTask(projectID, textOf(task)); err != nil {
		t.Fatal(err)
	}

	completed := waitRun(t, s, run.ID)
	if completed.Status != "completed" || completed.Task.Name != "snapshot report" || len(completed.ArtifactIDs) != 1 {
		t.Fatal(completed)
	}

	files := s.files.GetPipelineArtifacts(run.ID)
	if len(files) != 1 || files[0].Name != "result.md" || files[0].ProjectID != projectID {
		t.Fatal(files)
	}

	content, err := os.ReadFile(files[0].LocalPath)
	if err != nil || !strings.Contains(string(content), `"name": "test"`) {
		t.Fatal(string(content), err)
	}

	deadline := time.Now().Add(time.Second)
	for len(s.tasks.GetState().Notifications) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	if len(s.tasks.GetState().Notifications) != 1 {
		t.Fatal("missing result notification")
	}

	reopened, err := NewService(s.data, s.work, s.tasks, s.files, nil)
	if err != nil {
		t.Fatal(err)
	}

	defer reopened.ServiceShutdown()
	if reopened.GetState().Runs[0].Status != "completed" {
		t.Fatal("finished run was replayed")
	}
}

// TestValidationRejectsForwardWrongTypeAndForeignReferences 验证前向引用、错误类型与跨项目引用被拒绝
func TestValidationRejectsForwardWrongTypeAndForeignReferences(t *testing.T) {
	s, projectID := fixture(t)
	bad := snapshotTask()
	bad.Actions[0].Params = map[string]any{"versionId": map[string]any{"result": "report"}}
	if _, err := s.ValidateTask(projectID, textOf(bad)); err == nil {
		t.Fatal("forward reference accepted")
	}

	bad = snapshotTask()
	bad.Inputs = []pipelinemodel.InputField{{Name: "version", Type: "number", Required: true}}
	bad.Actions[0].Params = map[string]any{"versionId": map[string]any{"input": "version"}}
	if _, err := s.ValidateTask(projectID, textOf(bad)); err == nil {
		t.Fatal("wrong parameter type accepted")
	}

	bad = snapshotTask()
	bad.Actions[1].Params["content"] = map[string]any{"result": "snapshot", "path": "project.name"}
	if _, err := s.ValidateTask(projectID, textOf(bad)); err == nil {
		t.Fatal("path evaluation accepted")
	}

	bad = snapshotTask()
	bad.Actions[1].Params["fileName"] = "../result.md"
	if _, err := s.ValidateTask(projectID, textOf(bad)); err == nil {
		t.Fatal("report path escape accepted")
	}

	state, err := s.work.SaveProject(workspacemodel.Project{Name: "other", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	other := state.SelectedID
	state, err = s.work.SaveVersion(other, workspacemodel.Version{Name: "v"})
	if err != nil {
		t.Fatal(err)
	}

	state, err = s.work.SaveFeature(other, state.SelectedID, workspacemodel.Feature{Title: "f"})
	if err != nil {
		t.Fatal(err)
	}

	state, err = s.work.SaveStep(other, state.SelectedID, workspacemodel.Step{Title: "s"})
	if err != nil {
		t.Fatal(err)
	}

	bad = pipelinemodel.Task{Name: "foreign", Actions: []pipelinemodel.Action{{ID: "step", Type: "step.run", Params: map[string]any{"stepId": state.SelectedID, "instruction": "test"}}}}
	if _, err = s.ValidateTask(projectID, textOf(bad)); err == nil {
		t.Fatal("foreign project step accepted")
	}
}

// TestSchedulesMisfireAnchorAndNonReentry 验证错过触发策略、间隔锚点与禁止重入
func TestSchedulesMisfireAnchorAndNonReentry(t *testing.T) {
	s, projectID := fixture(t)
	task, err := s.SaveTask(projectID, textOf(snapshotTask()))
	if err != nil {
		t.Fatal(err)
	}

	anchor := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	at := anchor.Add(10*time.Minute + 3*time.Second)
	if next := nextInterval(anchor, 60, at); !next.Equal(anchor.Add(11 * time.Minute)) {
		t.Fatal(next)
	}

	for _, policy := range []string{"skip", "fire_once"} {
		_, err = s.SaveSchedule(pipelinemodel.Schedule{
			ProjectID:       projectID,
			TaskID:          task.ID,
			Name:            policy,
			Kind:            "interval",
			Enabled:         true,
			Anchor:          utc(anchor),
			IntervalSeconds: 60,
			Timezone:        "Asia/Shanghai",
			MisfirePolicy:   policy,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	s.tick(at, true)
	state := s.GetState()
	if len(state.Runs) != 1 || !strings.HasPrefix(state.Schedules[0].LastOutcome, "skipped") || state.Schedules[0].NextAt != utc(anchor.Add(11*time.Minute)) {
		t.Fatal(state)
	}

	completed := waitRun(t, s, state.Runs[0].ID)
	if completed.Status != "completed" {
		t.Fatal(completed)
	}

	s.tick(at, false)
	if len(s.GetState().Runs) != 1 {
		t.Fatal("missed cycles were replayed")
	}

	s.mu.Lock()
	s.state.Runs = append(s.state.Runs, pipelinemodel.Run{ID: "active", TaskID: task.ID, Status: "running"})
	s.mu.Unlock()
	if _, err = s.RunTask(pipelinemodel.RunInput{TaskID: task.ID}); err == nil {
		t.Fatal("same task reentered")
	}

	for _, schedule := range s.GetState().Schedules {
		if err = s.SetScheduleEnabled(projectID, schedule.ID, false); err != nil {
			t.Fatal(err)
		}
	}

	s.tick(at.Add(time.Hour), false)
	if len(s.GetState().Runs) != 2 {
		t.Fatal("disabled schedule triggered")
	}
}

// TestOneTimeScheduleAndTriggerSource 验证一次性调度、手动来源与时区转换
func TestOneTimeScheduleAndTriggerSource(t *testing.T) {
	s, projectID := fixture(t)
	task, err := s.SaveTask(projectID, textOf(snapshotTask()))
	if err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	schedule, err := s.SaveSchedule(pipelinemodel.Schedule{
		ProjectID:     projectID,
		TaskID:        task.ID,
		Name:          "once",
		Kind:          "once",
		Enabled:       true,
		At:            utc(at),
		Timezone:      "Asia/Shanghai",
		MisfirePolicy: "fire_once",
	})
	if err != nil {
		t.Fatal(err)
	}

	s.tick(at.Add(time.Hour), true)
	state := s.GetState()
	if state.Schedules[0].Enabled || len(state.Runs) != 1 {
		t.Fatal(state)
	}

	waitRun(t, s, state.Runs[0].ID)
	s.tick(at.Add(2*time.Hour), false)
	if len(s.GetState().Runs) != 1 {
		t.Fatal("one-time task repeated")
	}

	if err = s.SetScheduleEnabled(projectID, schedule.ID, true); err != nil {
		t.Fatal(err)
	}

	if s.GetState().Schedules[0].NextAt != utc(at) {
		t.Fatal("one-time deadline was not restored")
	}

	if err = s.SetScheduleEnabled(projectID, schedule.ID, false); err != nil {
		t.Fatal(err)
	}

	run, err := s.TriggerScheduleFor(projectID, schedule.ID, pipelinemodel.RunInput{
		Model:  "chosen",
		Effort: "high",
		Source: tasksmodel.Source{Type: "lark", ChatID: "test-chat", SenderID: "test-user"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if run.Source.ChatID != "test-chat" || run.Model != "chosen" || run.ScheduleID != schedule.ID {
		t.Fatal(run)
	}

	waitRun(t, s, run.ID)
	instant, err := s.ResolveScheduleTime("2026-10-20T09:30:00", "Asia/Shanghai")
	if err != nil || instant != "2026-10-20T01:30:00Z" {
		t.Fatal(instant, err)
	}
}

// fixture 创建隔离的流水线测试环境
func fixture(t *testing.T) (*Service, string) {
	t.Helper()
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	data, err := appdata.NewService("test")
	if err != nil {
		t.Fatal(err)
	}

	data.SetModelSelection("model", "medium")
	work, err := workspace.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		t.Fatal(err)
	}

	state, err := work.SaveProject(workspacemodel.Project{Name: "test", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	files, err := artifact.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		t.Fatal(err)
	}

	executions, err := tasks.NewService(data, nil, work, files)
	if err != nil {
		t.Fatal(err)
	}

	s, err := NewService(data, work, executions, files, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		s.ServiceShutdown()
		executions.ServiceShutdown()
	})
	return s, state.SelectedID
}

// textOf 将任务编码为测试使用的 JSON 配置
func textOf(task pipelinemodel.Task) string {
	raw, _ := json.Marshal(task)
	return string(raw)
}

// snapshotTask 创建读取快照并输出报告的测试任务
func snapshotTask() pipelinemodel.Task {
	return pipelinemodel.Task{
		Name:   "snapshot report",
		Inputs: []pipelinemodel.InputField{},
		Actions: []pipelinemodel.Action{
			{ID: "snapshot", Type: "workspace.snapshot", Params: map[string]any{}},
			{ID: "report", Type: "report.write", Params: map[string]any{
				"fileName": "result.md",
				"content":  map[string]any{"result": "snapshot"},
			}},
		},
	}
}

// waitRun 等待指定测试运行完成或超时
func waitRun(t *testing.T, s *Service, id string) pipelinemodel.Run {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, run := range s.GetState().Runs {
			if run.ID == id && run.Status != "running" && run.Status != "queued" {
				return run
			}
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("run did not finish")
	return pipelinemodel.Run{}
}
