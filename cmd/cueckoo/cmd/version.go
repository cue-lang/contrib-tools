// Copyright 2026 The CUE Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	runtimedebug "runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const (
	// checkInterval is how often we query the Go module proxy for a newer version.
	checkInterval = 24 * time.Hour

	modulePath = "github.com/cue-lang/contrib-tools"
)

func newVersionCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "print cueckoo version information",
		// Override the root's PersistentPreRun so that `cueckoo version`
		// and its subcommands don't trigger the auto-update check.
		PersistentPreRun: func(_ *cobra.Command, _ []string) {},
		RunE: mkRunE(c, func(cmd *Command, args []string) error {
			bi, ok := runtimedebug.ReadBuildInfo()
			if !ok {
				return fmt.Errorf("no build info available")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "cueckoo version %s\n", bi.Main.Version)
			return nil
		}),
	}
	cmd.AddCommand(newVersionUpdateCmd(c))
	return cmd
}

func newVersionUpdateCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "check for and install the latest version of cueckoo",
		RunE: mkRunE(c, func(cmd *Command, args []string) error {
			curVersion, latest, hasUpdate := checkForUpdate(true)
			if !hasUpdate {
				if curVersion == "" {
					return fmt.Errorf("no build info available")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "cueckoo: already up to date (%s)\n", curVersion)
				return nil
			}
			fmt.Fprintf(os.Stderr, "cueckoo: updating %s -> %s ...\n", curVersion, latest.Version)
			if err := installUpdate(latest.Version); err != nil {
				return fmt.Errorf("failed to install update: %w", err)
			}
			if exe, target, err := installTarget(); err == nil && exe != target {
				fmt.Fprintf(os.Stderr, "cueckoo: note: the running binary %s was not overwritten; the updated binary is at %s\n",
					exe, target)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "cueckoo: updated to %s\n", latest.Version)
			return nil
		}),
	}
	return cmd
}

// checkForUpdate checks if a newer version of cueckoo is available.
// If forceCheck is true, the cache is bypassed and the module proxy is always queried.
// It returns the current version, the latest proxy info, and whether an update is available.
// Any errors are silently ignored (logged via debugf) to avoid disrupting normal usage.
func checkForUpdate(forceCheck bool) (curVersion string, latest *proxyInfo, hasUpdate bool) {
	// Allow overriding the current version for testing.
	if v := os.Getenv("_CUECKOO_VERSION_OVERRIDE"); v != "" {
		curVersion = v
	} else {
		bi, ok := runtimedebug.ReadBuildInfo()
		if !ok {
			return "", nil, false
		}
		curVersion = bi.Main.Version
	}
	if !semver.IsValid(curVersion) {
		return curVersion, nil, false // local build or unknown version
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return curVersion, nil, false
	}
	cacheFile := filepath.Join(cacheDir, "cueckoo", "latest.info")

	info, err := cachedProxyInfo(cacheFile, forceCheck)
	if err != nil {
		debugf("update check error: %v\n", err)
		return curVersion, nil, false
	}
	cmp := semver.Compare(info.Version, curVersion)
	debugf("update check: current=%s latest=%s compare=%d\n", curVersion, info.Version, cmp)
	return curVersion, info, cmp > 0
}

// installUpdate installs the specified version of cueckoo via go install.
func installUpdate(version string) error {
	cmd := exec.Command("go", "install", "github.com/cue-lang/contrib-tools/cmd/cueckoo@"+version)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// reExec replaces the current process with a fresh invocation of the given
// binary. It sets _CUECKOO_SELF_UPDATED=1 so the new process skips the
// update check.
func reExec(exe string) error {
	os.Setenv("_CUECKOO_SELF_UPDATED", "1")
	return syscall.Exec(exe, os.Args, os.Environ())
}

// installTarget reports the path of the currently running binary and the path
// where `go install` would write the updated binary. When they differ,
// re-execing the current binary after install would just run the old code.
// TODO(mvdan): simplify once https://github.com/golang/go/issues/23439
// is resolved so that `go env GOBIN` always gives a directory.
func installTarget() (exe, target string, err error) {
	exe, err = os.Executable()
	if err != nil {
		return "", "", err
	}
	out, err := exec.Command("go", "env", "GOBIN", "GOPATH").Output()
	if err != nil {
		return exe, "", err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if gobin := lines[0]; gobin != "" {
		return exe, filepath.Join(gobin, "cueckoo"), nil
	}
	if gopath := lines[1]; gopath != "" {
		return exe, filepath.Join(gopath, "bin", "cueckoo"), nil
	}
	return exe, "", fmt.Errorf("neither GOBIN nor GOPATH is set")
}

// cachedProxyInfo returns the proxy info, fetching from the proxy
// if the cache is stale or missing. If forceCheck is true, the cache is ignored.
func cachedProxyInfo(cacheFile string, forceCheck bool) (*proxyInfo, error) {
	// Check if the cache is fresh enough.
	if !forceCheck {
		if stat, err := os.Stat(cacheFile); err == nil && time.Since(stat.ModTime()) < checkInterval {
			debugf("update check: using cached version from %s\n", cacheFile)
			if data, err := os.ReadFile(cacheFile); err == nil {
				var info proxyInfo
				if err := json.Unmarshal(data, &info); err == nil {
					return &info, nil
				}
			}
		}
	}

	// Fetch the latest info from the module proxy.
	data, err := fetchProxyInfo()
	if err != nil {
		return nil, err
	}

	// Validate before caching.
	var info proxyInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	if !semver.IsValid(info.Version) {
		return nil, fmt.Errorf("invalid version %q", info.Version)
	}

	// Write the cache file, creating the directory if needed.
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0o777); err != nil {
		return nil, err
	}
	if err := os.WriteFile(cacheFile, data, 0o666); err != nil {
		return nil, err
	}
	return &info, nil
}

type proxyInfo struct {
	Version string
}

// proxyBaseURL returns the module proxy base URL,
// using GOPROXY if set, falling back to proxy.golang.org.
func proxyBaseURL() string {
	base := os.Getenv("GOPROXY")
	if base == "" {
		return "https://proxy.golang.org"
	}
	// GOPROXY can be comma or pipe separated; use only the first entry.
	if i := strings.IndexAny(base, ",|"); i >= 0 {
		base = base[:i]
	}
	return strings.TrimRight(base, "/")
}

// fetchProxyInfo fetches the latest version info from the module proxy's @latest endpoint.
func fetchProxyInfo() ([]byte, error) {
	url := proxyBaseURL() + "/" + modulePath + "/@latest"
	debugf("update check: fetching latest info from %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}
