package utils

import (
	"fmt"
	"os"
)

var log *os.File

func WriteLog(str string, args ...interface{}) error {
	mes := fmt.Sprintf(str+"\n", args...)
	_, err := getLogFile().Write([]byte(mes))
	return err
}

func getLogFile() *os.File {
	if log == nil {
		log, _ = os.OpenFile("wonny2tv.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	}
	return log
}
