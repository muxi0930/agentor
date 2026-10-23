package main

import (
	"embed"
	"log"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	"agentor/internal/codex"
	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	"agentor/internal/lark"
	"agentor/internal/lifecycle"
	"agentor/internal/pipeline"
	"agentor/internal/tasks"
	"agentor/internal/toolrouter"
	"agentor/internal/workspace"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var version string = "0.2.5" // 应用版本

//go:embed all:frontend/dist
var assets embed.FS

// main 初始化业务服务并运行桌面窗口
func main() {
	data, err := appdata.NewService(version)
	if err != nil {
		log.Fatal(err)
	}
	files, err := artifact.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		log.Fatal(err)
	}
	work, err := workspace.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		log.Fatal(err)
	}

	runtime := codex.NewService(data, files)
	executions, err := tasks.NewService(data, runtime, work, files)
	if err != nil {
		log.Fatal(err)
	}
	pipelines, err := pipeline.NewService(data, work, executions, files, runtime)
	if err != nil {
		log.Fatal(err)
	}
	remote, err := lark.NewService(data.GetInfo().DataDirectory, executions, work, files, runtime, pipelines)
	if err != nil {
		log.Fatal(err)
	}
	lifetime := lifecycle.NewService(data, runtime, executions, pipelines)

	router := toolrouter.New(executions, work, pipelines, remote)
	executions.ConfigureTools(router.Specs)
	pipelines.ConfigureTools(router.Specs)
	runtime.SetToolHandler(router.Call)
	runtime.SetEventHandler(func(event codexmodel.Event) { executions.OnCodexEvent(event); remote.OnCodexEvent(event) })
	executions.SetNotificationHandler(remote.OnNotification)
	work.SetDeleteGuard(executions.PrepareDelete)
	runtime.SetAccountGuard(func(changing bool) error {
		state := executions.GetState()
		for _, execution := range state.Executions {
			if execution.Status == constant.TasksStatusRunning || changing && execution.Status == constant.TasksStatusQueued {
				return constant.ErrAppStepQueueActive
			}
		}
		for _, request := range state.Requests {
			if request.Status == constant.TasksStatusRunning || changing && request.Status == constant.TasksStatusQueued {
				return constant.ErrAppControllerRequestsActive
			}
		}
		pipelineState := pipelines.GetState()
		for _, run := range pipelineState.Runs {
			if run.Status == constant.TasksStatusRunning || changing && run.Status == constant.TasksStatusQueued {
				return constant.ErrAppPipelineRunsActive
			}
		}
		for _, request := range pipelineState.Requests {
			if request.Status == constant.TasksStatusRunning || changing && request.Status == constant.TasksStatusQueued {
				return constant.ErrAppPipelineSessionsActive
			}
		}
		return nil
	})

	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name: constant.AppDataApplicationName, Description: "Codex 对话与工作编排",
		Services: []application.Service{application.NewService(data), application.NewService(files), application.NewService(runtime), application.NewService(work), application.NewService(executions), application.NewService(pipelines), application.NewService(remote), application.NewService(lifetime)},
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(assets), Middleware: files.Middleware},
		Mac:      application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		PostShutdown: func() {
			if err := lifetime.CompleteRestart(); err != nil {
				log.Printf("应用重启失败：%v", err)
			}
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "io.agentor.desktop",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if window != nil {
					window.Show()
					window.Focus()
				}
			},
		},
	})

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: constant.AppDataApplicationName, Width: 1360, Height: 900, MinWidth: 900, MinHeight: 640,
		BackgroundColour: application.NewRGB(250, 250, 249), URL: "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
