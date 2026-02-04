//go:build windows

package lhmstreamdeckplugin

import (
	"errors"
	"os"

	"github.com/shayne/go-winpeg"
	"golang.org/x/sys/windows"
)

func attachProcessToJob(g winpeg.ProcessExitGroup, p *os.Process) error {
	if p == nil || p.Pid == 0 {
		return errors.New("process not started")
	}

	// Try the existing winpeg path first.
	if err := g.AddProcess(p); err == nil {
		return nil
	}

	// Fallback: open handle by PID (avoids unsafe handle layout issues).
	handle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		uint32(p.Pid),
	)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)

	return windows.AssignProcessToJobObject(windows.Handle(g), handle)
}
