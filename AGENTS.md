# AGENTS

## Project overview
- Go-based Stream Deck plugin for Libre Hardware Monitor.
- Main plugin assets live in `com.moeilijk.lhm.sdPlugin`.
- Go entrypoints are in `cmd/` (plugin, bridge, debugger).

## Tooling
- Go toolchain is pinned in `go.mod` (`go1.24.12`).
- Builds are Windows targets by default (see `Makefile` `GOOS=windows`, `GOARCH=amd64`).

## Common commands
- Build plugin + bridge: `make plugin`
- Build debugger: `make debug`
- Package release (requires Stream Deck CLI): `make release`
- Regenerate protobufs (expects `.cache/protoc/bin/protoc`): `make proto`

## Local deploy (WSL)
- Use `scripts/deploy-local.sh` to copy the plugin into the Windows Stream Deck plugins directory and restart Stream Deck.
- Set `WIN_USER` if the script cannot resolve your Windows username.

## Tests
- No automated test suite is defined. Validate changes by building and (when possible) running the plugin in Stream Deck.

## Notes
- `make plugin` calls `scripts/bump-manifest-version.sh` and then `install-plugin.bat`; on non-Windows hosts you may want to skip/replace the install step.
