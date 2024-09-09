package logger

import "go.uber.org/zap"

var _ Logger = (*zap.SugaredLogger)(nil)
var _ Logger = (*noLog)(nil)

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

func (*noLog) Debug(args ...interface{})                   {}
func (*noLog) Debugf(template string, args ...interface{}) {}
func (*noLog) Info(args ...interface{})                    {}
func (*noLog) Infof(template string, args ...interface{})  {}
func (*noLog) Error(args ...interface{})                   {}
func (*noLog) Errorf(template string, args ...interface{}) {}
func (*noLog) Warn(args ...interface{})                    {}
func (*noLog) Warnf(template string, args ...interface{})  {}
func (*noLog) Fatal(args ...interface{})                   {}
func (*noLog) Fatalf(template string, args ...interface{}) {}
func (*noLog) Sync() error                                 { return nil }
