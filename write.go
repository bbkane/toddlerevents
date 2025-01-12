package main

import (
	"errors"
	"fmt"
	"io"
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
