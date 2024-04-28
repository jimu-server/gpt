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

	SelectModelStatus func(any) (bool, error)
	ModelExists       func(any) (bool, error)
	ModelInfo         func(any) (*model.LLmModel, error)
	CreateModel       func(any) error

	CreateMessage func(any, *sql.Tx) error

	// 查询用户可用模型
	ModelList func(any) ([]model.LLmModel, error)

	// 管理擦汗寻查询 内置基础模型
	BaseModelList func(any) ([]model.LLmModel, error)

	// 更新模型状态
	UpdateModelDownloadStatus func(any) error

	GetUserAvatar  func(any) (string, error)
	GetModelAvatar func(any) (string, error)

	InsertKnowledge func(any) error
}
