//go:build linux

package lhmstreamdeckplugin

import (
	lhmplugin "github.com/moeilijk/lhm-streamdeck/internal/lhm/plugin"
)

// startLinuxSource wires rt.hw directly without spawning a bridge subprocess.
// Local profiles read /sys/class/hwmon; remote profiles poll the HTTP endpoint.
func startLinuxSource(rt *sourceRuntime) error {
	host := rt.profile.Host
	if host == "" || host == "127.0.0.1" || host == "localhost" {
		rt.hw = lhmplugin.NewHwmonService()
		return nil
	}
	rt.hw = lhmplugin.NewHTTPService(profileEndpoint(rt.profile))
	return nil
}
