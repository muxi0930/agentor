package lifecycle

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentor/internal/appdata"
	codexmodel "agentor/internal/codex/model"
	pipelinemodel "agentor/internal/pipeline/model"
	tasksmodel "agentor/internal/tasks/model"
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
	state := tasksmodel.State{
		Executions: []tasksmodel.Execution{
			{ID: "step", Status: "running", ThreadID: "step-thread", Instruction: "修复测试"},
			{ID: "finished", Status: "interrupted", ThreadID: "unfinished-stop"},
		},
		Requests: []tasksmodel.Request{{ID: "controller", Status: "queued", Text: "检查项目"}},
	}
	pipelines := pipelinemodel.State{
		Runs: []pipelinemodel.Run{{ID: "run", Status: "running", Task: pipelinemodel.Task{Name: "生成报告"}}},
	}
	active := []codexmodel.ActiveTurn{
		{ThreadID: "step-thread", Summary: "步骤轮次"},
		{ThreadID: "managed-thread", Summary: "专用会话"},
		{ThreadID: "chat", Summary: "解释错误"},
		{ThreadID: "unfinished-stop", Summary: "业务已中断但上游尚未停止的轮次"},
	}

	result := snapshot(state, pipelines, active, map[string]string{"managed-thread": "controller"})
	if result.Running != 5 || result.Queued != 1 || len(result.Tasks) != 6 {
		t.Fatalf("退出任务统计不完整或重复：%+v", result)
	}
	for _, task := range result.Tasks {
		if task.Summary == "" {
			t.Fatalf("任务缺少可辨识描述：%+v", task)
		}
	}
}

// TestDataDirectoryRestart 验证关闭后新进程读取全局配置中的目标目录且不迁移旧设置
func TestDataDirectoryRestart(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("AppData", filepath.Join(home, "config"))
	t.Setenv("AGENTOR_DATA_DIR", "")
	t.Setenv("CODEX_HOME", filepath.Join(home, "codex"))
	marker := filepath.Join(home, "restarted")
	target := filepath.Join(home, "new-data")
	t.Setenv("AGENTOR_DATA_RESTART_MARKER", marker)
	t.Setenv("AGENTOR_DATA_RESTART_TARGET", target)
	data, err := appdata.NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	previous := data.GetInfo().DataDirectory
	if err := data.SetDataDirectory(target); err != nil {
		t.Fatal(err)
	}
	if _, err := data.SetTheme("dark"); err != nil {
		t.Fatal(err)
	}
	if data.GetInfo().DataDirectory != previous {
		t.Fatal("收尾前不应更改当前目录")
	}
	previousArgs := os.Args
	os.Args = []string{os.Args[0], "-test.run=^TestDataDirectoryRestartChild$"}
	t.Cleanup(func() { os.Args = previousArgs })
	s := &Service{data: data, restarting: true}
	if err := s.CompleteRestart(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(marker); err == nil {
			if string(raw) != target {
				t.Fatalf("新进程目录错误：%s", raw)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("未观察到重启进程读取目标数据目录")
}

// TestDataDirectoryRestartChild 在新进程中验证数据根目录与默认设置
func TestDataDirectoryRestartChild(t *testing.T) {
	marker := os.Getenv("AGENTOR_DATA_RESTART_MARKER")
	if marker == "" {
		t.Skip("仅供数据目录重启验证")
	}
	data, err := appdata.NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if data.GetInfo().DataDirectory != os.Getenv("AGENTOR_DATA_RESTART_TARGET") || data.GetSettings().Theme != "light" {
		t.Fatal("新进程未读取目标目录或意外迁移旧设置")
	}
	if err := os.WriteFile(marker, []byte(data.GetInfo().DataDirectory), 0600); err != nil {
		t.Fatal(err)
	}
}
