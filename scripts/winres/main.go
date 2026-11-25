// Command winres writes the Windows resources that Go links into a windows/amd64 build: the app icon and version info,
// which Explorer shows under Properties > Details and Task Manager uses for the program's name.
//
//	go run ./scripts/winres -version VERSION -target gui   writes cmd/vita-decrypt-gui/rsrc_windows_amd64.syso
//	go run ./scripts/winres -version VERSION -target cli   writes rsrc_windows_amd64.syso (ignored by git)
//
// Run it from the repository root. It reads the icon from cmd/vita-decrypt-gui/winres/icon.ico and the copyright from
// NOTICE. The committed GUI file is made with -version dev; scripts/build.sh makes both for a release and puts the
// committed one back.
//
// The output is the same COFF object MinGW-w64's windres (binutils 2.41 and later) makes from the equivalent .rc file,
// byte for byte, so it's the same on every machine; the test checks that against windres's own output.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// Input files, relative to the repository root.
const (
	iconPath   = "cmd/vita-decrypt-gui/winres/icon.ico"
	noticePath = "NOTICE"
)

// The product and company named in both programs' version info.
const (
	productName = "Vita Decrypt"
	companyName = "Vita Workflow"
)

// Exit codes, the same ones the flag package uses when it exits on its own.
const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

// target is a program that gets Windows resources: where its .syso file goes and how its version info describes it.
type target struct {
	outputPath  string
	description string
	programName string
}

// targets maps each -target value to its program.
var targets = map[string]target{
	"gui": {"cmd/vita-decrypt-gui/rsrc_windows_amd64.syso", "Vita Decrypt", "vita-decrypt-gui"},
	"cli": {"rsrc_windows_amd64.syso", "Vita Decrypt command-line tool", "vita-decrypt"},
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run parses the command line, writes the resources, and returns the exit code.
func run(arguments []string, errorOutput io.Writer) int {
	flags := flag.NewFlagSet("winres", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	version := flags.String("version", "", "the version, such as 1.2.3, 1.2.3-rc.1, or dev")
	targetName := flags.String("target", "", "gui or cli")
	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		return exitUsage
	}
	if err := writeResources(*version, *targetName); err != nil {
		_, _ = fmt.Fprintln(errorOutput, "winres:", err) // nothing more can be done if stderr fails
		return exitFailure
	}
	return exitSuccess
}

// writeResources writes the .syso file for the target named targetName, with version as its version.
func writeResources(version, targetName string) error {
	program, ok := targets[targetName]
	if !ok {
		return errors.New("-target must be gui or cli")
	}
	icon, err := os.ReadFile(iconPath)
	if err != nil {
		return err
	}
	notice, err := os.ReadFile(noticePath)
	if err != nil {
		return err
	}
	copyright, err := copyrightLine(notice)
	if err != nil {
		return err
	}
	object, err := buildResourceObject(icon, versionDetails{
		version:     version,
		description: program.description,
		programName: program.programName,
		product:     productName,
		company:     companyName,
		copyright:   copyright,
	})
	if err != nil {
		return err
	}
	// The .syso is a build input that's committed to git, not a secret, so it's readable by everyone like any source.
	return os.WriteFile(program.outputPath, object, 0o644) //nolint:gosec // G306: a public build file, see above
}

// copyrightLine returns the first line of notice that starts with "Copyright ", without surrounding spaces.
func copyrightLine(notice []byte) (string, error) {
	for line := range strings.Lines(string(notice)) {
		if strings.HasPrefix(line, "Copyright ") {
			return strings.TrimSpace(line), nil
		}
	}
	return "", errors.New("NOTICE has no Copyright line")
}

// buildResourceObject returns the COFF object holding the icons from the .ico file icon (as icon group 1) and the
// version info.
func buildResourceObject(icon []byte, details versionDetails) ([]byte, error) {
	resources, err := parseICO(icon)
	if err != nil {
		return nil, err
	}
	versionInfo, err := encodeVersionInfo(details)
	if err != nil {
		return nil, err
	}
	resources = append(resources, resource{typeID: resourceTypeVersion, nameID: 1, data: versionInfo})
	return encodeCOFF(resources)
}
