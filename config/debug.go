package config

import "log"

var debug bool

// SetDebug 设置调试模式
func SetDebug(d bool) {
	debug = d
}

// DebugLog 调试日志，只在 debug=true 时打印
func DebugLog(format string, v ...interface{}) {
	if debug {
		log.Printf(format, v...)
	}
}
