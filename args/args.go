package args

import "github.com/ollama/ollama/api"

type CreateConversationArgs struct {
	Title string `json:"title" form:"title" binding:"required"`
}

type DelConversationArgs struct {
	Id string `json:"id" form:"id" binding:"required"`
}

type ChatArgs struct {
	// 会话id
	ConversationId string `json:"conversationId" form:"conversationId" binding:"required"`
	// 消息id
	Id        string `json:"id" form:"id" binding:"required"`
	MessageId string `json:"messageId" form:"messageId" binding:"required"`
	ModelId   string `json:"modelId" form:"modelId" binding:"required"`
	*api.ChatRequest
}

type SendMessageArgs struct {
	ConversationId string `json:"conversationId" form:"conversationId" binding:"required"`
	Content        string `json:"content" form:"content" binding:"required"`
	ModelId        string `json:"modelId" form:"modelId" binding:"required"`
	MessageId      string `json:"messageId" form:"messageId"`
}

type CreateModel struct {
	BaseModel string `json:"baseModel"`
	*api.CreateRequest
}

type KnowledgeArgs struct {
	Pid     string   `json:"pid" form:"pid"`
	Folders []string `json:"folders" form:"folders"`
}

type GenKnowledgeArgs struct {
	Name        string   `json:"name" form:"name"`
	Description string   `json:"description" form:"description"`
	Files       []string `json:"files" form:"files"`
}