//go:build linux

package plugin

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	hwsensorsservice "github.com/moeilijk/lhm-streamdeck/pkg/service"
)

// hwmonTracker tracks observed min/max for readings that lack hardware limits.
type hwmonTracker struct{ min, max float64 }

// HwmonService implements HardwareService by reading /sys/class/hwmon directly.
// Used on Linux in place of the lhm-bridge gRPC subprocess.
type HwmonService struct {
	fetchMu  sync.Mutex
	mu       sync.RWMutex
	pollTime uint64
	sensors  map[string]*sensor
	order    []string
	readings map[string][]*reading
	ready    bool

	trackMu  sync.Mutex
	tracking map[string]*hwmonTracker
}

// NewHwmonService returns an HwmonService ready to poll.
func NewHwmonService() *HwmonService {
	return &HwmonService{tracking: make(map[string]*hwmonTracker)}
}

func (s *HwmonService) ensureReady() error {
	s.mu.RLock()
	ready := s.ready
	s.mu.RUnlock()
	if ready {
		return nil
	}
	s.fetchMu.Lock()
	defer s.fetchMu.Unlock()
	return s.poll()
}

func (s *HwmonService) PollTime() (uint64, error) {
	s.fetchMu.Lock()
	defer s.fetchMu.Unlock()
	if err := s.poll(); err != nil {
		return 0, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pollTime, nil
}

func (s *HwmonService) Sensors() ([]hwsensorsservice.Sensor, error) {
	if err := s.ensureReady(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]hwsensorsservice.Sensor, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, &sensor{id: id, name: s.sensors[id].name})
	}
	return out, nil
}

func (s *HwmonService) ReadingsForSensorID(id string) ([]hwsensorsservice.Reading, error) {
	if err := s.ensureReady(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rs, ok := s.readings[id]
	if !ok {
		return nil, fmt.Errorf("hwmon: sensor %s not found", id)
	}
	out := make([]hwsensorsservice.Reading, len(rs))
	for i, r := range rs {
		out[i] = r
	}
	return out, nil
}

func (s *HwmonService) poll() error {
	root := s.buildRoot()
	sens, order, rdgs := buildSnapshot(&root)
	s.mu.Lock()
	s.pollTime = uint64(time.Now().UnixNano())
	s.sensors = sens
	s.order = order
	s.readings = rdgs
	s.ready = true
	s.mu.Unlock()
	return nil
}

// ── hwmon tree builder ────────────────────────────────────────────────────────

const sysfsHwmon = "/sys/class/hwmon"

func (s *HwmonService) buildRoot() node {
	root := node{Text: "Computer"}
	entries, err := os.ReadDir(sysfsHwmon)
	if err != nil {
		return root
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "hwmon") {
			continue
		}
		dir := filepath.Join(sysfsHwmon, e.Name())
		if n := s.readDevice(dir, e.Name()); n != nil {
			root.Children = append(root.Children, *n)
		}
	}
	return root
}

func (s *HwmonService) readDevice(dir, hwmonName string) *node {
	name := hwmonReadFile(filepath.Join(dir, "name"))
	if name == "" {
		name = hwmonName
	}
	baseID := hwmonDeviceBaseID(dir, hwmonName, name)
	displayName := hwmonDisplayName(name, baseID)

	groups := map[string][]node{}
	s.addReadings(dir, baseID, "temp", "Temperature", "°C", 1e-3, groups)
	s.addReadings(dir, baseID, "fan", "Fan", "RPM", 1, groups)
	s.addReadings(dir, baseID, "in", "Voltage", "V", 1e-3, groups)
	s.addReadings(dir, baseID, "power", "Power", "W", 1e-6, groups)
	s.addReadings(dir, baseID, "curr", "Current", "A", 1e-3, groups)
	s.addFreqReadings(dir, baseID, groups)

	if len(groups) == 0 {
		return nil
	}

	groupOrder := []string{"Temperature", "Fan", "Voltage", "Power", "Current", "Clock"}
	var children []node
	for _, g := range groupOrder {
		if readings, ok := groups[g]; ok {
			children = append(children, node{Text: g + "s", Children: readings})
		}
	}
	return &node{Text: displayName, Children: children}
}

