# Change Log

## v0.1.3

> This release fixes a bug preventing its use in Go 1.13 when vendoring is enabled. It also updates several first/third-party dependencies.

- fix
  - "Unable to find X in any go files under Y" errors when `GOFLAGS=-mod=vendor` is set in Go 1.13.
- feat
  - --version now prints details about the build's paths and modules.
- notable dependency changes
  - Bump github.com/pkg/errors to v0.9.1.
  - Bump internal/cage/... to latest from monorepo.
- refactor
  - Migrate to latest cage/cli/handler API (e.g. handler.Session and handler.Input) and conventions (e.g. "func NewCommand").

## v0.1.2

- chore: re-export from private repo

## v0.1.1

- dep: update first-party dependencies under `internal`
- fix: lint
- refactor: migrate to `./cmd/<project name>` convention

## v0.1.0

- feat: initial project export from private monorepo
