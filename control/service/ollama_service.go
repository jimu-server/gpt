package service

import (
	"database/sql"
	"github.com/jimu-server/db"
	"github.com/jimu-server/gpt/args"
	"github.com/jimu-server/gpt/mapper"
	"github.com/jimu-server/logger"
	"github.com/jimu-server/middleware/auth"
	"github.com/jimu-server/model"
	"time"
)

var logs = logger.Logger
var GptMapper = mapper.Gpt

func ChatUpdate(token *auth.Token, args args.ChatArgs, content string) error {
	var begin *sql.Tx
	var err error
	if begin, err = db.DB.Begin(); err != nil {
		return err
	}
	// 消息入库
	picture := ""
	if picture, err = GptMapper.GetModelAvatar(map[string]any{"Id": args.ModelId}); err != nil {
		logs.Error(err.Error())
		return err
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
		Content:        content,
		CreateTime:     format,
	}
	if err = GptMapper.CreateMessage(data, begin); err != nil {
		logs.Error(err.Error())
		return err
	}
	// 更新会话
	update := model.AppChatConversationItem{
		Id:         args.ConversationId,
		Picture:    picture,
		UserId:     "",
		Title:      "",
		LastModel:  args.Model,
		LastMsg:    content,
		LastTime:   format,
		CreateTime: "",
	}
	if err = GptMapper.UpdateConversationLastMsg(update, begin); err != nil {
		logs.Error(err.Error())
		if err = begin.Rollback(); err != nil {
			logs.Error(err.Error())
			return err
		}
		return err
	}
	return begin.Commit()
}
