package llmSdk

import "bytes"

const (
	Segmentation = "\n"
)

type LLMStream[T any] interface {
	// Body 获取完整消息
	Body() *bytes.Buffer
	Data() T
	// Close 关闭流
	Close() error
}
