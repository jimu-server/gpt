package control

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jimu-server/common/resp"
	args "github.com/jimu-server/gpt/args"
	"github.com/jimu-server/gpt/control/service"
	"github.com/jimu-server/gpt/llm-sdk"
	"github.com/jimu-server/gpt/vector"
	"github.com/jimu-server/middleware/auth"
	"github.com/jimu-server/model"
	"github.com/jimu-server/office"
	"github.com/jimu-server/util/treeutils/tree"
	"github.com/jimu-server/util/uuidutils/uuid"
	"github.com/jimu-server/web"
	"github.com/jimu-server/web/progress"
	jsoniter "github.com/json-iterator/go"
	"github.com/ollama/ollama/api"
	"github.com/philippgille/chromem-go"
	"io"
	"mime/multipart"
	"net/http"
)

func ChatStream(c *gin.Context) {
	var params args.ChatArgs
	web.BindJSON(c, &params)
	service.SendChatStreamMessage(c, params)
}

func KnowledgeChatStream(c *gin.Context) {
	var params args.KnowledgeChatArgs
	web.BindJSON(c, &params)
	service.SendKnowledgeChatStreamMessage(c, params)
}

func GetLLmModel(c *gin.Context) {
	var err error
	var result *api.ListResponse
	if result, err = llm_sdk.ModelList("http://127.0.0.1:11434"); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("拉取失败")))
		return
	}
	c.JSON(200, resp.Success(result.Models))
}

func PullLLmModel(c *gin.Context) {
	var err error
	var reqParams *api.PullRequest
	//var flag *model.LLmModel
	var send <-chan llm_sdk.LLMStream[api.ProgressResponse]
	web.BindJSON(c, &reqParams)
	if send, err = llm_sdk.Pull[api.ProgressResponse](reqParams); err != nil {
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
		buffer.WriteString(llm_sdk.Segmentation)
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
	}
}

func CreateLLmModel(c *gin.Context) {
	var err error
	var req_params *args.CreateModel
	var send <-chan llm_sdk.LLMStream[api.ProgressResponse]
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

	if send, err = llm_sdk.CreateModel[api.ProgressResponse](req_params.CreateRequest); err != nil {
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
		buffer.WriteString(llm_sdk.Segmentation)
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
	var reqParams *api.DeleteRequest
	//var flag *model.LLmModel
	web.BindJSON(c, &reqParams)
	//token := c.MustGet(auth.Key).(*auth.Token)
	//// 修改模型下载状态
	//params := map[string]any{
	//	"Model":  reqParams.Name,
	//	"Flag":   false,
	//	"UserId": token.Id,
	//}
	//if flag, err = GptMapper.SelectModel(params); err != nil {
	//	logs.Error(err.Error())
	//	c.JSON(500, resp.Error(err, resp.Msg("删除失败")))
	//	return
	//}
	//// 模型已删除 直接返回成功
	//if !flag.IsDownload {
	//	c.JSON(200, resp.Success(nil))
	//	return
	//}
	if err = llm_sdk.DeleteModel(reqParams); err != nil {
		logs.Error(err.Error())
		c.JSON(500, resp.Error(err, resp.Msg("ollama模型删除失败")))
		return
	}
	//params["Id"] = flag.Id
	//if flag.Id == flag.Pid {
	//	// 判断如果是系统内置模型 直接修改状态
	//	if err = GptMapper.UpdateModelDownloadStatus(params); err != nil {
	//		logs.Error(err.Error())
	//		c.JSON(500, resp.Error(err, resp.Msg("模型删除失败")))
	//		return
	//	}
	//} else {
	//	// 如果是用户自定义模型 则删除数据库记录
	//	if err = GptMapper.DeleteModel(params); err != nil {
	//		logs.Error(err.Error())
	//		c.JSON(500, resp.Error(err, resp.Msg("模型删除失败")))
	//		return
	//	}
	//}

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
	//var form *multipart.Form
	var reqParams args.KnowledgeArgs
	token := c.MustGet(auth.Key).(*auth.Token)
	var list []*model.AppChatKnowledgeFile
	web.BindJSON(c, &reqParams)
	/*if form, err = c.MultipartForm(); form == nil || err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("上传失败")))
		return
	}
	reqParams := args.KnowledgeArgs{
		Pid:     form.Value["pid"][0],
		Folders: form.Value["folders"],
	}*/

	// 处理文件夹创建
	if len(reqParams.Folders) > 0 {
		for _, v := range reqParams.Folders {
			list = append(list, &model.AppChatKnowledgeFile{
				Id:       uuid.String(),
				Pid:      reqParams.Pid,
				UserId:   token.Id,
				FileName: v,
				FileType: 0,
			})
		}
	}

	// 处理文件上传
	/*if files := form.File["files"]; files != nil {
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
				Pid:      reqParams.Pid,
				UserId:   token.Id,
				FileName: file.Filename,
				FilePath: full,
				FileType: 1,
			})
		}
	}*/

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

func DeleteKnowledge(c *gin.Context) {
	var err error
	var reqParams *args.DelKnowledge
	web.BindJSON(c, &reqParams)
	if err = vector.DB.DeleteCollection(reqParams.Id); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("删除失败")))
		return
	}
	if err = GptMapper.DeleteKnowledge(reqParams); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("删除失败")))
		return
	}
	c.JSON(200, resp.Success(nil))
}

