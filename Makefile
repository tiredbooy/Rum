# Rum release helpers.
#   make version              show the current version
#   make test                 backend + frontend + extension tests (headless)
#   make tag                  tag the current version (vX.Y.Z) and push it -> GitHub builds the release
#   make release V=1.2.3      bump the version, commit, tag and push

SHELL   := /bin/bash
VERSION := $(shell sed -n 's/.*"productVersion": *"\([^"]*\)".*/\1/p' wails.json)
TAG     := v$(VERSION)
HEADLESS := env -u DISPLAY -u WAYLAND_DISPLAY -u DBUS_SESSION_BUS_ADDRESS RUM_NO_DESKTOP=1 GOTOOLCHAIN=local

.PHONY: version test tag release check-clean

version:
	@echo $(VERSION)

test:
	cd backend && $(HEADLESS) go test ./...
	$(HEADLESS) go test -tags "desktop,production,webkit2_41" .
	cd frontend && $(HEADLESS) npx vitest run
	cd browser-extension && $(HEADLESS) npm test

check-clean:
	@git diff --quiet && git diff --cached --quiet || { echo "Commit your changes first."; exit 1; }
	@[ "$$(git branch --show-current)" = master ] || { echo "Switch to master first."; exit 1; }

tag: check-clean
	@if git rev-parse -q --verify "refs/tags/$(TAG)" >/dev/null; then echo "$(TAG) already exists. Use: make release V=x.y.z"; exit 1; fi
	git push origin master
	git tag -a $(TAG) -m "Rum $(TAG)"
	git push origin $(TAG)
	@echo "Pushed $(TAG). Release builds: https://github.com/tiredbooy/Rum/actions"

release: check-clean
	@[[ "$(V)" =~ ^[0-9]+\.[0-9]+\.[0-9]+$$ ]] || { echo "Usage: make release V=1.2.3"; exit 1; }
	sed -i 's/"productVersion": *"[^"]*"/"productVersion": "$(V)"/' wails.json
	sed -i 's/^var AppVersion = "[^"]*"/var AppVersion = "$(V)"/' backend/internal/pkg/api/handlers/diagnostics.go
	git add wails.json backend/internal/pkg/api/handlers/diagnostics.go
	git commit -m "chore: release v$(V)"
	$(MAKE) tag
