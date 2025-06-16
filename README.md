# toddlerevents

Find Bay Area toddler events (currently only from public libraries) to attend with my son that are outside of work hours. Updates https://github.com/bbkane/toddlerevents.bbkane.com each Friday and Monday.

## Project Status (2025-01-24)

This is "MVP" status... everythings hardcoded, there are bugs (see [issues]([url](https://github.com/bbkane/toddlerevents/issues))), and there are no tests. I plan to make it more useful to myself and others "at some point". I'm watching issues; please open one for any questions and especially BEFORE submitting a Pull request.

## Use

Make a config file:

```yaml
bibliocommons:
  days: 2
  feeds:
    - code: smcl
      url: https://gateway.bibliocommons.com/v2/libraries/smcl/rss/events?audiences=564274cf4d0090f742000016%2C564274cf4d0090f742000011&startDate=2025-01-10&endDate=2025-01-13
  filepath_template: tmp_rss_{{ .Code }}_{{ .Number }}.rss
  pages: 2
  start_date: today
log:
  level: DEBUG
```

```bash
toddlerevents download|write --config toddlerevents.yaml
```

## Install

- [Homebrew](https://brew.sh/): `brew install bbkane/tap/toddlerevents`
- [Scoop](https://scoop.sh/):

```
scoop bucket add bbkane https://github.com/bbkane/scoop-bucket
scoop install bbkane/toddlerevents
```

- Download Mac/Linux/Windows executable: [GitHub releases](https://github.com/bbkane/toddlerevents/releases)
- Go: `go install go.bbkane.com/toddlerevents@latest`
- Build with [goreleaser](https://goreleaser.com/) after cloning: `goreleaser --snapshot --clean`

## Notes

See [Go Developer Tooling](https://www.bbkane.com/blog/go-developer-tooling/) for notes on development tooling.
