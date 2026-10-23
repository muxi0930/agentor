package codex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
)

// GetBalance 查询指定 API 账号的余额，短时缓存与聊天连接互不影响
func (s *Service) GetBalance(id string, force bool) (codexmodel.BalanceResult, error) {
	stored, err := s.profile(id)
	if err != nil {
		return codexmodel.BalanceResult{}, err
	}
	if stored.Profile.Type != constant.CodexAccountTypeAPI || stored.Profile.Balance.URL == "" {
		return codexmodel.BalanceResult{}, constant.ErrCodexBalanceNotConfigured
	}

	s.mu.Lock()
	cached, exists := s.balances[id]
	s.mu.Unlock()
	if checked, err := time.Parse(time.RFC3339Nano, cached.CheckedAt); !force && exists && err == nil && time.Since(checked) < constant.CodexBalanceCacheDuration {
		return cached, nil
	}

	result := codexmodel.BalanceResult{ProfileID: id, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano), Unit: stored.Profile.Balance.Unit}
	config := stored.Profile.Balance
	var body io.Reader
	if config.Method == "POST" && config.Body != "" {
		var document any
		if err := json.Unmarshal([]byte(config.Body), &document); err != nil {
			return result, constant.ErrCodexBalanceBodyInvalid
		}

		raw, _ := json.Marshal(replaceAPIKey(document, stored.APIKey))
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(s.ctx, config.Method, config.URL, body)
	if err == nil {
		if config.HeaderName != "" {
			request.Header.Set(config.HeaderName, config.HeaderPrefix+stored.APIKey)
		}
		if config.Method == "POST" {
			request.Header.Set("Content-Type", "application/json")
		}
		var response *http.Response
		response, err = accountHTTP().Do(request)
		if err == nil {
			defer response.Body.Close()

			if response.StatusCode < 200 || response.StatusCode >= 300 {
				err = fmt.Errorf("余额接口返回 HTTP %d", response.StatusCode)
			} else {
				var document any
				decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
				decoder.UseNumber()
				if err = decoder.Decode(&document); err == nil {
					result.Balance, err = balanceNumber(document, config.BalancePath)
					if err == nil {
						result.Used, err = balanceNumber(document, config.UsedPath)
					}
					if err == nil {
						result.Total, err = balanceNumber(document, config.TotalPath)
					}
					if err == nil && config.UnitPath != "" {
						var unit any
						unit, err = jsonPointer(document, config.UnitPath)
						if text, ok := unit.(string); ok {
							result.Unit = text
						} else if err == nil {
							err = constant.ErrCodexBalanceUnitInvalid
						}
					}
				}
			}
		}
	}
	if err != nil {
		result.Balance, result.Used, result.Total = nil, nil, nil
		result.Error = constant.CodexBalanceFailurePrefix + strings.ReplaceAll(err.Error(), stored.APIKey, constant.CodexRedactedKeyLabel)
	}
	s.mu.Lock()
	s.balances[id] = result
	s.mu.Unlock()
	return result, nil
}

// CheckAccountConnection 对独立 API 发起一次模型接口请求或读取当前 ChatGPT 真实额度
func (s *Service) CheckAccountConnection(id string) (string, error) {
	stored, err := s.profile(id)
	if err != nil {
		return "", err
	}
	if stored.Profile.Type == constant.CodexAccountTypeChatGPT {
		settings := s.data.GetSettings()
		if settings.ReuseCLI || settings.ActiveAccountID != id {
			return "", constant.ErrCodexChatGPTProfileInactive
		}
		if _, err := s.GetRateLimits(); err != nil {
			return "", err
		}

		return constant.CodexChatGPTConnectionSuccess, nil
	}

	request, err := http.NewRequestWithContext(s.ctx, "GET", stored.Profile.BaseURL+"/models", nil)
	if err != nil {
		return "", constant.ErrCodexAPIBaseURLInvalid
	}

	request.Header.Set("Authorization", "Bearer "+stored.APIKey)
	response, err := accountHTTP().Do(request)
	if err != nil {
		return "", constant.ErrCodexAPIConnectionFailed
	}

	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("模型接口返回 HTTP %d", response.StatusCode)
	}

	return constant.CodexAPIConnectionSuccess, nil
}

// validateBalance 核验余额请求配置，保留空配置表示暂不启用
func validateBalance(config codexmodel.BalanceConfig) error {
	if config.URL == "" {
		return nil
	}
	if err := httpURL(config.URL); err != nil {
		return err
	}
	if config.Method != "GET" && config.Method != "POST" {
		return constant.ErrCodexBalanceMethodUnsupported
	}
	if config.Method == "POST" && config.Body != "" && !json.Valid([]byte(config.Body)) {
		return constant.ErrCodexBalancePostBodyInvalid
	}

	for _, path := range []string{config.BalancePath, config.UsedPath, config.TotalPath, config.UnitPath} {
		if path != "" && !strings.HasPrefix(path, "/") {
			return constant.ErrCodexBalancePointerInvalid
		}
	}

	return nil
}

// accountHTTP 为账号诊断提供有超时且不自动转发凭据的请求客户端
func accountHTTP() *http.Client {
	return &http.Client{Timeout: constant.CodexHTTPTimeout, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

// replaceAPIKey 只替换 JSON 完整字符串引用，不执行模板或脚本
func replaceAPIKey(value any, key string) any {
	switch item := value.(type) {
	case string:
		if item == "$API_KEY" {
			return key
		}
	case []any:
		for i := range item {
			item[i] = replaceAPIKey(item[i], key)
		}
	case map[string]any:
		for name, child := range item {
			item[name] = replaceAPIKey(child, key)
		}
	}

	return value
}

// jsonPointer 按 JSON Pointer 路径读取对象或数组中的字段
func jsonPointer(document any, path string) (any, error) {
	current := document
	for _, encoded := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		key := strings.ReplaceAll(strings.ReplaceAll(encoded, "~1", "/"), "~0", "~")
		switch item := current.(type) {
		case map[string]any:
			var found bool
			current, found = item[key]
			if !found {
				return nil, fmt.Errorf("映射字段 %s 不存在", path)
			}
		case []any:
			index, err := strconv.Atoi(key)
			if err != nil || index < 0 || index >= len(item) {
				return nil, fmt.Errorf("映射数组下标 %s 无效", path)
			}
			current = item[index]
		default:
			return nil, fmt.Errorf("映射路径 %s 无法继续读取", path)
		}
	}

	return current, nil
}

// balanceNumber 读取接口实际数值，未配置或空值保持未知
func balanceNumber(document any, path string) (*float64, error) {
	if path == "" {
		return nil, nil
	}

	value, err := jsonPointer(document, path)
	if err != nil || value == nil {
		return nil, err
	}

	var raw string
	switch number := value.(type) {
	case json.Number:
		raw = number.String()
	case string:
		raw = number
	default:
		return nil, fmt.Errorf("映射字段 %s 不是数字", path)
	}

	number, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return nil, fmt.Errorf("映射字段 %s 不是数字", path)
	}

	return &number, nil
}
