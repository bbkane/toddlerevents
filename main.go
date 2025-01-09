package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
)

type Event struct {
	Title       string
	Descripton  string
	Link        string
	StartTime   time.Time
	EndTime     time.Time
	City        string
	ParseErrors []error
}

func parseTime(bc map[string][]ext.Extension) (time.Time, error) {

	startDateList, exists := bc["start_date"]
	if !exists {
		return time.Time{}, errors.New("could not find start_date")
	}
	if len(startDateList) != 1 {
		return time.Time{}, errors.New("not exactly one start_date")
	}
	date := startDateList[0].Value
	sd, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return time.Time{}, fmt.Errorf("Could not parse start_date: %w", err)
	}
	return sd, nil
}

func NewEvent(item *gofeed.Item) Event {
	event := Event{
		Title:       item.Title,
		Descripton:  item.Description,
		Link:        item.Link,
		StartTime:   time.Time{},
		EndTime:     time.Time{},
		City:        "",
		ParseErrors: nil,
	}
	bc, exists := item.Extensions["bc"]
	if !exists {
		event.ParseErrors = append(event.ParseErrors, errors.New("No \"bc\" extension"))
		// everything else relies on "bc" extension, so go ahead and return early if we can't find it
		return event
	}

	startTime, err := parseTime(bc)
	if err != nil {
		event.ParseErrors = append(event.ParseErrors, fmt.Errorf("could not parse start_date: %w", err))
	}
	event.StartTime = startTime

	endTime, err := parseTime(bc)
	if err != nil {
		event.ParseErrors = append(event.ParseErrors, fmt.Errorf("Could not parse end_date: %w", err))
	}
	event.EndTime = endTime

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

func main() {
	// curl 'https://gateway.bibliocommons.com/v2/libraries/smcl/rss/events?audiences=564274cf4d0090f742000016%2C564274cf4d0090f742000011&startDate=2025-01-10&endDate=2025-01-13' > tmp_data.rss
	file, _ := os.Open("tmp_data.rss")
	defer file.Close()
	fp := gofeed.NewParser()
	feed, _ := fp.Parse(file)

	events := []Event{}
	for _, item := range feed.Items {
		events = append(events, NewEvent(item))
	}

	fmt.Println(events)

}
