package appdata

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func (s *Service) ToggleThreadPin(threadID string) (Settings, error) {
	return s.update(func(settings *Settings) {
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

func (s *Service) SetChatOrder(cwd string, ids []string) (Settings, error) {
	return s.update(func(settings *Settings) {
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

func (s *Service) CreateWorktree(cwd, branch, directory string) (Settings, error) {
	if cwd == "" || strings.TrimSpace(branch) == "" || strings.TrimSpace(directory) == "" {
		return Settings{}, errors.New("请填写源项目、分支和 worktree 目录")
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return Settings{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	check := exec.CommandContext(ctx, "git", "check-ref-format", "--branch", branch)
	if output, err := check.CombinedOutput(); err != nil {
		return Settings{}, fmt.Errorf("分支名称无效：%s", strings.TrimSpace(string(output)))
	}
	command := exec.CommandContext(ctx, "git", "-C", cwd, "worktree", "add", "-b", branch, directory, "HEAD")
	if output, err := command.CombinedOutput(); err != nil {
		return Settings{}, fmt.Errorf("创建 worktree 失败：%s (%w)", strings.TrimSpace(string(output)), err)
	}
	return s.SetProjectDirectory(directory)
}
