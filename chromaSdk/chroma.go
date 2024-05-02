package chromaSdk

import (
	"bytes"
	jsoniter "github.com/json-iterator/go"
	"net/http"
	"strings"
	"sync"
)

type Client struct {
	Address string
	pool    *sync.Pool
}

func NewClient(address string) *Client {
	pool := &sync.Pool{
		New: func() interface{} {
			return &http.Client{}
		},
	}
	if address == "" {
		address = "http://127.0.0.1:8000"
	}
	if strings.HasSuffix(address, "/") {
		address = address[:len(address)-1]
	}
	return &Client{
		Address: address,
		pool:    pool,
	}
}

func (client *Client) Version() (string, error) {
	var response *http.Response
	var request *http.Request
	var err error
	HttpClient := client.pool.Get().(*http.Client)
	if request, err = http.NewRequest(http.MethodGet, client.Address+"/api/v1/version", nil); err != nil {
		return "", err
	}

	if response, err = HttpClient.Do(request); err != nil {
		return "", err
	}
	defer response.Body.Close()
	client.pool.Put(HttpClient)
	return getResponseData[string](response)
}

func getResponseData[T any](response *http.Response) (T, error) {
	var buf bytes.Buffer
	var err error
	var v T
	var value any
	if _, err = buf.ReadFrom(response.Body); err != nil {
		return v, err
	}
	if err = jsoniter.Unmarshal(buf.Bytes(), &value); err != nil {
		return v, err
	}
	v = value.(T)
	return v, nil
}
