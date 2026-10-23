package lark

import (
	"fmt"
	"math"
	"strings"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	tasksmodel "agentor/internal/tasks/model"
)

// replyFooter 从通知所属请求读取实际模型、思考强度与上下文占用
func (s *Service) replyFooter(notification tasksmodel.Notification) string {
	model, effort, threadID := "", "", ""
	state := s.tasks.GetState()
	for _, request := range state.Requests {
		if notification.RequestID != "" && request.ID == notification.RequestID {
			model, effort, threadID = request.Model, request.Effort, request.ThreadID
			break
		}
	}
	for _, execution := range state.Executions {
		if notification.ExecutionID != "" && execution.ID == notification.ExecutionID {
			model, effort, threadID = execution.Model, execution.Effort, execution.ThreadID
			break
		}
	}
	if notification.PipelineSessionID != "" && s.pipelines != nil {
		for _, request := range s.pipelines.GetState().Requests {
			if request.ID == notification.PipelineSessionID {
				model, effort, threadID = request.Input.Model, request.Input.Effort, request.ThreadID
				break
			}
		}
	}
	var usage *codexmodel.TokenUsage
	if s.runtime != nil && threadID != "" {
		usage = s.runtime.GetTokenUsage(threadID)
	}
	return formatReplyFooter(model, effort, usage)
}

// formatReplyFooter 保留未知状态，不以默认设置冒充实际会话数据
func formatReplyFooter(model, effort string, usage *codexmodel.TokenUsage) string {
	if model == "" {
		model = constant.LarkUnknownModelLabel
	}
	if effort == "" {
		effort = constant.LarkUnknownEffortLabel
	}
	context := constant.LarkUnknownValueLabel
	if usage != nil && usage.ModelContextWindow != nil && *usage.ModelContextWindow > 0 {
		percent := math.Round(float64(usage.Last.TotalTokens) / float64(*usage.ModelContextWindow) * 100)
		context = fmt.Sprintf("%.0f%%", percent)
	}
	return model + " · " + effort + " · ctx " + context
}

// replyCard 将正文与右对齐的小字号脚注放入第二版卡片
func replyCard(content, footer string) map[string]any {
	return map[string]any{
		"schema": "2.0",
		"body": map[string]any{"elements": []any{
			map[string]string{"tag": "markdown", "content": content},
			map[string]any{"tag": "div", "text": map[string]string{"tag": "plain_text", "content": footer, "text_size": "notation", "text_align": "right"}},
		}},
	}
}

// splitReplyMarkdown 优先按完整段落、代码块与表格分卡，超长代码块补齐围栏
func splitReplyMarkdown(content string) []string {
	const limit = 10000
	parts := []string{}
	current, block, fence := "", "", ""
	table := false
	flush := func() {
		if current != "" {
			parts = append(parts, current)
			current = ""
		}
	}
	appendBlock := func() {
		if block == "" {
			return
		}
		if len(jsonText(current+block)) > limit {
			flush()
		}
		current += block
		block = ""
	}
	lines := strings.SplitAfter(content, "\n")
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if fence == "" {
			if table && !strings.Contains(line, "|") {
				appendBlock()
				table = false
			}
			if index+1 < len(lines) && strings.Contains(line, "|") && strings.Contains(lines[index+1], "-") && strings.Trim(lines[index+1], " |:-\t\r\n") == "" {
				appendBlock()
				table = true
			}
		}
		if fence == "" && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")) {
			appendBlock()
			fence = trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, trimmed[:1]))]
			block = line
			continue
		}
		block += line
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]) == "" {
				fence = ""
				appendBlock()
			}
		} else if trimmed == "" {
			appendBlock()
		}
	}
	appendBlock()
	flush()
	return splitLargeReplyBlocks(parts, limit)
}

// splitLargeReplyBlocks 对超过卡片预算的块按行拆分并延续代码围栏或表头
func splitLargeReplyBlocks(blocks []string, limit int) []string {
	parts := []string{}
	for _, block := range blocks {
		if len(jsonText(block)) <= limit {
			parts = append(parts, block)
			continue
		}
		lines := strings.SplitAfter(block, "\n")
		prefix, suffix := "", ""
		first := strings.TrimSpace(lines[0])
		if strings.HasPrefix(first, "```") || strings.HasPrefix(first, "~~~") {
			fence := first[:len(first)-len(strings.TrimLeft(first, first[:1]))]
			prefix, suffix = lines[0], "\n"+fence+"\n"
			lines = lines[1:]
			if len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == fence {
				lines = lines[:len(lines)-1]
			}
		} else if len(lines) > 1 && strings.Contains(lines[0], "|") && strings.Contains(lines[1], "---") {
			prefix = lines[0] + lines[1]
			lines = lines[2:]
		}
		current := prefix
		size := len(jsonText(current + suffix))
		for _, line := range lines {
			lineSize := len(jsonText(line)) - 2
			if size+lineSize > limit && current != prefix {
				parts = append(parts, current+suffix)
				current = prefix
				size = len(jsonText(current + suffix))
			}
			if size+lineSize <= limit {
				current += line
				size += lineSize
				continue
			}
			// 单行超过预算时按字符拆分，保留 UTF-8 字符完整性
			for _, r := range line {
				runeSize := len(jsonText(string(r))) - 2
				if size+runeSize > limit && current != prefix {
					parts = append(parts, current+suffix)
					current = prefix
					size = len(jsonText(current + suffix))
				}
				current += string(r)
				size += runeSize
			}
		}
		if current != "" && current != prefix {
			parts = append(parts, current+suffix)
		}
	}
	return parts
}
