//go:build linux

package lhmstreamdeckplugin

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"syscall"
	"time"

	lhmplugin "github.com/moeilijk/lhm-streamdeck/internal/lhm/plugin"
)

// startLinuxSource wires rt.hw to lhm-companion over HTTP — the only sensor
// source on Linux (#77): Windows = LHM (via lhm-bridge), Linux = lhm-companion.
// No hwmon path and no fallbacks; an unreachable endpoint surfaces as the
// explicit error state on the tiles.
//
// For local profiles the bundled companion is supervised: a companion already
// listening on the endpoint (e.g. a systemd service) is reused, otherwise the
// bundled ./lhm-companion is spawned next to the plugin binary.
func startLinuxSource(rt *sourceRuntime) error {
	if isLocalHost(rt.profile.Host) {
		ensureLocalCompanion(normalizePort(rt.profile.Port))
	}
	rt.hw = lhmplugin.NewHTTPService(profileEndpoint(rt.profile))
	return nil
}

func isLocalHost(host string) bool {
	switch host {
	case "", "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

func normalizePort(port int) int {
	if port <= 0 || port > 65535 {
		return 8085
	}
	return port
}

// companionSupervisor keeps one local lhm-companion available per port.
type companionSupervisor struct {
	port  int
	probe func() bool                 // true when something serves the endpoint
	spawn func() (func() bool, error) // starts the bundled companion; returns an "exited" check

	mu         sync.Mutex
	exited     func() bool // nil until we spawned a child
	lastSpawn  time.Time
	rapidFails int // consecutive spawns whose child died right away
}

// rapidFailLimit stops the supervisor from waging a spawn war over a port that
// something else holds (bind failure → instant exit → respawn, ad infinitum).
const rapidFailLimit = 3

// ensureOnce spawns the bundled companion when nothing serves the endpoint
// and no child of ours is still running:
//   - endpoint reachable            → nothing to do (external companion or our child)
//   - our child still alive         → give it time; no second spawn
//   - unreachable and no live child → spawn
func (s *companionSupervisor) ensureOnce() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.probe() {
		s.rapidFails = 0
		return
	}
	if s.exited != nil && !s.exited() {
		return
	}
	if s.exited != nil {
		if time.Since(s.lastSpawn) < 10*time.Second {
			s.rapidFails++
		} else {
			s.rapidFails = 0
		}
	}
	if s.rapidFails >= rapidFailLimit {
		if s.rapidFails == rapidFailLimit {
			s.rapidFails++ // log the give-up once
			log.Printf("lhm-companion on port %d keeps exiting immediately (port in use by something else?); giving up\n", s.port)
		}
		return
	}
	exited, err := s.spawn()
	if err != nil {
		log.Printf("lhm-companion spawn failed (port %d): %v\n", s.port, err)
		return
	}
	s.exited = exited
	s.lastSpawn = time.Now()
	// Give the fresh companion a moment to bind so the first poll can succeed.
	for i := 0; i < 10 && !s.probe(); i++ {
		time.Sleep(200 * time.Millisecond)
	}
}

var (
	companionMu sync.Mutex
	companions  = map[int]*companionSupervisor{}
)

// ensureLocalCompanion registers a supervisor for the port (once) and runs an
// ensure pass now plus periodically, so a crashed companion is respawned.
func ensureLocalCompanion(port int) {
	companionMu.Lock()
	sup, ok := companions[port]
	if !ok {
		sup = &companionSupervisor{
			port:  port,
			probe: func() bool { return endpointReachable(port) },
			spawn: func() (func() bool, error) { return spawnCompanion(port) },
		}
		companions[port] = sup
		go func() {
			for {
				time.Sleep(10 * time.Second)
				sup.ensureOnce()
			}
		}()
	}
	companionMu.Unlock()
	sup.ensureOnce()
}

func endpointReachable(port int) bool {
	client := http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/data.json", port))
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// spawnCompanion starts the bundled companion binary (working directory is the
// plugin directory, see main). Pdeathsig ties its lifetime to the plugin's.
func spawnCompanion(port int) (func() bool, error) {
	cmd := exec.Command("./lhm-companion", "-port", fmt.Sprint(port))
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	log.Printf("spawned bundled lhm-companion on port %d (pid %d)\n", port, cmd.Process.Pid)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	return func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, nil
}
