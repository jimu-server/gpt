package mapper

import (
	"database/sql"
	"github.com/jimu-server/model"
)

type GptMapper struct {
	SelectMessageItem func(any) (*model.AppChatMessage, error)

	// 创建会话
	CreateConversation        func(any) error
	DelConversation           func(any) error
	ConversationList          func(any) ([]model.AppChatConversationItem, error)
	ConversationHistory       func(any) ([]model.AppChatMessage, error)
	UpdateConversationLastMsg func(any, *sql.Tx) error

	CreateMessage func(any, *sql.Tx) error

	ModelList                 func() ([]model.LLmModel, error)
	UpdateModelDownloadStatus func(any) error

	GetUserAvatar  func(any) (string, error)
	GetModelAvatar func(any) (string, error)
}
