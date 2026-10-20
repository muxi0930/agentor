package lark

import (
	"context"
	"fmt"

	"agentor/internal/tasks"
	larksdk "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// startReaction 在原消息上登记处理中表态
func (s *Service) startReaction(ctx context.Context, client *larksdk.Client, source tasks.Source) {
	response, err := client.Im.V1.MessageReaction.Create(ctx, larkim.NewCreateMessageReactionReqBuilder().MessageId(source.MessageID).Body(larkim.NewCreateMessageReactionReqBodyBuilder().ReactionType(larkim.NewEmojiBuilder().EmojiType("Typing").Build()).Build()).Build())
	id := ""
	if err == nil && response.Success() && response.Data != nil {
		id = value(response.Data.ReactionId)
	} else if err == nil {
		err = fmt.Errorf("表态失败（%d）：%s", response.Code, response.Msg)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return
	}
	for i := range s.state.Incoming {
		item := &s.state.Incoming[i]
		if item.Source.AppID == source.AppID && item.Source.MessageID == source.MessageID {
			item.ReactionID = id
			item.ReactionState = "busy"
			if err != nil {
				item.ReactionState = "failed"
			}
			break
		}
	}
	s.persistLocked()
}

// sameMessage 比较消息的应用和会话归属
func sameMessage(a, b tasks.Source) bool {
	return a.AppID == b.AppID && a.MessageID == b.MessageID && a.ChatID == b.ChatID
}

// finishReaction 在关联任务全部结束后切换完成表态
func (s *Service) finishReaction(source tasks.Source) {
	if source.MessageID == "" {
		return
	}
	state := s.tasks.GetState()
	for _, execution := range state.Executions {
		if sameMessage(execution.Source, source) && (execution.Status == "running" || execution.Status == "queued") {
			return
		}
	}
	for _, request := range state.Requests {
		if sameMessage(request.Source, source) && (request.Status == "running" || request.Status == "queued") {
			return
		}
	}
	if s.pipelines != nil {
		pipelines := s.pipelines.GetState()
		for _, run := range pipelines.Runs {
			if sameMessage(run.Source, source) && (run.Status == "running" || run.Status == "queued") {
				return
			}
		}
		for _, request := range pipelines.Requests {
			if sameMessage(request.Input.Source, source) && (request.Status == "running" || request.Status == "queued") {
				return
			}
		}
	}
	client, ctx, err := s.deliveryClient(source)
	if err != nil {
		return
	}
	s.mu.Lock()
	id := ""
	found := false
	for i := range s.state.Incoming {
		item := &s.state.Incoming[i]
		if sameMessage(item.Source, source) && item.ReactionState == "busy" {
			item.ReactionState = "done"
			id = item.ReactionID
			found = true
			break
		}
	}
	if found {
		s.persistLocked()
	}
	s.mu.Unlock()
	if !found {
		return
	}
	if id != "" {
		client.Im.V1.MessageReaction.Delete(ctx, larkim.NewDeleteMessageReactionReqBuilder().MessageId(source.MessageID).ReactionId(id).Build())
	}
	client.Im.V1.MessageReaction.Create(ctx, larkim.NewCreateMessageReactionReqBuilder().MessageId(source.MessageID).Body(larkim.NewCreateMessageReactionReqBodyBuilder().ReactionType(larkim.NewEmojiBuilder().EmojiType("DONE").Build()).Build()).Build())
}
