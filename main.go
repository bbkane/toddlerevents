package main

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"text/template"
	"time"

	"go.bbkane.com/warg"
	"go.bbkane.com/warg/config/yamlreader"
	"go.bbkane.com/warg/path"
	"go.bbkane.com/warg/value/scalar"
	"go.bbkane.com/warg/value/slice"
)

var version string

func withInitGlobalLogger(f func(cmdCtx warg.CmdContext) error) warg.Action {
	return func(cmdCtx warg.CmdContext) error {
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

func renderTemplate(tmplStr string, data any) (string, error) {
	// Parse the template
	tmpl, err := template.New("template").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	// Render the template to a string
	var output bytes.Buffer
	err = tmpl.Execute(&output, data)
	if err != nil {
		return "", err
	}

	return output.String(), nil
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
			filePath, err := renderTemplate(
				args.filepathTemplate,
				map[string]any{
					"Code":   args.codes[i],
					"Number": j,
				},
			)
			if err != nil {
				return nil, fmt.Errorf("invalid filepathTemplate: %s: %w", args.filepathTemplate, err)
			}
			ret = append(ret, downloadFileArgs{
				url:      u,
				filePath: filePath,
				// filePath: fmt.Sprintf(args.filepathTemplate, args.codes[i], j),
			})
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("error parsing URLs: %w", errors.Join(errs...))
	}
	return ret, nil
}

func bibliocommonFlags() warg.FlagMap {
	return warg.FlagMap{
		"--bibliocommons-feed-url": warg.NewFlag(
			"Feed URL",
			slice.String(),
			warg.Required(),
			warg.ConfigPath("bibliocommons.feeds[].url"),
		),
		"--bibliocommons-feed-code": warg.NewFlag(
			"Unique Code for a feed",
			slice.String(),
			warg.Required(),
			warg.ConfigPath("bibliocommons.feeds[].code"),
		),
		"--bibliocommons-pages": warg.NewFlag(
			"Number of feed pages to download",
			scalar.Int(scalar.Default(5)),
			warg.Required(),
			warg.ConfigPath("bibliocommons.pages"),
		),
		"--bibliocommons-days": warg.NewFlag(
			"Number of days info to download",
			scalar.Int(scalar.Default(8)),
			warg.Required(),
			warg.ConfigPath("bibliocommons.days"),
		),
		"--bibliocommons-start-date": warg.NewFlag(
			"Date to start downloading",
			scalar.String(scalar.Default("today")),
			warg.Required(),
			warg.ConfigPath("bibliocommons.date"),
		),
		"--bibliocommons-filepath-template": warg.NewFlag(
			"Filepath template to save downloaded files to. `Code` is the string for each URL. `Number` is 1 to the number of pates",
			scalar.String(scalar.Default("tmp_rss_{{ .Code }}_{{ .Number }}.rss")),
			warg.Required(),
			warg.ConfigPath("bibliocommons.filepath_template"),
		),
	}
}

func withDownloadFileArgs(
	f func(cmdCtx warg.CmdContext, ds []downloadFileArgs) error,
) warg.Action {
	return func(cmdCtx warg.CmdContext) error {
		urls := cmdCtx.Flags["--bibliocommons-feed-url"].([]string)
		codes := cmdCtx.Flags["--bibliocommons-feed-code"].([]string)

		if len(urls) != len(codes) {
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

func buildApp() warg.App {
	app := warg.New(
		"toddlerevents",
		version,
		warg.NewSection(
			"Collate toddler events to take my kid to",
			warg.SubCmd("download", downloadCmd()),
			warg.SubCmd("write", writeCmd()),
		),
		warg.ConfigFlag(
			yamlreader.New,
			warg.FlagMap{
				"--config": warg.NewFlag(
					"Config filepath",
					scalar.Path(scalar.Default(path.New("toddlerevents.yaml"))),
					warg.Alias("-c"),
				),
			},
		),
		warg.NewGlobalFlag(
			"--log-level",
			"log level",
			scalar.String(
				scalar.Choices("DEBUG", "INFO", "WARN", "ERROR"),
				scalar.Default("DEBUG"),
			),
			warg.ConfigPath("log.level"),
			warg.Required(),
			warg.EnvVars("toddlerevents_LOG_LEVEL"),
		),
	)
	return app
}

func main() {
	app := buildApp()
	app.MustRun()
}
