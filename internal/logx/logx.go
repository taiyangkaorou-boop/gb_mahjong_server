// Package logx 提供进程内分级日志。级别从低到高：trace < info < warn < error < fatal。
// 低于当前级别的调用立即返回。Fatal 会打日志后结束进程。
// 不要把密码或完整 Token 写进日志。
package logx

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync/atomic"
)

// Level 是日志级别。数值越大越严重。
type Level int32

const (
	LevelTrace Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l Level) String() string {
	switch l {
	case LevelTrace:
		return "TRACE"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "INFO"
	}
}

// ParseLevel 解析配置字符串。无法识别时当作 info。
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return LevelTrace
	case "info", "":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// KnownLevel 报告 s 是否是合法级别名（空字符串视为 info）。
func KnownLevel(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "trace", "info", "warn", "warning", "error", "fatal":
		return true
	default:
		return false
	}
}

var curLevel int32 = int32(LevelInfo)

var std = log.New(os.Stderr, "", log.LstdFlags)

// exitFunc 测试里可替换，避免真的杀掉进程。
var exitFunc = os.Exit

// SetLevel 设置最低输出级别。
func SetLevel(l Level) {
	atomic.StoreInt32(&curLevel, int32(l))
}

// CurrentLevel 返回当前最低输出级别。
func CurrentLevel() Level {
	return Level(atomic.LoadInt32(&curLevel))
}

// Enabled 表示级别 l 是否会被写出去。
func Enabled(l Level) bool {
	return l >= CurrentLevel()
}

// SetOutput 重定向日志，单测用。
func SetOutput(w io.Writer) {
	std.SetOutput(w)
}

// SetExit 替换 Fatal 的退出函数，单测用。
func SetExit(fn func(int)) {
	if fn == nil {
		exitFunc = os.Exit
		return
	}
	exitFunc = fn
}

func logf(l Level, format string, args ...interface{}) {
	if !Enabled(l) {
		return
	}
	std.Printf("[%s] %s", l.String(), fmt.Sprintf(format, args...))
}

// Tracef 记录函数流向等细信息。
func Tracef(format string, args ...interface{}) { logf(LevelTrace, format, args...) }

// Infof 记录一般事件。
func Infof(format string, args ...interface{}) { logf(LevelInfo, format, args...) }

// Warnf 记录可继续运行的异常。
func Warnf(format string, args ...interface{}) { logf(LevelWarn, format, args...) }

// Errorf 记录服务端错误；不结束进程。
func Errorf(format string, args ...interface{}) { logf(LevelError, format, args...) }

// Fatalf 记录致命错误并退出进程。必须用 defer logx.Recover 无法兜住的场景（启动失败等）。
func Fatalf(format string, args ...interface{}) {
	logf(LevelFatal, format, args...)
	exitFunc(1)
}

// Recover 给 goroutine 入口 defer：接住 panic，打 error，避免整进程退出。
// 必须写成 defer logx.Recover("where")，不能包在匿名函数里再调，否则 recover 无效。
func Recover(where string) {
	if p := recover(); p != nil {
		Errorf("panic recovered where=%s panic=%v", where, p)
	}
}
