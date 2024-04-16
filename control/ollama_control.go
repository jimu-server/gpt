package control

import (
	"bytes"
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/jimu-server/common/resp"
	"github.com/jimu-server/db"
	"github.com/jimu-server/middleware/auth"
	"github.com/jimu-server/model"
	"github.com/jimu-server/web"
	jsoniter "github.com/json-iterator/go"
	"github.com/ollama/ollama/sdk"
	"net/http"
	"time"
)

func Stream(c *gin.Context) {
	var err error
	var args ChatArgs
	var send <-chan sdk.StreamData
	token := c.MustGet(auth.Key).(*auth.Token)
	web.BindJSON(c, &args)
	if send, err = sdk.Chat(args.ChatRequest); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("发送失败")))
		return
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(500, resp.Error(err, resp.Msg("发送失败")))
		return
	}
	content := bytes.NewBuffer(nil)
	//now := time.Now()
	for data := range send {
		var msg map[string]any
		_ = jsoniter.Unmarshal(data.Data, &msg)
		content.WriteString(msg["message"].(map[string]any)["content"].(string))
		_, err = c.Writer.Write(data.Data) // 根据你的实际情况调整
		if err != nil {
			data.Resp.Body.Close()
			return // 如果写入失败，结束函数
		}
		flusher.Flush() // 立即将缓冲数据发送给客户端
	}
	var begin *sql.Tx
	if begin, err = db.DB.Begin(); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("开启事务失败")))
		return
	}
	// 消息入库
	format := time.Now().Format("2006-01-02 15:04:05")
	contentStr := content.String()
	data := model.AppChatMessage{
		Id:             args.Id,
		ConversationId: args.ConversationId,
		UserId:         token.Id,
		Role:           "gpt",
		Content:        contentStr,
		CreateTime:     format,
	}
	if err = GptMapper.CreateMessage(data, begin); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("发送失败")))
		return
	}
	update := model.AppChatConversationItem{
		Id:         args.ConversationId,
		Picture:    "",
		UserId:     "",
		Title:      "",
		LastModel:  args.Model,
		LastMsg:    contentStr,
		LastTime:   format,
		CreateTime: "",
	}
	if err = GptMapper.UpdateConversationLastMsg(update, begin); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("发送失败")))
		return
	}
	begin.Commit()
}
