// SPDX-License-Identifier: Apache-2.0

//go:build genres

// genres writes the Windows resources (app icon, version info, manifest)
// that `go build` links into SnippingToolEnhanced.exe:
//
//	go run -tags genres tools/genres/main.go [version]
//
// It writes cmd/snippingtoolenhanced/rsrc_windows_{amd64,arm64}.syso and
// assets/SnippingToolEnhanced.ico.
package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"

	"snippingtoolenhanced/internal/icon"
)

func main() {
	ver := "1.5.1"
	if len(os.Args) > 1 {
		ver = os.Args[1]
	}
	var imgs []image.Image
	for _, s := range []int{256, 64, 48, 40, 32, 24, 20, 16} {
		imgs = append(imgs, icon.NRGBA(s))
	}
	ico, err := winres.NewIconFromImages(imgs)
	check(err)

	rs := &winres.ResourceSet{}
	check(rs.SetIcon(winres.ID(1), ico))
	rs.SetManifest(winres.AppManifest{
		Identity:            winres.AssemblyIdentity{Name: "SnippingToolEnhanced"},
		Description:         "Snipping Tool Enhanced",
		Compatibility:       winres.Win10AndAbove,
		ExecutionLevel:      winres.AsInvoker,
		DPIAwareness:        winres.DPIPerMonitorV2,
		UseCommonControlsV6: true, // Windows 11 look for message boxes and dialogs
		LongPathAware:       true,
	})
	var vi version.Info
	vi.SetFileVersion(ver)
	vi.SetProductVersion(ver)
	for k, v := range map[string]string{
		"ProductName":      "Snipping Tool Enhanced",
		"FileDescription":  "Snipping Tool Enhanced",
		"OriginalFilename": "SnippingToolEnhanced.exe",
		"LegalCopyright":   "Copyright 2026 The Snipping Tool Enhanced Authors. Apache License 2.0.",
	} {
		check(vi.Set(0x0409, k, v))
	}
	rs.SetVersionInfo(vi)

	for _, a := range []winres.Arch{winres.ArchAMD64, winres.ArchARM64} {
		f, err := os.Create(filepath.Join("cmd", "snippingtoolenhanced", fmt.Sprintf("rsrc_windows_%s.syso", a)))
		check(err)
		check(rs.WriteObject(f, a))
		check(f.Close())
	}
	check(os.MkdirAll("assets", 0o755))
	f, err := os.Create(filepath.Join("assets", "SnippingToolEnhanced.ico"))
	check(err)
	check(ico.SaveICO(f))
	check(f.Close())
	fmt.Println("resources written for version", ver)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "genres:", err)
		os.Exit(1)
	}
}
