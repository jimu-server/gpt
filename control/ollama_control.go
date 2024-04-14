package control

import (
	"github.com/gin-gonic/gin"
	"github.com/jimu-server/common/resp"
	"github.com/jimu-server/web"
	"github.com/ollama/ollama/api"
	"github.com/ollama/ollama/sdk"
	"net/http"
)

func Send(c *gin.Context) {
	var err error
	var req *api.ChatRequest
	var send <-chan sdk.StreamData
	web.BindJSON(c, req)

	if send, err = sdk.Chat(req); err != nil {
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
	for data := range send {
		// 假设data是你想要发送的数据
		_, err = c.Writer.Write(data.Data) // 根据你的实际情况调整
		if err != nil {
			data.Resp.Body.Close()
			return // 如果写入失败，结束函数
		}
		flusher.Flush() // 立即将缓冲数据发送给客户端
	}
}
