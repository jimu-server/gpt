package gpt

import (
	"embed"
	"github.com/jimu-server/db"
	"github.com/jimu-server/gpt/control"
	"github.com/jimu-server/middleware/auth"
	"github.com/jimu-server/web"
)

//go:embed mapper/file/*.xml
var mapperFile embed.FS

func init() {
	db.GoBatis.LoadByRootPath("mapper", mapperFile)
	db.GoBatis.ScanMappers(control.GptMapper)

	chat := web.Engine.Group("/api/chat", auth.Authorization())

	chat.POST("/conversation/create", control.CreateConversation) // 创建会话
	chat.POST("/conversation/del", control.DelConversation)       // 删除会话
	chat.GET("/conversation/get", control.GetConversation)        // 查询会话列表
	chat.POST("/conversation/update", control.UpdateConversation) // 修改会话
	chat.POST("/send", control.Send)
}
