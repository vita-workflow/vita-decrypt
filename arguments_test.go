package main

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

func TestParseKeygenArguments(t *testing.T) {
	tests := []struct {
		name          string
		arguments     []string
		wantName      string
		wantIsClassic bool
		wantErr       error
	}{
		{"name alone is hybrid", []string{"--name", "Ops"}, "Ops", false, nil},
		{"classic after the name is classic", []string{"--name", "Ops", "--classic"}, "Ops", true, nil},
		{"classic before the name is classic", []string{"--classic", "--name", "Ops"}, "Ops", true, nil},
		{"a later name wins", []string{"--name", "Ops", "--name", "Dev"}, "Dev", false, nil},
		{"an option can be the name", []string{"--name", "--classic"}, "--classic", false, nil},
		{"no arguments give an empty name", nil, "", false, nil},
		{"name without a value returns errUsage", []string{"--name"}, "", false, errUsage},
		{"unknown option returns errUsage", []string{"--name", "Ops", "--force"}, "", false, errUsage},
		{"bare name returns errUsage", []string{"Ops"}, "", false, errUsage},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			name, isClassic, err := parseKeygenArguments(test.arguments)
			if !errors.Is(err, test.wantErr) || name != test.wantName || isClassic != test.wantIsClassic {
				t.Errorf("got %q, %v, %v; want %q, %v, %v",
					name, isClassic, err, test.wantName, test.wantIsClassic, test.wantErr)
			}
		})
	}
}

// FuzzParseKeygenArguments splits its input on spaces into keygen's arguments.
func FuzzParseKeygenArguments(f *testing.F) {
	f.Fuzz(func(t *testing.T, commandLine string) {
		arguments := strings.Split(commandLine, " ")
		name, isClassic, err := parseKeygenArguments(arguments)
		if err != nil {
			if !errors.Is(err, errUsage) || name != "" || isClassic {
				t.Fatalf("failure returned %q, %v, %v", name, isClassic, err)
			}
			return
		}
		if isClassic && !slices.Contains(arguments, "--classic") {
			t.Errorf("isClassic without --classic in %q", arguments)
		}
		if name != "" && !slices.Contains(arguments, name) {
			t.Errorf("name %q isn't one of the arguments %q", name, arguments)
		}
	})
}

func TestResolvePublicKey(t *testing.T) {
	const key = "age1lvyvwawkr0mcnnnncaghunadrqkmuf9e6507x9y920xxpp866cnql7dp2z"
	tests := []struct {
		name     string
		argument string
		wantKey  string
		wantErr  bool
	}{
		{"key itself is returned as given", key, key, false},
		{
			"key in capitals with spaces is returned as given", " " + strings.ToUpper(key) + "\n",
			" " + strings.ToUpper(key) + "\n", false,
		},
		{"invalid key fails", "age1notarealkey", "age1notarealkey", true},
		{"missing file fails", "missing-public-key.txt", "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			got, err := resolvePublicKey(test.argument)
			if got != test.wantKey || (err != nil) != test.wantErr {
				t.Errorf("got %q, %v; want %q, error %v", got, err, test.wantKey, test.wantErr)
			}
		})
	}
}

// FuzzResolvePublicKey gives resolvePublicKey keys, never file names, so the fuzzer reads no files.
func FuzzResolvePublicKey(f *testing.F) {
	f.Fuzz(func(t *testing.T, keyRest string) {
		argument := "age1" + keyRest
		key, err := resolvePublicKey(argument)
		if key != argument {
			t.Fatalf("returned %q for %q", key, argument)
		}
		if (err == nil) != (vita.CheckRecipient(argument) == nil) {
			t.Errorf("resolvePublicKey and vita.CheckRecipient disagree about %q: %v", argument, err)
		}
	})
}
