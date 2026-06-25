// companion-probe drives the plugin's own NewHTTPService against a running
// lhm-companion endpoint and prints the sensors/readings the dial catalog would
// receive. Verifies the plugin code path that source_linux.go uses for a
// non-localhost source profile. Usage: companion-probe http://host:8085/data.json
package main

import (
	"fmt"
	"os"

	lhmplugin "github.com/moeilijk/lhm-streamdeck/internal/lhm/plugin"
)

func main() {
	url := "http://127.0.0.1:8085/data.json"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}
	hw := lhmplugin.NewHTTPService(url)

	if _, err := hw.PollTime(); err != nil {
		fmt.Printf("PollTime error: %v\n", err)
		os.Exit(1)
	}
	sensors, err := hw.Sensors()
	if err != nil {
		fmt.Printf("Sensors error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("endpoint: %s\nsensors: %d\n", url, len(sensors))
	total := 0
	for _, s := range sensors {
		readings, err := hw.ReadingsForSensorID(s.ID())
		if err != nil {
			continue
		}
		total += len(readings)
		for _, r := range readings {
			if total <= 12 {
				fmt.Printf("  %s / %s = %v %s\n", s.Name(), r.Label(), r.Value(), r.Unit())
			}
		}
	}
	fmt.Printf("total readings: %d\n", total)
}
