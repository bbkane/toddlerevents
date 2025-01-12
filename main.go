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

func generateDownloadFileArgs(now time.Time) []downloadFileArgs {
	end := now.AddDate(0, 0, 10)

	d := []downloadFileArgs{
		{
			url:      "https://gateway.bibliocommons.com/v2/libraries/smcl/rss/events?audiences=564274cf4d0090f742000016%2C564274cf4d0090f742000011&startDate=2025-01-10&endDate=2025-01-13",
			filePath: "tmp_data.rss",
		},
	}

	for i := range d {
		u, err := url.Parse(d[i].url)
		if err != nil {
			panic(err)
		}
		q := u.Query()
		q.Set("startDate", now.Format("2006-01-02"))
		q.Set("endDate", end.Format("2006-01-02"))
		u.RawQuery = q.Encode()
		d[i].url = u.String()
	}
	return d
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
		file, _ := os.Open("tmp_data.rss")
		defer file.Close()
		fp := gofeed.NewParser()
		feed, _ := fp.Parse(file)

		events := []Event{}
		for _, item := range feed.Items {
			event := NewEvent(item)
			events = append(events, event)
		}

		generateMarkdown(os.Stdout, events)
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
