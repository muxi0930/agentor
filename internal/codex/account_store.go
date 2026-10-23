package codex

import (
	"encoding/json"

	codexmodel "agentor/internal/codex/model"
)

type storedProfile struct {
	Profile codexmodel.AccountProfile `json:"profile"`          // 公开账号信息
	APIKey  string                    `json:"apiKey,omitempty"` // 只保存在应用私有文件
	Auth    json.RawMessage           `json:"auth,omitempty"`   // 原生 Codex 生成的本应用登录凭据
}

type profileStore struct {
	Profiles []storedProfile `json:"profiles"` // 本应用创建的账号
}
