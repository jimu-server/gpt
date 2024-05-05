package control

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jimu-server/common/resp"
	"github.com/jimu-server/config"
	args "github.com/jimu-server/gpt/args"
	"github.com/jimu-server/gpt/control/service"
	"github.com/jimu-server/gpt/llmSdk"
	"github.com/jimu-server/middleware/auth"
	"github.com/jimu-server/model"
	"github.com/jimu-server/office"
	"github.com/jimu-server/oss"
	"github.com/jimu-server/util/treeutils/tree"
	"github.com/jimu-server/util/uuidutils/uuid"
	"github.com/jimu-server/web"
	"github.com/jimu-server/web/progress"
	jsoniter "github.com/json-iterator/go"
	"github.com/ollama/ollama/api"
	"github.com/philippgille/chromem-go"
	"github.com/tencentyun/cos-go-sdk-v5"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

func Stream(c *gin.Context) {
	var params args.ChatArgs
	web.BindJSON(c, &params)
	service.SendChatStreamMessage(c, params)
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
	var reqParams *api.PullRequest
	var flag *model.LLmModel
	var send <-chan llmSdk.LLMStream[api.ProgressResponse]
	web.BindJSON(c, &reqParams)
	params := map[string]any{
		"Model": reqParams.Name,
		"Flag":  true,
	}
	// 检查模型是否已经下载
	if flag, err = GptMapper.SelectModel(params); err != nil {
		logs.Error(err.Error())
		c.JSON(500, resp.Error(err, resp.Msg("下载失败")))
		return
	}
	// 模型以下载
	if flag.IsDownload {
		c.JSON(200, resp.Success(nil))
		return
	}
	if send, err = llmSdk.Pull[api.ProgressResponse](reqParams); err != nil {
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
	var req_params *args.CreateModel
	var send <-chan llmSdk.LLMStream[api.ProgressResponse]
	web.BindJSON(c, &req_params)
	token := c.MustGet(auth.Key).(*auth.Token)
	// 检查模型是否存在
	var modelIbfo bool
	params := map[string]any{
		"Model": req_params.Name,
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
	params["Model"] = req_params.BaseModel
	if baseModeInfo, err = GptMapper.ModelInfo(params); err != nil {
		logs.Error(err.Error())
		c.JSON(500, resp.Error(err, resp.Msg("模型创建失败")))
		return
	}

	if !baseModeInfo.IsDownload {
		logs.Warn("模型已被删除")
		c.JSON(500, resp.Error(nil, resp.Msg("模型已被删除")))
	}

	if send, err = llmSdk.CreateModel[api.ProgressResponse](req_params.CreateRequest); err != nil {
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
			baseModeInfo.Name = req_params.Name
			baseModeInfo.Model = req_params.Name
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
	var req_params *api.DeleteRequest
	var flag *model.LLmModel
	web.BindJSON(c, &req_params)
	token := c.MustGet(auth.Key).(*auth.Token)
	// 修改模型下载状态
	params := map[string]any{
		"Model":  req_params.Name,
		"Flag":   false,
		"UserId": token.Id,
	}
	if flag, err = GptMapper.SelectModel(params); err != nil {
		logs.Error(err.Error())
		c.JSON(500, resp.Error(err, resp.Msg("删除失败")))
		return
	}
	// 模型已删除 直接返回成功
	if !flag.IsDownload {
		c.JSON(200, resp.Success(nil))
		return
	}
	if err = llmSdk.DeleteModel(req_params); err != nil {
		logs.Error(err.Error())
		c.JSON(500, resp.Error(err, resp.Msg("ollama模型删除失败")))
		return
	}
	params["Id"] = flag.Id
	if flag.Id == flag.Pid {
		// 判断如果是系统内置模型 直接修改状态
		if err = GptMapper.UpdateModelDownloadStatus(params); err != nil {
			logs.Error(err.Error())
			c.JSON(500, resp.Error(err, resp.Msg("模型删除失败")))
			return
		}
	} else {
		// 如果是用户自定义模型 则删除数据库记录
		if err = GptMapper.DeleteModel(params); err != nil {
			logs.Error(err.Error())
			c.JSON(500, resp.Error(err, resp.Msg("模型删除失败")))
			return
		}
	}

	// 如果使用户自建模型则直接删除

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

func CreateKnowledgeFile(c *gin.Context) {
	var err error
	var form *multipart.Form
	token := c.MustGet(auth.Key).(*auth.Token)
	var list []*model.AppChatKnowledgeFile
	if form, err = c.MultipartForm(); form == nil || err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("上传失败")))
		return
	}
	req_params := args.KnowledgeArgs{
		Pid:     form.Value["pid"][0],
		Folders: form.Value["folders"],
	}

	// 处理文件夹创建
	if len(req_params.Folders) > 0 {
		for _, v := range req_params.Folders {
			list = append(list, &model.AppChatKnowledgeFile{
				Id:       uuid.String(),
				Pid:      req_params.Pid,
				UserId:   token.Id,
				FileName: v,
				FileType: 0,
			})
		}
	}

	// 处理文件上传
	if files := form.File["files"]; files != nil {
		for _, file := range files {
			if !strings.HasSuffix(file.Filename, ".docx") {
				c.JSON(500, resp.Error(err, resp.Msg("上传失败")))
				return
			}
			open, err := file.Open()
			if err != nil {
				c.JSON(500, resp.Error(err, resp.Msg("上传失败")))
				return
			}
			// 上传文件服务器
			// 创建存储路径
			id := uuid.String()
			name := fmt.Sprintf("%s/knowledge/%s.docx", token.Id, id)
			// 执行推送到对象存储
			if _, err = oss.Tencent.Object.Put(context.Background(), name, open, nil); err != nil {
				c.JSON(500, resp.Error(err, resp.Msg("上传失败")))
				return
			}
			full := fmt.Sprintf("%s/%s", config.Evn.App.Tencent.BucketURL, name)
			list = append(list, &model.AppChatKnowledgeFile{
				Id:       id,
				Pid:      req_params.Pid,
				UserId:   token.Id,
				FileName: file.Filename,
				FilePath: full,
				FileType: 1,
			})
		}
	}

	if len(list) == 0 {
		c.JSON(200, resp.Success(nil))
		return
	}
	params := map[string]any{
		"list": list,
	}
	if err = GptMapper.InsertKnowledgeFile(params); err != nil {
		logs.Error(err.Error())
		c.JSON(500, resp.Error(err, resp.Msg("创建失败")))
		return
	}
	c.JSON(200, resp.Success(nil))
}

func GetKnowledgeFileList(c *gin.Context) {
	var err error
	pid := c.Query("pid")
	token := c.MustGet(auth.Key).(*auth.Token)
	params := map[string]any{
		"Pid":    pid,
		"UserId": token.Id,
	}
	var list []*model.AppChatKnowledgeFile
	if list, err = GptMapper.KnowledgeFileList(params); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("查询失败")))
		return
	}
	trees := tree.BuildTree(pid, list)
	c.JSON(200, resp.Success(trees))
}

