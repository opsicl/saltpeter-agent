# Saltpeter Agent (Go Wrapper)

Static Go binary wrapper for Saltpeter cron job monitoring system.

## Why Go?

Replaces Python + PyInstaller wrapper to eliminate:
- ❌ ncurses library race conditions
- ❌ glibc version compatibility issues  
- ❌ Large binary sizes (20-30MB)
- ❌ Slow startup times

Benefits:
- ✅ Single static binary (~5-10MB)
- ✅ Works on any Linux (kernel 2.6.23+)
- ✅ No dependencies whatsoever
- ✅ Fast startup (<10ms)
- ✅ No extraction race conditions

## Building

```bash
# Build static binary
make build

# Or manually:
CGO_ENABLED=0 go build -ldflags="-w -s -extldflags '-static'" -o sp_wrapper .
```

## Usage

Same environment variables as Python wrapper:

```bash
export SP_WEBSOCKET_URL="ws://server:8000/ws"
export SP_JOB_NAME="my-job"
export SP_JOB_INSTANCE="unique-id"
export SP_COMMAND="your command here"

./sp_wrapper
```

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SP_WEBSOCKET_URL` | Yes | - | WebSocket server URL |
| `SP_JOB_NAME` | Yes | - | Job name |
| `SP_JOB_INSTANCE` | Yes | - | Unique instance ID |
| `SP_COMMAND` | Yes | - | Command to execute |
| `SP_MACHINE_ID` | No | hostname | Machine identifier |
| `SP_CWD` | No | - | Working directory |
| `SP_USER` | No | - | Run as user (requires root) |
| `SP_TIMEOUT` | No | 0 | Timeout in seconds |
| `SP_OUTPUT_INTERVAL_MS` | No | 1000 | Output buffer interval |
| `SP_WRAPPER_LOGLEVEL` | No | normal | Log level: normal/debug/off |
| `SP_WRAPPER_LOGDIR` | No | /var/log/sp_wrapper | Log directory |
| `SP_ALLOW_OVERLAP` | No | true | Allow concurrent job instances |
| `SP_LOCKFILE` | No | /tmp/sp_wrapper_{job}.lock | Lockfile path |

## Compatibility

Tested on:
- Ubuntu 12.04 through 24.04
- Debian 7 through 12
- RHEL/CentOS 6 through 9
- Alpine Linux
- Any Linux x86_64 with kernel 2.6.23+ (2007)

## Features

- ✅ Fire-and-forget Salt execution
- ✅ WebSocket streaming with reconnection
- ✅ Process group termination (SIGTERM → SIGKILL)
- ✅ Timeout enforcement
- ✅ Kill command support
- ✅ Output buffering and sequencing
- ✅ Message ACK tracking
- ✅ Overlap prevention with lockfiles
- ✅ Comprehensive logging

## Architecture

```
Salt → Wrapper (re-execs as daemon) → Subprocess (process group)
                ↓
           WebSocket Client (streaming output)
```

The wrapper:
1. Re-executes itself as daemon (detached from Salt)
2. Returns success to Salt immediately
3. Starts subprocess with new session (process group)
4. Streams output via WebSocket
5. Handles kill commands and timeouts
6. Terminates entire process group on exit

## Development

```bash
# Install dependencies
make deps

# Build for local platform
make build-local

# Run tests
make test

# Clean
make clean
```

## License

Same as saltpeter project

Saltpeter agent that runs on target machines to manage the state and output of the crons.
