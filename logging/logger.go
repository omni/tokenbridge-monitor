package logging

import (
	"log"
	"os"
)

func GetLogger(prefix string) *log.Logger {
	flags := log.LstdFlags | log.Lmicroseconds | log.Lshortfile
	return log.New(os.Stdout, prefix+": ", flags)
}
