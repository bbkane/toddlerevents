package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"time"

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

type downloadFileArgs struct {
	url      string
	filePath string
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
