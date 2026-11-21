// SPDX-License-Identifier: Apache-2.0

package visualsearch

import (
	"net/url"
	"strings"
)

// fileURL turns a Windows path into a file:// URL. Spaces and other special
// characters are escaped, so the URL is a single command-line word.
func fileURL(path string) string {
	return (&url.URL{Scheme: "file", Path: "/" + strings.ReplaceAll(path, `\`, "/")}).String()
}

// splitCommand returns the program of a registry open command
// (`"C:\…\msedge.exe" --single-argument %1`) and the rest.
func splitCommand(cmd string) (exe, rest string) {
	cmd = strings.TrimSpace(cmd)
	if strings.HasPrefix(cmd, `"`) {
		if i := strings.Index(cmd[1:], `"`); i >= 0 {
			return cmd[1 : i+1], strings.TrimSpace(cmd[i+2:])
		}
		return "", ""
	}
	if i := strings.IndexByte(cmd, ' '); i >= 0 {
		return cmd[:i], strings.TrimSpace(cmd[i+1:])
	}
	return cmd, ""
}

// fillCommand puts target in place of %1 (or %L) exactly as the registered
// command has it: browsers register `--single-argument %1` (Chrome, Edge),
// which passes the rest of the line through literally, so added quotes would
// become part of the address. target is a URL, which has no spaces. Without
// a placeholder target is appended.
func fillCommand(cmd, target string) string {
	for _, ph := range []string{"%1", "%L", "%l"} {
		if strings.Contains(cmd, ph) {
			return strings.Replace(cmd, ph, target, 1)
		}
	}
	return cmd + " " + target
}
