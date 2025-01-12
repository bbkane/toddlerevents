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

	d := []downloadFileArgs{
		{
			url: "https://gateway.bibliocommons.com/v2/libraries/smcl/rss/events?audiences=564274cf4d0090f742000016%2C564274cf4d0090f742000011&startDate=2025-01-10&endDate=2025-01-13",
			// small hack - put the unique part of the feed here and we'll format it with the page namber later
			filePath: "smcl",
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
	logLevelStr := os.Getenv("toddlerevents_LOG_LEVEL")
	logLevel := logLevels[logLevelStr]

	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: logLevel,
			}),
		),
	)

	const usage = "Usage: toddlerevents [download|write]"

	// curl 'https://gateway.bibliocommons.com/v2/libraries/smcl/rss/events?audiences=564274cf4d0090f742000016%2C564274cf4d0090f742000011&startDate=2025-01-10&endDate=2025-01-13' > tmp_data.rss
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
		eventCount := make(map[string]bool)
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
			}
			file.Close()

			for _, item := range feed.Items {
				event := NewEvent(item)

				if eventCount[event.GUID] {
					slog.Debug("skipping duplicate event",
						"title", event.Title,
						"city", event.City,
						"startTimeLocal", event.StartTimeLocal.Format("Mon 2006-01-02 15:04"),
					)
					continue
				} else {
					eventCount[event.GUID] = true
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
		const readmeFilePath = "tmp.md"
		readmeFile, err := os.Create(readmeFilePath)
		if err != nil {
			slog.Error("could not create README",
				"readmeFilePath", readmeFilePath,
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
