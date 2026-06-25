package embedding

// SidecarStatusOptions controls read-only shared sidecar inspection.
type SidecarStatusOptions struct {
	IncludeFootprint bool
}

// SidecarStatusReport is a read-only snapshot of local embedding sidecar state.
// It never starts the sidecar and does not change embedding contracts.
type SidecarStatusReport struct {
	SharedEnabled bool                 `json:"shared_enabled"`
	SocketDir     string               `json:"socket_dir"`
	BinaryFound   bool                 `json:"binary_found"`
	Binary        string               `json:"binary,omitempty"`
	Entries       []SidecarStatusEntry `json:"entries"`
	Warnings      []string             `json:"warnings,omitempty"`
}

type SidecarStatusEntry struct {
	Key             string  `json:"key"`
	State           string  `json:"state"`
	SocketPath      string  `json:"socket_path,omitempty"`
	LockPath        string  `json:"lock_path,omitempty"`
	SocketExists    bool    `json:"socket_exists"`
	LockExists      bool    `json:"lock_exists"`
	PID             int     `json:"pid,omitempty"`
	PPID            int     `json:"ppid,omitempty"`
	Model           string  `json:"model,omitempty"`
	CacheDir        string  `json:"cache_dir,omitempty"`
	RequestedDim    int     `json:"requested_dim"`
	DimLabel        string  `json:"dim_label"`
	IdleTimeoutSecs int     `json:"idle_timeout_secs,omitempty"`
	RSSKB           int64   `json:"rss_kb,omitempty"`
	VSZKB           int64   `json:"vsz_kb,omitempty"`
	FootprintMB     float64 `json:"footprint_mb,omitempty"`
	FootprintError  string  `json:"footprint_error,omitempty"`
	Command         string  `json:"command,omitempty"`
}
