// Package updater checks public releases without accessing connection profiles or secrets.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const releasesURL = "https://api.github.com/repos/SeredaLabs/mcp-connect/releases?per_page=100"
const maxInstallerSize int64 = 256 << 20

type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}
type Release struct {
	Tag    string  `json:"tag_name"`
	Draft  bool    `json:"draft"`
	Assets []Asset `json:"assets"`
}
type Update struct {
	Version string
	Asset   Asset
}

func versionParts(s string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if len(parts) != 3 {
		return out, errors.New("Invalid release version")
	}
	for i, p := range parts {
		n, e := strconv.Atoi(p)
		if e != nil || n < 0 || strconv.Itoa(n) != p {
			return out, errors.New("Invalid release version")
		}
		out[i] = n
	}
	return out, nil
}
func Newer(candidate, current string) bool {
	a, e := versionParts(candidate)
	if e != nil {
		return false
	}
	b, e := versionParts(current)
	if e != nil {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
func InstallerName(platform, arch string) (string, error) {
	if arch != "amd64" && arch != "arm64" {
		return "", errors.New("No installer for this processor")
	}
	stem := "mcp-connect-" + platform + "-" + arch
	switch platform {
	case "darwin":
		return stem + "-installer.pkg", nil
	case "windows":
		return stem + "-setup.exe", nil
	case "linux":
		return stem + ".deb", nil
	}
	return "", errors.New("No installer for this operating system")
}
func validateAsset(u *Update) error {
	if _, e := versionParts(u.Version); e != nil {
		return e
	}
	a := u.Asset
	if a.Size <= 0 || a.Size > maxInstallerSize {
		return errors.New("Invalid installer size")
	}
	hash, e := hex.DecodeString(strings.TrimPrefix(a.Digest, "sha256:"))
	if e != nil || len(hash) != sha256.Size || !strings.HasPrefix(a.Digest, "sha256:") {
		return errors.New("Release installer has no valid SHA-256 digest")
	}
	expected := "https://github.com/SeredaLabs/mcp-connect/releases/download/v" + u.Version + "/" + a.Name
	if a.URL != expected || filepath.Base(a.Name) != a.Name {
		return errors.New("Unexpected installer download URL")
	}
	return nil
}
func Select(releases []Release, current, platform, arch string) (*Update, error) {
	name, e := InstallerName(platform, arch)
	if e != nil {
		return nil, e
	}
	var latest *Release
	for i := range releases {
		r := &releases[i]
		if r.Draft || !strings.HasPrefix(r.Tag, "v") || !Newer(r.Tag, current) {
			continue
		}
		if latest == nil || Newer(r.Tag, latest.Tag) {
			latest = r
		}
	}
	if latest == nil {
		return nil, nil
	}
	for _, asset := range latest.Assets {
		if asset.Name == name {
			u := &Update{Version: strings.TrimPrefix(latest.Tag, "v"), Asset: asset}
			if e := validateAsset(u); e != nil {
				return nil, e
			}
			return u, nil
		}
	}
	return nil, errors.New("The newest release has no installer for this computer; check the download page")
}
func trustedURL(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	switch u.Hostname() {
	case "api.github.com", "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com", "github-releases.githubusercontent.com":
		return true
	}
	return false
}
func client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !trustedURL(r.URL.String()) {
			return errors.New("Untrusted update redirect")
		}
		return nil
	}}
}
func Check(ctx context.Context, current string) (*Update, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "MCP-Connect/"+current)
	resp, e := client(15 * time.Second).Do(req)
	if e != nil {
		return nil, errors.New("Cannot reach GitHub to check updates")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Update check unavailable (HTTP %d); try again later", resp.StatusCode)
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if e != nil || len(data) > 8<<20 {
		return nil, errors.New("Invalid release response")
	}
	var releases []Release
	if e := json.Unmarshal(data, &releases); e != nil {
		return nil, errors.New("Invalid release response")
	}
	return Select(releases, current, runtime.GOOS, runtime.GOARCH)
}

// Download saves only a complete installer with the exact published size and hash.
func Download(ctx context.Context, u *Update) (string, error) {
	return download(ctx, u, client(5*time.Minute))
}

func download(ctx context.Context, u *Update, httpClient *http.Client) (path string, err error) {
	if u == nil {
		return "", errors.New("No update selected")
	}
	if err = validateAsset(u); err != nil {
		return "", err
	}
	expected, err := InstallerName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	if u.Asset.Name != expected {
		return "", errors.New("Installer does not match this computer")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "MCP Connect", "updates")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	folder, err := os.MkdirTemp(dir, "update-"+u.Version+"-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(folder)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.Asset.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", errors.New("Installer download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Installer download unavailable (HTTP %d)", resp.StatusCode)
	}
	path = filepath.Join(folder, u.Asset.Name)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, u.Asset.Size+1))
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil || n != u.Asset.Size {
		return "", errors.New("Incomplete or oversized installer download")
	}
	if "sha256:"+hex.EncodeToString(h.Sum(nil)) != strings.ToLower(u.Asset.Digest) {
		return "", errors.New("Installer checksum mismatch; file removed")
	}
	if err = os.Chmod(path, 0700); err != nil {
		return "", err
	}
	return path, nil
}

// OpenInstaller delegates approval and replacement to the operating system.
func OpenInstaller(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("Installer path must be absolute")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("/usr/bin/open", path).Run()
	case "windows":
		cmd = exec.Command(path)
	case "linux":
		if _, e := os.Stat("/usr/bin/pkexec"); e != nil {
			return errors.New("System authorization helper is unavailable; install the verified .deb with your package manager")
		}
		if _, e := os.Stat("/usr/bin/apt-get"); e != nil {
			return errors.New("This installer requires an apt-based Linux distribution")
		}
		return exec.Command("/usr/bin/pkexec", "/usr/bin/apt-get", "install", "--assume-yes", "--", path).Run()
	default:
		return errors.New("Unsupported installer platform")
	}
	if e := cmd.Start(); e != nil {
		return errors.New("Cannot open the installer; open the downloaded file manually")
	}
	go cmd.Wait()
	return nil
}

type preferences struct {
	CheckOnLaunch bool `json:"check_on_launch"`
}

func CheckOnLaunch(dir string) bool {
	data, e := os.ReadFile(filepath.Join(dir, "updates.json"))
	if os.IsNotExist(e) {
		return true
	}
	if e != nil {
		return false
	}
	var p preferences
	if json.Unmarshal(data, &p) != nil {
		return false
	}
	return p.CheckOnLaunch
}
func SetCheckOnLaunch(dir string, enabled bool) error {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	data, _ := json.MarshalIndent(preferences{enabled}, "", "  ")
	f, e := os.CreateTemp(dir, ".updates-")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(data); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	// Windows cannot atomically replace an existing file with os.Rename.
	target := filepath.Join(dir, "updates.json")
	if runtime.GOOS == "windows" {
		return os.WriteFile(target, data, 0600)
	}
	return os.Rename(name, target)
}

// Installed distinguishes an installer-managed copy from a portable executable.
func Installed() bool {
	path, e := os.Executable()
	if e != nil {
		return false
	}
	path, e = filepath.EvalSymlinks(path)
	if e != nil {
		return false
	}
	switch runtime.GOOS {
	case "darwin":
		return path == "/Applications/MCP Connect.app/Contents/MacOS/mcp-connect"
	case "linux":
		return path == "/opt/mcp-connect/mcp-connect"
	case "windows":
		_, e := os.Stat(filepath.Join(filepath.Dir(path), "unins000.exe"))
		return e == nil
	}
	return false
}
