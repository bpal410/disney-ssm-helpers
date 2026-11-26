# SSM Helpers

Helpers to manage you systems with [AWS Systems Manager](https://aws.amazon.com/systems-manager/) suite of management tools.

![](img/ssm-helpers.gif)

## Tools in this repo

* `ssm` subcommands:
    
    * [`session`](cmd/ssm-session/README.md) - Interactive shell with an instance via AWS Systems Manager Session Manager (`ssh` and `cssh` replacement)

    * [`run`](cmd/ssm-run/README.md)     - Run a command on multiple instances based on instance tags or names (`mco` and `knife` replacement)

If you would like more information about the available commands, see the README for each in `./cmd/<command-name>/`.

## Install

![goreleaser](https://github.com/bpal410/ssm-helpers/actions/workflows/release.yml/badge.svg)

### Docker

You can run the tools from docker containers

```bash
# ssm run
docker run -it --rm -v $HOME/.aws:/root/.aws \
    -e AWS_PROFILE=$AWS_PROFILE -e AWS_REGION=$AWS_REGION \
    ghcr.io/bpal410/ssm run

# ssm session (change detach keys for tmux)
docker run -it --rm --detach-keys 'ctrl-e,e' \
    -v $HOME/.aws:/root/.aws -e AWS_PROFILE=$AWS_PROFILE \
    -e AWS_REGION=$AWS_REGION \
    ghcr.io/bpal410/ssm session
```

### Manually

You can find tagged releases (archives, `.deb` and `.rpm` packages) for Windows, macOS, and Linux on the [releases page](https://github.com/bpal410/ssm-helpers/releases).

### Latest from main

To install the latest code from git (the binary is installed as `ssm-helpers`):

```
go install github.com/bpal410/ssm-helpers@latest
```

## Build

```
make build
```

### Testing

```
make test
```

### Linting

```
make check
```

## Develop

Each subcommand lives in the `cmd/<command-name>` folder and is written in go.  
They use the [aws-sdk-go](https://github.com/aws/aws-sdk-go); `ssm session` drives `tmux` directly through the small [internal/tmux](internal/tmux) package.  
If you find bugs or would like to suggest improvements please use [GitHub issues](https://github.com/bpal410/ssm-helpers/issues).
