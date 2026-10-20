package main

import (
	"embed"
	"log"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	"agentor/internal/codex"
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
	router := &toolRouter{tasks: executions, work: work, pipeline: pipelines}
	executions.ConfigureTools(router.specs)
	pipelines.ConfigureTools(router.specs)
	runtime.SetToolHandler(router.call)
	runtime.SetEventHandler(executions.OnCodexEvent)
	work.SetDeleteGuard(executions.PrepareDelete)
	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name: "agentor", Description: "Codex 对话与工作编排",
		Services: []application.Service{application.NewService(data), application.NewService(files), application.NewService(runtime), application.NewService(work), application.NewService(executions), application.NewService(pipelines)},
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
