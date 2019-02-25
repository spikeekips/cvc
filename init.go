package cvc

import (
	logging "github.com/inconshreveable/log15"
)

var log logging.Logger = logging.New("module", "cvc")

func SetLogging(level logging.Lvl, handler logging.Handler) {
	log.SetHandler(logging.LvlFilterHandler(level, handler))
}
