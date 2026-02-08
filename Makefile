GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOOS?=windows
GOARCH?=amd64
GOTARGETENV=GOOS=$(GOOS) GOARCH=$(GOARCH)

SDPLUGINDIR=./com.moeilijk.lhm.sdPlugin

PROTOS=$(wildcard ./*/**/**/*.proto)
PROTOPB=$(PROTOS:.proto=.pb.go)

plugin: bump-version
	$(GOTARGETENV) $(GOBUILD) -o $(SDPLUGINDIR)/lhm.exe ./cmd/lhm_streamdeck_plugin
	$(GOTARGETENV) $(GOBUILD) -o $(SDPLUGINDIR)/lhm-bridge.exe ./cmd/lhm-bridge
	-@install-plugin.bat

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
# 	@go build -o com.moeilijk.lhm.sdPlugin\\lhm.exe github.com/shayne/lhm-streamdeck/cmd/lhm_streamdeck_plugin
# 	@xcopy com.moeilijk.lhm.sdPlugin $(APPDATA)\\Elgato\\StreamDeck\\Plugins\\com.moeilijk.lhm.sdPlugin\\ /E /Q /Y
# 	@start-streamdeck.bat

debug:
	$(GOTARGETENV) $(GOBUILD) -o $(SDPLUGINDIR)/lhm.exe ./cmd/lhm_debugger
	-@install-plugin.bat
# @xcopy com.moeilijk.lhm.sdPlugin $(APPDATA)\\Elgato\\StreamDeck\\Plugins\\com.moeilijk.lhm.sdPlugin\\ /E /Q /Y

verify:
	$(GOTARGETENV) $(GOCMD) build ./...
	$(GOCMD) test ./...
	bash scripts/verify-settings-pi.sh
	streamdeck validate $(SDPLUGINDIR)

release: verify bump-version
	-@rm build/com.moeilijk.lhm.streamDeckPlugin
	streamdeck pack com.moeilijk.lhm.sdPlugin --output build --force

bump-version:
	./scripts/bump-manifest-version.sh
