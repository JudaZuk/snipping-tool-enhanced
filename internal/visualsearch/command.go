// SPDX-License-Identifier: Apache-2.0

package visualsearch

import "strings"

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

// fillCommand puts target in place of %1 (or %L); without a placeholder it is
// appended. An unquoted placeholder gets quotes.
func fillCommand(cmd, target string) string {
	for _, ph := range []string{`"%1"`, `"%L"`, `"%l"`} {
		if strings.Contains(cmd, ph) {
			return strings.Replace(cmd, ph, `"`+target+`"`, 1)
		}
	}
	for _, ph := range []string{"%1", "%L", "%l"} {
		if strings.Contains(cmd, ph) {
			return strings.Replace(cmd, ph, `"`+target+`"`, 1)
		}
	}
	return cmd + ` "` + target + `"`
}
