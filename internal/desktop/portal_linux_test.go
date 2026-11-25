package desktop

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

const (
	testUniqueName = ":1.42"
	testHandle     = dbus.ObjectPath("/org/freedesktop/portal/desktop/request/1_42/old_portal_7")
)

// answeringPortal is a bus whose portal answers OpenFile with a Response signal at the handle it returns.
func answeringPortal(code uint32, uris ...string) *fakeBus {
	return &fakeBus{
		names: []string{testUniqueName},
		reply: func(bus *fakeBus, _ fakeCall) *dbus.Call {
			bus.send(response(testHandle, code, uris))
			return openFileReply()
		},
	}
}

// response is a Response signal from the request at path.
func response(path dbus.ObjectPath, code uint32, uris []string) *dbus.Signal {
	return &dbus.Signal{Path: path, Name: responseSignal, Body: []any{code, chosen(uris...)}}
}

// chosen is the results dictionary of a Response signal, listing uris. Without uris, it has no "uris" entry.
func chosen(uris ...string) map[string]dbus.Variant {
	results := map[string]dbus.Variant{}
	if len(uris) > 0 {
		results["uris"] = dbus.MakeVariant(uris)
	}
	return results
}

// openFileReply is the reply to OpenFile: the path of the request object, testHandle.
func openFileReply() *dbus.Call { return &dbus.Call{Body: []any{testHandle}} }

// tokenOf reads handle_token from an OpenFile call.
func tokenOf(t *testing.T, call fakeCall) string {
	t.Helper()
	options, ok := call.arguments[2].(map[string]dbus.Variant)
	if !ok {
		t.Fatalf("OpenFile's options are a %T", call.arguments[2])
	}
	token, ok := options["handle_token"].Value().(string)
	if !ok {
		t.Fatalf("handle_token is a %T", options["handle_token"].Value())
	}
	return token
}

func TestAskPortal(t *testing.T) {
	otherPath := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/1_42/other")
	tests := []struct {
		name      string
		bus       *fakeBus
		wantPaths []string
		wantErr   error
	}{
		{
			name:      "an answer at the returned handle is read",
			bus:       answeringPortal(responseSuccess, "file:///home/a/x.age", "https://example.com/y.age"),
			wantPaths: []string{"/home/a/x.age"},
		},
		{
			name: "an answer at the requested path is read before OpenFile returns",
			bus: &fakeBus{names: []string{testUniqueName}, reply: func(bus *fakeBus, call fakeCall) *dbus.Call {
				bus.send(response(requestPath(testUniqueName, tokenOf(t, call)), responseSuccess,
					[]string{"file:///home/a/x.age"}))
				return openFileReply()
			}},
			wantPaths: []string{"/home/a/x.age"},
		},
		{
			name: "other signals are skipped",
			bus: &fakeBus{names: []string{testUniqueName}, reply: func(bus *fakeBus, _ fakeCall) *dbus.Call {
				bus.send(
					response(otherPath, responseSuccess, []string{"file:///home/a/wrong.age"}),
					&dbus.Signal{Path: testHandle, Name: "org.freedesktop.portal.Request.Other", Body: []any{0, 0}},
					&dbus.Signal{Path: testHandle, Name: responseSignal, Body: []any{uint32(0)}},
					response(testHandle, responseSuccess, []string{"file:///home/a/x.age"}),
				)
				return openFileReply()
			}},
			wantPaths: []string{"/home/a/x.age"},
		},
		{
			name:      "a cancelled dialog returns nothing",
			bus:       answeringPortal(responseCancelled),
			wantPaths: nil,
		},
		{
			name:    "a failed dialog returns errPortalFailed",
			bus:     answeringPortal(2),
			wantErr: errPortalFailed,
		},
		{
			name: "a lost connection returns errPortalUnreachable",
			bus: &fakeBus{names: []string{testUniqueName}, reply: func(bus *fakeBus, _ fakeCall) *dbus.Call {
				close(bus.signals)
				return openFileReply()
			}},
			wantErr: errPortalUnreachable,
		},
		{
			name:    "a bus without a name has no portal",
			bus:     &fakeBus{},
			wantErr: errNoPortal,
		},
		{
			name:    "a bus that refuses the match has no portal",
			bus:     &fakeBus{names: []string{testUniqueName}, matchErr: errors.New("refused")},
			wantErr: errNoPortal,
		},
		{
			name:    "a failed OpenFile means no portal",
			bus:     &fakeBus{names: []string{testUniqueName}, reply: failingCall},
			wantErr: errNoPortal,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths, err := askPortal(test.bus, Dialog{Title: "Open"}, singleFile)
			if !reflect.DeepEqual(paths, test.wantPaths) || !errors.Is(err, test.wantErr) {
				t.Fatalf("askPortal = %q, %v; want %q, %v", paths, err, test.wantPaths, test.wantErr)
			}
		})
	}
}