func UpdateKnowledgeFile(c *gin.Context) {

}

func GenKnowledge(c *gin.Context) {
	var err error
	token := c.MustGet(auth.Key).(*auth.Token)
	var reqParams *args.GenKnowledgeArgs
	var percent float64 = 0
	//web.BindJSON(c, &reqParams)
	taskProgress, err := progress.NewProgress(c.Writer)
	if err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
		return
	}
	// 读取上传的文件
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
		return
	}
	genList := form.File["files"]
	if genList == nil {
		c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
		return
	}
	reqParams = &args.GenKnowledgeArgs{
		Name: form.Value["name"][0],
	}

	if err = taskProgress.Progress(percent, "加载数据文件.."); err != nil {
		c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
		return
	}
	var arr []model.EmbeddingAnalysis
	var buf *bytes.Buffer
	count := len(genList)
	base := 5.00
	step := base / float64(count)
	for _, header := range genList {
		buf = bytes.NewBuffer(nil)
		var fileContent multipart.File
		var e error
		if fileContent, e = header.Open(); e != nil {
			c.JSON(500, resp.Error(err, resp.Msg("文件读取失败")))
		}
		io.Copy(buf, fileContent)
		arr = append(arr, model.EmbeddingAnalysis{
			FileName: header.Filename,
			FileBody: buf.Bytes(),
		})

		msg := fmt.Sprintf("加载 %s 数据文件..", header.Filename)
		percent += step
		if err = taskProgress.Progress(percent, msg); err != nil {
			c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
			return
		}
	}
	// 处理文件数据转化为纯文本
	for i, file := range arr {
		var text string
		/*if text, err = office.DocxToString(file.FileBody); err != nil {
			err = fmt.Errorf("%s 文件解析失败--> error:%s", file.FileName, err.Error())
			if err = taskProgress.Progress(1, err.Error(), progress.Error()); err != nil {
				c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
				return
			}
			return
		}
		arr[i].Block = office.WordSplitter(text, 2)*/

		text, err = office.ExtractTextFromPDF(file.FileBody)
		if err != nil {
			return
		}
		fmt.Println(text)
		count += len(arr[i].Block)

	}

	// 对文件内容进行向量化存储
	instanceId := uuid.String()
	var collection *chromem.Collection
	taskProgress.ErrorCallback = func(err error) error {
		if collection == nil {
			return err
		}
		if e := vector.DB.DeleteCollection(collection.Name); e != nil {
			return e
		}
		return err
	}
	// 创建一个集合存储知识库种的文本片段
	if collection, err = vector.DB.GetOrCreateCollection(instanceId, nil, chromem.NewEmbeddingFuncOllama("nomic-embed-text", "")); err != nil {
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
		count = len(file.Block)
		step = base / float64(count)
		for _, line := range file.Block {
			doc := chromem.Document{
				ID:      uuid.String(),
				Content: "search_document: " + line,
			}
			if err = collection.AddDocument(context.Background(), doc); err != nil {
				logs.Error(err.Error())
				err = fmt.Errorf("%s 文件解析失败--> error:%s", file.FileName, err.Error())
				if err = taskProgress.Progress(100, err.Error(), progress.Error()); err != nil {
					c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
					return
				}
				return
			}
			msg := fmt.Sprintf("加载: %s 数据文件: %s", file.FileName, line)
			percent += step
			if err = taskProgress.Progress(percent, msg); err != nil {
				c.JSON(500, resp.Error(err, resp.Msg("任务失败")))
				return
			}
		}
	}

	// 数据入库
	files, _ := jsoniter.Marshal(reqParams.Files)
	instance := &model.AppChatKnowledgeInstance{
		Id:                   instanceId,
		UserId:               token.Id,
		KnowledgeName:        reqParams.Name,
		KnowledgeFiles:       string(files),
		KnowledgeDescription: reqParams.Description,
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
