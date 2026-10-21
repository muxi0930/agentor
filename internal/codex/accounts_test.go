package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"agentor/internal/appdata"
)

// TestProfilesKeepCredentialsPrivateAndRestoreOwnAuth 验证独立账号凭据隔离及原生登录状态保存
func TestProfilesKeepCredentialsPrivateAndRestoreOwnAuth(t *testing.T) {
	s := accountFixture(t)
	view, err := s.SaveProfile(ProfileInput{Name: "API check", Type: "api", BaseURL: "https://example.test/v1", APIKey: "private-test-key"})
	if err != nil {
		t.Fatal(err)
	}
	id := view.Profiles[0].ID
	if _, err = s.SaveProfile(ProfileInput{ID: id, Name: "Renamed", Type: "api", BaseURL: "https://example.test/v1"}); err != nil {
		t.Fatal(err)
	}
	stored, _ := s.profile(id)
	if stored.APIKey != "private-test-key" || stored.Profile.Name != "Renamed" {
		t.Fatal("空密钥编辑未保留已有凭据")
	}
	view, err = s.SaveProfile(ProfileInput{Name: "ChatGPT check", Type: "chatgpt"})
	if err != nil {
		t.Fatal(err)
	}
	chatID := view.Profiles[1].ID
	if _, err = s.data.SelectAccount(chatID, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DeleteProfile(chatID); err == nil {
		t.Fatal("当前账号被直接删除")
	}
	if err = appdata.WriteJSON(filepath.Join(s.data.GetInfo().CodexHome, "auth.json"), map[string]any{"tokens": map[string]string{"access_token": "own-test-access"}}); err != nil {
		t.Fatal(err)
	}
	s.profileID = chatID
	if err = s.captureAuth(); err != nil {
		t.Fatal(err)
	}
	chat, _ := s.profile(chatID)
	if err = s.prepareProfile(stored); err != nil {
		t.Fatal(err)
	}
	config, _ := os.ReadFile(filepath.Join(s.data.GetInfo().CodexHome, "config.toml"))
	if strings.Contains(string(config), stored.APIKey) || !strings.Contains(string(config), "agentor_api") {
		t.Fatal("独立配置泄露密钥或未选择 API provider")
	}
	if err = s.prepareProfile(chat); err != nil {
		t.Fatal(err)
	}
	auth, _ := os.ReadFile(filepath.Join(s.data.GetInfo().CodexHome, "auth.json"))
	if !strings.Contains(string(auth), "own-test-access") {
		t.Fatal("独立 ChatGPT 凭据没有恢复")
	}
	view, _ = s.GetAccounts()
	raw, _ := json.Marshal(view)
	if strings.Contains(string(raw), "private-test-key") || strings.Contains(string(raw), "own-test-access") {
		t.Fatal("公开账号列表泄露凭据")
	}
	s.SetAccountGuard(func(bool) error {
		return errors.New("queue active")
	})
	if _, err = s.UseAccount(""); err == nil || s.data.GetSettings().ReuseCLI {
		t.Fatal("活动队列下切换了账号")
	}
	s.data.SelectAccount(id, false)
	s.data.SetAccountModel(id, "custom-api-model")
	params, err := s.resumeParameters("shared-thread")
	if err != nil || params["modelProvider"] != "agentor_api" || params["model"] != "custom-api-model" {
		t.Fatal("恢复共享历史时未切换提供方和模型", err)
	}
}

// TestBalanceMappingCacheAndFailureStaySeparate 验证密钥引用、JSON Pointer、短时缓存与查询失败隔离
func TestBalanceMappingCacheAndFailureStaySeparate(t *testing.T) {
	s := accountFixture(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/failed" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("请求头没有引用所属账号密钥")
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["key"] != "test-key" || r.Method != "POST" {
			t.Error("POST JSON 密钥引用错误")
		}
		fmt.Fprint(w, `{"data":{"balance":"12.5","used":2,"total":null,"unit":"credits"},"a/b":{"~key":[10]}}`)
	}))
	defer server.Close()

	config := BalanceConfig{
		URL:          server.URL,
		Method:       "POST",
		HeaderName:   "Authorization",
		HeaderPrefix: "Bearer ",
		Body:         `{"key":"$API_KEY"}`,
		BalancePath:  "/data/balance",
		UsedPath:     "/data/used",
		TotalPath:    "/data/total",
		UnitPath:     "/data/unit",
	}
	view, err := s.SaveProfile(ProfileInput{Name: "Balance check", Type: "api", BaseURL: server.URL, APIKey: "test-key", Balance: config})
	if err != nil {
		t.Fatal(err)
	}
	id := view.Profiles[0].ID
	result, err := s.GetBalance(id, false)
	if err != nil || result.Error != "" || result.Balance == nil || *result.Balance != 12.5 ||
		result.Used == nil || *result.Used != 2 || result.Total != nil || result.Unit != "credits" {
		t.Fatal(result, err)
	}
	s.GetBalance(id, false)
	if calls.Load() != 1 {
		t.Fatal("余额短时缓存没有生效")
	}
	s.GetBalance(id, true)
	if calls.Load() != 2 {
		t.Fatal("手动刷新未查询接口")
	}
	var document any
	json.Unmarshal([]byte(`{"a/b":{"~key":[10]}}`), &document)
	if value, err := jsonPointer(document, "/a~1b/~0key/0"); err != nil || value != float64(10) {
		t.Fatal(value, err)
	}
	config.URL = server.URL + "/failed"
	if _, err = s.SaveProfile(ProfileInput{ID: id, Name: "Balance check", Type: "api", BaseURL: server.URL, Balance: config}); err != nil {
		t.Fatal(err)
	}
	previous := s.GetStatus()
	result, err = s.GetBalance(id, true)
	if err != nil || result.Error == "" || result.Balance != nil || s.GetStatus() != previous {
		t.Fatal("余额错误改变了聊天状态或保留了错误数值", result, err)
	}
}

