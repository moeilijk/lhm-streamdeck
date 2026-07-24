GOCMD=go
GOBUILD=$(GOCMD) build
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

plugin-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(SDPLUGINDIR)/lhm ./cmd/lhm_streamdeck_plugin
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(SDPLUGINDIR)/lhm-bridge ./cmd/lhm-bridge

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
	$(GOCMD) test $$($(GOCMD) list ./... 2>/dev/null | grep -v 'cmd/lhm_streamdeck_plugin\|cmd/lhm_debugger\|app/lhmstreamdeckplugin')
	bash scripts/verify-settings-pi.sh
	python3 scripts/test-linux-manifest.py
	streamdeck validate $(SDPLUGINDIR)

release: verify
	-@rm build/com.moeilijk.lhm.streamDeckPlugin
	streamdeck pack com.moeilijk.lhm.sdPlugin --output build --force

# The Linux manifest tweaks (CodePathLin + OS linux entry, see
# scripts/make-linux-manifest.py) are injected only into the packed copy. The
# source manifest.json is backed up byte-for-byte and restored afterwards, so
# the release path never leaves the working tree dirty (json.dumps would
# otherwise reformat the file).
release-linux: verify plugin-linux
	-@rm build/com.moeilijk.lhm-linux.streamDeckPlugin
	@mkdir -p build
	@cp $(SDPLUGINDIR)/manifest.json build/.manifest.orig
	python3 scripts/make-linux-manifest.py $(SDPLUGINDIR)/manifest.json
	streamdeck pack $(SDPLUGINDIR) --output build --force --ignore-validation
	mv build/com.moeilijk.lhm.streamDeckPlugin build/com.moeilijk.lhm-linux.streamDeckPlugin
	@cp build/.manifest.orig $(SDPLUGINDIR)/manifest.json
	@rm -f build/.manifest.orig
	python3 scripts/verify-linux-package.py build/com.moeilijk.lhm-linux.streamDeckPlugin
	$(MAKE) plugin

# Version bumps are explicit. Commit/release paths must not mutate manifest.json.
bump-version:
	./scripts/bump-manifest-version.sh
