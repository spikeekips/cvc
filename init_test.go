package cvc

import (
	"os"

	logging "github.com/inconshreveable/log15"
)

func init() {
	SetLogging(
		logging.LvlCrit,
		//logging.LvlDebug,
		logging.StreamHandler(os.Stdout, logging.TerminalFormat()),
	)
}
