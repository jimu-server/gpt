package llmSdk

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/ollama/ollama/api"
	"github.com/ollama/ollama/format"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

var pool *sync.Pool

var ollama_host, ollama_port = "127.0.0.1", "11434"

func init() {
	// 初始化 sdk ollama 默认读取环境中的配置信息
	var err error
	ollama_host, ollama_port, err = net.SplitHostPort(strings.Trim(os.Getenv("OLLAMA_HOST"), "\"'"))
	if err != nil {
		ollama_host, ollama_port = "127.0.0.1", "11434"
		if ip := net.ParseIP(strings.Trim(os.Getenv("OLLAMA_HOST"), "[]")); ip != nil {
			ollama_host = ip.String()
		}
	}
	pool = &sync.Pool{New: func() interface{} {
		return &http.Client{}
	}}
}

type ollamaStream[T any] struct {
	entity T
	body   *bytes.Buffer
	err    error
	resp   *http.Response
}

func (s *ollamaStream[T]) Close() error {
	return s.resp.Body.Close()
}

func (s *ollamaStream[T]) Body() *bytes.Buffer {
	return s.body
}

func (s *ollamaStream[T]) Data() T {
	return s.entity
}

type DataEvent[T any] <-chan LLMStream[T]

func Chat[T any](req *api.ChatRequest) (DataEvent[T], error) {
	client := pool.Get().(*http.Client)
	var err error
	var request *http.Request
	marshal, err := jsoniter.Marshal(req)
	if err != nil {
		return nil, err
	}
	buffer := bytes.NewBuffer(marshal)
	url := fmt.Sprintf("http://%s:%s/api/chat", ollama_host, ollama_port)
	request, err = http.NewRequest(http.MethodPost, url, buffer)
	if err != nil {
		return nil, err
	}
	do, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	return chat[T](do)
}

func chat[T any](response *http.Response) (DataEvent[T], error) {
	send := make(chan LLMStream[T], 100)
	go func() {
		var err error
		var buf []byte
		var data T

		// 使用 bufio.NewReader 创建一个读取器，方便按行读取
		scanner := bufio.NewScanner(response.Body)
		scanBuf := make([]byte, 0, 512*format.KiloByte)
		scanner.Buffer(scanBuf, 512*format.KiloByte)
		// 创建一个通道
		defer close(send)
		for scanner.Scan() {
			buf = scanner.Bytes()
			stream := &ollamaStream[T]{}
			if err = scanner.Err(); err == io.EOF {
				break // 文件结束
			} else if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.ErrClosedPipe) {
				stream.err = err
				send <- stream
				break
			} else if err != nil {
				stream.err = err
				send <- stream
				break
			}
			if err = jsoniter.Unmarshal(buf, &data); err != nil {
				stream.err = err
				send <- stream
				break
			}
			stream.entity = data
			stream.resp = response
			stream.body = bytes.NewBuffer(buf)
			send <- stream
		}
	}()
	return send, nil
}
