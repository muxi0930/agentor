package codex

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"agentor/internal/appdata"
	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
)

// GetAccounts 读取独立账号列表和登录进度，不返回任何凭据
func (s *Service) GetAccounts() (codexmodel.AccountsView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.profilesErr != nil {
		return codexmodel.AccountsView{}, s.profilesErr
	}

	settings := s.data.GetSettings()
	view := codexmodel.AccountsView{ActiveID: settings.ActiveAccountID, ReuseCLI: settings.ReuseCLI, Login: s.login, Profiles: []codexmodel.AccountProfile{}}
	for _, stored := range s.profiles.Profiles {
		profile := stored.Profile
		profile.Model = settings.AccountPreferences[profile.ID].Model
		if !settings.ReuseCLI && settings.ActiveAccountID == profile.ID {
			profile.Model = settings.Model
		}
		profile.KeyConfigured, profile.Authenticated = stored.APIKey != "", len(stored.Auth) > 2
		view.Profiles = append(view.Profiles, profile)
	}

	return view, nil
}

// SaveProfile 创建或编辑独立账号，空密钥保留原值
func (s *Service) SaveProfile(input codexmodel.ProfileInput) (codexmodel.AccountsView, error) {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()

	input.Name, input.Provider, input.BaseURL = strings.TrimSpace(input.Name), strings.TrimSpace(input.Provider), strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	if input.Provider == "" {
		input.Provider = input.Name
	}
	if input.Name == "" || input.Type != constant.CodexAccountTypeChatGPT && input.Type != constant.CodexAccountTypeAPI {
		return codexmodel.AccountsView{}, constant.ErrCodexProfileInputInvalid
	}
	if input.Type == constant.CodexAccountTypeAPI {
		if err := httpURL(input.BaseURL); err != nil {
			return codexmodel.AccountsView{}, err
		}
		if err := validateBalance(input.Balance); err != nil {
			return codexmodel.AccountsView{}, err
		}
	}
	settings := s.data.GetSettings()
	if !settings.ReuseCLI && settings.ActiveAccountID == input.ID {
		if err := s.accountIdle(true); err != nil {
			return codexmodel.AccountsView{}, err
		}
	}

	s.mu.Lock()
	if s.profilesErr != nil {
		err := s.profilesErr
		s.mu.Unlock()
		return codexmodel.AccountsView{}, err
	}

	stored, index := storedProfile{}, -1
	for i, candidate := range s.profiles.Profiles {
		if candidate.Profile.ID == input.ID {
			stored, index = candidate, i
			break
		}
	}

	if input.ID != "" && index < 0 || index >= 0 && stored.Profile.Type != input.Type {
		s.mu.Unlock()
		return codexmodel.AccountsView{}, constant.ErrCodexProfileTypeChanged
	}
	if input.ID == "" {
		input.ID = rand.Text()
	}
	if input.APIKey != "" {
		stored.APIKey = input.APIKey
	}
	if input.Type == constant.CodexAccountTypeAPI && stored.APIKey == "" {
		s.mu.Unlock()
		return codexmodel.AccountsView{}, constant.ErrCodexAPIKeyRequired
	}

	stored.Profile = codexmodel.AccountProfile{ID: input.ID, Name: input.Name, Type: input.Type, Provider: input.Provider, BaseURL: input.BaseURL, Balance: input.Balance}
	if index < 0 {
		s.profiles.Profiles = append(s.profiles.Profiles, stored)
	} else {
		s.profiles.Profiles[index] = stored
	}
	delete(s.balances, input.ID)
	err := appdata.WriteJSON(s.profilesPath(), s.profiles)
	s.mu.Unlock()
	if err != nil {
		return codexmodel.AccountsView{}, err
	}
	if _, err = s.data.SetAccountModel(input.ID, strings.TrimSpace(input.Model)); err != nil {
		return codexmodel.AccountsView{}, err
	}
	if !settings.ReuseCLI && settings.ActiveAccountID == input.ID {
		if _, err = s.connect(); err != nil {
			return codexmodel.AccountsView{}, err
		}
	}
	return s.GetAccounts()
}

// DeleteProfile 删除未使用账号的私有记录，保留共享会话库
func (s *Service) DeleteProfile(id string) (codexmodel.AccountsView, error) {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()

	settings := s.data.GetSettings()
	if !settings.ReuseCLI && settings.ActiveAccountID == id {
		return codexmodel.AccountsView{}, constant.ErrCodexActiveProfileDeletion
	}

	s.mu.Lock()
	index := -1
	for i, stored := range s.profiles.Profiles {
		if stored.Profile.ID == id {
			index = i
			break
		}
	}

	if index < 0 {
		s.mu.Unlock()
		return codexmodel.AccountsView{}, constant.ErrCodexProfileNotFound
	}

	s.profiles.Profiles = append(s.profiles.Profiles[:index], s.profiles.Profiles[index+1:]...)
	delete(s.balances, id)
	err := appdata.WriteJSON(s.profilesPath(), s.profiles)
	s.mu.Unlock()
	if err != nil {
		return codexmodel.AccountsView{}, err
	}

	return s.GetAccounts()
}

