package model

import ()

type AccountPreferences struct {
	Model      string `json:"model"`      // 当前模型
	Effort     string `json:"effort"`     // 推理强度
	FastMode   bool   `json:"fastMode"`   // 是否启用快速模式
	Permission string `json:"permission"` // 账号默认权限
}
