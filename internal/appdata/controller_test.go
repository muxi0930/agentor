package appdata

import (
	"reflect"
	"testing"

	appdatamodel "agentor/internal/appdata/model"
	"agentor/internal/constant"
)

// TestControllerSettingsPersistAndRequireSelections 验证默认选择、持久化和至少一项限制
func TestControllerSettingsPersistAndRequireSelections(t *testing.T) {
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	service, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	defaults := service.GetSettings().Controller
	if !reflect.DeepEqual(defaults.Models, []string{""}) || !reflect.DeepEqual(defaults.Efforts, []string{""}) || defaults.Prompt != constant.AppDataDefaultControllerPrompt {
		t.Fatal(defaults)
	}
	securityPrompt := "自定义安全提示词"
	input := appdatamodel.ControllerSettings{SecurityPrompt: &securityPrompt, Models: []string{"", "executor-model"}, Efforts: []string{"high"}, Prompt: "自定义控制中心", DecisionPrompt: "复杂任务优先 high"}
	if _, err = service.SetControllerSettings(input); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reopened.GetSettings().Controller, input) {
		t.Fatal(reopened.GetSettings().Controller)
	}
	securityPrompt = ""
	if _, err := service.SetControllerSettings(input); err != nil {
		t.Fatal(err)
	}
	reopened, err = NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	saved := reopened.GetSettings().Controller.SecurityPrompt
	if saved == nil || *saved != "" {
		t.Fatal("cleared security prompt was not preserved")
	}
	for _, invalid := range []appdatamodel.ControllerSettings{{Models: []string{}, Efforts: []string{""}}, {Models: []string{""}, Efforts: nil}} {
		if _, err = service.SetControllerSettings(invalid); err == nil {
			t.Fatal("empty selection accepted")
		}
	}
	if !reflect.DeepEqual(service.GetSettings().Controller, input) {
		t.Fatal("invalid update changed saved settings")
	}
}