var (
	cpuCoreLabelRE = regexp.MustCompile(`(?i)^core ([0-9]+)$`)
	cpuPkgLabelRE  = regexp.MustCompile(`(?i)^package id [0-9]+$`)
)

func (s *HwmonService) addReadings(dir, baseID, prefix, typeName, unit string, scale float64, groups map[string][]node) {
	files, _ := filepath.Glob(filepath.Join(dir, prefix+"*_input"))
	sortHwmonFilesNumeric(files, prefix, "_input")

	for _, f := range files {
		idx := hwmonExtractIndex(filepath.Base(f), prefix, "_input")
		raw := hwmonReadInt(f)
		if raw == math.MinInt64 {
			continue
		}
		val := float64(raw) * scale

		label := hwmonReadFile(filepath.Join(dir, prefix+idx+"_label"))
		if label == "" {
			label = typeName + " " + idx
		}
		if baseID == "/cpu" && typeName == "Temperature" {
			label = normalizeCPULabel(label)
		}

		sensorId := fmt.Sprintf("%s/%s/%s", baseID, strings.ToLower(typeName), idx)

		minRaw := hwmonReadInt(filepath.Join(dir, prefix+idx+"_min"))
		maxRaw := hwmonReadInt(filepath.Join(dir, prefix+idx+"_max"))
		if maxRaw == math.MinInt64 {
			maxRaw = hwmonReadInt(filepath.Join(dir, prefix+idx+"_crit"))
		}

		tMin, tMax := s.track(sensorId, val)
		var minStr, maxStr string
		if minRaw != math.MinInt64 {
			minStr = hwmonFmtVal(float64(minRaw)*scale, unit)
		} else {
			minStr = hwmonFmtVal(tMin, unit)
		}
		if maxRaw != math.MinInt64 {
			maxStr = hwmonFmtVal(float64(maxRaw)*scale, unit)
		} else {
			maxStr = hwmonFmtVal(tMax, unit)
		}

		groups[typeName] = append(groups[typeName], node{
			Text:     label,
			SensorID: sensorId,
			Type:     typeName,
			Value:    hwmonFmtVal(val, unit),
			Min:      minStr,
			Max:      maxStr,
		})
	}
}

func (s *HwmonService) addFreqReadings(dir, baseID string, groups map[string][]node) {
	files, _ := filepath.Glob(filepath.Join(dir, "freq*_input"))
	sortHwmonFilesNumeric(files, "freq", "_input")

	for _, f := range files {
		idx := hwmonExtractIndex(filepath.Base(f), "freq", "_input")
		raw := hwmonReadInt(f)
		if raw == math.MinInt64 {
			continue
		}
		val := float64(raw) / 1e6 // Hz → MHz

		label := hwmonReadFile(filepath.Join(dir, "freq"+idx+"_label"))
		if label == "" {
			label = "Clock " + idx
		}
		sensorId := fmt.Sprintf("%s/clock/%s", baseID, idx)
		tMin, tMax := s.track(sensorId, val)

		groups["Clock"] = append(groups["Clock"], node{
			Text:     label,
			SensorID: sensorId,
			Type:     "Clock",
			Value:    hwmonFmtVal(val, "MHz"),
			Min:      hwmonFmtVal(tMin, "MHz"),
			Max:      hwmonFmtVal(tMax, "MHz"),
		})
	}
}

func (s *HwmonService) track(id string, val float64) (min, max float64) {
	s.trackMu.Lock()
	defer s.trackMu.Unlock()
	t, ok := s.tracking[id]
	if !ok {
		t = &hwmonTracker{min: val, max: val}
		s.tracking[id] = t
	}
	if val < t.min {
		t.min = val
	}
	if val > t.max {
		t.max = val
	}
	return t.min, t.max
}

