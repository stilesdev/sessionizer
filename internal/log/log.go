package log

import (
	"fmt"
	"log"
)

var debug bool

func ShowDebugMessages(enable bool) {
	debug = enable
}

func Debug(message ...any) {
	if debug {
		fmt.Println(message...)
	}
}

func Fatal(err error) {
	log.Fatalln(err)
}

func FatalIfError(err error) {
	if err != nil {
		Fatal(err)
	}
}
