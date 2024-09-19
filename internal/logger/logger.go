package logger

import "go.uber.org/zap"

var (
	_ Logger = (*zap.SugaredLogger)(nil)
	_ Logger = (*noLog)(nil)
)

type Logger interface {
	Debug(args ...interface{})
	Debugf(template string, args ...interface{})
	Info(args ...interface{})
	Infof(template string, args ...interface{})
	Warn(args ...interface{})
	Warnf(template string, args ...interface{})
	Error(args ...interface{})
	Errorf(template string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(template string, args ...interface{})
	Sync() error
}

var NoLog = noLog{}

type noLog struct{}

func (*noLog) Debug(_ ...interface{})            {}
func (*noLog) Debugf(_ string, _ ...interface{}) {}
func (*noLog) Info(_ ...interface{})             {}
func (*noLog) Infof(_ string, _ ...interface{})  {}
func (*noLog) Error(_ ...interface{})            {}
func (*noLog) Errorf(_ string, _ ...interface{}) {}
func (*noLog) Warn(_ ...interface{})             {}
func (*noLog) Warnf(_ string, _ ...interface{})  {}
func (*noLog) Fatal(_ ...interface{})            {}
func (*noLog) Fatalf(_ string, _ ...interface{}) {}
func (*noLog) Sync() error                       { return nil }
