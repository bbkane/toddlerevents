package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"go.bbkane.com/warg/command"
)

func downloadCmd() command.Command {
	return command.New(
		"Download RSS feeds",
		withInitGlobalLogger(withDownloadFileArgs(downloadRun)),
		command.FlagMap(bibliocommonFlags()),
	)
}

func downloadRun(cmdCtx command.Context, ds []downloadFileArgs) error {
	for _, d := range ds {
		err := downloadFile(d)
		if err != nil {
			slog.Error("could not download file",
				"file_path", d.filePath,
				"url", d.url,
				"err", err.Error(),
			)
		}
		slog.Info("downloaded file",
			"file_path", d.filePath,
			"url", d.url,
		)
	}
	return nil
}

func downloadFile(d downloadFileArgs) error {
	file, err := os.Create(d.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	resp, err := http.Get(d.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
