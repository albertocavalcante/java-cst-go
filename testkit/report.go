package testkit

// Report is the machine-readable output of one M0 evidence run.
type Report struct {
	SchemaVersion int          `json:"schemaVersion"`
	Run           RunMetadata  `json:"run"`
	Cases         []CaseResult `json:"cases"`
}

// RunMetadata identifies the machine and immutable parser inputs.
type RunMetadata struct {
	GoVersion      string `json:"goVersion"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	CSTGoCommit    string `json:"cstGoCommit"`
	RuntimeVersion string `json:"runtimeVersion"`
	RuntimeCommit  string `json:"runtimeCommit"`
	GrammarCommit  string `json:"grammarCommit"`
}

// CaseResult records one fixture at one explicit Java level.
type CaseResult struct {
	ID                string   `json:"id"`
	Release           uint8    `json:"release"`
	Preview           bool     `json:"preview"`
	Bytes             int      `json:"bytes"`
	BackendNodes      int      `json:"backendNodes"`
	BackendErrors     int      `json:"backendErrors"`
	MissingNodes      int      `json:"missingNodes"`
	ConvertedElements int      `json:"convertedElements"`
	RoundTrip         bool     `json:"roundTrip"`
	SpanInvariants    bool     `json:"spanInvariants"`
	Panic             bool     `json:"panic"`
	ElapsedNS         int64    `json:"elapsedNs"`
	AllocatedBytes    uint64   `json:"allocatedBytes"`
	Classification    string   `json:"classification"`
	Notes             []string `json:"notes"`
}
