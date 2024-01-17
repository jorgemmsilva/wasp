package isc

type LogInterface interface {
	LogInfof(format string, param ...interface{})
	LogDebugf(format string, param ...interface{})
	LogPanicf(format string, param ...interface{})
}
