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

	manage := chat.Group("/manage")

	manage.GET("/modelList", control.ModelList)

	chat.GET("/model/list", control.GetLLmModel)                      // 获取模型
	chat.POST("/model/pull", control.PullLLmModel)                    // 获取模型
	chat.POST("/model/delete", control.DeleteLLmModel)                // 获取模型
	chat.POST("/model/create", control.CreateLLmModel)                // 创建模型
	chat.POST("/conversation/create", control.CreateConversation)     // 创建会话
	chat.POST("/conversation/del", control.DelConversation)           // 删除会话
	chat.GET("/conversation/get", control.GetConversation)            // 查询会话列表
	chat.GET("/conversation/message", control.GetConversationHistory) // 查询会话历史数据
	chat.POST("/conversation/update", control.UpdateConversation)     // 修改会话
	chat.POST("/conversation", control.Stream)                        // 获取消息流回答
	chat.POST("/send", control.Send)                                  // 发送消息
	chat.GET("/uuid", control.GetUid)                                 // 生成消息uuid
	chat.GET("/msg", control.GetMessageItem)                          // 查询指定消息

	chat.POST("/knowledge/create", control.CreateKnowledge) // 创建知识
}
