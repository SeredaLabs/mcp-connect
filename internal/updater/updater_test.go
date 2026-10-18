package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func updateFor(body string) *Update {
	name, _ := InstallerName(runtime.GOOS, runtime.GOARCH)
	h := sha256.Sum256([]byte(body))
	return &Update{Version: "0.4.99", Asset: Asset{Name: name, URL: "https://github.com/SeredaLabs/mcp-connect/releases/download/v0.4.99/" + name, Digest: "sha256:" + hex.EncodeToString(h[:]), Size: int64(len(body))}}
}
func TestSelection(t *testing.T) {
	for _, pair := range [][2]string{{"0.4.11", "0.4.10"}, {"0.10.0", "0.9.99"}, {"1.0.0", "0.99.99"}} {
		if !Newer(pair[0], pair[1]) {
			t.Fatal(pair)
		}
	}
	for _, v := range []string{"0.4.10", "0.4.9", "garbage", "0.04.99", "0.5.0-beta", "../9.0.0", "-1.0.0"} {
		if Newer(v, "0.4.10") {
			t.Fatal(v)
		}
	}
	for _, platform := range []string{"darwin", "windows", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			name, e := InstallerName(platform, arch)
			if e != nil {
				t.Fatal(e)
			}
			u := updateFor("installer")
			u.Asset.Name = name
			u.Asset.URL = "https://github.com/SeredaLabs/mcp-connect/releases/download/v0.4.99/" + name
			r := []Release{{Tag: "v0.4.99", Assets: []Asset{u.Asset}}, {Tag: "v9.0.0", Draft: true}}
			got, e := Select(r, "0.4.10", platform, arch)
			if e != nil || got == nil || got.Asset.Name != name {
				t.Fatal(got, e)
			}
			if same, e := Select(r, "0.4.99", platform, arch); same != nil || e != nil {
				t.Fatal(same, e)
			}
			r = append(r, Release{Tag: "v1.0.0"})
			if _, e = Select(r, "0.4.10", platform, arch); e == nil {
				t.Fatal("missing latest installer silently accepted")
			}
		}
	}
}
func TestDownloadIntegrity(t *testing.T) {
	for _, scenario := range []string{"valid", "tampered", "truncated", "oversized", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CACHE_HOME", home)
			t.Setenv("LOCALAPPDATA", home)
			body := "signed-by-hash-only-installer"
			u := updateFor(body)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			actual := body
			switch scenario {
			case "tampered":
				actual = strings.Repeat("x", len(body))
			case "truncated":
				actual = body[:3]
			case "oversized":
				actual = body + "x"
			case "cancelled":
				cancel()
			}
			c := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				if e := r.Context().Err(); e != nil {
					return nil, e
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(actual)), Header: make(http.Header)}, nil
			})}
			path, e := download(ctx, u, c)
			if scenario == "valid" {
				if e != nil {
					t.Fatal(e)
				}
				b, e := os.ReadFile(path)
				if e != nil || string(b) != body {
					t.Fatal(e)
				}
				return
			}
			if e == nil || path != "" {
				t.Fatal("unsafe installer retained", path, e)
			}
			cache, _ := os.UserCacheDir()
			entries, _ := os.ReadDir(filepath.Join(cache, "MCP Connect", "updates"))
			if len(entries) != 0 {
				t.Fatal("failed download not removed")
			}
		})
	}
}
func TestTrustAndPreferences(t *testing.T) {
	u := updateFor("installer")
	u.Asset.URL = "https://evil.example/installer"
	if validateAsset(u) == nil {
		t.Fatal("untrusted URL")
	}
	u = updateFor("installer")
	u.Asset.Digest = ""
	if validateAsset(u) == nil {
		t.Fatal("missing digest")
	}
	u = updateFor("installer")
	u.Asset.Size = maxInstallerSize + 1
	if validateAsset(u) == nil {
		t.Fatal("size limit")
	}
	for _, s := range []string{"http://github.com/a", "https://github.com.evil.test/a", "https://user@github.com/a", "https://github.com:8443/a"} {
		if trustedURL(s) {
			t.Fatal(s)
		}
	}
	r := &http.Request{URL: &url.URL{Scheme: "https", Host: "evil.example"}}
	if client(0).CheckRedirect(r, nil) == nil {
		t.Fatal("untrusted redirect")
	}
	dir := t.TempDir()
	if !CheckOnLaunch(dir) {
		t.Fatal("default")
	}
	if e := SetCheckOnLaunch(dir, false); e != nil || CheckOnLaunch(dir) {
		t.Fatal(e)
	}
	if e := SetCheckOnLaunch(dir, true); e != nil || !CheckOnLaunch(dir) {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "updates.json"), []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	if CheckOnLaunch(dir) {
		t.Fatal("malformed settings should disable automatic networking")
	}
	if _, e := InstallerName("linux", "386"); e == nil {
		t.Fatal("unsupported architecture")
	}
}