// TestNativeIndependentAccountsLoginAndCancel 验证真实原生进程的独立 API 配置及 ChatGPT 登录取消
func TestNativeIndependentAccountsLoginAndCancel(t *testing.T) {
	if os.Getenv("AGENTOR_CODEX_ACCOUNTS_SMOKE") != "1" {
		t.Skip("设置 AGENTOR_CODEX_ACCOUNTS_SMOKE=1 后验证原生独立账号")
	}
	s := accountFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	view, err := s.SaveProfile(ProfileInput{Name: "Native API check", Type: "api", Model: "local-custom-model", BaseURL: server.URL + "/v1", APIKey: "local-native-test-key"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.UseAccount(view.Profiles[0].ID); err != nil {
		t.Fatal(err)
	}
	account, err := s.GetAccount()
	if err != nil || account.RequiresOpenAIAuth || s.GetStatus().ReuseCLI || s.GetStatus().Home != s.data.GetInfo().CodexHome {
		t.Fatal("原生进程未使用独立 API 账号", err)
	}
	if _, err = s.CheckAccountConnection(view.Profiles[0].ID); err != nil {
		t.Fatal(err)
	}
	models, err := s.GetModels()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, model := range models {
		found = found || model.Model == "local-custom-model"
	}

	if !found {
		t.Fatal("自定义 API 模型没有进入选择列表")
	}
	view, err = s.SaveProfile(ProfileInput{Name: "Native ChatGPT check", Type: "chatgpt"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.UseAccount(view.Profiles[1].ID); err != nil {
		t.Fatal(err)
	}
	account, err = s.GetAccount()
	if err != nil || account.Account != nil || !account.RequiresOpenAIAuth {
		t.Fatal("未登录 ChatGPT 账号混用了旧身份", err)
	}
	login, err := s.StartLogin()
	if err != nil || login.LoginID == "" || login.URL == "" || login.Status != "pending" {
		t.Fatal("原生登录未进入等待状态", err)
	}
	login, err = s.CancelLogin()
	if err != nil || login.Status != "cancelled" {
		t.Fatal("原生登录未取消", err)
	}
	s.mu.Lock()
	s.login = LoginState{ProfileID: view.Profiles[1].ID, LoginID: "00000000-0000-4000-8000-000000000000", Status: "pending"}
	s.mu.Unlock()
	login, err = s.CancelLogin()
	if err != nil || login.Status != "error" || login.Error == "" {
		t.Fatal("原生登录已不存在却仍显示取消成功", err)
	}
}

// accountFixture 建立全新应用数据和可关闭的账号运行时
func accountFixture(t *testing.T) *Service {
	t.Helper()
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	data, err := appdata.NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(data, nil)
	t.Cleanup(func() {
		s.ServiceShutdown()
	})
	return s
}
