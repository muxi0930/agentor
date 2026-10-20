package codex

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// StartLogin 通过当前原生协议启动独立 ChatGPT 账号登录
func (s *Service) StartLogin() (LoginState, error) {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()
	if err := s.accountIdle(true); err != nil {
		return LoginState{}, err
	}
	settings := s.data.GetSettings()
	if settings.ReuseCLI {
		return LoginState{}, errors.New("请先选择 agentor 的独立 ChatGPT 账号；CLI 登录保持原样")
	}
	stored, err := s.profile(settings.ActiveAccountID)
	if err != nil || stored.Profile.Type != "chatgpt" {
		return LoginState{}, errors.New("请先选择独立 ChatGPT 账号")
	}
	s.mu.Lock()
	if s.login.Status == "pending" {
		current := s.login
		s.mu.Unlock()
		return current, nil
	}
	s.mu.Unlock()
	var result struct {
		Type    string `json:"type"`
		LoginID string `json:"loginId"`
		AuthURL string `json:"authUrl"`
	}
	if err = s.call("account/login/start", map[string]any{"type": "chatgpt"}, &result); err != nil {
		return LoginState{}, err
	}
	if result.LoginID == "" || result.AuthURL == "" {
		return LoginState{}, errors.New("原生登录响应缺少登录标识或浏览器地址")
	}
	s.mu.Lock()
	s.login = LoginState{ProfileID: stored.Profile.ID, LoginID: result.LoginID, URL: result.AuthURL, Status: "pending"}
	current := s.login
	s.mu.Unlock()
	return current, nil
}

// ContinueLogin 用系统浏览器继续当前仍有效的登录
func (s *Service) ContinueLogin() error {
	s.mu.Lock()
	current := s.login
	s.mu.Unlock()
	if current.Status != "pending" || current.URL == "" {
		return errors.New("当前没有等待中的登录，请重新发起")
	}
	return application.Get().Browser.OpenURL(current.URL)
}

// CancelLogin 取消当前原生登录流程并保留可重试状态
func (s *Service) CancelLogin() (LoginState, error) {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()
	s.mu.Lock()
	current := s.login
	s.mu.Unlock()
	if current.Status != "pending" {
		return current, nil
	}
	var result struct {
		Status string `json:"status"` // 上游取消结果
	}
	if err := s.call("account/login/cancel", map[string]string{"loginId": current.LoginID}, &result); err != nil {
		return current, err
	}
	status, message := "cancelled", ""
	if result.Status == "notFound" {
		status, message = "error", "原登录流程已经结束，请重新发起登录"
		if account, err := s.GetAccount(); err == nil && account.Account != nil && account.Account.Type == "chatgpt" {
			status, message = "completed", ""
			if err := s.captureAuth(); err != nil {
				status, message = "error", err.Error()
			}
		}
	}
	s.mu.Lock()
	s.login.Status, s.login.URL, s.login.Error = status, "", message
	current = s.login
	s.mu.Unlock()
	return current, nil
}

// GetRateLimits 读取上游真实额度，不用模型列表推断账号权益
func (s *Service) GetRateLimits() (RateLimits, error) {
	var result RateLimits
	if err := s.call("account/rateLimits/read", map[string]any{}, &result); err != nil {
		return result, err
	}
	result.CheckedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return result, nil
}

// trackLogin 接收原始登录结果并保存本应用生成的凭据
func (s *Service) trackLogin(generation uint64, raw json.RawMessage) {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()
	var result struct {
		LoginID string `json:"loginId"`
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return
	}
	s.mu.Lock()
	if generation != s.status.Generation || s.login.Status != "pending" || result.LoginID != s.login.LoginID {
		s.mu.Unlock()
		return
	}
	s.login.Status, s.login.Error, s.login.URL = "error", result.Error, ""
	if result.Success {
		s.login.Status = "completed"
	} else if s.login.Error == "" {
		s.login.Error = "登录未完成，请重新尝试"
	}
	s.mu.Unlock()
	if result.Success {
		if err := s.captureAuth(); err != nil {
			s.mu.Lock()
			s.login.Status, s.login.Error = "error", err.Error()
			s.mu.Unlock()
		}
	}
	s.mu.Lock()
	current := s.login
	s.mu.Unlock()
	if app := application.Get(); app != nil {
		app.Event.Emit("codex:event", Event{Generation: generation, Method: "host/loginUpdated", Params: mustJSON(current)})
	}
}
