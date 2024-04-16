package control

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/jimu-server/common/resp"
	"github.com/jimu-server/db"
	"github.com/jimu-server/middleware/auth"
	"github.com/jimu-server/model"
	"github.com/jimu-server/util/uuidutils/uuid"
	"github.com/jimu-server/web"
	"time"
)

func CreateConversation(c *gin.Context) {
	var err error
	var args CreateConversationArgs
	token := c.MustGet(auth.Key).(*auth.Token)
	web.BindJSON(c, &args)
	params := map[string]interface{}{
		"Id":     uuid.String(),
		"UserId": token.Id,
		"Title":  args.Title,
	}
	if err = GptMapper.CreateConversation(params); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("创建失败")))
		return
	}
	c.JSON(200, resp.Success(params["Id"], resp.Msg("创建成功")))
}

func DelConversation(c *gin.Context) {
	var err error
	var args map[string]string
	web.BindJSON(c, &args)
	token := c.MustGet(auth.Key).(*auth.Token)
	args["UserId"] = token.Id
	if err = GptMapper.DelConversation(args); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("创建失败")))
		return
	}
	c.JSON(200, resp.Success(nil, resp.Msg("创建成功")))
}

func GetConversation(c *gin.Context) {
	var err error
	token := c.MustGet(auth.Key).(*auth.Token)
	var list []model.AppChatConversationItem
	params := map[string]interface{}{
		"UserId": token.Id,
	}
	if list, err = GptMapper.ConversationList(params); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("查询失败")))
		return
	}
	c.JSON(200, resp.Success(list, resp.Msg("查询成功")))
}

func GetConversationHistory(c *gin.Context) {
	var err error
	token := c.MustGet(auth.Key).(*auth.Token)
	var list []model.AppChatMessage
	var conversationId string
	if conversationId = c.Query("conversationId"); conversationId == "" {
		c.JSON(500, resp.Error(err, resp.Msg("会话id不能为空")))
		return
	}
	params := map[string]interface{}{
		"UserId":         token.Id,
		"ConversationId": conversationId,
	}
	if list, err = GptMapper.ConversationHistory(params); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("查询失败")))
		return
	}
	c.JSON(200, resp.Success(list, resp.Msg("查询成功")))
}

func UpdateConversation(c *gin.Context) {
	var err error
	var args *CreateConversationArgs
	web.BindJSON(c, args)
	if err = GptMapper.CreateConversation(args); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("创建失败")))
		return
	}
	c.JSON(200, resp.Success(args, resp.Msg("创建成功")))
}

func Send(c *gin.Context) {
	var err error
	var args SendMessageArgs
	token := c.MustGet(auth.Key).(*auth.Token)
	web.BindJSON(c, &args)
	var begin *sql.Tx
	if begin, err = db.DB.Begin(); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("开启事务失败")))
		return
	}
	data := model.AppChatMessage{
		Id:             uuid.String(),
		ConversationId: args.ConversationId,
		UserId:         token.Id,
		Role:           "user",
		Content:        args.Content,
		CreateTime:     time.Now().Format("2006-01-02 15:04:05"),
	}
	if err = GptMapper.CreateMessage(data, begin); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("发送失败")))
		return
	}
	begin.Commit()
	c.JSON(200, resp.Success(data, resp.Msg("发送成功")))
}

func GetUid(c *gin.Context) {
	c.JSON(200, resp.Success(uuid.String(), resp.Msg("获取成功")))
}

func GetMessageItem(c *gin.Context) {
	var err error
	id := c.Query("id")
	params := map[string]interface{}{
		"Id": id,
	}
	var data *model.AppChatMessage
	if data, err = GptMapper.SelectMessageItem(params); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("查询失败")))
		return
	}
	c.JSON(200, resp.Success(data, resp.Msg("查询成功")))
}
