package codex

import (
	"encoding/json"
	"time"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// StartLogin 通过当前原生协议启动独立 ChatGPT 账号登录
func (s *Service) StartLogin() (codexmodel.LoginState, error) {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()

	if err := s.accountIdle(true); err != nil {
		return codexmodel.LoginState{}, err
	}

	settings := s.data.GetSettings()
	if settings.ReuseCLI {
		return codexmodel.LoginState{}, constant.ErrCodexIndependentChatGPTProfileRequired
	}

	stored, err := s.profile(settings.ActiveAccountID)
	if err != nil || stored.Profile.Type != constant.CodexAccountTypeChatGPT {
		return codexmodel.LoginState{}, constant.ErrCodexChatGPTProfileRequired
	}

	s.mu.Lock()
	if s.login.Status == constant.CodexStatusPending {
		current := s.login
		s.mu.Unlock()
		return current, nil
	}

	s.mu.Unlock()
	var result struct {
		Type    string `json:"type"`    // 类型标识
		LoginID string `json:"loginId"` // 原生登录流程标识
		AuthURL string `json:"authUrl"` // 浏览器登录地址
	}
	if err = s.call(constant.CodexMethodAccountLoginStart, map[string]any{"type": constant.CodexAccountTypeChatGPT}, &result); err != nil {
		return codexmodel.LoginState{}, err
	}
	if result.LoginID == "" || result.AuthURL == "" {
		return codexmodel.LoginState{}, constant.ErrCodexLoginResponseInvalid
	}

	s.mu.Lock()
	s.login = codexmodel.LoginState{ProfileID: stored.Profile.ID, LoginID: result.LoginID, URL: result.AuthURL, Status: constant.CodexStatusPending}
	current := s.login
	s.mu.Unlock()
	return current, nil
}

// ContinueLogin 用系统浏览器继续当前仍有效的登录
func (s *Service) ContinueLogin() error {
	s.mu.Lock()
	current := s.login
	s.mu.Unlock()
	if current.Status != constant.CodexStatusPending || current.URL == "" {
		return constant.ErrCodexLoginNotPending
	}

	return application.Get().Browser.OpenURL(current.URL)
}

// CancelLogin 取消当前原生登录流程并保留可重试状态
func (s *Service) CancelLogin() (codexmodel.LoginState, error) {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()

	s.mu.Lock()
	current := s.login
	s.mu.Unlock()
	if current.Status != constant.CodexStatusPending {
		return current, nil
	}

	var result struct {
		Status string `json:"status"` // 上游取消结果
	}
	if err := s.call(constant.CodexMethodAccountLoginCancel, map[string]string{"loginId": current.LoginID}, &result); err != nil {
		return current, err
	}

	status, message := constant.CodexStatusCancelled, ""
	if result.Status == "notFound" {
		status, message = constant.CodexStatusError, constant.CodexLoginEndedMessage
		if account, err := s.GetAccount(); err == nil && account.Account != nil && account.Account.Type == constant.CodexAccountTypeChatGPT {
			status, message = constant.CodexStatusCompleted, ""
			if err := s.captureAuth(); err != nil {
				status, message = constant.CodexStatusError, err.Error()
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
func (s *Service) GetRateLimits() (codexmodel.RateLimits, error) {
	var result codexmodel.RateLimits
	if err := s.call(constant.CodexMethodAccountRateLimitsRead, map[string]any{}, &result); err != nil {
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
		LoginID string `json:"loginId"` // 原生登录流程标识
		Success bool   `json:"success"` // 登录是否成功
		Error   string `json:"error"`   // 失败原因
	}
	if json.Unmarshal(raw, &result) != nil {
		return
	}

	s.mu.Lock()
	if generation != s.status.Generation || s.login.Status != constant.CodexStatusPending || result.LoginID != s.login.LoginID {
		s.mu.Unlock()
		return
	}

	s.login.Status, s.login.Error, s.login.URL = constant.CodexStatusError, result.Error, ""
	if result.Success {
		s.login.Status = constant.CodexStatusCompleted
	} else if s.login.Error == "" {
		s.login.Error = constant.CodexLoginFailedMessage
	}
	s.mu.Unlock()
	if result.Success {
		if err := s.captureAuth(); err != nil {
			s.mu.Lock()
			s.login.Status, s.login.Error = constant.CodexStatusError, err.Error()
			s.mu.Unlock()
		}
	}
	s.mu.Lock()
	current := s.login
	s.mu.Unlock()
	if app := application.Get(); app != nil {
		app.Event.Emit(constant.CodexEventMessage, codexmodel.Event{Generation: generation, Method: constant.CodexEventLoginUpdated, Params: mustJSON(current)})
	}
}
