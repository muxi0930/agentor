package lark

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"

	"agentor/internal/artifact"
	"agentor/internal/constant"
	larkmodel "agentor/internal/lark/model"
	tasksmodel "agentor/internal/tasks/model"

	larksdk "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// value 读取可空的 SDK 字符串
func value(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}

// normalize 按原消息顺序解析文字与资源并保留引用
func normalize(event *larkim.P2MessageReceiveV1, appID, botID string) (larkmodel.Message, error) {
	var result larkmodel.Message
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Sender == nil || event.Event.Sender.SenderId == nil {
		return result, constant.ErrLarkMessageSourceMissing
	}

	message, sender := event.Event.Message, event.Event.Sender
	result.Source = tasksmodel.Source{Type: "lark", AppID: appID, ChatID: value(message.ChatId), SenderID: value(sender.SenderId.OpenId), MessageID: value(message.MessageId), ReplyToMessageID: value(message.ParentId)}
	if result.Source.ReplyToMessageID == "" {
		result.Source.ReplyToMessageID = value(message.RootId)
	}
	result.ChatType, result.SenderType = value(message.ChatType), value(sender.SenderType)
	result.Source.ChatType = result.ChatType
	var content map[string]json.RawMessage
	if err := json.Unmarshal([]byte(value(message.Content)), &content); err != nil {
		return result, err
	}
	read := func(name string) string {
		var text string
		json.Unmarshal(content[name], &text)
		return text
	}

	switch value(message.MessageType) {
	case "text":
		result.Text = read("text")
	case "image":
		result.Resources = append(result.Resources, larkmodel.Resource{Key: read("image_key"), Name: "image.png", Kind: "image"})
		result.Text = constant.LarkImagePlaceholder
	case "file":
		result.Resources = append(result.Resources, larkmodel.Resource{Key: read("file_key"), Name: read("file_name"), Kind: "file"})
		result.Text = constant.LarkFilePlaceholderPrefix + read("file_name") + "]"
	case "post":
		post := content
		if post["content"] == nil {
			// 多语言富文本优先读取简体中文，再按语言键顺序查找可用正文
			keys := make([]string, 0, len(content))
			for key := range content {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			if _, ok := content["zh_cn"]; ok {
				keys = append([]string{"zh_cn"}, keys...)
			}
			for _, key := range keys {
				var translated map[string]json.RawMessage
				if json.Unmarshal(content[key], &translated) == nil && translated["content"] != nil {
					post = translated
					break
				}
			}
		}
		var title string
		json.Unmarshal(post["title"], &title)
		lines := []string{}
		if title != "" {
			lines = append(lines, title)
		}
		var rows [][]map[string]any
		if err := json.Unmarshal(post["content"], &rows); err != nil {
			return result, err
		}
		for _, row := range rows {
			var line strings.Builder
			for _, node := range row {
				tag, _ := node[constant.WorkspaceKindTag].(string)
				text, _ := node["text"].(string)
				switch tag {
				case "text", "md":
					line.WriteString(text)
				case "a":
					href, _ := node["href"].(string)
					line.WriteString(text + " (" + href + ")")
				case "at":
					id, _ := node["user_id"].(string)
					if id != botID {
						line.WriteString("@" + id)
					}
				case "img":
					key, _ := node["image_key"].(string)
					name := fmt.Sprintf("image-%d.png", len(result.Resources)+1)
					result.Resources = append(result.Resources, larkmodel.Resource{Key: key, Name: name, Kind: "image"})
					line.WriteString(constant.LarkImagePlaceholderPrefix + name + "]")
				}
			}
			lines = append(lines, line.String())
		}
		result.Text = strings.Join(lines, "\n")
	default:
		return result, constant.ErrLarkMessageTypeUnsupported
	}

	for _, mention := range message.Mentions {
		if mention == nil || mention.Id == nil {
			continue
		}
		id := value(mention.Id.OpenId)
		result.Mentions = append(result.Mentions, id)
		if id == botID {
			result.Text = strings.ReplaceAll(result.Text, value(mention.Key), "")
		}
	}

	result.Text = strings.TrimSpace(result.Text)
	if result.Source.SenderID == "" || result.Source.MessageID == "" || result.Source.ChatID == "" {
		return result, constant.ErrLarkMessageSourceIncomplete
	}
	return result, nil
}

// hasIdentity 检查身份是否在允许列表内
func hasIdentity(values []larkmodel.Identity, id string) bool {
	return slices.ContainsFunc(values, func(identity larkmodel.Identity) bool { return identity.ID == id })
}

// authorized 核验单聊联系人或群内对当前机器人的提及
func authorized(config larkmodel.Settings, botID string, message larkmodel.Message) bool {
	if message.SenderType != "user" {
		return false
	}
	if message.ChatType == constant.LarkChatTypePrivate {
		return hasIdentity(config.AllowedUsers, message.Source.SenderID)
	}
	return message.ChatType == constant.LarkChatTypeGroup && hasIdentity(config.AllowedChats, message.Source.ChatID) && botID != "" && slices.Contains(message.Mentions, botID)
}

// receive 先核验范围和去重再异步处理消息
func (s *Service) receive(ctx context.Context, generation uint64, client *larksdk.Client, message larkmodel.Message) error {
	if message.SenderType != "user" {
		return nil
	}
	s.mu.Lock()
	if s.status.Generation != generation || s.ctx.Err() != nil {
		s.mu.Unlock()
		return nil
	}
	changed := false
	candidates := []larkmodel.Candidate{{Kind: "user", ID: message.Source.SenderID, LastSeen: now()}}
	if message.ChatType == constant.LarkChatTypeGroup {
		candidates = append(candidates, larkmodel.Candidate{Kind: "chat", ID: message.Source.ChatID, LastSeen: now()})
	}
	for _, candidate := range candidates {
		if candidate.ID == "" {
			continue
		}
		found := false
		for i := range s.state.Candidates {
			if s.state.Candidates[i].Kind == candidate.Kind && s.state.Candidates[i].ID == candidate.ID {
				s.state.Candidates[i].LastSeen = candidate.LastSeen
				found = true
				break
			}
		}
		if !found {
			s.state.Candidates = append(s.state.Candidates, candidate)
		}
		changed = true
	}
	if !authorized(s.config.Settings, s.status.BotOpenID, message) {
		if changed {
			s.persistLocked()
		}
		s.mu.Unlock()
		s.changed()
		return nil
	}

	for _, incoming := range s.state.Incoming {
		if incoming.Source.AppID == message.Source.AppID && incoming.Source.MessageID == message.Source.MessageID {
			s.mu.Unlock()
			return nil
		}
	}

	requestID := rand.Text()
	s.state.Incoming = append(s.state.Incoming, larkmodel.Incoming{Source: message.Source, RequestID: requestID, Status: constant.LarkStatusReceiving, CreatedAt: now()})
	err := s.persistLocked()
	if err != nil {
		s.state.Incoming = s.state.Incoming[:len(s.state.Incoming)-1]
	}
	projectID := s.config.DefaultProjectID
	s.mu.Unlock()
	s.changed()
	if err != nil {
		return err
	}
	go s.processMessage(ctx, generation, client, message, requestID, projectID)
	return nil
}

// updateIncoming 更新原消息的接收记录
func (s *Service) updateIncoming(source tasksmodel.Source, status string, err error) {
	s.mu.Lock()
	if s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	for i := range s.state.Incoming {
		item := &s.state.Incoming[i]
		if item.Source.AppID == source.AppID && item.Source.MessageID == source.MessageID {
			item.Status = status
			if err != nil {
				item.Error = err.Error()
			}
			break
		}
	}
	s.persistLocked()
	s.mu.Unlock()
	s.changed()
}

// processMessage 下载请求附件并提交统一控制中心入口
func (s *Service) processMessage(ctx context.Context, generation uint64, client *larksdk.Client, message larkmodel.Message, requestID, projectID string) {
	if handled, err := s.textControl(message); handled {
		s.updateIncoming(message.Source, "control", err)
		body := constant.LarkControlHandledMessage
		if err != nil {
			body = err.Error()
		}
		s.tasks.RecordNotification(tasksmodel.Notification{Title: constant.LarkControlReplyTitle, Body: body, Source: message.Source})
		return
	}

	s.startReaction(ctx, client, message.Source)
	for _, resource := range message.Resources {
		var savedID string
		name := resource.Name
		if name == "" {
			name = "attachment"
		}
		response, err := client.Im.V1.MessageResource.Get(ctx, larkim.NewGetMessageResourceReqBuilder().MessageId(message.Source.MessageID).FileKey(resource.Key).Type(resource.Kind).Build())
		if err == nil && (!response.Success() || response.File == nil) {
			err = fmt.Errorf("下载资源失败（%d）：%s", response.Code, response.Msg)
		}
		if err == nil {
			var data []byte
			data, err = io.ReadAll(response.File)
			if err == nil {
				if resource.Kind != "image" && response.FileName != "" {
					name = response.FileName
				}
				var item artifact.Attachment
				item, err = s.files.SaveRequestInput(requestID, name, http.DetectContentType(data), bytes.NewReader(data))
				savedID = item.ID
			}
		}
		if err != nil {
			if savedID == "" {
				s.files.RecordFailedInput(requestID, name, err.Error())
			}
			message.Text += constant.LarkAttachmentFailurePrefix + name + constant.LarkAttachmentFailureSeparator + err.Error()
		}
	}

	s.mu.Lock()
	current := s.status.Generation == generation
	s.mu.Unlock()
	if !current || ctx.Err() != nil {
		s.updateIncoming(message.Source, constant.LarkStatusInterrupted, constant.ErrLarkConnectionClosed)
		return
	}
	_, err := s.tasks.SubmitRemoteRequest(tasksmodel.ControllerInput{Text: message.Text, ProjectID: projectID, Source: message.Source}, requestID)
	if err != nil {
		s.updateIncoming(message.Source, constant.LarkStatusError, err)
		s.tasks.RecordNotification(tasksmodel.Notification{Title: constant.LarkRequestRejectedTitle, Body: err.Error(), Source: message.Source})
		return
	}
	s.updateIncoming(message.Source, constant.LarkStatusSubmitted, nil)
}
