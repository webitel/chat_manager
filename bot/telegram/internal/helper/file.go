package helper

import (
	"path"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type File telegram.File

func (fd *File) Link(token string) string {
	return ((*telegram.File)(fd)).Link(token)
}

func (fd *File) FileName() string {

	if fd == nil {
		return ""
	}

	// name := path.Base(fd.FilePath)
	// switch name {
	// case ".", "/":
	// 	name = ""
	// }

	if s := fd.FilePath; s != "" {
		if s = path.Base(s); s == "/" {
			s = ""
		}
		return s
	}

	return ""
}
