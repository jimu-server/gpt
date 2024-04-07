package mapper

import "model"

type GptMapper struct {

	// 创建会话
	CreateConversation func(any) error

	// 查询会话
	SelectConversation func(any) ([]model.Conversation, int64, error)
}