func DeleteKnowledgeFile(c *gin.Context) {

}

func UpdateKnowledgeFile(c *gin.Context) {

}

func GenKnowledge(c *gin.Context) {
	var err error
	token := c.MustGet(auth.Key).(*auth.Token)
	var req_params *args.GenKnowledgeArgs
	var percent float64 = 0
	web.BindJSON(c, &req_params)
	taskProgress, err := progress.NewProgress(c.Writer)
	if err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
		return
	}
	if len(req_params.Files) == 0 {
		percent = 100
		if err = taskProgress.Progress(percent, "完成"); err != nil {
			c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
			return
		}
		return
	}
	if err = taskProgress.Progress(percent, "加载数据文件.."); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
		return
	}
	// 获取对应文件数据
	var filelist []*model.AppChatKnowledgeFile
	params := map[string]any{
		"UserId": token.Id,
		"list":   req_params.Files,
	}
	// 解析文件内容
	if filelist, err = GptMapper.KnowledgeFileListById(params); err != nil {
		logs.Error(err.Error())
		if err = taskProgress.Progress(1, errors.New("加载数据文件失败--> error:"+err.Error()).Error(), progress.Error()); err != nil {
			c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
			return
		}
		return
	}
	var get *cos.Response
	var arr []model.EmbeddingAnalysis
	var buf *bytes.Buffer
	count := len(filelist)
	base := 5.00
	step := base / float64(count)
	for _, file := range filelist {
		split := len(config.Evn.App.Tencent.BucketURL)
		get, err = oss.Tencent.Object.Get(context.Background(), file.FilePath[split:], nil)
		if err != nil {
			logs.Error(err.Error())
			err = fmt.Errorf("%s 文件读取失败--> error:%s", file.FileName, err.Error())
			if err = taskProgress.Progress(1, err.Error(), progress.Error()); err != nil {
				c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
				return
			}
			return
		}
		buf = bytes.NewBuffer(nil)
		io.Copy(buf, get.Body)
		arr = append(arr, model.EmbeddingAnalysis{
			AppChatKnowledgeFile: file,
			FileBody:             buf.Bytes(),
		})
		get.Body.Close()
		msg := fmt.Sprintf("加载 %s 数据文件..", file.FileName)
		percent += step
		if err = taskProgress.Progress(percent, msg); err != nil {
			c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
			return
		}
	}
	// 处理文件数据转化为纯文本
	var db *chromem.DB
	if db, err = chromem.NewPersistentDB("./chromemDB", false); err != nil {
		if err = taskProgress.Progress(0, "向量存储初始化失败"); err != nil {
			c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
			return
		}
	}

	for i, docxs := range arr {
		if arr[i].Lines, err = office.DocxToStringSlice(docxs.FileBody); err != nil {
			err = fmt.Errorf("%s 文件解析失败--> error:%s", docxs.AppChatKnowledgeFile.FileName, err.Error())
			if err = taskProgress.Progress(1, err.Error(), progress.Error()); err != nil {
				c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
				return
			}
			return
		}
		count += len(arr[i].Lines)
	}

	// 对文件内容进行向量化存储
	instanceId := uuid.String()
	var collection *chromem.Collection
	if collection, err = db.GetOrCreateCollection(instanceId, nil, chromem.NewEmbeddingFuncOllama("nomic-embed-text", "")); err != nil {
		if err = taskProgress.Progress(100, err.Error(), progress.Error()); err != nil {
			c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
			return
		}
		return
	}
	count = len(arr)
	base = 90.00
	base = base / float64(count)
	for _, file := range arr {
		count = len(file.Lines)
		step = base / float64(count)
		//docs := make([]chromem.Document, 0, count)
		for _, line := range file.Lines {
			doc := chromem.Document{
				ID:      uuid.String(),
				Content: "search_document: " + line,
			}
			if err = collection.AddDocument(context.Background(), doc); err != nil {
				logs.Error(err.Error())
				err = fmt.Errorf("%s 文件解析失败--> error:%s", file.AppChatKnowledgeFile.FileName, err.Error())
				if err = taskProgress.Progress(100, err.Error(), progress.Error()); err != nil {
					c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
					return
				}
				return
			}
			msg := fmt.Sprintf("加载: %s 数据文件: %s", file.AppChatKnowledgeFile.FileName, line)
			percent += step
			if err = taskProgress.Progress(percent, msg); err != nil {
				c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
				return
			}
		}
	}

	// 数据入库
	files, _ := jsoniter.Marshal(req_params.Files)
	instance := &model.AppChatKnowledgeInstance{
		Id:                   instanceId,
		UserId:               token.Id,
		KnowledgeName:        req_params.Name,
		KnowledgeFiles:       string(files),
		KnowledgeDescription: req_params.Description,
		KnowledgeType:        0,
	}
	if err = GptMapper.CreateKnowledge(instance); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("生成失败")))
		return
	}
	if err = taskProgress.Progress(100, "知识库生成成功"); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
		return
	}
}

func GetKnowledgeList(c *gin.Context) {
	var err error
	token := c.MustGet(auth.Key).(*auth.Token)
	params := map[string]any{
		"UserId": token.Id,
	}
	var list []*model.AppChatKnowledgeInstance
	if list, err = GptMapper.KnowledgeList(params); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("查询失败")))
		return
	}
	c.JSON(200, resp.Success(list))
}
