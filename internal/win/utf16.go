// SPDX-License-Identifier: Apache-2.0

package win

import "unicode/utf16"

// MultiString encodes a Win32 double-NUL-terminated string list, such as
// the file-type filter of the Open and Save dialogs ("PNG\x00*.png\x00…").
// windows.UTF16FromString cannot be used: it rejects embedded NULs.
func MultiString(s string) []uint16 {
	u := utf16.Encode([]rune(s))
	for len(u) > 0 && u[len(u)-1] == 0 {
		u = u[:len(u)-1]
	}
	return append(u, 0, 0)
}
