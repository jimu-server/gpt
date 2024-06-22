package vector

import "github.com/philippgille/chromem-go"

var DB *chromem.DB

func init() {
	var err error
	if DB, err = chromem.NewPersistentDB("", false); err != nil {
		panic(err)
	}
}
