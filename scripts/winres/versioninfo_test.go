package main

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		wantVersion fileVersion
	}{
		{"release", "1.2.3", fileVersion{numbers: [4]uint16{1, 2, 3, 0}}},
		{"pre-release", "1.2.3-rc.1", fileVersion{numbers: [4]uint16{1, 2, 3, 0}, isPrerelease: true}},
		{"largest numbers", "65535.65535.65535", fileVersion{numbers: [4]uint16{65535, 65535, 65535, 0}}},
		{"leading zeros", "01.002.0003", fileVersion{numbers: [4]uint16{1, 2, 3, 0}}},
		{"dev", "dev", fileVersion{isPrerelease: true}},
		{"build metadata", "1.2.3+build.5", fileVersion{isPrerelease: true}},
		{"two numbers", "1.2", fileVersion{isPrerelease: true}},
	}
	for _, test := range tests {
		t.Run(test.name+" "+test.version+" is parsed", func(t *testing.T) {
			got, err := parseVersion(test.version)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.wantVersion {
				t.Errorf("parseVersion(%q) = %+v, want %+v", test.version, got, test.wantVersion)
			}
		})
	}
}

func TestParseVersion_RejectsBadInput(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantError string
	}{
		{"empty", "", "has characters other than"},
		{"space", "1.0.0 beta", "has characters other than"},
		{"quote", `1.0"`, "has characters other than"},
		{"non-ASCII digit", "1.2.٣", "has characters other than"},
		{"first number over 65535", "65536.0.0", "at most 65535"},
		{"last number over 65535", "1.0.99999999999999999999", "at most 65535"},
	}
	for _, test := range tests {
		t.Run(test.name+" returns an error", func(t *testing.T) {
			_, err := parseVersion(test.version)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Errorf("error = %v, want one containing %q", err, test.wantError)
			}
		})
	}
}

func TestEncodeVersionInfo_RejectsOversizedText(t *testing.T) {
	tests := []struct {
		name      string
		details   versionDetails
		wantError string
	}{
		{
			"copyright longer than its value length field",
			versionDetails{version: "1.0.0", copyright: strings.Repeat("c", 70000)},
			`length of the version info value "LegalCopyright" is 70001`,
		},
		{
			"version that makes the block longer than its length field",
			versionDetails{version: strings.Repeat("1", 20000)},
			`length of the version info block "040904b0"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name+" returns an error", func(t *testing.T) {
			_, err := encodeVersionInfo(test.details)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Errorf("error = %v, want one containing %q", err, test.wantError)
			}
		})
	}
}

// FuzzParseVersion checks that parseVersion never panics, that it accepts only the allowed characters, that what it
// accepts has a zero fourth number, and that its version info encodes. Seeds also live in
// testdata/fuzz/FuzzParseVersion.
func FuzzParseVersion(f *testing.F) {
	for _, seed := range []string{"1.2.3", "1.2.3-rc.1", "dev", "65536.0.0", "1.0 beta", "", "1.2.3+build"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, version string) {
		parsed, err := parseVersion(version)
		if err != nil {
			return
		}
		if !validVersion.MatchString(version) {
			t.Errorf("parseVersion accepted %q, which has characters it doesn't allow", version)
		}
		if parsed.numbers[3] != 0 {
			t.Errorf("parseVersion(%q) has fourth number %d, want 0", version, parsed.numbers[3])
		}
		if !semanticVersion.MatchString(version) && parsed != (fileVersion{isPrerelease: true}) {
			t.Errorf("parseVersion(%q) = %+v, want 0.0.0.0 flagged as a pre-release", version, parsed)
		}
		// Two copies of a version this long and the other strings stay well inside a block's 65535 bytes.
		if len(version) > 1000 {
			return
		}
		encoded, err := encodeVersionInfo(versionDetails{version: version})
		if err != nil {
			t.Fatalf("encoding version info for %q: %v", version, err)
		}
		if length := binary.LittleEndian.Uint16(encoded); int(length) != len(encoded) {
			t.Errorf("version info says it's %d bytes, but it's %d", length, len(encoded))
		}
	})
}
