package mocks

import "github.com/stretchr/testify/mock"

type Logger struct {
	mock.Mock
}

func (_m *Logger) Debugf(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *Logger) Infof(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *Logger) Warnf(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *Logger) Errorf(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *Logger) Fatalf(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *Logger) Panicf(format string, args ...interface{}) {
	_m.Called(format, args)
}
