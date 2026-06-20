# Changelog

All notable changes to this project will be documented in this file. The format
is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

Note that I update this changelog as I make changes, so the top version (right
below this description) is likely unreleased.

# v0.0.5

## Changed

Update Homebrew installation, --help format

# v0.0.4

## Changed

Updated warg to v0.40.2, adding bash/fish completion and changes to default command help output

Changed from using:

```bash
--bibliocommons-feed-code <CODE> --bibliocommons-feed-url <URL>
```

To:

```bash
--bibliocommons-feed <CODE>,<URL>
```

To accommodate the warg (CLI parsing lib) update. The config format remains the same

# v0.0.3

## Added

Tab completion via warg update

# v0.0.2

## Changed

Switch from using env vars to warg

# v0.0.1

First release!
