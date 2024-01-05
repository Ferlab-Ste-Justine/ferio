package utils

import (
	"os"

	"github.com/Ferlab-Ste-Justine/ferio/logger"
)

func AbortOnErr(err error, log logger.Logger) {
	if err != nil {
		log.Errorf(err.Error())
		os.Exit(1)
	}
}