// ── device classification ─────────────────────────────────────────────────────

var (
	pciAddrRE      = regexp.MustCompile(`^[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-9a-fA-F]$`)
	nvmeNsRE       = regexp.MustCompile(`^nvme[0-9]+n[0-9]+(?:p[0-9]+)?$`)
)

func hwmonDeviceBaseID(dir, hwmonName, name string) string {
	canonical := hwmonCanonicalPath(dir)
	sanitized := hwmonSanitize(name)
	if sanitized == "" {
		sanitized = hwmonSanitize(hwmonName)
	}

	switch {
	case hwmonIsCPU(name, canonical):
		return "/cpu"
	case hwmonIsGPU(name, canonical):
		tok := hwmonFirstMatch(canonical, pciAddrRE)
		if tok == "" {
			tok = hwmonNonEmpty(sanitized, hwmonName)
		}
		return "/gpu-amd/" + hwmonSanitize(tok)
	case hwmonIsStorage(name, canonical):
		tok := hwmonStorageToken(canonical)
		if tok == "" {
			tok = hwmonNonEmpty(sanitized, hwmonName)
		}
		return "/storage/" + hwmonSanitize(tok)
	case hwmonIsNetwork(canonical):
		tok := hwmonSegmentAfter(canonical, "net")
		if tok == "" {
			tok = hwmonNonEmpty(sanitized, hwmonName)
		}
		return "/network/" + hwmonSanitize(tok)
	case hwmonIsLPC(name, canonical):
		tok := hwmonSegmentAfter(canonical, "platform")
		if tok == "" {
			tok = hwmonNonEmpty(sanitized, hwmonName)
		}
		return "/lpc/" + hwmonSanitize(tok)
	case hwmonIsThermal(name, canonical):
		tok := hwmonNonEmpty(hwmonThermalToken(canonical), sanitized, hwmonName)
		return "/thermal/" + hwmonSanitize(tok)
	default:
		segs := hwmonPathSegments(canonical)
		tok := ""
		if len(segs) > 0 {
			tok = segs[len(segs)-1]
		}
		tok = hwmonNonEmpty(tok, sanitized, hwmonName)
		return "/" + hwmonSanitize(tok)
	}
}

func hwmonDisplayName(name, baseID string) string {
	switch {
	case baseID == "/cpu":
		return "CPU"
	case strings.HasPrefix(baseID, "/gpu-amd/"):
		return "AMD GPU"
	case strings.HasPrefix(baseID, "/storage/"):
		segs := strings.SplitN(baseID, "/", 3)
		if len(segs) == 3 && segs[2] != "" {
			return "Storage " + segs[2]
		}
		return "Storage"
	case strings.HasPrefix(baseID, "/network/"):
		return "Network " + strings.TrimPrefix(baseID, "/network/")
	case strings.HasPrefix(baseID, "/lpc/"):
		return strings.ToUpper(name)
	case strings.HasPrefix(baseID, "/thermal/"):
		return name
	default:
		return name
	}
}

func hwmonIsCPU(name, canonical string) bool {
	switch name {
	case "coretemp", "k10temp", "zenpower", "k8temp", "cpu_thermal", "fam15h_power":
		return true
	}
	return strings.Contains(canonical, "/cpu") ||
		strings.Contains(canonical, "coretemp.") ||
		strings.Contains(canonical, "k10temp.")
}

func hwmonIsGPU(name, canonical string) bool {
	return name == "amdgpu" || strings.Contains(canonical, "/drm/") || strings.Contains(canonical, "/gpu")
}

func hwmonIsStorage(name, canonical string) bool {
	return name == "nvme" || name == "drivetemp" ||
		strings.Contains(canonical, "/block/") || strings.Contains(canonical, "/nvme/")
}

func hwmonIsNetwork(canonical string) bool { return strings.Contains(canonical, "/net/") }

