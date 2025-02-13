package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
	"go.bbkane.com/warg/command"
	"go.bbkane.com/warg/flag"
	"go.bbkane.com/warg/path"
	"go.bbkane.com/warg/value/scalar"
)

type Event struct {
	Title          string
	Description    string
	Link           string
	StartTimeLocal time.Time
	EndTimeLocal   time.Time
	City           string
	ParseErrors    []error
	GUID           string
}

func filter(event Event) bool {

	// party on the weekends
	if event.StartTimeLocal.Weekday() == time.Saturday || event.StartTimeLocal.Weekday() == time.Sunday {
		return true
	}

	// party after work
	return event.StartTimeLocal.Hour() >= 17

}

func parseTime(bc map[string][]ext.Extension, key string) (time.Time, error) {

	// TODO: use start_date instead of start_date_local to avoid errors like the following:
	// time=2025-01-11T20:17:44.103-08:00 level=ERROR msg="parse error" title="Closure: Martin Luther King, Jr. Day" city="" err="Could not parse end_date: Could not parse \"end_date_local\": \"2025-01-20\": parsing time \"2025-01-20\" as \"2006-01-02T15:04\": cannot parse \"\" as \"T\""

	// TODO: just use "all day" for these

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
		return time.Time{}, fmt.Errorf("Could not parse %#v: %#v: %w", key, date, err)
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
		GUID:           item.GUID,
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

func writeCmd() command.Command {
	return command.New(
		"Write markdown file",
		withInitGlobalLogger(withDownloadFileArgs(writeRun)),
		command.FlagMap(bibliocommonFlags()),
		command.NewFlag(
			"--readme-path",
			"Path to output README",
			scalar.Path(
				scalar.Default(path.New("tmp.md")),
			),
			flag.ConfigPath("write.readme_path"),
			flag.EnvVars("toddlerevents_README_PATH"),
			flag.Required(),
		),
	)
}

func writeRun(cmdCxt command.Context, ds []downloadFileArgs) error {
	readmePath := cmdCxt.Flags["--readme-path"].(path.Path).MustExpand()

	events := []Event{}
	seenEvent := make(map[string]bool)
	for _, d := range ds {
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

			// These are non-fatal errors
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
	slog.Info("Wrote README!", "readmePath", readmePath)
	return nil
}
