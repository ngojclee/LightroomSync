package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultRepository = "ngojclee/LightroomSync"
)

// CheckerOptions configures update release checking and download behavior.
type CheckerOptions struct {
	Repository       string
	LatestReleaseURL string
	HTTPClient       *http.Client
}

// Checker queries latest release metadata and downloads update assets.
type Checker struct {
	repository string
	latestURL  string
	client     *http.Client
}

// LatestRelease represents the latest known release data.
type LatestRelease struct {
	CurrentVersion string
	LatestVersion  string
	HasUpdate      bool
	ReleaseNotes   string
	ReleaseURL     string
	PublishedAt    string
	AssetName      string
	AssetURL       string
	CheckedAt      string
}

type releaseAPIResponse struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt string        `json:"published_at"`
	Assets      []releaseItem `json:"assets"`
}

type releaseItem struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// NewChecker constructs a release checker.
func NewChecker(opts CheckerOptions) *Checker {
	repo := strings.TrimSpace(opts.Repository)
	if repo == "" {
		repo = defaultRepository
	}

	latestURL := strings.TrimSpace(opts.LatestReleaseURL)
	if latestURL == "" {
		latestURL = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 2500 * time.Millisecond}
	}

	return &Checker{
		repository: repo,
		latestURL:  latestURL,
		client:     client,
	}
}

// CheckLatest retrieves release metadata and computes update availability.
func (c *Checker) CheckLatest(ctx context.Context, currentVersion string) (LatestRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.latestURL, nil)
	if err != nil {
		return LatestRelease{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LightroomSync-Agent")

	resp, err := c.client.Do(req)
	if err != nil {
		return LatestRelease{}, fmt.Errorf("request latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return LatestRelease{}, fmt.Errorf("latest release endpoint status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed releaseAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return LatestRelease{}, fmt.Errorf("decode release response: %w", err)
	}

	latestVersion := strings.TrimSpace(parsed.TagName)
	if latestVersion == "" {
		latestVersion = strings.TrimSpace(parsed.Name)
	}
	selected := selectPreferredAsset(parsed.Assets)

	info := LatestRelease{
		CurrentVersion: strings.TrimSpace(currentVersion),
		LatestVersion:  latestVersion,
		ReleaseNotes:   strings.TrimSpace(parsed.Body),
		ReleaseURL:     strings.TrimSpace(parsed.HTMLURL),
		PublishedAt:    strings.TrimSpace(parsed.PublishedAt),
		AssetName:      strings.TrimSpace(selected.Name),
		AssetURL:       strings.TrimSpace(selected.BrowserDownloadURL),
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	if info.CurrentVersion != "" && info.LatestVersion != "" {
		if cmp, err := CompareVersions(info.CurrentVersion, info.LatestVersion); err == nil && cmp < 0 {
			info.HasUpdate = true
		}
	}
	return info, nil
}

// DownloadToFile downloads an asset URL to destination path atomically.
func (c *Checker) DownloadToFile(ctx context.Context, assetURL, destinationPath string, onProgress func(downloaded, total int64)) error {
	if strings.TrimSpace(assetURL) == "" {
		return fmt.Errorf("asset url is required")
	}
	if strings.TrimSpace(destinationPath) == "" {
		return fmt.Errorf("destination path is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("User-Agent", "LightroomSync-Agent")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download status=%d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return fmt.Errorf("create download directory: %w", err)
	}

	tempPath := destinationPath + ".part"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("create temp download file: %w", err)
	}

	downloaded := int64(0)
	total := resp.ContentLength
	reader := io.TeeReader(resp.Body, writerFunc(func(p []byte) (int, error) {
		n := len(p)
		downloaded += int64(n)
		if onProgress != nil {
			onProgress(downloaded, total)
		}
		return n, nil
	}))

	_, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("download copy: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close temp download file: %w", closeErr)
	}

	if err := os.Rename(tempPath, destinationPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("rename temp download file: %w", err)
	}
	return nil
}

// ResolveAssetName resolves a filename for an asset URL.
func ResolveAssetName(assetURL, fallback string) string {
	name := strings.TrimSpace(fallback)
	if name != "" {
		return filepath.Base(name)
	}

	u, err := url.Parse(strings.TrimSpace(assetURL))
	if err != nil {
		return "lightroomsync_update.bin"
	}
	base := strings.TrimSpace(path.Base(u.Path))
	if base == "" || base == "." || base == "/" {
		return "lightroomsync_update.bin"
	}
	return base
}

func selectPreferredAsset(assets []releaseItem) releaseItem {
	if len(assets) == 0 {
		return releaseItem{}
	}

	bestIdx := 0
	bestScore := scoreAsset(assets[0])
	for i := 1; i < len(assets); i++ {
		score := scoreAsset(assets[i])
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return assets[bestIdx]
}

func scoreAsset(asset releaseItem) int {
	name := strings.ToLower(strings.TrimSpace(asset.Name))
	score := 0
	switch {
	case strings.HasSuffix(name, ".exe"):
		score += 40
	case strings.HasSuffix(name, ".msi"):
		score += 35
	case strings.HasSuffix(name, ".zip"):
		score += 20
	}
	if strings.Contains(name, "lightroomsync") || strings.Contains(name, "lightroom-sync") {
		score += 30
	}
	if strings.Contains(name, "setup") || strings.Contains(name, "installer") {
		score += 20
	}
	return score
}

type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) {
	return f(p)
}
