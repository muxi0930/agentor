package lifecycle

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"

	"agentor/internal/appdata"
	"agentor/internal/codex"
	"agentor/internal/pipeline"
	"agentor/internal/tasks"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type PendingTask struct {
	ID      string `json:"id"`      // 分类内的任务标识
	Kind    string `json:"kind"`    // 用户可辨识的任务类型
	Status  string `json:"status"`  // 运行或排队状态
	Summary string `json:"summary"` // 任务输入或名称简述
}

type ExitState struct {
	Running int           `json:"running"` // 正在运行的任务数
	Queued  int           `json:"queued"`  // 尚未开始的任务数
	Tasks   []PendingTask `json:"tasks"`   // 确认退出会影响的任务
}

type Service struct {
	data        *appdata.Service  // 应用设置与专用会话标记
	runtime     *codex.Service    // 原生模型连接
	tasks       *tasks.Service    // 步骤和控制中心任务
	pipelines   *pipeline.Service // 流水线与专用会话
	mu          sync.Mutex        // 重启请求锁
	restarting  bool              // 关闭完成后是否重新启动应用
	restartMode string            // 退出后应用的目录模式
	restartPath string            // 退出后应用的自定义目录
}

// NewService 组合任务快照与应用退出入口
func NewService(data *appdata.Service, runtime *codex.Service, executions *tasks.Service, pipelines *pipeline.Service) *Service {
	return &Service{data: data, runtime: runtime, tasks: executions, pipelines: pipelines}
}

// GetExitState 读取确认退出时受影响的任务及数量
func (s *Service) GetExitState() ExitState {
	return snapshot(s.tasks.GetState(), s.pipelines.GetState(), s.runtime.GetActiveTurns(), s.data.GetSettings().ManagedThreads)
}

// Quit 请求应用在服务完成关闭后退出
func (s *Service) Quit() error {
	app := application.Get()
	if app == nil {
		return errors.New("应用尚未启动")
	}

	go app.Quit()
	return nil
}

// Restart 复用立即退出流程，等待关闭完成后以当前配置重启应用
func (s *Service) Restart() error {
	if application.Get() == nil {
		return errors.New("应用尚未启动")
	}
	s.mu.Lock()
	s.restarting = true
	s.mu.Unlock()
	return s.Quit()
}

// RestartCodexHome 确认目录后复用立即退出流程，等待关闭完成再重启
func (s *Service) RestartCodexHome(mode, directory string) error {
	if application.Get() == nil {
		return errors.New("应用尚未启动")
	}
	home, err := s.data.ResolveCodexHome(mode, directory)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return err
	}
	s.mu.Lock()
	s.restarting = true
	s.restartMode, s.restartPath = mode, home
	s.mu.Unlock()
	return s.Quit()
}

// CompleteRestart 在服务关闭和单实例锁释放后按需保存目录并启动完整应用
//
//wails:ignore
func (s *Service) CompleteRestart() error {
	s.mu.Lock()
	restarting, mode, directory := s.restarting, s.restartMode, s.restartPath
	s.restarting = false
	s.restartMode = ""
	s.mu.Unlock()
	if !restarting {
		return nil
	}
	if mode != "" {
		if err := s.data.SetCodexHome(mode, directory); err != nil {
			return err
		}
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	command := exec.Command(executable, os.Args[1:]...)
	command.Env = os.Environ()
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

// ServiceShutdown 在各服务等待退出前同时取消所有任务
func (s *Service) ServiceShutdown() error {
	pipelineErr := s.pipelines.PrepareShutdown()
	taskErr := s.tasks.PrepareShutdown()
	s.runtime.PrepareShutdown()
	return errors.Join(pipelineErr, taskErr)
}

// snapshot 汇总实际任务状态并去除已归属业务任务的对话轮次
func snapshot(state tasks.State, pipelines pipeline.State, active []codex.ActiveTurn, managed map[string]string) ExitState {
	result := ExitState{Tasks: []PendingTask{}}
	ownedThreads := make(map[string]bool)
	appendTask := func(id, kind, status, summary string) {
		if status != "running" && status != "queued" {
			return
		}
		summary = strings.Join(strings.Fields(summary), " ")
		if summary == "" {
			summary = id
		}
		if runes := []rune(summary); len(runes) > 120 {
			summary = string(runes[:120]) + "…"
		}
		result.Tasks = append(result.Tasks, PendingTask{ID: id, Kind: kind, Status: status, Summary: summary})
		if status == "running" {
			result.Running++
		} else {
			result.Queued++
		}
	}

	for _, execution := range state.Executions {
		if execution.Status == "running" || execution.Status == "queued" {
			ownedThreads[execution.ThreadID] = true
		}
		summary := execution.Context.Step.Title
		if execution.Instruction != "" {
			summary += " · " + execution.Instruction
		}
		appendTask(execution.ID, "步骤", execution.Status, summary)
	}
	for _, request := range state.Requests {
		if request.Status == "running" || request.Status == "queued" {
			ownedThreads[request.ThreadID] = true
		}
		appendTask(request.ID, "控制中心", request.Status, request.Text)
	}
	for _, run := range pipelines.Runs {
		appendTask(run.ID, "流水线", run.Status, run.Task.Name)
	}
	for _, request := range pipelines.Requests {
		if request.Status == "running" || request.Status == "queued" {
			ownedThreads[request.ThreadID] = true
		}
		appendTask(request.ID, "流水线会话", request.Status, request.Input.Text)
	}
	for _, turn := range active {
		if ownedThreads[turn.ThreadID] {
			continue
		}
		kind := "对话"
		switch managed[turn.ThreadID] {
		case "step":
			kind = "步骤对话"
		case "controller":
			kind = "控制中心对话"
		case "pipeline_manager", "pipeline_config":
			kind = "流水线对话"
		}
		appendTask(turn.ThreadID, kind, "running", turn.Summary)
	}
	return result
}
