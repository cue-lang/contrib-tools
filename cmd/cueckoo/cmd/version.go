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
	"path/filepath"
	runtimedebug "runtime/debug"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const (
	// checkInterval is how often we query the Go module proxy for a newer version.
	checkInterval = 24 * time.Hour

	moduleProxyURL = "https://proxy.golang.org/github.com/cue-lang/contrib-tools/@latest"
)

func newVersionCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "print cueckoo version information",
		RunE: mkRunE(c, func(cmd *Command, args []string) error {
			bi, ok := runtimedebug.ReadBuildInfo()
			if !ok {
				return fmt.Errorf("no build info available")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "cueckoo version %s\n", bi.Main.Version)
			return nil
		}),
	}
	return cmd
}

// checkForUpdate prints a warning to stderr if a newer version of cueckoo
// is available. It caches the latest known version in a file under UserCacheDir
// and only queries the module proxy at most once per day.
// Any errors are silently ignored to avoid disrupting normal usage.
func checkForUpdate() {
	bi, ok := runtimedebug.ReadBuildInfo()
	if !ok {
		return
	}
	curVersion := bi.Main.Version
	if !semver.IsValid(curVersion) {
		return // local build or unknown version
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	cacheFile := filepath.Join(cacheDir, "cueckoo", "latest.info")

	info, err := cachedProxyInfo(cacheFile)
	if err != nil {
		debugf("update check error: %v\n", err)
		return
	}
	cmp := semver.Compare(info.Version, curVersion)
	debugf("update check: current=%s latest=%s compare=%d\n", curVersion, info.Version, cmp)
	if cmp > 0 {
		fmt.Fprintf(os.Stderr, `
cueckoo: a newer version is available: %s (current: %s)

	go install github.com/cue-lang/contrib-tools/cmd/cueckoo@latest
`[1:], info.Version, curVersion)
	}
}

// cachedProxyInfo returns the proxy info, fetching from the proxy
// if the cache is stale or missing.
func cachedProxyInfo(cacheFile string) (*proxyInfo, error) {
	// Check if the cache is fresh enough.
	if stat, err := os.Stat(cacheFile); err == nil {
		if time.Since(stat.ModTime()) < checkInterval {
			debugf("update check: using cached version from %s\n", cacheFile)
			return readProxyInfo(cacheFile)
		}
	}

	// Fetch the latest info from the module proxy.
	debugf("update check: fetching latest version from %s\n", moduleProxyURL)
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

func readProxyInfo(path string) (*proxyInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info proxyInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func fetchProxyInfo() ([]byte, error) {
	resp, err := http.Get(moduleProxyURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}
