package mapper

import "github.com/jimu-server/model"

type GptMapper struct {

	// 创建会话
	CreateConversation func(any) error
	DelConversation    func(any) error
	ConversationList   func(any) ([]model.AppChatConversationItem, error)
	UpdateConversation func(any) error
}
