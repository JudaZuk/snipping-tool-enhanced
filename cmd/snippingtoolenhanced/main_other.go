// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "Snipping Tool Enhanced runs on Windows 11.")
	os.Exit(1)
}
