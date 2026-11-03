# cable

A terminal HTTP client, usable as a TUI and a CLI, whose request collections
live as plain YAML files in a Git repository.

- Recipes, folders and profiles in a reviewable `cable.yaml`.
- Secrets declared in the collection but valued elsewhere: a secrets file,
  environment variables, a command, the OS keyring or a prompt at send time.
- Quick edits of a recipe as a draft before sending, written back only on
  request, with a diff.
- Pagination and for-each runs over JSON responses.
- The look and shortcut grammar of [portfolio-cli](https://github.com/adejoux/portfolio-cli),
  through the shared [tuikit](https://github.com/adejoux/tuikit) module.

## Status

Early development: only `cable version` works so far.

## Development

```sh
make build   # bin/cable
make test    # go test -race ./...
make lint    # gofmt, go vet, staticcheck
```
