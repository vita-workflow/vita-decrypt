package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testCopyright is the copyright line the golden files were made with.
const testCopyright = "Copyright 2026 TAnderson Enterprises LLC, doing business as Vita Workflow"

// readTestIcon returns testdata/test.ico, the icon the golden files were made with.
func readTestIcon(tb testing.TB) []byte {
	tb.Helper()
	icon, err := os.ReadFile(filepath.Join("testdata", "test.ico"))
	if err != nil {
		tb.Fatal(err)
	}
	return icon
}

// readGolden returns the golden file called name from testdataFolder.
func readGolden(t *testing.T, testdataFolder, name string) []byte {
	t.Helper()
	golden, err := os.ReadFile(filepath.Join(testdataFolder, name))
	if err != nil {
		t.Fatal(err)
	}
	return golden
}

// The golden files are MinGW-w64 windres 2.45's output (Ubuntu 26.04's gcc-mingw-w64-x86-64) for the equivalent .rc
// file with testdata/test.ico as icon 1. To remake them, write that .rc (1 ICON plus the 1 VERSIONINFO block that
// buildResourceObject describes) and run:
//
//	x86_64-w64-mingw32-windres --target=pe-x86-64 -O coff -i FILE.rc -o FILE.golden
func TestBuildResourceObject(t *testing.T) {
	icon := readTestIcon(t)
	tests := []struct {
		name       string
		version    string
		targetName string
		golden     string
	}{
		{"pre-release sets the flag and the numbers", "1.2.3-rc.4", "gui", "gui-1.2.3-rc.4.golden"},
		{"dev is 0.0.0.0 with the pre-release flag", "dev", "gui", "gui-dev.golden"},
		{"release with the largest number", "65535.0.7", "cli", "cli-65535.0.7.golden"},
	}
	for _, test := range tests {
		t.Run(test.name+" matches windres", func(t *testing.T) {
			program := targets[test.targetName]
			got, err := buildResourceObject(icon, versionDetails{
				version: test.version, description: program.description, programName: program.programName,
				product: productName, company: companyName, copyright: testCopyright,
			})
			if err != nil {
				t.Fatal(err)
			}
			want := readGolden(t, "testdata", test.golden)
			if !bytes.Equal(got, want) {
				t.Errorf("output differs from windres's (%d bytes, want %d)", len(got), len(want))
			}
		})
	}
}

func TestBuildResourceObject_RejectsBadInput(t *testing.T) {
	icon := readTestIcon(t)
	tests := []struct {
		name      string
		icon      []byte
		version   string
		wantError string
	}{
		{"empty version", icon, "", "has characters other than"},
		{"version with a space", icon, "1.0.0 beta", "has characters other than"},
		{"version with a quote", icon, `1.0"`, "has characters other than"},
		{"version number over 65535", icon, "65536.0.0", "at most 65535"},
		{"no icon", nil, "1.0.0", "not an .ico file"},
		{"text instead of an icon", []byte("not an icon"), "1.0.0", "not an .ico file"},
		{"icon cut short", icon[:20], "1.0.0", "cut short"},
		{"version too long for its block", icon, strings.Repeat("a", 40000), "doesn't fit in 16 bits"},
		{"more icons than relocations", makeICO(65534), "1.0.0", "relocation count is 65536"},
	}
	for _, test := range tests {
		t.Run(test.name+" returns an error", func(t *testing.T) {
			_, err := buildResourceObject(test.icon, versionDetails{version: test.version})
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Errorf("error = %v, want one containing %q", err, test.wantError)
			}
		})
	}
}

// repositorySetup describes the files a test repository starts with.
type repositorySetup struct {
	notice          string // the NOTICE file's text; empty means the usual one
	isNoticeMissing bool
	isIconMissing   bool
	isOutputBlocked bool // a folder sits where the command-line tool's .syso goes, so writing it fails
}

