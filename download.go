package main

import (
	"errors"

	"go.bbkane.com/warg/command"
)

func downloadCmd() command.Command {
	return command.New(
		"Download RSS feeds",
		withInitGlobalLogger(downloadRun),
	)
}

func downloadRun(cmdCtx command.Context) error {
	return errors.New("NOTE: replace this command.DoNothing call")
}