func TestAskPortal_CallsOpenFile(t *testing.T) {
	bus := answeringPortal(responseCancelled)
	if _, err := askPortal(bus, Dialog{Title: "Open a file"}, multipleFiles); err != nil {
		t.Fatal(err)
	}
	if len(bus.calls) != 1 {
		t.Fatalf("made %d calls, want 1", len(bus.calls))
	}
	call := bus.calls[0]
	token := tokenOf(t, call)
	if call.destination != portalDestination || call.path != portalPath || call.method != openFileMethod {
		t.Fatalf("called %s %s %s", call.destination, call.path, call.method)
	}
	if call.arguments[0] != "" || call.arguments[1] != "Open a file" {
		t.Fatalf("the parent and title are %q and %q", call.arguments[0], call.arguments[1])
	}
	if !reflect.DeepEqual(call.arguments[2], portalOptions(Dialog{}, multipleFiles, token)) {
		t.Fatalf("the options are %v", call.arguments[2])
	}
	if !strings.HasPrefix(token, "vita_decrypt_") {
		t.Fatalf("the token %q doesn't name the app", token)
	}
}

func TestAskPortal_UsesANewTokenEachTime(t *testing.T) {
	bus := answeringPortal(responseCancelled)
	for range 2 {
		if _, err := askPortal(bus, Dialog{}, singleFile); err != nil {
			t.Fatal(err)
		}
	}
	if first, second := tokenOf(t, bus.calls[0]), tokenOf(t, bus.calls[1]); first == second {
		t.Fatalf("both requests used the token %q", first)
	}
}

func TestPortalOptions(t *testing.T) {
	const requestName = "vita_decrypt_TEST"
	ageFilters := []Filter{{Name: "age files", Patterns: []string{"*.age", "*.AGE"}}}
	tests := []struct {
		name        string
		dialog      Dialog
		kind        selection
		wantOptions map[string]any
	}{
		{
			name: "one file without filters or folder",
			kind: singleFile,
			wantOptions: map[string]any{
				"handle_token": requestName, "modal": true, "multiple": false, "directory": false,
			},
		},
		{
			name: "several files",
			kind: multipleFiles,
			wantOptions: map[string]any{
				"handle_token": requestName, "modal": true, "multiple": true, "directory": false,
			},
		},
		{
			name:   "a folder starting somewhere",
			dialog: Dialog{Folder: "/home/a"},
			kind:   singleFolder,
			wantOptions: map[string]any{
				"handle_token": requestName, "modal": true, "multiple": false, "directory": true,
				"current_folder": []byte("/home/a\x00"),
			},
		},
		{
			name:   "filters end with All files",
			dialog: Dialog{Filters: ageFilters},
			kind:   singleFile,
			wantOptions: map[string]any{
				"handle_token": requestName, "modal": true, "multiple": false, "directory": false,
				"filters": []portalFilter{
					{
						Name:  "age files",
						Rules: []portalRule{{Kind: globRule, Pattern: "*.age"}, {Kind: globRule, Pattern: "*.AGE"}},
					},
					{Name: allFilesName, Rules: []portalRule{{Kind: globRule, Pattern: "*"}}},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := map[string]any{}
			for key, variant := range portalOptions(test.dialog, test.kind, requestName) {
				options[key] = variant.Value()
			}
			if !reflect.DeepEqual(options, test.wantOptions) {
				t.Fatalf("portalOptions = %v, want %v", options, test.wantOptions)
			}
		})
	}
}

func TestPortalOptions_FiltersHaveTheDBusSignature(t *testing.T) {
	dialog := Dialog{Filters: []Filter{{Name: "age files", Patterns: []string{"*.age"}}}}
	options := portalOptions(dialog, singleFile, "")
	if signature := options["filters"].Signature().String(); signature != "a(sa(us))" {
		t.Fatalf("the filters' signature is %s, want a(sa(us))", signature)
	}
}

func TestRequestPath(t *testing.T) {
	want := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/1_42/vita_decrypt_TOKEN")
	if path := requestPath(":1.42", "vita_decrypt_TOKEN"); path != want {
		t.Fatalf("requestPath = %s, want %s", path, want)
	}
}

func TestDecodeResponse(t *testing.T) {
	tests := []struct {
		name      string
		body      []any
		wantPaths []string
		wantErr   error
	}{
		{
			name:      "chosen file URIs become paths",
			body:      []any{uint32(0), chosen("file:///home/a/x%20y.age")},
			wantPaths: []string{"/home/a/x y.age"},
		},
		{
			name:      "a missing code reads as success",
			body:      []any{"zero", chosen("file:///home/a/x.age")},
			wantPaths: []string{"/home/a/x.age"},
		},
		{name: "a missing dictionary means nothing was chosen", body: []any{uint32(0), "none"}, wantPaths: nil},
		{name: "missing uris mean nothing was chosen", body: []any{uint32(0), chosen()}, wantPaths: nil},
		{
			name:      "uris that aren't strings mean nothing was chosen",
			body:      []any{uint32(0), map[string]dbus.Variant{"uris": dbus.MakeVariant(uint32(7))}},
			wantPaths: nil,
		},
		{name: "a cancelled dialog returns nothing", body: []any{uint32(1), nil}, wantPaths: nil},
		{name: "another code is a failure", body: []any{uint32(2), nil}, wantErr: errPortalFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths, err := decodeResponse(test.body)
			if !reflect.DeepEqual(paths, test.wantPaths) || !errors.Is(err, test.wantErr) {
				t.Fatalf("decodeResponse = %q, %v; want %q, %v", paths, err, test.wantPaths, test.wantErr)
			}
		})
	}
}
