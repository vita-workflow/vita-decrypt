// Command coverage merges Go coverage profiles from several operating systems and fails when the module, or any one
// of its packages, is below the minimum statement coverage. A statement counts as covered when any profile ran it, so
// platform-specific files are covered by the run on their own platform.
//
// Usage, from the module root:
//
//	go run ./scripts/coverage [-minimum 90] [-exclusions .github/coverage-exclusions.txt] PROFILE...
//
// The exclusions file lists one file per line, relative to the module root, followed by "# reason". Generated files
// (those with a "Code generated ... DO NOT EDIT." line) are always excluded. See docs/coding-standards.md section 5.1.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// Exit codes.
const (
	exitPassed       = 0
	exitBelowMinimum = 1
	exitUsage        = 2
)

func run(arguments []string, stdout, stderr io.Writer) int {
	settings, err := parseSettings(arguments, stderr)
	if err != nil {
		return exitUsage
	}
	summary, err := buildReport(settings)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "coverage:", err)
		return exitUsage
	}
	if !summary.print(stdout, settings.minimum) {
		return exitBelowMinimum
	}
	return exitPassed
}

type settings struct {
	minimum        float64
	exclusionsPath string
	profilePaths   []string
}

var errNoProfiles = errors.New("give at least one coverage profile")

// parseSettings parses the command line. Errors are written to stderr.
func parseSettings(arguments []string, stderr io.Writer) (settings, error) {
	flags := flag.NewFlagSet("coverage", flag.ContinueOnError)
	flags.SetOutput(stderr)
	minimum := flags.Float64("minimum", 90, "minimum statement coverage, in percent, for the module and each package")
	exclusionsPath := flags.String("exclusions", ".github/coverage-exclusions.txt", "file listing excluded files")
	if err := flags.Parse(arguments); err != nil {
		return settings{}, err
	}
	if flags.NArg() == 0 {
		_, _ = fmt.Fprintln(stderr, "coverage:", errNoProfiles)
		return settings{}, errNoProfiles
	}
	return settings{minimum: *minimum, exclusionsPath: *exclusionsPath, profilePaths: flags.Args()}, nil
}

// buildReport merges the profiles and totals them by package, leaving out excluded and generated files.
func buildReport(options settings) (report, error) {
	module, err := readModulePath("go.mod")
	if err != nil {
		return report{}, err
	}
	excluded, err := readExclusions(options.exclusionsPath)
	if err != nil {
		return report{}, err
	}
	blocks := make(map[string]block)
	for _, profilePath := range options.profilePaths {
		if err := mergeProfile(profilePath, blocks); err != nil {
			return report{}, err
		}
	}
	isExcluded := func(file string) bool {
		relativePath := strings.TrimPrefix(file, module+"/")
		return excluded[relativePath] || isGenerated(relativePath)
	}
	return summarize(blocks, isExcluded), nil
}

// A block is one basic block of a coverage profile: a run of statements that either all ran or all didn't.
type block struct {
	file       string
	statements int
	wasRun     bool
}

// mergeProfile adds the blocks in the profile at profilePath to blocks. A block run by any profile stays run.
func mergeProfile(profilePath string, blocks map[string]block) error {
	profile, err := os.Open(profilePath)
	if err != nil {
		return err
	}
	defer func() { _ = profile.Close() }() // read-only, so closing can't lose data

	scanner := bufio.NewScanner(profile)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if lineNumber == 1 && strings.HasPrefix(line, "mode: ") || line == "" {
			continue
		}
		position, parsed, err := parseBlock(line)
		if err != nil {
			return fmt.Errorf("%s:%d: %w", profilePath, lineNumber, err)
		}
		if existing, ok := blocks[position]; ok {
			parsed.wasRun = parsed.wasRun || existing.wasRun
		}
		blocks[position] = parsed
	}
	return scanner.Err()
}

var errMalformedLine = errors.New("not a coverage profile line")

