package lhmstreamdeckplugin

// globalSettings represents plugin-wide settings (not per-action)
type globalSettings struct {
	PollInterval int `json:"pollInterval"` // milliseconds: 250, 500, 1000
}

// settingsTileSettings stores per-tile appearance settings for the settings action
type settingsTileSettings struct {
	TileBackground   string `json:"tileBackground"`   // hex color for tile background
	TileTextColor    string `json:"tileTextColor"`    // hex color for tile value text
	ShowLabel        bool   `json:"showLabel"`        // toggles startup placeholder background
	Title            string `json:"title"`            // title text, like graph tiles
	TitleColor       string `json:"titleColor"`       // title color, like graph tiles
	ShowTitleInGraph *bool  `json:"showTitleInGraph"` // mirrors graph tile title behavior
}

// Threshold represents a single configurable threshold level
type Threshold struct {
	ID              string  `json:"id"`              // Unique identifier
	Name            string  `json:"name"`            // User-friendly name
	Text            string  `json:"text"`            // Optional alert text to display when triggered
	TextColor       string  `json:"textColor"`       // Color for alert text
	Enabled         bool    `json:"enabled"`         // Is this threshold active?
	Operator        string  `json:"operator"`        // ">", "<", ">=", "<=", "=="
	Value           float64 `json:"value"`           // Threshold value
	BackgroundColor string  `json:"backgroundColor"` // Background color when triggered
	ForegroundColor string  `json:"foregroundColor"` // Graph foreground color
	HighlightColor  string  `json:"highlightColor"`  // Graph highlight color
	ValueTextColor  string  `json:"valueTextColor"`  // Value text color
}

type actionSettings struct {
	SensorUID        string  `json:"sensorUid"`
	ReadingID        int32   `json:"readingId,string"`
	ReadingLabel     string  `json:"readingLabel"`
	Title            string  `json:"title"`
	TitleFontSize    float64 `json:"titleFontSize"`
	ValueFontSize    float64 `json:"valueFontSize"`
	ShowTitleInGraph *bool   `json:"showTitleInGraph"`
	Min              int     `json:"min"`
	Max              int     `json:"max"`
	Format           string  `json:"format"`
	Divisor          string  `json:"divisor"`
	GraphUnit        string  `json:"graphUnit"` // B, KB, MB, GB, TB - normalizes graph values to this unit
	IsValid          bool    `json:"isValid"`
	TitleColor       string  `json:"titleColor"`
	ForegroundColor  string  `json:"foregroundColor"`
	BackgroundColor  string  `json:"backgroundColor"`
	HighlightColor   string  `json:"highlightColor"`
	ValueTextColor   string  `json:"valueTextColor"`
	InErrorState     bool    `json:"inErrorState"`

	// Dynamic threshold system
	Thresholds         []Threshold `json:"thresholds"`
	CurrentThresholdID string      `json:"currentThresholdId"`

	// Legacy Warning Threshold Settings (kept for migration, omitempty)
	WarningEnabled         bool    `json:"warningEnabled,omitempty"`
	WarningOperator        string  `json:"warningOperator,omitempty"`
	WarningValue           float64 `json:"warningValue,omitempty"`
	WarningBackgroundColor string  `json:"warningBackgroundColor,omitempty"`
	WarningForegroundColor string  `json:"warningForegroundColor,omitempty"`
	WarningHighlightColor  string  `json:"warningHighlightColor,omitempty"`
	WarningValueTextColor  string  `json:"warningValueTextColor,omitempty"`
	// Legacy Critical Threshold Settings (kept for migration, omitempty)
	CriticalEnabled         bool    `json:"criticalEnabled,omitempty"`
	CriticalOperator        string  `json:"criticalOperator,omitempty"`
	CriticalValue           float64 `json:"criticalValue,omitempty"`
	CriticalBackgroundColor string  `json:"criticalBackgroundColor,omitempty"`
	CriticalForegroundColor string  `json:"criticalForegroundColor,omitempty"`
	CriticalHighlightColor  string  `json:"criticalHighlightColor,omitempty"`
	CriticalValueTextColor  string  `json:"criticalValueTextColor,omitempty"`
	// Legacy alert state (kept for migration)
	CurrentAlertState string `json:"currentAlertState,omitempty"`
}

type actionData struct {
	action   string
	context  string
	settings *actionSettings
}

type evStatus struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

type evSendSensorsPayloadSensor struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
}

type evSendSensorsPayload struct {
	Sensors  []*evSendSensorsPayloadSensor `json:"sensors"`
	Settings *actionSettings               `json:"settings"`
}

type evSendReadingsPayloadReading struct {
	ID     int32  `json:"id,string"`
	Label  string `json:"label"`
	Prefix string `json:"prefix"`
	Unit   string `json:"unit"`
}

type evSendReadingsPayload struct {
	Readings []*evSendReadingsPayloadReading `json:"readings"`
	Settings *actionSettings                 `json:"settings"`
}

type evSdpiCollection struct {
	Group       bool     `json:"group"`
	Index       int      `json:"index"`
	Key         string   `json:"key"`
	Selection   []string `json:"selection"`
	Value       string   `json:"value"`
	Checked     bool     `json:"checked"`     // For checkbox inputs
	ThresholdID string   `json:"thresholdId"` // For threshold-specific operations
}
