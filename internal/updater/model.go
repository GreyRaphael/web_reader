package updater

// CheckResult represents the response for an update check request.
type CheckResult struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	ReleaseNotes   string `json:"releaseNotes"`
	PublishedAt    string `json:"publishedAt"`
	ReleaseURL     string `json:"releaseUrl"`
	AssetName      string `json:"assetName"`
	AssetSize      int64  `json:"assetSize"`
	CanAutoUpdate  bool   `json:"canAutoUpdate"`
	Reason         string `json:"reason,omitempty"`
}

// ApplyResult represents the response after triggering an update.
type ApplyResult struct {
	Status        string `json:"status"` // "success" | "error"
	Message       string `json:"message"`
	TargetVersion string `json:"targetVersion"`
}

type gitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type gitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Assets      []gitHubAsset `json:"assets"`
}
