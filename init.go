package cvc

import (
	"os"

	logging "github.com/inconshreveable/log15"
)

var log logging.Logger = logging.New("module", "cvc")
var verbose bool

func init() {
	var logLevel logging.Lvl = logging.LvlError
	var logFormat logging.Format = logging.TerminalFormat()

	if verboseEnv, found := os.LookupEnv("CVC_VERBOSE"); found && verboseEnv == "1" {
		verbose = true
		logLevel = logging.LvlDebug
	}

	SetLogging(logLevel, logging.StreamHandler(os.Stdout, logFormat))
}

func SetLogging(level logging.Lvl, handler logging.Handler) {
	log.SetHandler(logging.LvlFilterHandler(level, handler))
}
