# Saltpeter Go Wrapper - Implementation Summary

## Status: Ready for Testing

All core functionality has been ported from Python to Go.

## Files Created

### Core Implementation
- `main.go` - Entry point and daemonization
- `daemon.go` - Re-exec based daemonization (no fork issues)
- `config.go` - Environment variable parsing
- `logger.go` - File logging
- `runner.go` - Job execution and process management
- `websocket.go` - WebSocket client with reconnection
- `messages.go` - Message sending (connect, start, output, complete, heartbeat)
- `lockfile.go` - Overlap prevention

### Build Configuration
- `go.mod` - Go module definition
- `Makefile` - Build targets
- `.github/workflows/build.yml` - CI/CD pipeline

### Documentation
- `README.md` - Usage and compatibility guide

## Feature Parity with Python Wrapper

| Feature | Python | Go | Notes |
|---------|--------|-----|-------|
| Daemonization | ✅ (double-fork) | ✅ (re-exec) | Go uses re-exec to avoid fork issues |
| Process groups | ✅ | ✅ | Uses Setsid for new session |
| WebSocket streaming | ✅ | ✅ | gorilla/websocket library |
| Output buffering | ✅ | ✅ | Configurable interval |
| Sequence/ACK tracking | ✅ | ✅ | Identical logic |
| Reconnection | ✅ | ✅ | 2s retry interval |
| Heartbeats | ✅ | ✅ | 5s interval |
| Kill command | ✅ | ✅ | SIGTERM → SIGKILL escalation |
| Timeout enforcement | ✅ | ✅ | Same termination logic |
| Lockfiles | ✅ | ✅ | PID-based overlap detection |
| Logging | ✅ | ✅ | File-based with levels |
| FIPS compatibility | ✅ (patched) | ✅ (native) | Go handles SHA1 correctly |

## Advantages Over Python

### No Dependencies
- Python: Requires websockets, pyinstaller, python runtime
- Go: Zero dependencies - fully static binary

### Binary Size
- Python + PyInstaller: 20-30MB
- Go static: ~5-10MB

### Startup Time
- Python: 100-200ms (interpreter + extraction)
- Go: <10ms (direct execution)

### Compatibility
- Python: glibc version issues, ncurses race conditions
- Go: Works on any Linux kernel 2.6.23+ (2007)

### Reliability
- Python: Intermittent extraction race conditions
- Go: No extraction - single file execution

## Testing Plan

1. **Build Test** (GitHub Actions will handle)
   ```bash
   CGO_ENABLED=0 go build -ldflags="-w -s -extldflags '-static'" -o sp_wrapper .
   ```

2. **Static Binary Verification**
   ```bash
   ldd sp_wrapper  # Should show "not a dynamic executable"
   file sp_wrapper # Should show "statically linked"
   ```

3. **Old System Compatibility**
   ```bash
   docker run --rm -v $PWD:/w ubuntu:12.04 /w/sp_wrapper
   ```

4. **Functional Testing**
   - Test with simple command
   - Test with ncurses output (mysql)
   - Test timeout enforcement
   - Test kill command
   - Test overlap prevention
   - Test on target production systems

## Migration Strategy

### Phase 1: Parallel Deployment
- Build Go wrapper via GitHub Actions
- Deploy alongside Python wrapper
- Test on subset of jobs
- Monitor for issues

### Phase 2: Gradual Rollout
- Migrate non-critical jobs to Go wrapper
- Validate output, timeouts, kills work correctly
- Monitor logs for any edge cases

### Phase 3: Full Migration
- Switch all jobs to Go wrapper
- Remove Python wrapper from build pipeline
- Archive Python implementation for reference

## Known Differences

### Minor Behavioral Changes
1. **Daemonization**: Re-exec instead of fork (functionally equivalent)
2. **Error messages**: Slightly different wording (same meaning)
3. **Log format**: Identical structure, may have minor spacing differences

### Binary Compatibility
- Go binary named `sp_wrapper` (same as Python)
- Same command-line interface (no args, all env vars)
- Same exit codes and signals
- Same file paths and permissions

## Rollback Plan

If issues are discovered:
1. Python wrapper still exists in `saltpeter/wrapper.py`
2. Can rebuild Python version from `build-wrapper.yml`
3. Salt state files can point to either binary
4. No data/config changes needed

## Next Steps

1. Push to GitHub → triggers build workflow
2. Download artifact from GitHub Actions
3. Test locally with simple job
4. Deploy to dev environment
5. Test with production workloads
6. Gradual rollout

## Questions/Concerns

- Should we keep Python wrapper as fallback for a grace period?
- Any specific edge cases in production to test?
- Preferred timeline for migration?
