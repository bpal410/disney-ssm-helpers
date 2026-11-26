# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This project is a fork of [disneystreaming/ssm-helpers](https://github.com/disneystreaming/ssm-helpers)
at v1.2.0. Version numbering restarted at 0.0.1 for this fork. For changes
before the fork, see the upstream repository's history.

## [0.0.2] - Unreleased

### Added

- `ssm session` selection prompt always shows a **Name** column (the
  instance's `Name` tag) after the instance ID. Passing `-t Name` does not
  duplicate it.
- `--sort-tag` flag for `ssm session` (default `Name`) sorts the selection
  prompt by the given tag, case-insensitively, with instances missing the tag
  listed last and instance ID breaking ties. A sort tag that is not already
  displayed is added as a column. `--sort-tag ""` sorts by instance ID.

### Changed

- Go module path is now `github.com/bpal410/ssm-helpers`; install with
  `go install github.com/bpal410/ssm-helpers@latest`.
- Docker image is published as `ghcr.io/bpal410/ssm`.
- `.deb`/`.rpm` packages list the license as Apache-2.0 (was incorrectly MIT)
  and bpal410 as maintainer.
- tmux sessions are managed by the new in-repo `internal/tmux` package,
  replacing the external gomux dependency.
- A single instance found by searching now goes to the selection prompt
  instead of connecting automatically. A single instance given with
  `-i`/`--instance` still connects directly.
- Panes in a multi-instance tmux session appear in the order the instances
  were selected.
- Instances in the selection prompt are listed in a stable, sorted order
  instead of a random one.

### Fixed

- Selecting exactly one instance at the prompt no longer opens a second
  session to it in tmux after the first session ends.
- Panes are targeted by tmux ID, so user `base-index`/`pane-base-index`
  settings or running from inside another tmux session no longer send
  commands to the wrong pane. The empty starting pane is now also removed
  when running inside tmux.
- Replacing a tmux session matches the session name exactly (`ssm-1` no
  longer matches `ssm-10`).
- Pane setup waits for each tmux command to finish, and tmux errors are
  reported with tmux's own message.
- Commands sent to tmux panes are typed exactly as given (quotes are no
  longer escaped).
- Formatting the selection prompt no longer modifies instance tags, and no
  longer panics for an instance without tags.

### Removed

- Homebrew installation instructions (no tap is published).
- References to the archived go-ssm-helpers library.

## [0.0.1]

First release of the fork.

### Changed

- Go toolchain updated: `go` directive 1.26, CI builds with Go 1.27.
- Release pipeline migrated to goreleaser v2, adding arm64 builds and
  publishing GitHub Releases and a multi-arch image to
  `ghcr.io/<owner>/ssm`.
- Docker image rebuilt on `debian:trixie-slim` with AWS CLI v2 and the
  architecture-specific Session Manager plugin.
- Devcontainer updated to a current Go image, with the AWS CLI, tmux and the
  Session Manager plugin installed.
- Makefile tool installs use `go install ...@latest`, fixing `make build`
  and `make format.check` on current Go.

### Removed

- Publishing to the upstream organization's registries and Homebrew tap.

[0.0.2]: https://github.com/bpal410/ssm-helpers/compare/v0.0.1...HEAD
[0.0.1]: https://github.com/bpal410/ssm-helpers/releases/tag/v0.0.1