// setUpRepository makes a temporary repository root with the files winres reads, and makes it the current folder.
func setUpRepository(t *testing.T, setup repositorySetup) {
	t.Helper()
	icon := readTestIcon(t)
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Dir(iconPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if !setup.isIconMissing {
		writeTestFile(t, iconPath, icon)
	}
	notice := setup.notice
	if notice == "" {
		notice = "vita-decrypt\n" + testCopyright + "\n\nThis product includes software under its own licenses.\n"
	}
	if !setup.isNoticeMissing {
		writeTestFile(t, noticePath, []byte(notice))
	}
	if setup.isOutputBlocked {
		if err := os.Mkdir(targets["cli"].outputPath, 0o750); err != nil {
			t.Fatal(err)
		}
	}
}

func writeTestFile(t *testing.T, path string, contents []byte) {
	t.Helper()
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRun(t *testing.T) {
	testdataFolder, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		arguments    []string
		setup        repositorySetup
		wantExitCode int
		wantGolden   string // the golden file the target's .syso must match; empty means nothing is written
		wantError    string // text that standard error must contain
	}{
		{
			name:         "gui pre-release writes the GUI's resources",
			arguments:    []string{"-version", "1.2.3-rc.4", "-target", "gui"},
			wantExitCode: exitSuccess,
			wantGolden:   "gui-1.2.3-rc.4.golden",
		},
		{
			name:         "gui dev writes the GUI's resources",
			arguments:    []string{"-version", "dev", "-target", "gui"},
			wantExitCode: exitSuccess,
			wantGolden:   "gui-dev.golden",
		},
		{
			name:         "cli release writes the command-line tool's resources",
			arguments:    []string{"-target", "cli", "-version", "65535.0.7"},
			setup:        repositorySetup{notice: "vita-decrypt\r\n" + testCopyright + " \t\r\n"},
			wantExitCode: exitSuccess,
			wantGolden:   "cli-65535.0.7.golden",
		},
		{
			name:         "help exits successfully",
			arguments:    []string{"-h"},
			wantExitCode: exitSuccess,
			wantError:    "-target",
		},
		{
			name:         "unknown flag is a usage error",
			arguments:    []string{"-output", "x"},
			wantExitCode: exitUsage,
			wantError:    "flag provided but not defined",
		},
		{
			name:         "unknown target fails",
			arguments:    []string{"-version", "1.0.0", "-target", "mac"},
			wantExitCode: exitFailure,
			wantError:    "winres: -target must be gui or cli",
		},
		{
			name:         "missing target fails",
			arguments:    []string{"-version", "1.0.0"},
			wantExitCode: exitFailure,
			wantError:    "-target must be gui or cli",
		},
		{
			name:         "bad version fails",
			arguments:    []string{"-version", "1.0 beta", "-target", "gui"},
			wantExitCode: exitFailure,
			wantError:    "has characters other than",
		},
		{
			name:         "missing icon fails",
			arguments:    []string{"-version", "1.0.0", "-target", "gui"},
			setup:        repositorySetup{isIconMissing: true},
			wantExitCode: exitFailure,
			wantError:    "icon.ico",
		},
		{
			name:         "missing NOTICE fails",
			arguments:    []string{"-version", "1.0.0", "-target", "gui"},
			setup:        repositorySetup{isNoticeMissing: true},
			wantExitCode: exitFailure,
			wantError:    "NOTICE",
		},
		{
			name:         "NOTICE without a copyright line fails",
			arguments:    []string{"-version", "1.0.0", "-target", "gui"},
			setup:        repositorySetup{notice: "vita-decrypt\n(c) someone\n"},
			wantExitCode: exitFailure,
			wantError:    "NOTICE has no Copyright line",
		},
		{
			name:         "unwritable output fails",
			arguments:    []string{"-version", "1.0.0", "-target", "cli"},
			setup:        repositorySetup{isOutputBlocked: true},
			wantExitCode: exitFailure,
			wantError:    "rsrc_windows_amd64.syso",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setUpRepository(t, test.setup)
			var errorOutput bytes.Buffer
			exitCode := run(test.arguments, &errorOutput)
			if exitCode != test.wantExitCode {
				t.Errorf("exit code = %d, want %d; standard error: %s", exitCode, test.wantExitCode,
					errorOutput.String())
			}
			if !strings.Contains(errorOutput.String(), test.wantError) {
				t.Errorf("standard error = %q, want it to contain %q", errorOutput.String(), test.wantError)
			}
			checkOutputs(t, testdataFolder, test.arguments, test.wantGolden)
		})
	}
}

// checkOutputs checks that the .syso of the target named in arguments matches wantGolden, and that no other .syso was
// written. An empty wantGolden means no .syso should have been written at all.
func checkOutputs(t *testing.T, testdataFolder string, arguments []string, wantGolden string) {
	t.Helper()
	for targetName, program := range targets {
		got, err := os.ReadFile(program.outputPath)
		isWanted := wantGolden != "" && strings.Contains(strings.Join(arguments, " "), "-target "+targetName)
		if !isWanted {
			if err == nil {
				t.Errorf("%s was written, want it left alone", program.outputPath)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if want := readGolden(t, testdataFolder, wantGolden); !bytes.Equal(got, want) {
			t.Errorf("%s differs from %s (%d bytes, want %d)", program.outputPath, wantGolden, len(got), len(want))
		}
	}
}
