package logger

import (
	"log"
	"os"
)

const LogPath = "application.log"
const FailLogPath = "application.fail.log"

var (
	Log     *log.Logger
	FailLog *log.Logger
)

func init() {
	var file, err1 = os.Create(LogPath)
	var failFile, err2 = os.Create(FailLogPath)

	if err1 != nil {
		panic(err1)
	}

	if err2 != nil {
		panic(err1)
	}

	Log = log.New(file, "", log.LstdFlags)
	FailLog = log.New(failFile, "", log.LstdFlags)

	Log.Println("LogFile : " + LogPath)
	FailLog.Println("FailLogFile : " + FailLogPath)
}
