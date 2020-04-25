// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codereviewcfg

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"
)

// Config returns the code review config rooted at root.  Configs consist of
// lines of the form "key: value". Lines beginning with # are comments. If
// there is no config or the config is malformed, an error is returned.
func Config(root string) (map[string]string, error) {
	configPath := filepath.Join(root, "codereview.cfg")
	b, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %v: %v", configPath, err)
	}
	cfg := make(map[string]string)
	for _, line := range nonBlankLines(string(b)) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			// comment line
			continue
		}
		fields := strings.SplitN(line, ":", 2)
		if len(fields) != 2 {
			return nil, fmt.Errorf("bad config line in %v; expected 'key: value': %q", configPath, line)
		}
		cfg[strings.TrimSpace(fields[0])] = strings.TrimSpace(fields[1])
	}
	return cfg, nil
}

// lines returns the lines in text.
func lines(text string) []string {
	out := strings.Split(text, "\n")
	// Split will include a "" after the last line. Remove it.
	if n := len(out) - 1; n >= 0 && out[n] == "" {
		out = out[:n]
	}
	return out
}

// nonBlankLines returns the non-blank lines in text.
func nonBlankLines(text string) []string {
	var out []string
	for _, s := range lines(text) {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func GerritURLToServer(urlString string) (string, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL from %q: %v", urlString, err)
	}
	u.Path = ""
	return u.String(), nil
}

func GithubURLToParts(urlString string) (string, string, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse URL from %q: %v", urlString, err)
	}

	pathSplit := strings.Split(u.Path, "/")
	if len(pathSplit) != 3 {
		return "", "", fmt.Errorf("unexpected URL format %q", urlString)
	}
	return pathSplit[1], pathSplit[2], nil
}
