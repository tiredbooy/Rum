# Releasing Rum

1. Make sure `master` is green (CI workflow).
2. Tag and push: `git tag v1.2.3 && git push origin v1.2.3`
3. The **Release** workflow builds everything and publishes the GitHub Release.
   Tags containing `-` (e.g. `v1.2.3-rc1`) are marked as pre-releases.

## What gets published

| Asset | Built on |
|-------|----------|
| `Rum-<ver>-x86_64.AppImage`, `Rum_<ver>_amd64.deb` | Linux |
| `Rum-Setup-<ver>.exe` (NSIS) | Windows |
| `Rum-<ver>-macOS-unsigned.dmg` (universal) | macOS |
| `rum-<os>-<arch>[.exe]` CLI (linux/windows/darwin, amd64+arm64) | Linux (cross) |
| `rum-chrome-<ver>.zip`, `rum-firefox-<ver>.xpi` | Linux |
| `SHA256SUMS` | publish job |

`<ver>` is the tag without the leading `v`. The Wails CLI version is read from
the `wails/v2` line in `go.mod`, so bumping Wails there is enough.

## SHA256SUMS

Standard `sha256sum` format, one `<hash>  <file name>` line per asset, never
listing itself. The in-app update check and users verify downloads against it:
`sha256sum -c --ignore-missing SHA256SUMS`.

## Notes

- Only the publish job has `contents: write`; every other job is read-only.
- The macOS build is unsigned; the Windows installer is unsigned.
- Actions are pinned by commit SHA. To update one, look up the new tag's commit
  and change the SHA and the trailing `# vX.Y.Z` comment together.
- `appimagetool` is downloaded at a pinned version and checked by SHA-256 in
  `.github/scripts/package-linux.sh`.
