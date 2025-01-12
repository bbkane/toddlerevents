package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
)

type Event struct {
	Title          string
	Description    string
	Link           string
	StartTimeLocal time.Time
	EndTimeLocal   time.Time
	City           string
	ParseErrors    []error
}

func parseTime(bc map[string][]ext.Extension, key string) (time.Time, error) {

	startDateList, exists := bc[key]
	if !exists {
		return time.Time{}, fmt.Errorf("could not find %v", key)
	}
	if len(startDateList) != 1 {
		return time.Time{}, fmt.Errorf("not exactly one instance of %v", key)
	}
	date := startDateList[0].Value
	sd, err := time.Parse("2006-01-02T15:04", date)
	if err != nil {
		return time.Time{}, fmt.Errorf("Could not parse %v: %w", key, err)
	}
	return sd, nil
}

func NewEvent(item *gofeed.Item) Event {
	event := Event{
		Title:          item.Title,
		Description:    strings.TrimSpace(bluemonday.StrictPolicy().Sanitize(item.Description)),
		Link:           item.Link,
		StartTimeLocal: time.Time{},
		EndTimeLocal:   time.Time{},
		City:           "",
		ParseErrors:    nil,
	}
	bc, exists := item.Extensions["bc"]
	if !exists {
		event.ParseErrors = append(event.ParseErrors, errors.New("No \"bc\" extension"))
		// everything else relies on "bc" extension, so go ahead and return early if we can't find it
		return event
	}

	startTimeLocal, err := parseTime(bc, "start_date_local")
	if err != nil {
		event.ParseErrors = append(event.ParseErrors, fmt.Errorf("could not parse start_date: %w", err))
	}
	event.StartTimeLocal = startTimeLocal

	endTimeLocal, err := parseTime(bc, "end_date_local")
	if err != nil {
		event.ParseErrors = append(event.ParseErrors, fmt.Errorf("Could not parse end_date: %w", err))
	}
	event.EndTimeLocal = endTimeLocal

	// what I wish I could do...
	// city := bc["locationList"][0].Children["city"][0].Value
	var city string
	if locationList, ok := bc["location"]; ok && len(locationList) == 1 {
		if cityList, ok := locationList[0].Children["city"]; ok && len(cityList) == 1 {
			city = cityList[0].Value
		} else {
			event.ParseErrors = append(event.ParseErrors, errors.New("could not find city"))
		}
	} else {
		event.ParseErrors = append(event.ParseErrors, errors.New("could not find location"))
	}
	event.City = city

	return event

}

func generateMarkdown(w io.Writer, events []Event) {
	// Group events by date, then by city
	groupedByDate := make(map[time.Time]map[string][]Event)

	for _, event := range events {
		dateKey := time.Date(event.StartTimeLocal.Year(), event.StartTimeLocal.Month(), event.StartTimeLocal.Day(), 0, 0, 0, 0, event.StartTimeLocal.Location())
		if _, exists := groupedByDate[dateKey]; !exists {
			groupedByDate[dateKey] = make(map[string][]Event)
		}
		groupedByDate[dateKey][event.City] = append(groupedByDate[dateKey][event.City], event)
	}

	// Sort dates for consistent output
	sortedDates := make([]time.Time, 0, len(groupedByDate))
	for date := range groupedByDate {
		sortedDates = append(sortedDates, date)
	}
	sort.Slice(sortedDates, func(i, j int) bool {
		return sortedDates[i].Before(sortedDates[j])
	})

	for _, date := range sortedDates {
		fmt.Fprintf(w, "# %s\n\n", date.Format("Mon 2006-01-02"))
		cities := groupedByDate[date]

		// Sort cities for consistent output
		sortedCities := make([]string, 0, len(cities))
		for city := range cities {
			sortedCities = append(sortedCities, city)
		}
		sort.Strings(sortedCities)

		for _, city := range sortedCities {
			fmt.Fprintf(w, "## %s\n\n", city)
			events := cities[city]

			// Sort events by start time
			sort.Slice(events, func(i, j int) bool {
				return events[i].StartTimeLocal.Before(events[j].StartTimeLocal)
			})

			for i, event := range events {
				if i > 0 {
					fmt.Fprintf(w, "---\n\n")
				}
				fmt.Fprintf(w, "%s - %s [%s](%s)\n\n",
					event.StartTimeLocal.Format("15:04"),
					event.EndTimeLocal.Format("15:04"),
					event.Title,
					event.Link)
				fmt.Fprintf(w, "%s\n\n", event.Description)
			}
		}
	}
}

type downloadFileArgs struct {
	url      string
	filePath string
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

func main() {
	// curl 'https://gateway.bibliocommons.com/v2/libraries/smcl/rss/events?audiences=564274cf4d0090f742000016%2C564274cf4d0090f742000011&startDate=2025-01-10&endDate=2025-01-13' > tmp_data.rss
	if len(os.Args) != 2 {
		fmt.Println("Usage: toddlerevents [download|write]")
		os.Exit(1)
	}

	now := time.Now()
	downloadFileArgs := generateDownloadFileArgs(now)

	if os.Args[1] == "download" {
		for _, d := range downloadFileArgs {
			err := downloadFile(d)
			if err != nil {
				slog.Error("could not download file",
					"url", d.url,
					"file_path", d.filePath,
					"err", err.Error(),
				)
			}
			slog.Info("downloaded file",
				"url", d.url,
				"file_path", d.filePath,
			)
		}

	} else if os.Args[1] == "write" {
		file, _ := os.Open("tmp_data.rss")
		defer file.Close()
		fp := gofeed.NewParser()
		feed, _ := fp.Parse(file)

		events := []Event{}
		for _, item := range feed.Items {
			events = append(events, NewEvent(item))
		}

		generateMarkdown(os.Stdout, events)
	} else {
		fmt.Println("Usage: toddlerevents [download|write]")
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
