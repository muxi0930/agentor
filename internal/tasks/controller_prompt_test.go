package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentor/internal/artifact"
	"agentor/internal/codex"
	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	tasksmodel "agentor/internal/tasks/model"
)

// TestMain 在子进程中模拟原生协议服务，记录实际发送载荷而不调用模型
func TestMain(m *testing.M) {
	path := os.Getenv("AGENTOR_CONTROLLER_PROTOCOL_CAPTURE")
	if path == "" {
		os.Exit(m.Run())
	}
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("controller-protocol-test")
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		os.Exit(1)
	}
	defer file.Close()
	decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	threads := 0
	for {
		var request struct {
			ID     json.RawMessage `json:"id"`     // 请求标识
			Method string          `json:"method"` // 请求方法
			Params json.RawMessage `json:"params"` // 实际参数
		}
		if decoder.Decode(&request) != nil {
			return
		}
		if json.NewEncoder(file).Encode(request) != nil {
			os.Exit(1)
		}
		if len(request.ID) == 0 {
			continue
		}
		response := map[string]any{"id": request.ID, "result": map[string]any{}}
		switch request.Method {
		case "thread/start":
			threads++
			var params struct {
				Cwd string `json:"cwd"` // 工作目录
			}
			json.Unmarshal(request.Params, &params)
			response["result"] = map[string]any{"thread": map[string]any{"id": fmt.Sprintf("controller-%d", threads), "cwd": params.Cwd}}
		case "turn/start":
			delete(response, "result")
			response["error"] = map[string]any{"code": -32000, "message": "test payload captured"}
		}
		if encoder.Encode(response) != nil {
			return
		}
	}
}

// TestControllerSecurityPromptPayload 验证配置经实际协议发送，并按既有规则复用或重建会话
func TestControllerSecurityPromptPayload(t *testing.T) {
	data := testData(t)
	capture := filepath.Join(t.TempDir(), "protocol.jsonl")
	t.Setenv("AGENTOR_CONTROLLER_PROTOCOL_CAPTURE", capture)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = data.SetCodexPath(executable); err != nil {
		t.Fatal(err)
	}
	files, err := artifact.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	runtime := codex.NewService(data, files)
	defer runtime.ServiceShutdown()
	service, err := NewService(data, runtime, nil, files)
	if err != nil {
		t.Fatal(err)
	}
	defer service.ServiceShutdown()
	service.state.Paused = true
	// 模拟升级前已存在但缺少默认安全提示词的上下文
	config := data.GetSettings().Controller
	empty := ""
	old := config
	old.SecurityPrompt = &empty
	service.state.ControllerThreadID = "legacy-controller"
	service.state.ControllerInstructions = controllerPrompt(old)
	if _, err = runtime.Connect(); err != nil {
		t.Fatal(err)
	}
	custom := "TEST_ONLY_SECURITY_PROMPT"
	for _, test := range []struct {
		name    string  // 场景名称
		value   *string // 保存的配置
		want    string  // 发送的安全提示词
		threads int     // 累计创建会话数
	}{
		{"默认配置替换旧会话", nil, constant.AppDataDefaultControllerSecurityPrompt, 1},
		{"不变配置复用会话", nil, constant.AppDataDefaultControllerSecurityPrompt, 1},
		{"自定义配置重建会话", &custom, custom, 2},
		{"清空配置重建会话", &empty, "", 3},
		{"清空配置复用会话", &empty, "", 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			config.SecurityPrompt = test.value
			if _, err := data.SetControllerSettings(config); err != nil {
				t.Fatal(err)
			}
			request, err := service.submitRequest(tasksmodel.ControllerInput{Text: "测试请求", Model: "controller", Effort: "medium"}, "execution_result", "test")
			if err != nil {
				t.Fatal(err)
			}
			// 提交后的配置变更不应改写已入队请求的快照
			changed := config
			changed.SecurityPrompt = &custom
			if _, err = data.SetControllerSettings(changed); err != nil {
				t.Fatal(err)
			}
			_, _, err = service.executeController(context.Background(), request)
			if err == nil || !strings.Contains(err.Error(), "test payload captured") {
				t.Fatal("未到达最终发送边界", err)
			}
			file, err := os.Open(capture)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			decoder := json.NewDecoder(file)
			threads, instructions, input, threadID := 0, "", "", ""
			for decoder.More() {
				var message struct {
					Method string `json:"method"` // 请求方法
					Params struct {
						Cwd          string             `json:"cwd"`                   // 工作目录
						Instructions string             `json:"developerInstructions"` // 会话指令
						ThreadID     string             `json:"threadId"`              // 会话标识
						Input        []codexmodel.Input `json:"input"`                 // 模型输入
					} `json:"params"` // 发送参数
				}
				if err := decoder.Decode(&message); err != nil {
					t.Fatal(err)
				}
				if (message.Method == "thread/start" || message.Method == "turn/start") && message.Params.Cwd != data.GetInfo().CodexHome {
					t.Fatalf("控制中心工作目录未使用 CODEX_HOME：%s", message.Params.Cwd)
				}
				if message.Method == "thread/start" {
					threads++
					instructions = message.Params.Instructions
				}
				if message.Method == "turn/start" && len(message.Params.Input) > 0 {
					input, threadID = message.Params.Input[0].Text, message.Params.ThreadID
				}
			}
			if threads != test.threads || threadID != fmt.Sprintf("controller-%d", test.threads) {
				t.Fatal("会话未按配置变化复用或重建")
			}
			if instructions != controllerPrompt(request.Executor) || !strings.HasPrefix(input, instructions+"\n\n请求类型：") || !strings.HasSuffix(input, request.Text) {
				t.Fatal("实际会话指令或最终轮次载荷与请求配置不一致")
			}
			if test.want != "" && !strings.HasSuffix(instructions, "\n\n"+test.want) {
				t.Fatal("最终载荷缺少配置的安全提示词")
			}
			if test.want == "" && (strings.Contains(input, custom) || strings.Contains(input, constant.AppDataDefaultControllerSecurityPrompt)) {
				t.Fatal("明确清空后仍发送安全提示词")
			}
		})
	}
}
