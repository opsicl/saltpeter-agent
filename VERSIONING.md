# Versioning Guide

## Where to Set Version

Edit `main.go`, line 9:

```go
const Version = "1.0.0-beta.1"
```

## Version Format

Use semantic versioning: `MAJOR.MINOR.PATCH[-SUFFIX]`

Examples:
- `1.0.0` - Production release
- `1.0.0-beta.1` - Beta release
- `1.0.0-dev` - Development version
- `1.1.0` - Minor feature update
- `2.0.0` - Breaking changes

## Branch-Based Artifacts

GitHub Actions automatically prefixes artifacts based on branch:

| Branch | Version in Code | Artifact Name | Full Version |
|--------|----------------|---------------|--------------|
| `main` | `1.0.0` | `sp_wrapper` | `1.0.0` |
| `dev` | `1.0.0-beta.1` | `sp_wrapper-dev` | `dev-1.0.0-beta.1` |
| `feature-x` | `1.0.0-alpha` | `sp_wrapper-feature-x` | `feature-x-1.0.0-alpha` |

## Release Process

### Development
1. Work on `dev` branch
2. Set version to `X.Y.Z-beta.N` or `X.Y.Z-dev`
3. Push to `dev` branch
4. Download `sp_wrapper-dev` artifact
5. Test on dev systems

### Production Release
1. Merge `dev` to `main`
2. Update version to `X.Y.Z` (remove suffix)
3. Push to `main`
4. Download `sp_wrapper` artifact (no prefix)
5. Deploy to production

## Checking Version

```bash
# The wrapper prints version on startup
./sp_wrapper
# Output: Saltpeter Wrapper version 1.0.0-beta.1
```

Or check VERSION.txt in artifact:
```bash
cat VERSION.txt
# Shows: Version: dev-1.0.0-beta.1
```
