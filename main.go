package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/mmcdole/gofeed"
)

type downloadFileArgs struct {
	url      string
	filePath string
}

// Turn a hardcoded list of feeds into a list of multiple pages for the correct dates
func generateDownloadFileArgs(now time.Time) []downloadFileArgs {
	end := now.AddDate(0, 0, 10)

	// Find the URLs by going to URL, clicking "Options" on the top right, then "RSS Feed", then copying the URL
	d := []downloadFileArgs{
		{
			// https://smcl.bibliocommons.com/v2/events?audiences=564274cf4d0090f742000016%2C564274cf4d0090f742000011
			url: "https://gateway.bibliocommons.com/v2/libraries/smcl/rss/events?audiences=564274cf4d0090f742000016%2C564274cf4d0090f742000011&startDate=2025-01-10&endDate=2025-01-13",
			// small hack - put the unique part of the feed here and we'll format it with the page namber later
			filePath: "smcl",
		},
		{
			// https://sjpl.bibliocommons.com/v2/events?audiences=5d5f09306bb98139001cffcc
			url:      "https://gateway.bibliocommons.com/v2/libraries/sjpl/rss/events?audiences=5d5f09306bb98139001cffcc",
			filePath: "sjpl",
		},
		{
			// https://sccl.bibliocommons.com/v2/events?audiences=5b2a5dcb2c1d736b168c62ac%2C5b28181c4727c7344c796677%2C5b28181c4727c7344c796676
			url:      "https://gateway.bibliocommons.com/v2/libraries/sccl/rss/events?audiences=5b2a5dcb2c1d736b168c62ac%2C5b28181c4727c7344c796677%2C5b28181c4727c7344c796676",
			filePath: "sccl",
		},
		{
			// https://paloalto.bibliocommons.com/v2/events?audiences=59a6e0705e7f62711a36e6ae%2C59a6e0705e7f62711a36e6ad%2C59a6e0705e7f62711a36e6ac
			url:      "https://gateway.bibliocommons.com/v2/libraries/paloalto/rss/events?audiences=59a6e0705e7f62711a36e6ae%2C59a6e0705e7f62711a36e6ad%2C59a6e0705e7f62711a36e6ac",
			filePath: "paloalto",
		},
	}

	var ret []downloadFileArgs
	for _, e := range d {
		parsedURL, err := url.Parse(e.url)
		if err != nil {
			panic(err)
		}
		q := parsedURL.Query()
		q.Set("startDate", now.Format("2006-01-02"))
		q.Set("endDate", end.Format("2006-01-02"))
		// get 5 pages
		for i := 1; i <= 5; i++ {
			q.Set("page", fmt.Sprintf("%d", i))
			parsedURL.RawQuery = q.Encode()
			u := parsedURL.String()
			ret = append(ret, downloadFileArgs{
				url:      u,
				filePath: fmt.Sprintf("tmp_rss_%s_%d.rss", e.filePath, i),
			})
		}
	}
	return ret
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

func main() {
	logLevels := map[string]slog.Level{
		"DEBUG": slog.LevelDebug,
		"INFO":  slog.LevelInfo,
		"WARN":  slog.LevelWarn,
		"ERROR": slog.LevelError,
	}

	// Due to Go's magic empty type behavior, if this isn't set, or if it's set to an empty string, the log will default to 0, corrrsponding to slog.LevelInfo
	// Example: toddlerevents_LOG_LEVEL=DEBUG go run . write
	logLevelStr := os.Getenv("toddlerevents_LOG_LEVEL")
	logLevel := logLevels[logLevelStr]

	readmePath := os.Getenv("toddlerevents_README_PATH")
	if readmePath == "" {
		readmePath = "tmp.md"
	}

	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level:       logLevel,
				AddSource:   false,
				ReplaceAttr: nil,
			}),
		),
	)

	const usage = "Usage: toddlerevents_LOG_LEVEL=INFO toddlerevents_README_PATH=tmp.md [download|write]"

	if len(os.Args) != 2 {
		fmt.Println(usage)
		os.Exit(1)
	}

	now := time.Now()
	downloadFileArgs := generateDownloadFileArgs(now)

	if os.Args[1] == "download" {
		for _, d := range downloadFileArgs {
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

	} else if os.Args[1] == "write" {
		events := []Event{}
		seenEvent := make(map[string]bool)
		for _, d := range downloadFileArgs {
			file, err := os.Open(d.filePath)
			if err != nil {
				slog.Error("could not open file",
					"file_path", d.filePath,
					"err", err.Error(),
				)
				continue
			}
			parser := gofeed.NewParser()
			feed, err := parser.Parse(file)
			if err != nil {
				slog.Error("could not parse file",
					"file_path", d.filePath,
					"err", err.Error(),
				)
				continue
			}
			file.Close()

			for _, item := range feed.Items {
				event := NewEvent(item)

				if seenEvent[event.GUID] {
					slog.Debug("skipping duplicate event",
						"title", event.Title,
						"city", event.City,
						"startTimeLocal", event.StartTimeLocal.Format("Mon 2006-01-02 15:04"),
					)
					continue
				} else {
					seenEvent[event.GUID] = true
				}

				for _, err := range event.ParseErrors {
					slog.Error("parse error",
						"title", event.Title,
						"city", event.City,
						"err", err.Error(),
					)
				}
				if filter(event) {
					events = append(events, event)
				} else {
					slog.Debug("filtering out event",
						"title", event.Title,
						"city", event.City,
						"startTimeLocal", event.StartTimeLocal.Format("Mon 2006-01-02 15:04"),
					)
				}
			}
		}
		readmeFile, err := os.Create(readmePath)
		if err != nil {
			slog.Error("could not create README",
				"readmeFilePath", readmePath,
				"err", err.Error(),
			)
			os.Exit(1)
		}
		defer readmeFile.Close()
		generateMarkdown(readmeFile, events)
	} else {
		fmt.Println(usage)
		os.Exit(1)
	}

}

// func main2() {
// 	events := []Event{
// 		{
// 			Title:          "Morning Yoga",
// 			Description:    "Relaxing yoga session.",
// 			Link:           "http://example.com/yoga",
// 			StartTimeLocal: time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC),
// 			EndTimeLocal:   time.Date(2025, 1, 10, 11, 0, 0, 0, time.UTC),
// 			City:           "Half Moon Bay",
// 		},
// 		{
// 			Title:          "Evening Surf",
// 			Description:    "Surfing with friends.",
// 			Link:           "http://example.com/surf",
// 			StartTimeLocal: time.Date(2025, 1, 10, 14, 30, 0, 0, time.UTC),
// 			EndTimeLocal:   time.Date(2025, 1, 10, 17, 0, 0, 0, time.UTC),
// 			City:           "Half Moon Bay",
// 		},
// 		{
// 			Title:          "Cooking Class",
// 			Description:    "Learn to cook Italian dishes.",
// 			Link:           "http://example.com/cooking",
// 			StartTimeLocal: time.Date(2025, 1, 11, 9, 0, 0, 0, time.UTC),
// 			EndTimeLocal:   time.Date(2025, 1, 11, 11, 0, 0, 0, time.UTC),
// 			City:           "San Francisco",
// 		},
// 	}

// 	GenerateMarkdown(os.Stdout, events)
// }