// parseBlock parses a profile line such as "example.com/module/file.go:12.34,15.2 3 1" into the block's position
// ("example.com/module/file.go:12.34,15.2") and the block.
func parseBlock(line string) (string, block, error) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return "", block{}, errMalformedLine
	}
	position := fields[0]
	colon := strings.LastIndex(position, ":")
	if colon <= 0 {
		return "", block{}, errMalformedLine
	}
	statements, statementsErr := strconv.Atoi(fields[1])
	runCount, runCountErr := strconv.Atoi(fields[2])
	if statementsErr != nil || runCountErr != nil {
		return "", block{}, errMalformedLine
	}
	return position, block{file: position[:colon], statements: statements, wasRun: runCount > 0}, nil
}

// tally counts statements: all of them, and those that ran.
type tally struct{ covered, total int }

func (count tally) percent() float64 {
	if count.total == 0 {
		return 100
	}
	return 100 * float64(count.covered) / float64(count.total)
}

type report struct {
	packages map[string]tally
	module   tally
}

func summarize(blocks map[string]block, isExcluded func(file string) bool) report {
	summary := report{packages: make(map[string]tally)}
	for _, current := range blocks {
		if isExcluded(current.file) {
			continue
		}
		packagePath := path.Dir(current.file)
		packageTally := summary.packages[packagePath]
		packageTally.total += current.statements
		summary.module.total += current.statements
		if current.wasRun {
			packageTally.covered += current.statements
			summary.module.covered += current.statements
		}
		summary.packages[packagePath] = packageTally
	}
	return summary
}

// print writes one line per package and one for the module, and reports whether all of them meet the minimum.
func (summary report) print(output io.Writer, minimum float64) bool {
	packagePaths := make([]string, 0, len(summary.packages))
	for packagePath := range summary.packages {
		packagePaths = append(packagePaths, packagePath)
	}
	sort.Strings(packagePaths)

	allPassed := true
	printLine := func(name string, count tally) {
		verdict := "ok"
		if count.percent() < minimum {
			verdict = fmt.Sprintf("FAIL (minimum %.1f%%)", minimum)
			allPassed = false
		}
		_, _ = fmt.Fprintf(output, "%6.1f%%  %5d/%-5d  %s  %s\n",
			count.percent(), count.covered, count.total, name, verdict)
	}
	for _, packagePath := range packagePaths {
		printLine(packagePath, summary.packages[packagePath])
	}
	printLine("total", summary.module)
	return allPassed
}

// readModulePath returns the module path declared in the go.mod file at goModPath.
func readModulePath(goModPath string) (string, error) {
	contents, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}
	for line := range strings.Lines(string(contents)) {
		if modulePath, found := strings.CutPrefix(strings.TrimSpace(line), "module "); found {
			return strings.Trim(strings.TrimSpace(modulePath), `"`), nil
		}
	}
	return "", fmt.Errorf("%s has no module line", goModPath)
}

// readExclusions reads the exclusions file at exclusionsPath. Each entry must give a reason.
func readExclusions(exclusionsPath string) (map[string]bool, error) {
	contents, err := os.ReadFile(exclusionsPath)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	excluded := make(map[string]bool)
	lineNumber := 0
	for line := range strings.Lines(string(contents)) {
		lineNumber++
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		file, reason, _ := strings.Cut(line, "#")
		if strings.TrimSpace(reason) == "" {
			return nil, fmt.Errorf("%s:%d: %q needs a reason after a #", exclusionsPath, lineNumber, line)
		}
		excluded[strings.TrimSpace(file)] = true
	}
	return excluded, nil
}

var generatedMarker = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

// isGenerated reports whether the file at relativePath is generated code. A file that can't be read isn't.
func isGenerated(relativePath string) bool {
	contents, err := os.ReadFile(relativePath)
	return err == nil && generatedMarker.Match(contents)
}
