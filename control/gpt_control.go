package control

import (
	"github.com/gin-gonic/gin"
	"github.com/jimu-server/common/resp"
	"github.com/jimu-server/middleware/auth"
	"github.com/jimu-server/model"
	"github.com/jimu-server/util/uuidutils/uuid"
	"github.com/jimu-server/web"
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
	c.JSON(200, resp.Success(nil, resp.Msg("创建成功")))
}

func DelConversation(c *gin.Context) {
	var err error
	var args *CreateConversationArgs
	web.BindJSON(c, args)
	if err = GptMapper.CreateConversation(args); err != nil {
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