func hwmonIsLPC(name, canonical string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(canonical, "/platform/") ||
		strings.Contains(canonical, "/isa") ||
		strings.HasPrefix(lower, "nct") ||
		strings.HasPrefix(lower, "it8") ||
		strings.HasPrefix(lower, "w836") ||
		strings.HasPrefix(lower, "f718")
}

func hwmonIsThermal(name, canonical string) bool {
	return strings.ToLower(name) == "acpitz" || strings.Contains(canonical, "thermal")
}

func hwmonCanonicalPath(dir string) string {
	for _, candidate := range []string{filepath.Join(dir, "device"), dir} {
		if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
			return filepath.Clean(resolved)
		}
	}
	return filepath.Clean(dir)
}

func hwmonStorageToken(canonical string) string {
	for _, seg := range hwmonPathSegments(canonical) {
		if nvmeNsRE.MatchString(seg) {
			return seg
		}
	}
	for _, seg := range hwmonPathSegments(canonical) {
		switch {
		case strings.HasPrefix(seg, "nvme") && len(seg) > 4:
			return seg
		case strings.HasPrefix(seg, "sd"), strings.HasPrefix(seg, "vd"),
			strings.HasPrefix(seg, "hd"), strings.HasPrefix(seg, "md"):
			return seg
		}
	}
	return hwmonSegmentAfter(canonical, "block")
}

func hwmonThermalToken(canonical string) string {
	for _, seg := range hwmonPathSegments(canonical) {
		lower := strings.ToLower(seg)
		if strings.Contains(lower, "thermal") || strings.Contains(lower, "tz") {
			return seg
		}
	}
	return ""
}

func hwmonFirstMatch(path string, re *regexp.Regexp) string {
	for _, seg := range hwmonPathSegments(path) {
		if re.MatchString(seg) {
			return seg
		}
	}
	return ""
}

func hwmonSegmentAfter(path, marker string) string {
	segs := hwmonPathSegments(path)
	for i := 0; i < len(segs)-1; i++ {
		if segs[i] == marker {
			return segs[i+1]
		}
	}
	return ""
}

func hwmonPathSegments(path string) []string {
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" && p != "." {
			out = append(out, p)
		}
	}
	return out
}

func hwmonNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "device"
}

func hwmonSanitize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		case r == ':':
			b.WriteRune('_')
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	out = regexp.MustCompile(`-{2,}`).ReplaceAllString(out, "-")
	if out == "" {
		return "device"
	}
	return out
}

// ── label helpers ─────────────────────────────────────────────────────────────

func normalizeCPULabel(label string) string {
	if m := cpuCoreLabelRE.FindStringSubmatch(label); m != nil {
		if core, err := strconv.Atoi(m[1]); err == nil {
			return fmt.Sprintf("CPU Core #%d", core+1)
		}
	}
	if cpuPkgLabelRE.MatchString(label) {
		return "CPU Package"
	}
	return label
}

// ── low-level sysfs helpers ───────────────────────────────────────────────────

func hwmonReadFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func hwmonReadInt(path string) int64 {
	s := hwmonReadFile(path)
	if s == "" {
		return math.MinInt64
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return math.MinInt64
	}
	return v
}

func hwmonFmtVal(v float64, unit string) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	s = strings.ReplaceAll(s, ".", ",")
	if unit == "" {
		return s
	}
	return s + " " + unit
}

func hwmonExtractIndex(filename, prefix, suffix string) string {
	s := strings.TrimPrefix(filename, prefix)
	return strings.TrimSuffix(s, suffix)
}

func sortHwmonFilesNumeric(files []string, prefix, suffix string) {
	sort.Slice(files, func(i, j int) bool {
		ni := hwmonIndexFromBase(filepath.Base(files[i]), prefix, suffix)
		nj := hwmonIndexFromBase(filepath.Base(files[j]), prefix, suffix)
		return ni < nj
	})
}

func hwmonIndexFromBase(base, prefix, suffix string) int {
	s := strings.TrimPrefix(base, prefix)
	s = strings.TrimSuffix(s, suffix)
	n, _ := strconv.Atoi(s)
	return n
}
