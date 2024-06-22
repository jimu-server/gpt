package service

import (
	"bytes"
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/jimu-server/common/resp"
	"github.com/jimu-server/db"
	"github.com/jimu-server/gpt/args"
	"github.com/jimu-server/gpt/llm-sdk"
	"github.com/jimu-server/gpt/mapper"
	"github.com/jimu-server/logger"
	"github.com/jimu-server/middleware/auth"
	"github.com/jimu-server/model"
	"github.com/ollama/ollama/api"
	"net/http"
	"time"
)

var logs = logger.Logger
var GptMapper = mapper.Gpt

func ChatUpdate(token *auth.Token, args args.ChatArgs, content string) error {
	var begin *sql.Tx
	var err error
	if begin, err = db.LocalDB.Begin(); err != nil {
		return err
	}
	// 消息入库
	picture := ""
	if picture, err = GptMapper.GetModelAvatar(map[string]any{"Id": args.ModelId}); err != nil {
		logs.Error(err.Error())
		return err
	}
	format := time.Now().Format("2006-01-02 15:04:05")
	data := model.AppChatMessage{
		Id:             args.Id,
		ConversationId: args.ConversationId,
		MessageId:      args.MessageId,
		UserId:         token.Id,
		ModelId:        args.ModelId,
		Picture:        picture,
		Role:           "assistant",
		Content:        content,
		CreateTime:     format,
	}
	if err = GptMapper.CreateMessage(data, begin); err != nil {
		logs.Error(err.Error())
		return err
	}
	// 更新会话
	update := model.AppChatConversationItem{
		Id:         args.ConversationId,
		Picture:    picture,
		UserId:     "",
		Title:      "",
		LastModel:  args.Model,
		LastMsg:    content,
		LastTime:   format,
		CreateTime: "",
	}
	if err = GptMapper.UpdateConversationLastMsg(update, begin); err != nil {
		logs.Error(err.Error())
		if err = begin.Rollback(); err != nil {
			logs.Error(err.Error())
			return err
		}
		return err
	}
	return begin.Commit()
}

// SendChatStreamMessage 聊天流消息
func SendChatStreamMessage(c *gin.Context, params args.ChatArgs) {
	var err error
	var send <-chan llm_sdk.LLMStream[api.ChatResponse]
	token := c.MustGet(auth.Key).(*auth.Token)
	if send, err = llm_sdk.Chat[api.ChatResponse](params.ChatRequest); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("消息回复失败")))
		return
	}
	// 写入流式响应头
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(500, resp.Error(err, resp.Msg("消息回复失败")))
		return
	}
	content := bytes.NewBuffer(nil)
	//now := time.Now()
	for data := range send {
		v := data.Data()
		buffer := data.Body()
		buffer.WriteString(llm_sdk.Segmentation)
		_, err = c.Writer.Write(buffer.Bytes()) // 根据你的实际情况调整
		if err != nil {
			logs.Error(err.Error())
			if err = data.Close(); err != nil {
				logs.Error(err.Error())
				break
			}
			break // 如果写入失败，结束函数
		}
		flusher.Flush() // 立即将缓冲数据发送给客户端
		msg := v.Message.Content
		content.WriteString(msg)
	}
	contentStr := content.String()
	if err = ChatUpdate(token, params, contentStr); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("消息回复失败")))
		return
	}
}

func SendKnowledgeChatStreamMessage(c *gin.Context, params args.KnowledgeChatArgs) {
	var err error
	var send <-chan llm_sdk.LLMStream[api.ChatResponse]
	token := c.MustGet(auth.Key).(*auth.Token)
	IndexKnowledge(token, params)
	if send, err = llm_sdk.Chat[api.ChatResponse](params.ChatRequest); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("消息回复失败")))
		return
	}
	// 写入流式响应头
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(500, resp.Error(err, resp.Msg("消息回复失败")))
		return
	}
	content := bytes.NewBuffer(nil)

	for data := range send {
		v := data.Data()
		buffer := data.Body()
		buffer.WriteString(llm_sdk.Segmentation)
		_, err = c.Writer.Write(buffer.Bytes()) // 根据你的实际情况调整
		if err != nil {
			logs.Error(err.Error())
			if err = data.Close(); err != nil {
				logs.Error(err.Error())
				break
			}
			break // 如果写入失败，结束函数
		}
		flusher.Flush() // 立即将缓冲数据发送给客户端
		msg := v.Message.Content
		content.WriteString(msg)
	}
	contentStr := content.String()
	if err = ChatUpdate(token, params.ChatArgs, contentStr); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("消息回复失败")))
		return
	}
}

func IndexKnowledge(token *auth.Token, param args.KnowledgeChatArgs) {

}
