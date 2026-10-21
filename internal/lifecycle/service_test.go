package lifecycle

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/codex"
	"agentor/internal/pipeline"
	"agentor/internal/tasks"
)

// TestCompleteRestartLaunchesFreshProcess 验证普通重启和切换目录重启均以新进程保留原账号
func TestCompleteRestartLaunchesFreshProcess(t *testing.T) {
	for _, changeHome := range []bool{false, true} {
		name := "保留当前目录"
		if changeHome {
			name = "切换目录"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
			dataHome := t.TempDir()
			t.Setenv("CODEX_HOME", dataHome)
			marker := filepath.Join(t.TempDir(), "restarted")
			t.Setenv("AGENTOR_RESTART_TEST_MARKER", marker)
			target := dataHome
			if changeHome {
				target = t.TempDir()
			}
			t.Setenv("AGENTOR_RESTART_TEST_HOME", target)
			data, err := appdata.NewService("test")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := data.SelectAccount("preserved-account", false); err != nil {
				t.Fatal(err)
			}
			s := &Service{data: data, restarting: true}
			if changeHome {
				s.restartMode, s.restartPath = "custom", target
			}
			if changeHome && data.GetInfo().CodexHome == target {
				t.Fatal("退出前提前更改了运行时目录")
			}
			previousArgs := os.Args
			os.Args = []string{os.Args[0], "-test.run=^TestRestartChildProcess$"}
			t.Cleanup(func() { os.Args = previousArgs })
			if err := s.CompleteRestart(); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if result, err := os.ReadFile(marker); err == nil {
					if string(result) != "preserved-account" {
						t.Fatal("新进程未保留账号")
					}
					return
				}
				time.Sleep(20 * time.Millisecond)
			}
			t.Fatal("未观察到新应用进程读取目标目录")
		})
	}
}

// TestRestartChildProcess 在重启子进程中核验持久化的账号和目录
func TestRestartChildProcess(t *testing.T) {
	marker := os.Getenv("AGENTOR_RESTART_TEST_MARKER")
	if marker == "" {
		t.Skip("仅供重启子进程验证")
	}
	data, err := appdata.NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if data.GetInfo().CodexHome != os.Getenv("AGENTOR_RESTART_TEST_HOME") || data.GetSettings().ReuseCLI {
		t.Fatal("新进程使用了错误的目录或身份")
	}
	if err := os.WriteFile(marker, []byte(data.GetSettings().ActiveAccountID), 0600); err != nil {
		t.Fatal(err)
	}
}

// TestSnapshotIncludesEveryTaskSourceWithoutCountingManagedTurnsTwice 验证退出覆盖所有任务来源且会话不重复计数
func TestSnapshotIncludesEveryTaskSourceWithoutCountingManagedTurnsTwice(t *testing.T) {
	state := tasks.State{
		Executions: []tasks.Execution{
			{ID: "step", Status: "running", ThreadID: "step-thread", Instruction: "修复测试"},
			{ID: "finished", Status: "interrupted", ThreadID: "unfinished-stop"},
		},
		Requests: []tasks.Request{{ID: "controller", Status: "queued", Text: "检查项目"}},
	}
	pipelines := pipeline.State{
		Runs:     []pipeline.Run{{ID: "run", Status: "running", Task: pipeline.Task{Name: "生成报告"}}},
		Requests: []pipeline.SessionRequest{{ID: "session", Status: "queued", Input: pipeline.SessionInput{Text: "调整流水线"}}},
	}
	active := []codex.ActiveTurn{
		{ThreadID: "step-thread", Summary: "步骤轮次"},
		{ThreadID: "managed-thread", Summary: "专用会话"},
		{ThreadID: "chat", Summary: "解释错误"},
		{ThreadID: "unfinished-stop", Summary: "业务已中断但上游尚未停止的轮次"},
	}

	result := snapshot(state, pipelines, active, map[string]string{"managed-thread": "controller"})
	if result.Running != 5 || result.Queued != 2 || len(result.Tasks) != 7 {
		t.Fatalf("退出任务统计不完整或重复：%+v", result)
	}
	for _, task := range result.Tasks {
		if task.Summary == "" {
			t.Fatalf("任务缺少可辨识描述：%+v", task)
		}
	}
}
