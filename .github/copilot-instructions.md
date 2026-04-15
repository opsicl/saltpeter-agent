# Copilot Instructions

## Project Overview

Saltpeter Agent is a static Go binary that wraps cron job execution for the Saltpeter monitoring system. It replaces a previous Python/PyInstaller wrapper. Salt invokes the wrapper, which immediately re-execs itself as a daemon (detaching from Salt), then runs the target command as a subprocess while streaming output to a server over WebSocket.

## Build & Run

```bash
make build          # Static Linux amd64 binary → ./sp_wrapper
make build-local    # Build for current platform
make test           # go test -v ./...
make deps           # go mod download && go mod tidy
```

Manual static build: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s -extldflags '-static'" -o sp_wrapper .`

All configuration is via environment variables (no CLI flags). See `config.go` for the full list — the four required ones are `SP_WEBSOCKET_URL`, `SP_JOB_NAME`, `SP_JOB_INSTANCE`, `SP_COMMAND`.

## Architecture

```
Salt → sp_wrapper (re-execs as daemon via _SP_DAEMON=1) → subprocess (in new process group)
                      ↕
                 WebSocket server (bidirectional: output streaming + kill commands)
```

### Execution flow

1. **main.go** — Entry point. On first invocation, validates config early (so errors reach Salt), then calls `daemonize()`. On second invocation (`_SP_DAEMON=1` set), calls `runWrapper()` which creates a `JobRunner` and calls `Run()`.
2. **daemon.go** — Re-exec based daemonization. Creates a new session (`Setsid`), redirects stdio to `/dev/null`. No fork — avoids Go runtime issues.
3. **runner.go** — Core orchestration. Starts the WebSocket client goroutine, spawns the subprocess in a new process group, runs two `readOutput()` goroutines (stdout + stderr), then enters a `select{}` event loop handling: process exit, output flush ticks, heartbeat ticks, and timeout checks.
4. **websocket.go** — WebSocket client with automatic reconnection (2s retry). Runs in its own goroutine. Incoming messages: `ack`, `nack`, `sync_response`, `kill`. The kill handler calls back into the runner to terminate the subprocess.
5. **messages.go** — Message construction and sending. Message types: `connect`, `start`, `output`, `complete`, `heartbeat`. Output messages are sequence-numbered with ACK tracking. Complete message has a dedicated retry loop (30 attempts × 2s).
6. **lockfile.go** — PID-based overlap prevention when `SP_ALLOW_OVERLAP=false`.

### Concurrency model

Three mutexes protect shared state:
- `outputMutex` — guards the output line buffer (written by readOutput goroutines, read by flushOutput)
- `msgMutex` — guards pending message queue, sequence counters, and ACK state
- `killMutex` — guards killed/killedByTimeout flags

Key channels: `processDone` (subprocess exit code). Tickers: output flush (configurable, default 1s), heartbeat (5s).

## Conventions

- **No CLI flags** — all config comes from `SP_*` environment variables, parsed in `config.go`
- **Version** — set in `main.go` as `const Version`. Follow semver with suffixes (`-beta.N`, `-dev`). CI extracts this for artifact naming.
- **Error wrapping** — use `fmt.Errorf("context: %w", err)`
- **Logging** — file-based only (not stdout). Format: `YYYY-MM-DD HH:MM:SS [instance_id] message`. Logger initialized in `runWrapper()`, use `logger.Log()` / `logger.Debug()`.
- **Static binary** — `CGO_ENABLED=0` is mandatory. The binary must run on any Linux kernel 2.6.23+ with no dynamic dependencies.
- **Process termination** — always terminate the entire process group, not just the child PID. Escalation: SIGTERM → wait → SIGKILL.
- **Lockfile-based overlap prevention** — uses `syscall.Flock(LOCK_EX|LOCK_NB)` on a per-job file (`/tmp/sp_wrapper_<jobname>.lock`). The flock is held for the lifetime of the daemon process and auto-released by the OS on exit/crash. PID is written to the file for admin observability only — the lock mechanism is flock, not PID-checking.
- **Flat package structure** — everything is in `package main`, no subdirectories. Keep it that way unless there's a strong reason to split.

## CI/CD

GitHub Actions workflow at `.github/workflows/build.yml`:
- Triggers on push to main/master/dev and workflow_dispatch
- Builds static binaries for amd64, arm64, 386
- Runs compatibility test on Ubuntu 12.04 Docker image
- Artifacts named `sp_wrapper` (main) or `sp_wrapper-<branch>` (other branches), retained 90 days
