package llm_sdk

import (
	"bytes"
	"net/http"
)

const (
	// Segmentation 流消息分割符号
	Segmentation = "\n"
)

type LLMStream[T any] interface {
	// Body 获取完整消息
	Body() *bytes.Buffer
	Data() T
	// Close 关闭流
	Close() error
}

type OnError func(err error)

// LLMChatStream 聊天流
type LLMChatStream struct {
	w       http.ResponseWriter
	f       http.Flusher
	onError OnError
}

func NewChatStream(w http.ResponseWriter) *LLMChatStream {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	return &LLMChatStream{
		w: w,
		f: w.(http.Flusher),
	}
}

func (receiver *LLMChatStream) Send(data []byte) {
	buffer := bytes.NewBuffer(data)
	buffer.WriteString(Segmentation)
	if _, err := receiver.w.Write(buffer.Bytes()); err != nil {
		receiver.onError(err)
		return
	}
	receiver.f.Flush()
	return
}

func (receiver *LLMChatStream) OnError(callback OnError) {
	receiver.onError = callback
}
