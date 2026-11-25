package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// allowedNetImporters may import net: the D-Bus client (Linux file chooser) and golang.org/x/sys/windows need it for
// local sockets and types.
var allowedNetImporters = map[string]bool{
	"github.com/godbus/dbus/v5": true,
	"golang.org/x/sys/windows":  true,
}

// TestNoNetworkPackages keeps the app offline on every platform it's built for. The only network-adjacent packages
// allowed are net/url and net/netip (parsing only), and net itself where the D-Bus client (Linux file chooser) or
// golang.org/x/sys/windows needs it for local sockets and types. HTTP, TLS, and every other net/* package are banned.
func TestNoNetworkPackages(t *testing.T) {
	for _, target := range []struct{ goos, cgo string }{{"linux", "1"}, {"windows", "0"}, {"darwin", "1"}} {
		command := exec.Command("go", "list", "-deps", "-f", `{{.ImportPath}}{{range .Imports}} {{.}}{{end}}`, ".")
		command.Env = append(os.Environ(), "GOOS="+target.goos, "CGO_ENABLED="+target.cgo)
		output, err := command.Output()
		if err != nil {
			t.Skipf("go list unavailable for %s: %v", target.goos, err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			fields := strings.Fields(line)
			for _, problem := range networkProblems(fields[0], fields[1:]) {
				t.Errorf("%s: %s", target.goos, problem)
			}
		}
	}
}

// networkProblems lists what's wrong with one package in the build graph, given the packages it imports.
func networkProblems(importPath string, imports []string) []string {
	var problems []string
	if isBannedNetworkPackage(importPath) {
		problems = append(problems, "network package "+importPath+" is in the build graph")
	}
	for _, imported := range imports {
		if imported == "net" && !allowedNetImporters[importPath] && !strings.HasPrefix(importPath, "net/") {
			problems = append(problems, importPath+" imports net")
		}
	}
	return problems
}

func isBannedNetworkPackage(importPath string) bool {
	return strings.HasPrefix(importPath, "net/") && importPath != "net/url" && importPath != "net/netip" ||
		strings.HasPrefix(importPath, "crypto/tls")
}

func TestNetworkProblems(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		imports    []string
		want       int
	}{
		{"plain package", "example.com/tool", []string{"fmt"}, 0},
		{"url parsing", "net/url", nil, 0},
		{"http", "net/http", nil, 1},
		{"tls", "crypto/tls", nil, 1},
		{"imports net", "example.com/tool", []string{"net"}, 1},
		{"allowed net importer", "golang.org/x/sys/windows", []string{"net"}, 0},
		{"net subpackage importing net", "net/netip", []string{"net"}, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := networkProblems(test.importPath, test.imports); len(got) != test.want {
				t.Errorf("got %q, want %d problems", got, test.want)
			}
		})
	}
}
