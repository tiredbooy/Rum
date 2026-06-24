# Linux Installer Improvement Roadmap

## Overview

The current Linux installer successfully builds and installs Rum from source, creates desktop entries, handles Wails dependencies, and supports uninstallation.

The next goal is to evolve it from a developer-oriented install script into a production-grade installer suitable for end users.

---

# Priority 1 — Core Reliability

## Release Binary Installation

### Description

Allow users to install prebuilt binaries directly from GitHub Releases instead of building from source.

### Benefits

- No Go installation required
- No Node.js installation required
- No npm installation required
- No Wails installation required
- Much faster installation
- Lower failure rate

### Expected Usage

```bash
./install-linux.sh
```

Installs latest release.

```bash
./install-linux.sh --build
```

Builds from source.

---

## Architecture Detection

### Description

Automatically detect the user's CPU architecture and install the correct binary.

### Supported Architectures

- x86_64 → amd64
- aarch64 → arm64
- armv7l → armv7

### Benefits

Users do not need to manually select downloads.

---

## Network Diagnostics

### Description

Perform connectivity checks before any downloads occur.

### Checks

- Internet connectivity
- DNS resolution
- GitHub accessibility
- Go proxy accessibility

### Example Output

```text
✓ Internet
✓ DNS
✓ GitHub
✓ Go Proxy
```

### Benefits

Provides actionable error messages before installation begins.

---

## Rollback Support

### Description

Automatically revert partial installations when an error occurs.

### Rollback Targets

- Installed binary
- Desktop entry
- Installed icon
- Completion files

### Benefits

Prevents broken or partially-installed systems.

---

## Installation Logging

### Description

Store installation logs for troubleshooting.

### Log Location

```text
~/.local/share/rum/install.log
```

### Information Logged

- OS information
- Architecture
- Commands executed
- Errors
- Installation timestamps

### Benefits

Simplifies support and bug reports.

---

# Priority 2 — User Experience

## Existing Installation Detection

### Description

Detect previously-installed versions before continuing.

### Example

```text
Rum 1.2.0 is already installed.

1. Upgrade
2. Reinstall
3. Remove
4. Exit
```

### Benefits

Avoids accidental overwrites.

---

## Version Checking

### Description

Compare installed version against the latest GitHub Release.

### Example

```text
Installed Version: 1.2.0
Latest Version: 1.3.0
```

### Benefits

Provides upgrade awareness.

---

## Root User Detection

### Description

Detect installer execution under sudo/root.

### Behavior

- Warn user
- Prevent installation into `/root/.local`
- Offer installation into `/usr/local`

### Benefits

Prevents common installation mistakes.

---

## PATH Auto-Fix

### Description

Detect whether the installation directory exists in the user's PATH.

### Supported Shells

- Bash
- Zsh

### Example

```text
~/.local/bin is not in PATH.

1. Update ~/.bashrc
2. Update ~/.zshrc
3. Update both
4. Skip
```

### Benefits

Allows immediate terminal access to Rum.

---

## Progress Indicators

### Description

Display progress indicators during long-running operations.

### Targets

- Downloads
- Wails installation
- Binary extraction
- Build process

### Benefits

Improves installer feedback and user confidence.

---

# Priority 3 — Security

## SHA256 Verification

### Description

Verify release downloads before extraction.

### Verification Files

```text
checksums.txt
checksums.txt.sha256
```

### Benefits

Protects against corrupted or tampered downloads.

---

## Secure Download Validation

### Description

Verify all required release assets exist before installation.

### Validation

- Binary archive exists
- Checksum file exists
- Download succeeded

### Benefits

Improves release reliability.

---

# Priority 4 — Linux Integration

## Enhanced Desktop Entry

### Description

Improve desktop launcher integration.

### Additions

```ini
StartupNotify=true
Keywords=download;manager;network;
```

### Benefits

Improves desktop search and launcher behavior.

---

## Shell Completions

### Description

Install command-line completions.

### Supported Shells

- Bash
- Zsh
- Fish

### Benefits

Improves CLI usability.

---

# Priority 5 — Maintenance Features

## Self Update Command

### Description

Allow Rum to update itself.

### Example

```bash
rum self-update
```

### Benefits

Reduces installer usage after initial installation.

---

## Offline Build Cache

### Description

Reuse previously downloaded dependencies.

### Cached Resources

- Go modules
- Node packages
- Wails CLI

### Benefits

Faster developer builds.

---

## Dependency Auto-Install

### Description

Offer automatic installation of missing dependencies.

### Supported Package Managers

- apt
- dnf
- pacman

### Example

```text
Missing dependencies detected.

Install automatically? [Y/n]
```

### Benefits

Reduces setup friction.

---

# Future Enhancements

## GitHub Release Fallback

### Description

Attempt multiple release retrieval strategies.

### Order

1. GitHub Release API
2. Direct release URL
3. Source build fallback

### Benefits

Improves resilience against GitHub outages.

---

## Native Linux Packages

### Description

Publish distro-native packages.

### Targets

- .deb
- .rpm
- .pkg.tar.zst

### Benefits

Provides native installation experiences for major distributions.

---

# Success Criteria

The installer should:

- Install Rum in under one minute using release binaries
- Support source builds for developers
- Recover automatically from failures
- Provide actionable diagnostics
- Verify downloads securely
- Integrate cleanly with Linux desktops
- Require minimal user intervention
- Support future self-updating functionality