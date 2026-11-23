GOCMD=go
# Release binaries must not depend on where or from which commit they are
# built (#121): -trimpath drops the absolute source paths, -buildvcs=false
# drops the commit stamp, so a commit that leaves the Go code alone leaves
# the binaries byte for byte the same.
GOBUILDFLAGS=-trimpath -buildvcs=false
GOBUILD=$(GOCMD) build $(GOBUILDFLAGS)
GOCLEAN=$(GOCMD) clean
GOOS?=windows
GOARCH?=amd64
GOTARGETENV=GOOS=$(GOOS) GOARCH=$(GOARCH)

SDPLUGINDIR=./com.moeilijk.lhm.sdPlugin

PROTOS=$(wildcard ./*/**/**/*.proto)
PROTOPB=$(PROTOS:.proto=.pb.go)

plugin:
	$(GOTARGETENV) $(GOBUILD) -o $(SDPLUGINDIR)/lhm.exe ./cmd/lhm_streamdeck_plugin
	$(GOTARGETENV) $(GOBUILD) -o $(SDPLUGINDIR)/lhm-bridge.exe ./cmd/lhm-bridge
	-@install-plugin.bat

# The Linux plugin bundles lhm-companion (built from the sibling repo): it is
# the only sensor source on Linux (#77) and gets spawned by the plugin when no
# companion is already listening on the local endpoint.
COMPANION_DIR?=$(HOME)/projects/GitHub/lhm-companion

plugin-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(SDPLUGINDIR)/lhm ./cmd/lhm_streamdeck_plugin
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(SDPLUGINDIR)/lhm-bridge ./cmd/lhm-bridge
	cd $(COMPANION_DIR) && GOOS=linux GOARCH=amd64 go build $(GOBUILDFLAGS) -o $(abspath $(SDPLUGINDIR))/lhm-companion ./cmd/lhm-companion

proto: $(PROTOPB)

$(PROTOPB): $(PROTOS)
	.cache/protoc/bin/protoc \
 		--go_out=Mgrpc/service_config/service_config.proto=/internal/proto/grpc_service_config:. \
		--go-grpc_out=Mgrpc/service_config/service_config.proto=/internal/proto/grpc_service_config:. \
  		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		$(<)

# plugin:
# 	-@kill-streamdeck.bat
# 	@go build -o com.moeilijk.lhm.sdPlugin\\lhm.exe github.com/moeilijk/lhm-streamdeck/cmd/lhm_streamdeck_plugin
# 	@xcopy com.moeilijk.lhm.sdPlugin $(APPDATA)\\Elgato\\StreamDeck\\Plugins\\com.moeilijk.lhm.sdPlugin\\ /E /Q /Y
# 	@start-streamdeck.bat

debug:
	$(GOTARGETENV) $(GOBUILD) -o $(SDPLUGINDIR)/lhm.exe ./cmd/lhm_debugger
	-@install-plugin.bat
# @xcopy com.moeilijk.lhm.sdPlugin $(APPDATA)\\Elgato\\StreamDeck\\Plugins\\com.moeilijk.lhm.sdPlugin\\ /E /Q /Y

verify:
	$(GOTARGETENV) $(GOCMD) build ./...
	$(GOTARGETENV) $(GOBUILD) -o $(SDPLUGINDIR)/lhm.exe ./cmd/lhm_streamdeck_plugin
	$(GOTARGETENV) $(GOBUILD) -o $(SDPLUGINDIR)/lhm-bridge.exe ./cmd/lhm-bridge
	$(GOCMD) test $$($(GOCMD) list ./... 2>/dev/null | grep -v 'cmd/lhm_streamdeck_plugin\|cmd/lhm_debugger')
	bash scripts/verify-settings-pi.sh
	python3 scripts/test-linux-manifest.py
	python3 scripts/test-manifest-action-names.py
	bash scripts/test-av-check-files.sh
	streamdeck validate $(SDPLUGINDIR)

# Both artifacts carry the manifest version in their filename (issue #78).
# Verified against Elgato: the Marketplace guidelines and distribution docs set
# no filename rule, the CLI's <uuid>.streamDeckPlugin is only its default
# output name, and Marketplace itself serves downloads with the version in the
# name (e.g. "Libre Hardware Monitor-2-windows.streamDeckPlugin").
#
# When the project's own Go code is unchanged since the previous release,
# release-binaries.sh puts that release's lhm.exe and lhm-bridge.exe back,
# after checking their hashes and scanning them, so the hashes stay the same
# (#121). REBUILD_BINARIES=1 ships the fresh ones regardless. Both targets
# record what they ship in release-hashes.txt: run them before the release
# commit and commit that file with the release.
release: verify
	bash scripts/release-binaries.sh windows
	@mkdir -p build
	-@rm -f build/com.moeilijk.lhm.streamDeckPlugin build/com.moeilijk.lhm-[0-9]*.streamDeckPlugin
	streamdeck pack com.moeilijk.lhm.sdPlugin --output build --force
	@VER=$$(python3 scripts/manifest-version.py); \
	mv build/com.moeilijk.lhm.streamDeckPlugin "build/com.moeilijk.lhm-$$VER.streamDeckPlugin"; \
	echo "artifact: build/com.moeilijk.lhm-$$VER.streamDeckPlugin"
	$(MAKE) av-check AV_CHECK_PKG="build/com.moeilijk.lhm-$$(python3 scripts/manifest-version.py).streamDeckPlugin"

# The Linux manifest tweaks (CodePathLin + OS linux entry, see
# scripts/make-linux-manifest.py) are injected only into the packed copy. The
# source manifest.json is backed up byte-for-byte and restored afterwards, so
# the release path never leaves the working tree dirty (json.dumps would
# otherwise reformat the file).
release-linux: verify plugin-linux
	bash scripts/release-binaries.sh linux
	-@rm -f build/com.moeilijk.lhm-linux*.streamDeckPlugin
	@mkdir -p build
	@cp $(SDPLUGINDIR)/manifest.json build/.manifest.orig
	python3 scripts/make-linux-manifest.py $(SDPLUGINDIR)/manifest.json
	streamdeck pack $(SDPLUGINDIR) --output build --force --ignore-validation
	@cp build/.manifest.orig $(SDPLUGINDIR)/manifest.json
	@rm -f build/.manifest.orig
	@VER=$$(python3 scripts/manifest-version.py); \
	mv build/com.moeilijk.lhm.streamDeckPlugin "build/com.moeilijk.lhm-linux-$$VER.streamDeckPlugin"; \
	python3 scripts/verify-linux-package.py "build/com.moeilijk.lhm-linux-$$VER.streamDeckPlugin"
	$(MAKE) av-check AV_CHECK_PKG="build/com.moeilijk.lhm-linux-$$(python3 scripts/manifest-version.py).streamDeckPlugin"
	$(MAKE) plugin

# Builds the Windows and the Linux binaries from two clones of HEAD in
# different directories and fails when they differ (#121).
reproducible:
	bash scripts/check-reproducible.sh

# Rebuilds a published release from its tag and compares the binaries with
# release-hashes.txt: make verify-release TAG=v2.2.0
verify-release:
	bash scripts/verify-release.sh $(TAG)

# Antivirus check of a packed artifact (issue #93): the Microsoft Defender
# engine with current definitions runs offline under wine64 and is the hard
# gate; the Defender test VM joins when its baseline exists; VirusTotal,
# MetaDefender and OpenTIP run when their API keys are set (shared file
# ~/.config/av-check/av.env). Both release targets run it on the package they
# just packed.
# AV_CHECK_PKG: package to scan (default: newest build/com.moeilijk.lhm-*.streamDeckPlugin).
# AV_CHECK_FLAGS: --upload (submit unknown files), --reanalyze (VirusTotal rescan), --offline, --update.
AV_CHECK_FLAGS?=
AV_CHECK_PKG?=
av-check:
	bash scripts/av-check.sh $(AV_CHECK_FLAGS) $(AV_CHECK_PKG)

# Version bumps are explicit. Commit/release paths must not mutate manifest.json.
bump-version:
	./scripts/bump-manifest-version.sh
