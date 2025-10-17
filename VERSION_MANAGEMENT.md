# ChaosBlade Exec OS Version Management System

This document describes the version management system for the ChaosBlade Exec OS project, including the complete workflow from Git Tag to CI build-time version injection.

## 🎯 System Overview

The version management system implements the following features:

1. **Automatic Version Detection**: Automatically retrieves version information from Git Tags or Git descriptions
2. **Build-time Version Injection**: Injects version information into binary files during compilation via ldflags
3. **Multi-platform Support**: Supports Linux/AMD64, Linux/ARM64, Darwin/AMD64, Darwin/ARM64
4. **CI/CD Integration**: GitHub Actions automatic build and version management
5. **Version Verification Tools**: Provides tools for viewing and verifying version information

## 🏗️ Architecture Design

### Version Information Structure

```go
type VersionInfo struct {
    Version      string    `json:"version"`      // Main version number
    GitCommit    string    `json:"git_commit"`   // Git commit hash
    BuildTime    time.Time `json:"build_time"`   // Build time
    GoVersion    string    `json:"go_version"`   // Go version
    Platform     string    `json:"platform"`     // Target platform
    Architecture string    `json:"architecture"` // Target architecture
}
```

### Build Workflow

```
Git Tag → CI Detection → Version Extraction → Build-time Injection → Compiled Artifacts Contain Version
    ↓
Version information is injected into binary files via ldflags
```

## 🚀 Usage Guide

### 1. Local Build

#### View Version Information
```bash
make version
```

#### Build Current Platform Version
```bash
make build
```

#### Build Specific Platform Version
```bash
make linux_amd64
make darwin_arm64
make linux_arm64
make darwin_amd64
```

#### Build All Platform Versions
```bash
make build_all
```

### 2. Version Tools

#### Build Version Tool
```bash
make version_tool
```

#### Test Version Injection
```bash
make test_version_injection
```

#### Use Version Tool Directly
```bash
# Display complete version information
./bin/version

# JSON format output
./bin/version -json

# Display version number only
./bin/version -short

# Display complete version string
./bin/version -full
```

### 3. Version Verification

#### Verify Version Injection
```bash
make verify_version
```

#### Check Version Information in Binary Files
```bash
strings target/chaosblade-1.7.4-darwin_arm64/bin/chaos_os | grep -E "(chaosblade-exec-os|version|git)"
```

## 🏷️ Git Tag Management

### Create Version Tags

```bash
# Create new version tag
git tag -a v1.7.4 -m "Release version 1.7.4"

# Push tag to remote repository
git push origin v1.7.4
```

### Version Naming Convention

- Use semantic versioning: `v1.7.4`
- Pre-release versions: `v1.7.4-rc1`
- Development versions: `v1.7.4-dev`

## 🔧 CI/CD Configuration

### GitHub Actions Trigger Conditions

- **Branch Push**: `main` or `master` branch
- **Tag Push**: Tags in `v*` format
- **Pull Request**: Against `main` or `master` branch

### Build Matrix

| Platform | Operating System | Architecture | Directory Name |
|----------|------------------|--------------|----------------|
| Linux AMD64 | Ubuntu Latest | amd64 | `chaosblade-{version}-linux_amd64` |
| Linux ARM64 | Ubuntu Latest | arm64 | `chaosblade-{version}-linux_arm64` |
| Darwin AMD64 | macOS Latest | amd64 | `chaosblade-{version}-darwin_amd64` |
| Darwin ARM64 | macOS Latest | arm64 | `chaosblade-{version}-darwin_arm64` |

### Version Information Retrieval

The CI system automatically:

1. Detects Git Tags or uses Git descriptions
2. Retrieves Git commit hashes and build times
3. Injects version information via ldflags
4. Generates directory structures with version information
5. Uploads build artifacts (release versions only)

## 📁 Output Directory Structure

```
target/
├── chaosblade-1.7.4-linux_amd64/
│   ├── bin/
│   │   ├── chaos_os          # Binary file containing version information
│   │   └── strace
│   └── yaml/
│       └── chaosblade-os-spec-1.7.4.yaml
├── chaosblade-1.7.4-linux_arm64/
│   ├── bin/
│   │   ├── chaos_os
│   │   └── strace
│   └── yaml/
│       └── chaosblade-os-spec-1.7.4.yaml
└── ...
```

## 🔍 Version Information Verification

### 1. Runtime Verification

```bash
# Run version tool
./bin/version

# Output example:
# ChaosBlade Exec OS
# ==================
# Version:     1.7.4
# Git Commit:  a1b2c3d4
# Build Time:  2024-01-01 12:00:00
# Go Version:  go1.20
# Platform:    darwin
# Architecture: arm64
# Is Release:  true
```

### 2. Binary File Verification

```bash
# Check version information in binary files
strings target/chaosblade-1.7.4-darwin_arm64/bin/chaos_os | grep version
```

### 3. JSON Format Verification

```bash
./bin/version -json

# Output example:
# {
#   "version": "1.7.4",
#   "git_commit": "a1b2c3d4e5f6g7h8i9j0",
#   "build_time": "2024-01-01T12:00:00Z",
#   "go_version": "go1.20",
#   "platform": "darwin",
#   "architecture": "arm64"
# }
```

## 🛠️ Troubleshooting

### Common Issues

1. **Version information shows as "dev"**
   - Check if Git tags exist
   - Confirm commands are run in Git repository

2. **Version injection fails**
   - Check if Go module path is correct
   - Confirm version package import path

3. **CI build fails**
   - Check Git tag format
   - Confirm GitHub Actions permission settings

### Debug Commands

```bash
# Check Git status
git status
git describe --tags --always --dirty

# Check version information
make version

# Verify version injection
make verify_version

# Test version tool
make test_version_injection
```

## 📚 Related Files

- `version/version.go` - Version management core code
- `cmd/version/main.go` - Version information display tool
- `Makefile` - Build and version management script
- `.github/workflows/ci.yml` - CI/CD configuration
- `VERSION_MANAGEMENT.md` - This document

## 🤝 Contributing Guidelines

1. Follow semantic versioning conventions
2. Add detailed commit messages when creating Git tags
3. Test version injection functionality
4. Update related documentation

## 📄 License

This project is licensed under the Apache License 2.0.
