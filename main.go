package main

import (
	"embed"
	"errors"
	"log"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	"agentor/internal/codex"
	"agentor/internal/lark"
	"agentor/internal/pipeline"
	"agentor/internal/tasks"
	"agentor/internal/workspace"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	data, err := appdata.NewService()
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
	router := &toolRouter{tasks: executions, work: work, pipeline: pipelines, lark: remote}
	executions.ConfigureTools(router.specs)
	pipelines.ConfigureTools(router.specs)
	runtime.SetToolHandler(router.call)
	runtime.SetEventHandler(func(event codex.Event) { executions.OnCodexEvent(event); remote.OnCodexEvent(event) })
	executions.SetNotificationHandler(remote.OnNotification)
	work.SetDeleteGuard(executions.PrepareDelete)
	runtime.SetAccountGuard(func(changing bool) error {
		state := executions.GetState()
		for _, execution := range state.Executions {
			if execution.Status == "running" || changing && execution.Status == "queued" {
				return errors.New("请先结束或取消步骤队列，再切换连接")
			}
		}
		for _, request := range state.Requests {
			if request.Status == "running" || changing && request.Status == "queued" {
				return errors.New("请先结束或取消控制器请求，再切换连接")
			}
		}
		pipelineState := pipelines.GetState()
		for _, run := range pipelineState.Runs {
			if run.Status == "running" || changing && run.Status == "queued" {
				return errors.New("请先结束流水线运行，再切换连接")
			}
		}
		for _, request := range pipelineState.Requests {
			if request.Status == "running" || changing && request.Status == "queued" {
				return errors.New("请先结束流水线会话请求，再切换连接")
			}
		}
		return nil
	})
	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name: "agentor", Description: "Codex 对话与工作编排",
		Services: []application.Service{application.NewService(data), application.NewService(files), application.NewService(runtime), application.NewService(work), application.NewService(executions), application.NewService(pipelines), application.NewService(remote)},
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(assets), Middleware: files.Middleware},
		Mac:      application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
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
		Title: "agentor", Width: 1360, Height: 900, MinWidth: 900, MinHeight: 640,
		BackgroundColour: application.NewRGB(250, 250, 249), URL: "/",
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
