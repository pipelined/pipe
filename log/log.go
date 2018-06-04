package log

import (
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
)

var debug bool

// Logger is a global interface for phono loggers
type Logger interface {
	Debug(...interface{})
	Info(...interface{})
}

func init() {
	var err error
	debug, err = strconv.ParseBool(os.Getenv("PHONO_DEBUG"))
	if err != nil {
		debug = false
	}
}

// GetLogger returns a new logger instance
func GetLogger() *logrus.Logger {
	l := logrus.New()
	if debug {
		l.SetLevel(logrus.DebugLevel)
	}
	return l
}
