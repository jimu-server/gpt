package gpt

import (
	"db"
	"embed"
	"gpt/control"
)

//go:embed mapper/file/*.xml
var mapperFile embed.FS

func init() {
	db.GoBatis.LoadByRootPath("mapper", mapperFile)
	db.GoBatis.ScanMappers(control.GptMapper)

}
