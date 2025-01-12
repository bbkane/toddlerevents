# toddlerevents

Download and write local Bay Area toddler events to a README. This is meant for personal use, so I've hardcoded things instead of making them configurable.

## Use

```bash
toddlerevents_LOG_LEVEL=INFO toddlerevents_README_PATH=tmp.md [download|write]
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
- Build with [goreleaser](https://goreleaser.com/) after cloning: `goreleaser --snapshot --skip-publish --clean`

## Notes

See [Go Developer Tooling](https://www.bbkane.com/blog/go-developer-tooling/) for notes on development tooling.
