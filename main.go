package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/mmcdole/gofeed"
	"go.bbkane.com/warg"
	"go.bbkane.com/warg/command"
	"go.bbkane.com/warg/config/yamlreader"
	"go.bbkane.com/warg/flag"
	"go.bbkane.com/warg/path"
	"go.bbkane.com/warg/section"
	"go.bbkane.com/warg/value/scalar"
	"go.bbkane.com/warg/value/slice"
)

var version string

type downloadFileArgs struct {
	url      string
	filePath string
}

// Turn a hardcoded list of feeds into a list of multiple pages for the correct dates
func generateDownloadFileArgs(now time.Time) []downloadFileArgs {

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

	end := now.AddDate(0, 0, 10)
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

func withInitGlobalLogger(f func(cmdCtx command.Context) error) command.Action {
	return func(cmdCtx command.Context) error {
		logLevel := cmdCtx.Flags["--log-level"].(string)
		slogLevel := map[string]slog.Level{
			"DEBUG": slog.LevelDebug,
			"INFO":  slog.LevelInfo,
			"WARN":  slog.LevelWarn,
			"ERROR": slog.LevelError,
		}[logLevel]

		slog.SetDefault(
			slog.New(
				slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level:       slogLevel,
					AddSource:   false,
					ReplaceAttr: nil,
				}),
			),
		)
		return f(cmdCtx)
	}
}

type buildDownloadFileArgsArgs struct {
	urls             []string
	codes            []string
	pages            int
	startDateTime    time.Time
	filepathTemplate string
	days             int
}

func buildDownloadFileArgs(args buildDownloadFileArgsArgs) ([]downloadFileArgs, error) {
	lenURLs := len(args.urls)
	end := args.startDateTime.AddDate(0, 0, args.days)
	var ret []downloadFileArgs
	errs := []error{}
	for i := range lenURLs {
		parsedURL, err := url.Parse(args.urls[i])
		if err != nil {
			errs = append(errs, err)
		}
		q := parsedURL.Query()
		q.Set("startDate", args.startDateTime.Format("2006-01-02"))
		q.Set("endDate", end.Format("2006-01-02"))
		for j := 1; j <= args.pages; j++ {
			q.Set("page", strconv.Itoa(j))
			parsedURL.RawQuery = q.Encode()
			u := parsedURL.String()
			ret = append(ret, downloadFileArgs{
				url:      u,
				filePath: fmt.Sprintf(args.filepathTemplate, args.codes[i], j),
			})
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("error parsing URLs: %w", errors.Join(errs...))
	}
	return ret, nil
}

func bibliocommonFlags() flag.FlagMap {
	return flag.FlagMap{
		"--bibliocommons-feed-url": flag.New(
			"Feed URL",
			slice.String(),
			flag.Required(),
			flag.ConfigPath("bibliocommons.feeds[].url"),
		),
		"--bibliocommons-feed-code": flag.New(
			"Unique Code for a feed",
			slice.String(),
			flag.Required(),
			flag.ConfigPath("bibliocommons.feeds[].code"),
		),
		"--bibliocommons-pages": flag.New(
			"Number of feed pages to download",
			scalar.Int(scalar.Default(5)),
			flag.Required(),
			flag.ConfigPath("bibliocommons.pages"),
		),
		"--bibliocommons-days": flag.New(
			"Number of days info to download",
			scalar.Int(scalar.Default(8)),
			flag.Required(),
			flag.ConfigPath("bibliocommons.days"),
		),
		"--bibliocommons-start-date": flag.New(
			"Date to start downloading",
			scalar.String(scalar.Default("today")),
			flag.Required(),
			flag.ConfigPath("bibliocommons.date"),
		),
		"--bibliocommons-filepath-template": flag.New(
			"Filepath template to save downloaded files to",
			scalar.String(scalar.Default("tmp_rss_%s_%d.rss")),
			flag.Required(),
			flag.ConfigPath("bibliocommons.filepath_template"),
		),
	}
}

func withDownloadFileArgs(
	f func(cmdCtx command.Context, ds []downloadFileArgs) error,
) command.Action {
	return func(cmdCtx command.Context) error {
		urls := cmdCtx.Flags["--bibliocommons-feed-url"].([]string)
		codes := cmdCtx.Flags["--bibliocommons-feed-code"].([]string)

		if !(len(urls) == len(codes)) {
			slog.Error(
				"The following lengths should be equal",
				"--bibliocommons-feed-url", len(urls),
				"--bibliocommons-feed-code", len(codes),
			)
			return errors.New("non-matching flag lengths")
		}
		pages := cmdCtx.Flags["--bibliocommons-pages"].(int)
		startDate := cmdCtx.Flags["--bibliocommons-start-date"].(string)
		filepathTemplate := cmdCtx.Flags["--bibliocommons-filepath-template"].(string)
		days := cmdCtx.Flags["--bibliocommons-days"].(int)
		var startDateTime time.Time
		if startDate == "today" {
			startDateTime = time.Now()
		} else {
			var err error
			startDateTime, err = time.Parse("2006-01-02", startDate)
			if err != nil {
				return fmt.Errorf("could not parse --bibliocommons-start-date (%s) as a date: %w", startDate, err)
			}
		}

		args, err := buildDownloadFileArgs(buildDownloadFileArgsArgs{
			urls:             urls,
			codes:            codes,
			pages:            pages,
			startDateTime:    startDateTime,
			filepathTemplate: filepathTemplate,
			days:             days,
		})
		if err != nil {
			return fmt.Errorf("could not build downloadFileArgs: %w", err)
		}
		return f(cmdCtx, args)
	}
}

func main() {
	main2()
	panic("hi")
	// TODO:
	//   - move "write" to subcommand
	//   - make yaml config and put in toddlerevents.bbkane.com repo
	//   - push a new version
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

func writeCmd() command.Command {
	return command.New(
		"Write markdown file",
		command.DoNothing,
	)
}

func main2() {
	app := warg.New(
		"toddlerevents",
		version,
		section.New(
			"Collate toddler events to take my kid to",
			section.Command("download", downloadCmd()),
			section.Command("write", writeCmd()),
			section.CommandMap(warg.VersionCommandMap()),
		),
		warg.ConfigFlag(
			"--config",
			[]scalar.ScalarOpt[path.Path]{
				scalar.Default(path.New("toddlerevents.yaml")),
			},
			yamlreader.New,
			"Config filepath",
			flag.Alias("-c"),
		),
		warg.GlobalFlagMap(warg.ColorFlagMap()),
		warg.NewGlobalFlag(
			"--log-level",
			"log level",
			scalar.String(
				scalar.Choices("DEBUG", "INFO", "WARN", "ERROR"),
				scalar.Default("DEBUG"),
			),
			flag.ConfigPath("log.level"),
			flag.Required(),
			flag.EnvVars("toddlerevents_LOG_LEVEL"),
		),
	)
	app.MustRun()
}
