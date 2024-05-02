package chromaSdk

import (
	"testing"
)

func TestPdf(t *testing.T) {
	client := NewClient("http://127.0.0.1:8000")
	version, err := client.Version()
	if err != nil {
		t.Error(err.Error())
		return
	}
	t.Log(version)
}
