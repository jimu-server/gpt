package gpt

import (
	"embed"
	"fmt"
	"github.com/jimu-server/db"
	"github.com/jimu-server/gpt/control"
	"github.com/jimu-server/gpt/mapper"
	"github.com/jimu-server/logger"
	"github.com/jimu-server/middleware/auth"
	"github.com/jimu-server/web"
	"go.uber.org/zap"
	"os/exec"
)

//go:embed mapper/file/*.xml
var mapperFile embed.FS

//go:embed app_gpt_sqlite.sql
var initSQL string

func init() {
	db.LocalGoBatis.LoadByRootPath("mapper", mapperFile)
	db.LocalGoBatis.ScanMappers(mapper.Gpt)
	initFileDb()
	chat := web.Engine.Group("/api/chat", auth.Authorization())
	chat.GET("/model/list", control.GetLLmModel)                      // 获取模型 可以走第三方获取
	chat.POST("/model/pull", control.PullLLmModel)                    // 获取模型 只能操作本地
	chat.POST("/model/delete", control.DeleteLLmModel)                // 删除模型 只能操作本地
	chat.POST("/user/model/create", control.CreateLLmModel)           // 创建模型 只能操作本地
	chat.POST("/user/model/delete", control.DeleteLLmModel)           // 删除用户模型  只能操作本地
	chat.POST("/conversation/create", control.CreateConversation)     // 创建会话
	chat.POST("/conversation/del", control.DelConversation)           // 删除会话
	chat.GET("/conversation/get", control.GetConversation)            // 查询会话列表
	chat.GET("/conversation/message", control.GetConversationHistory) // 查询会话历史数据
	chat.POST("/conversation/update", control.UpdateConversation)     // 修改会话
	chat.POST("/conversation", control.Stream)                        // 获取消息流回答  可以走第三方获取
	chat.POST("/send", control.Send)                                  // 发送消息
	chat.GET("/msg", control.GetMessageItem)                          // 查询指定消息
	chat.POST("/msg/delete", control.DeleteMessage)                   // 删除消息
	chat.POST("/knowledge/file/create", control.CreateKnowledgeFile)  // 创建知识库文件
	chat.GET("/knowledge/file/list", control.GetKnowledgeFileList)    // 查询知识库文件列表
	chat.GET("/knowledge/list", control.GetKnowledgeList)             // 查询知识库
	chat.POST("/knowledge/gen", control.GenKnowledge)                 // 生成知识库

}

func initFileDb() {
	check, _ := mapper.Gpt.InitCheck()
	if check == 0 {
		// 初始化db
		err := executeSQLScript("gpt.db", initSQL)
		if err != nil {
			logger.Logger.Panic("failed to initialize database", zap.Error(err))
		}
		logger.Info("init gpt.db success")
	}
}

// executeSQLScript runs the given SQL script on the specified SQLite database
func executeSQLScript(dbFile string, script string) error {
	cmd := exec.Command("sqlite3", dbFile, script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute SQL script: %s, output: %s", err, output)
	}
	return nil
}
