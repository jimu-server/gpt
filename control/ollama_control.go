package control

import (
	"bytes"
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/jimu-server/common/resp"
	"github.com/jimu-server/db"
	"github.com/jimu-server/gpt/llmSdk"
	"github.com/jimu-server/middleware/auth"
	"github.com/jimu-server/model"
	"github.com/jimu-server/util/uuidutils/uuid"
	"github.com/jimu-server/web"
	"github.com/ollama/ollama/api"
	"net/http"
	"time"
)

func Stream(c *gin.Context) {
	var err error
	var args ChatArgs
	var send <-chan llmSdk.LLMStream[api.ChatResponse]
	token := c.MustGet(auth.Key).(*auth.Token)
	web.BindJSON(c, &args)
	if send, err = llmSdk.Chat[api.ChatResponse](args.ChatRequest); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("消息回复失败")))
		return
	}
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
		buffer.WriteString(llmSdk.Segmentation)
		_, err = c.Writer.Write(buffer.Bytes()) // 根据你的实际情况调整
		if err != nil {
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
	var begin *sql.Tx
	if begin, err = db.DB.Begin(); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("开启事务失败")))
		return
	}
	// 消息入库
	picture := ""
	if picture, err = GptMapper.GetModelAvatar(map[string]any{"Id": args.ModelId}); err != nil {
		logs.Error(err.Error())
		c.JSON(500, resp.Error(err, resp.Msg("发送失败")))
		return
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
		Content:        contentStr,
		CreateTime:     format,
	}
	if err = GptMapper.CreateMessage(data, begin); err != nil {
		logs.Error(err.Error())
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
		logs.Error(err.Error())
		if err = begin.Rollback(); err != nil {
			logs.Error(err.Error())
			c.JSON(500, resp.Error(err, resp.Msg("消息回复失败")))
			return
		}
		c.JSON(500, resp.Error(err, resp.Msg("消息回复失败")))
		return
	}
	begin.Commit()
}

func GetLLmModel(c *gin.Context) {
	var err error
	var models []model.LLmModel
	token := c.MustGet(auth.Key).(*auth.Token)
	params := map[string]any{"UserId": token.Id}
	if models, err = GptMapper.ModelList(params); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("查询失败")))
		return
	}
	c.JSON(200, resp.Success(models))
}

func PullLLmModel(c *gin.Context) {
	var err error
	var args *api.PullRequest
	var flag bool
	var send <-chan llmSdk.LLMStream[api.ProgressResponse]
	web.BindJSON(c, &args)
	params := map[string]any{
		"Model": args.Name,
		"Flag":  true,
	}
	// 检查模型是否已经下载
	if flag, err = GptMapper.SelectModelStatus(params); err != nil {
		logs.Error(err.Error())
		c.JSON(500, resp.Error(err, resp.Msg("下载失败")))
		return
	}
	// 模型以下载
	if flag {
		c.JSON(200, resp.Success(nil))
		return
	}
	if send, err = llmSdk.Pull[api.ProgressResponse](args); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("拉取失败")))
		return
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(500, resp.Error(err, resp.Msg("模型下载失败")))
		return
	}
	for data := range send {
		buffer := data.Body()
		buffer.WriteString(llmSdk.Segmentation)
		_, err = c.Writer.Write(buffer.Bytes()) // 根据你的实际情况调整
		if err != nil {
			logs.Error(err.Error())
			if err = data.Close(); err != nil {
				logs.Error(err.Error())
				c.JSON(500, resp.Error(err, resp.Msg("模型下载失败")))
				return
			}
			c.JSON(500, resp.Error(err, resp.Msg("模型下载失败")))
			return // 如果写入失败，结束函数
		}
		flusher.Flush() // 立即将缓冲数据发送给客户端
		progressResponse := data.Data()
		if progressResponse.Status == "success" {
			// 更新模型下载情况
			if err = GptMapper.UpdateModelDownloadStatus(params); err != nil {
				logs.Error("模型拉取数据库状态更新失败")
				logs.Error(err.Error())
				c.JSON(500, resp.Error(err, resp.Msg("模型下载失败")))
				return
			}
		}
	}
}

