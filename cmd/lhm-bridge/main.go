package main

import (
	"github.com/hashicorp/go-plugin"
	lhmplugin "github.com/shayne/lhm-streamdeck/internal/lhm/plugin"
	hwsensorsservice "github.com/shayne/lhm-streamdeck/pkg/service"
)

func main() {
	service := lhmplugin.StartService()

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: hwsensorsservice.Handshake,
		Plugins: map[string]plugin.Plugin{
			"lhmplugin": &hwsensorsservice.HardwareServicePlugin{Impl: &lhmplugin.Plugin{Service: service}},
		},

		// A non-nil value here enables gRPC serving for this plugin...
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
