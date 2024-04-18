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
		if err = jsoniter.Unmarshal(data.Data, &msg); err != nil {
			logs.Error(err.Error())
			break
		}
		v := msg["message"].(map[string]any)["content"].(string)
		_, err = c.Writer.Write(data.Data) // 根据你的实际情况调整
		flusher.Flush()                    // 立即将缓冲数据发送给客户端
		if err != nil {
			if err = data.Resp.Body.Close(); err != nil {
				logs.Error(err.Error())
				break
			}
			break // 如果写入失败，结束函数
		}
		content.WriteString(v)
	}
	contentStr := content.String()
	logs.Warn(contentStr)
	var begin *sql.Tx
	if begin, err = db.DB.Begin(); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("开启事务失败")))
		return
	}
	// 消息入库
	picture := ""
	if picture, err = GptMapper.GetModelAvatar(map[string]any{"Id": args.ModelId}); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("发送失败")))
		return
	}
	format := time.Now().Format("2006-01-02 15:04:05")
	data := model.AppChatMessage{
		Id:             args.Id,
		ConversationId: args.ConversationId,
		UserId:         token.Id,
		ModelId:        args.ModelId,
		Picture:        picture,
		Role:           "assistant",
		Content:        contentStr,
		CreateTime:     format,
	}
	if err = GptMapper.CreateMessage(data, begin); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("发送失败")))
		return
	}

	update := model.AppChatConversationItem{
		Id:         args.ConversationId,
		Picture:    picture,
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

func GetLLmModel(c *gin.Context) {
	var err error
	var models []model.LLmModel
	if models, err = GptMapper.ModelList(); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("查询失败")))
		return
	}
	c.JSON(200, resp.Success(models))
}