// UseAccount 在所有业务空闲后切换身份，空标识表示沿用 CLI
func (s *Service) UseAccount(id string) (codexmodel.Status, error) {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()

	if err := s.accountIdle(true); err != nil {
		return s.GetStatus(), err
	}
	if id != "" {
		if _, err := s.profile(id); err != nil {
			return s.GetStatus(), err
		}
	}
	previous := s.GetStatus()
	s.mu.Lock()
	if len(s.active) > 0 {
		s.mu.Unlock()
		return previous, constant.ErrCodexTurnStarting
	}

	s.status.Phase = constant.CodexPhaseStarting
	s.mu.Unlock()
	if _, err := s.data.SelectAccount(id, id == ""); err != nil {
		s.setStatus(previous.Generation, previous.Phase, previous.Message)
		return s.GetStatus(), err
	}

	return s.connect()
}

// SetAccountGuard 将连接切换限制到控制中心和流水线均空闲时
//
//wails:ignore
func (s *Service) SetAccountGuard(guard func(bool) error) {
	s.mu.Lock()
	s.accountGuard = guard
	s.mu.Unlock()
}

// profilesPath 返回当前应用的私有账号记录路径
func (s *Service) profilesPath() string {
	return filepath.Join(s.data.GetInfo().DataDirectory, constant.CodexAccountsFilename)
}

// profile 读取供内部请求使用的账号副本
func (s *Service) profile(id string) (storedProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.profilesErr != nil {
		return storedProfile{}, s.profilesErr
	}

	for _, stored := range s.profiles.Profiles {
		if stored.Profile.ID == id {
			return stored, nil
		}
	}

	return storedProfile{}, constant.ErrCodexProfileRequired
}

// accountIdle 核验普通对话和正式任务都已空闲
func (s *Service) accountIdle(changing bool) error {
	s.mu.Lock()
	busy, guard := len(s.active) > 0, s.accountGuard
	s.mu.Unlock()
	if busy {
		return constant.ErrCodexActiveTurns
	}
	if guard != nil {
		return guard(changing)
	}

	return nil
}

// captureAuth 仅保存本应用实际运行过的 ChatGPT 身份，不读取 CLI 凭据
func (s *Service) captureAuth() error {
	s.mu.Lock()
	id := s.profileID
	s.mu.Unlock()
	if id == "" {
		return nil
	}

	stored, err := s.profile(id)
	if err != nil || stored.Profile.Type != constant.CodexAccountTypeChatGPT {
		return err
	}

	raw, err := os.ReadFile(filepath.Join(s.data.GetInfo().CodexHome, constant.CodexAuthFilename))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return constant.ErrCodexAuthSaveFailed
	}
	if !json.Valid(raw) {
		return constant.ErrCodexAuthInvalid
	}

	var auth struct {
		Tokens json.RawMessage `json:"tokens"` // 原生登录令牌
	}
	if err := json.Unmarshal(raw, &auth); err != nil {
		return err
	}
	if len(auth.Tokens) == 0 || string(auth.Tokens) == "null" || string(auth.Tokens) == "{}" {
		raw = nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.profiles.Profiles {
		if s.profiles.Profiles[i].Profile.ID == id {
			s.profiles.Profiles[i].Auth = append(json.RawMessage{}, raw...)
			return appdata.WriteJSON(s.profilesPath(), s.profiles)
		}
	}

	return nil
}

// prepareProfile 生成独立配置并恢复该账号的原生登录文件
func (s *Service) prepareProfile(stored storedProfile) error {
	home := s.data.GetInfo().CodexHome
	config := "cli_auth_credentials_store = \"file\"\napproval_policy = \"on-request\"\nsandbox_mode = \"read-only\"\n"
	if model := s.data.GetSettings().Model; model != "" {
		config += "model = " + mustJSON(model) + "\n"
	}
	if stored.Profile.Type == constant.CodexAccountTypeAPI {
		config += "model_provider = \"agentor_api\"\n[model_providers.agentor_api]\nname = " +
			mustJSON(stored.Profile.Provider) + "\nbase_url = " + mustJSON(stored.Profile.BaseURL) +
			"\nwire_api = \"responses\"\nenv_key = \"AGENTOR_ACCOUNT_API_KEY\"\nrequires_openai_auth = false\n"
	}
	if err := os.WriteFile(filepath.Join(home, constant.CodexConfigFilename), []byte(config), 0600); err != nil {
		return err
	}

	auth := stored.Auth
	if stored.Profile.Type != constant.CodexAccountTypeChatGPT || len(auth) == 0 {
		err := os.Remove(filepath.Join(home, constant.CodexAuthFilename))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	return appdata.WriteJSON(filepath.Join(home, constant.CodexAuthFilename), auth)
}

// httpURL 核验用户配置的是明确的 HTTP 服务地址
func httpURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.User != nil {
		return constant.ErrCodexBaseURLInvalid
	}

	return nil
}
