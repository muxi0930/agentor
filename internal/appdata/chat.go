package appdata

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	appdatamodel "agentor/internal/appdata/model"
	"agentor/internal/constant"
)

// ToggleThreadPin 切换会话的置顶状态
func (s *Service) ToggleThreadPin(threadID string) (appdatamodel.Settings, error) {
	return s.update(func(settings *appdatamodel.Settings) {
		next := map[string]bool{}
		for id, pinned := range settings.PinnedThreads {
			next[id] = pinned
		}
		if next[threadID] {
			delete(next, threadID)
		} else {
			next[threadID] = true
		}
		settings.PinnedThreads = next
	})
}

// SetChatOrder 保存项目或项目内会话的显示顺序
func (s *Service) SetChatOrder(cwd string, ids []string) (appdatamodel.Settings, error) {
	return s.update(func(settings *appdatamodel.Settings) {
		previous := settings.ProjectOrder
		if cwd != "" {
			previous = settings.ChatOrder[cwd]
		}
		order := []string{}
		for _, id := range append(append([]string{}, ids...), previous...) {
			if !slices.Contains(order, id) {
				order = append(order, id)
			}
		}
		if cwd == "" {
			settings.ProjectOrder = order
			return
		}
		next := map[string][]string{}
		for path, old := range settings.ChatOrder {
			next[path] = old
		}
		next[cwd] = order
		settings.ChatOrder = next
	})
}

// CreateWorktree 基于当前提交创建分支工作树并选中其目录
func (s *Service) CreateWorktree(cwd, branch, directory string) (appdatamodel.Settings, error) {
	if cwd == "" || strings.TrimSpace(branch) == "" || strings.TrimSpace(directory) == "" {
		return appdatamodel.Settings{}, constant.ErrAppDataWorktreeFieldsRequired
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return appdatamodel.Settings{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), constant.AppDataWorktreeTimeout)
	defer cancel()
	check := exec.CommandContext(ctx, "git", "check-ref-format", "--branch", branch)
	if output, err := check.CombinedOutput(); err != nil {
		return appdatamodel.Settings{}, fmt.Errorf("分支名称无效：%s", strings.TrimSpace(string(output)))
	}

	command := exec.CommandContext(ctx, "git", "-C", cwd, "worktree", "add", "-b", branch, directory, "HEAD")
	if output, err := command.CombinedOutput(); err != nil {
		return appdatamodel.Settings{}, fmt.Errorf("创建 worktree 失败：%s (%w)", strings.TrimSpace(string(output)), err)
	}
	return s.SetProjectDirectory(directory)
}
