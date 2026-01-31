package lhmstreamdeckplugin

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
	InErrorState bool `json:"inErrorState"`
	// Warning Threshold Settings
	WarningEnabled         bool    `json:"warningEnabled"`         // Enable warning threshold
	WarningOperator        string  `json:"warningOperator"`        // ">", "<", ">=", "<=", "=="
	WarningValue           float64 `json:"warningValue"`           // Warning threshold value
	WarningBackgroundColor string  `json:"warningBackgroundColor"` // Background color for warning
	WarningForegroundColor string  `json:"warningForegroundColor"` // Graph foreground for warning
	WarningHighlightColor  string  `json:"warningHighlightColor"`  // Graph highlight for warning
	WarningValueTextColor  string  `json:"warningValueTextColor"`  // Value text color for warning
	// Critical Threshold Settings
	CriticalEnabled         bool    `json:"criticalEnabled"`         // Enable critical threshold
	CriticalOperator        string  `json:"criticalOperator"`        // ">", "<", ">=", "<=", "=="
	CriticalValue           float64 `json:"criticalValue"`           // Critical threshold value
	CriticalBackgroundColor string  `json:"criticalBackgroundColor"` // Background color for critical
	CriticalForegroundColor string  `json:"criticalForegroundColor"` // Graph foreground for critical
	CriticalHighlightColor  string  `json:"criticalHighlightColor"`  // Graph highlight for critical
	CriticalValueTextColor  string  `json:"criticalValueTextColor"`  // Value text color for critical
	// Current alert state: "none", "warning", or "critical"
	CurrentAlertState string `json:"currentAlertState"`
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
	Group     bool     `json:"group"`
	Index     int      `json:"index"`
	Key       string   `json:"key"`
	Selection []string `json:"selection"`
	Value     string   `json:"value"`
	Checked   bool     `json:"checked"` // For checkbox inputs
}
