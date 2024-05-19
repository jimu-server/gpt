package control

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/jimu-server/common/resp"
	"github.com/jimu-server/db"
	"github.com/jimu-server/gpt/args"
	"github.com/jimu-server/middleware/auth"
	"github.com/jimu-server/model"
	"github.com/jimu-server/util/uuidutils/uuid"
	"github.com/jimu-server/web"
	"time"
)

func CreateConversation(c *gin.Context) {
	var err error
	var reqParams args.CreateConversationArgs
	token := c.MustGet(auth.Key).(*auth.Token)
	web.BindJSON(c, &reqParams)
	params := map[string]interface{}{
		"Id":     uuid.String(),
		"UserId": token.Id,
		"Title":  reqParams.Title,
	}
	if err = GptMapper.CreateConversation(params); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("创建失败")))
		return
	}
	c.JSON(200, resp.Success(params["Id"], resp.Msg("创建成功")))
}

func DelConversation(c *gin.Context) {
	var err error
	var reqParams map[string]string
	var begin *sql.Tx
	web.BindJSON(c, &reqParams)
	token := c.MustGet(auth.Key).(*auth.Token)
	reqParams["UserId"] = token.Id
	if begin, err = db.DB.Begin(); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("开启事务失败")))
		return
	}
	if err = GptMapper.DelConversation(reqParams, begin); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("创建失败")))
		return
	}
	if err = GptMapper.DeleteConversationMessage(reqParams, begin); err != nil {
		begin.Rollback()
		c.JSON(500, resp.Error(err, resp.Msg("创建失败")))
		return
	}
	begin.Commit()
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
	var reqParams *args.CreateConversationArgs
	web.BindJSON(c, reqParams)
	if err = GptMapper.CreateConversation(reqParams); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("创建失败")))
		return
	}
	c.JSON(200, resp.Success(reqParams, resp.Msg("创建成功")))
}

func Send(c *gin.Context) {
	var err error
	var reqParams args.SendMessageArgs
	token := c.MustGet(auth.Key).(*auth.Token)
	web.BindJSON(c, &reqParams)
	var begin *sql.Tx
	if begin, err = db.DB.Begin(); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("开启事务失败")))
		return
	}
	// 获取当前用户头像信息
	var avatar string
	if avatar, err = GptMapper.GetUserAvatar(map[string]string{"Id": token.Id}); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("获取头像失败")))
		return
	}
	id := uuid.String()
	if reqParams.MessageId == "" {
		reqParams.MessageId = id
	}
	data := model.AppChatMessage{
		Id:             id,
		ConversationId: reqParams.ConversationId,
		MessageId:      reqParams.MessageId,
		UserId:         token.Id,
		ModelId:        reqParams.ModelId,
		Picture:        avatar,
		Role:           "user",
		Content:        reqParams.Content,
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

func DeleteMessage(c *gin.Context) {
	var err error
	var reqParams args.DeleteChatMsg
	token := c.MustGet(auth.Key).(*auth.Token)
	web.BindJSON(c, &reqParams)
	params := map[string]interface{}{
		"list":   reqParams.Ids,
		"UserId": token.Id,
	}
	if err = GptMapper.DeleteMessage(params); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("删除失败")))
		return
	}
	c.JSON(200, resp.Success(nil, resp.Msg("删除成功")))
}