func CreateLLmModel(c *gin.Context) {
	var err error
	var args *CreateModel
	var send <-chan llmSdk.LLMStream[api.ProgressResponse]
	web.BindJSON(c, &args)
	token := c.MustGet(auth.Key).(*auth.Token)
	// 检查模型是否存在
	var modelIbfo bool
	params := map[string]any{
		"Model": args.Name,
	}
	if modelIbfo, err = GptMapper.ModelExists(params); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("模型已存在")))
		return
	}
	if modelIbfo {
		c.JSON(500, resp.Error(err, resp.Msg("模型已存在")))
		return
	}

	var baseModeInfo *model.LLmModel
	params["Model"] = args.BaseModel
	if baseModeInfo, err = GptMapper.ModelInfo(params); err != nil {
		logs.Error(err.Error())
		c.JSON(500, resp.Error(err, resp.Msg("模型创建失败")))
		return
	}

	if !baseModeInfo.IsDownload {
		logs.Warn("模型已被删除")
		c.JSON(500, resp.Error(nil, resp.Msg("模型已被删除")))
	}

	if send, err = llmSdk.CreateModel[api.ProgressResponse](args.CreateRequest); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("模型创建失败")))
		return
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(500, resp.Error(err, resp.Msg("模型创建失败")))
		return
	}
	for data := range send {
		buffer := data.Body()
		buffer.WriteString(llmSdk.Segmentation)
		_, err = c.Writer.Write(buffer.Bytes()) // 根据你的实际情况调整
		if err != nil {
			logs.Error(err.Error())
			if err = data.Close(); err != nil {
				logs.Error(err.Error())
				c.JSON(500, resp.Error(err, resp.Msg("模型创建失败")))
				return
			}
			c.JSON(500, resp.Error(err, resp.Msg("模型创建失败")))
			return // 如果写入失败，结束函数
		}
		flusher.Flush() // 立即将缓冲数据发送给客户端
		progressResponse := data.Data()
		if progressResponse.Status == "success" {
			// 更新模型下载情况
			baseModeInfo.Name = args.Name
			baseModeInfo.Model = args.Name
			baseModeInfo.UserId = token.Id
			baseModeInfo.Pid = baseModeInfo.Id
			baseModeInfo.Id = uuid.String()
			if err = GptMapper.CreateModel(baseModeInfo); err != nil {
				logs.Error("模型拉取数据库状态更新失败")
				logs.Error(err.Error())
				c.JSON(500, resp.Error(err, resp.Msg("模型下载失败")))
				return
			}
		}
	}

	c.JSON(200, resp.Success(nil))
}

func DeleteLLmModel(c *gin.Context) {
	var err error
	var args *api.DeleteRequest
	var flag bool
	web.BindJSON(c, &args)
	// 修改模型下载状态
	params := map[string]any{
		"Model": args.Name,
		"Flag":  false,
	}
	if flag, err = GptMapper.SelectModelStatus(params); err != nil {
		logs.Error(err.Error())
		c.JSON(500, resp.Error(err, resp.Msg("删除失败")))
		return
	}
	// 模型已删除 直接返回成功
	if !flag {
		c.JSON(200, resp.Success(nil))
		return
	}
	if err = llmSdk.DeleteModel(args); err != nil {
		logs.Error(err.Error())
		c.JSON(500, resp.Error(err, resp.Msg("ollama模型删除失败")))
		return
	}

	if err = GptMapper.UpdateModelDownloadStatus(params); err != nil {
		logs.Error(err.Error())
		c.JSON(500, resp.Error(err, resp.Msg("模型删除失败")))
		return
	}
	c.JSON(200, resp.Success(nil))
}

func ModelList(c *gin.Context) {
	var err error
	var models []model.LLmModel
	token := c.MustGet(auth.Key).(*auth.Token)
	params := map[string]any{"UserId": token.Id}
	if models, err = GptMapper.BaseModelList(params); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("查询失败")))
		return
	}
	c.JSON(200, resp.Success(models))
}
