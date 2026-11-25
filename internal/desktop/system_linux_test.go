package desktop

import (
	"errors"
	"reflect"
	"testing"

	"github.com/godbus/dbus/v5"
)

// fakeBus is a busConnection that records its method calls and answers them with reply.
type fakeBus struct {
	names    []string
	matchErr error
	signals  chan<- *dbus.Signal
	calls    []fakeCall
	reply    func(bus *fakeBus, call fakeCall) *dbus.Call
	isClosed bool
}

// fakeCall is one method call made on a fakeBus.
type fakeCall struct {
	destination string
	path        dbus.ObjectPath
	method      string
	arguments   []any
}

func (bus *fakeBus) Names() []string { return bus.names }

func (bus *fakeBus) AddMatchSignal(...dbus.MatchOption) error { return bus.matchErr }

func (bus *fakeBus) Signal(channel chan<- *dbus.Signal) { bus.signals = channel }

func (bus *fakeBus) Object(destination string, path dbus.ObjectPath) dbus.BusObject {
	return &fakeObject{bus: bus, destination: destination, path: path}
}

func (bus *fakeBus) Close() error {
	bus.isClosed = true
	return nil
}

// send delivers signals as the bus would. The channel holds signalBufferSize, which is all a test needs.
func (bus *fakeBus) send(signals ...*dbus.Signal) {
	for _, signal := range signals {
		bus.signals <- signal
	}
}

// fakeObject is a D-Bus object on a fakeBus. Only Call is implemented; the embedded interface is nil, so any other
// method panics.
type fakeObject struct {
	dbus.BusObject
	bus         *fakeBus
	destination string
	path        dbus.ObjectPath
}

func (object *fakeObject) Call(method string, _ dbus.Flags, arguments ...any) *dbus.Call {
	call := fakeCall{destination: object.destination, path: object.path, method: method, arguments: arguments}
	object.bus.calls = append(object.bus.calls, call)
	if object.bus.reply == nil {
		return &dbus.Call{}
	}
	return object.bus.reply(object.bus, call)
}

// fakePrograms stands in for the programs on the computer.
type fakePrograms struct {
	installed map[string]string // program name to path
	output    []byte
	runErr    error
	startErr  error
	runs      [][]string // the path and arguments of each run
	starts    [][]string // the path and arguments of each start
}

func (programs *fakePrograms) find(name string) (string, error) {
	if path, ok := programs.installed[name]; ok {
		return path, nil
	}
	return "", errors.New("not installed")
}

func (programs *fakePrograms) run(path string, arguments ...string) ([]byte, error) {
	programs.runs = append(programs.runs, append([]string{path}, arguments...))
	return programs.output, programs.runErr
}

func (programs *fakePrograms) start(path string, arguments ...string) error {
	programs.starts = append(programs.starts, append([]string{path}, arguments...))
	return programs.startErr
}

// fakeSystem is a linuxSystem connected to bus and programs. A nil bus means no session bus.
func fakeSystem(bus *fakeBus, programs *fakePrograms) linuxSystem {
	return linuxSystem{
		connectBus: func() (busConnection, error) {
			if bus == nil {
				return nil, errors.New("no session bus")
			}
			return bus, nil
		},
		findProgram:  programs.find,
		runProgram:   programs.run,
		startProgram: programs.start,
	}
}

func TestLinuxSystemShowDialog(t *testing.T) {
	errFailed := errors.New("zenity crashed")
	zenity := map[string]string{"zenity": "/usr/bin/zenity"}
	tests := []struct {
		name        string
		bus         *fakeBus
		programs    *fakePrograms
		wantPaths   []string
		wantErr     error
		wantZenity  bool
		wantClosed  bool
		wantErrText string
	}{
		{
			name:       "the portal's answer is used",
			bus:        answeringPortal(responseSuccess, "file:///home/a/x.age"),
			programs:   &fakePrograms{installed: zenity},
			wantPaths:  []string{"/home/a/x.age"},
			wantClosed: true,
		},
		{
			name:       "a failed portal isn't retried with zenity",
			bus:        answeringPortal(2),
			programs:   &fakePrograms{installed: zenity},
			wantErr:    errPortalFailed,
			wantClosed: true,
		},
		{
			name:       "no session bus falls back to zenity",
			bus:        nil,
			programs:   &fakePrograms{installed: zenity, output: []byte("/home/a/x.age\n")},
			wantPaths:  []string{"/home/a/x.age"},
			wantZenity: true,
		},
		{
			name:       "a bus without the portal falls back to zenity",
			bus:        &fakeBus{names: []string{":1.42"}, reply: failingCall},
			programs:   &fakePrograms{installed: zenity, output: []byte("/home/a/x.age\n")},
			wantPaths:  []string{"/home/a/x.age"},
			wantZenity: true,
			wantClosed: true,
		},
		{
			name:     "no portal and no zenity is ErrUnavailable",
			bus:      nil,
			programs: &fakePrograms{},
			wantErr:  ErrUnavailable,
		},
		{
			name:       "cancelling zenity returns nothing",
			bus:        nil,
			programs:   &fakePrograms{installed: zenity, runErr: fakeExitError{code: zenityCancelledExitCode}},
			wantZenity: true,
		},
		{
			name:        "a failed zenity returns its error",
			bus:         nil,
			programs:    &fakePrograms{installed: zenity, runErr: errFailed},
			wantErr:     errFailed,
			wantErrText: "zenity: zenity crashed",
			wantZenity:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths, err := fakeSystem(test.bus, test.programs).showDialog(Dialog{Title: "Open"}, singleFile)
			if !reflect.DeepEqual(paths, test.wantPaths) || !errors.Is(err, test.wantErr) {
				t.Fatalf("showDialog = %q, %v; want %q, %v", paths, err, test.wantPaths, test.wantErr)
			}
			if test.wantErrText != "" && err.Error() != test.wantErrText {
				t.Fatalf("the error says %q, want %q", err, test.wantErrText)
			}
			if ranZenity := len(test.programs.runs) > 0; ranZenity != test.wantZenity {
				t.Fatalf("ran zenity: %v, want %v", ranZenity, test.wantZenity)
			}
			if test.bus != nil && test.bus.isClosed != test.wantClosed {
				t.Fatalf("closed the bus: %v, want %v", test.bus.isClosed, test.wantClosed)
			}
		})
	}
}

func TestLinuxSystemShowDialog_RunsZenityWithItsArguments(t *testing.T) {
	programs := &fakePrograms{installed: map[string]string{"zenity": "/usr/bin/zenity"}}
	dialog := Dialog{Title: "Choose a folder", Folder: "/home/a"}
	if _, err := fakeSystem(nil, programs).showDialog(dialog, singleFolder); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{
		"/usr/bin/zenity", "--file-selection", "--title=Choose a folder", "--directory", "--filename=/home/a/",
	}}
	if !reflect.DeepEqual(programs.runs, want) {
		t.Fatalf("ran %q, want %q", programs.runs, want)
	}
}

func TestLinuxSystemReveal(t *testing.T) {
	errStart := errors.New("xdg-open is missing")
	tests := []struct {
		name       string
		bus        *fakeBus
		startErr   error
		wantStarts [][]string
		wantErr    error
	}{
		{
			name:       "the file manager shows the file",
			bus:        &fakeBus{},
			wantStarts: nil,
		},
		{
			name:       "a file manager that refuses falls back to xdg-open",
			bus:        &fakeBus{reply: failingCall},
			wantStarts: [][]string{{"xdg-open", "/home/a"}},
		},
		{
			name:       "no session bus falls back to xdg-open",
			bus:        nil,
			wantStarts: [][]string{{"xdg-open", "/home/a"}},
		},
		{
			name:       "a failed xdg-open returns its error",
			bus:        nil,
			startErr:   errStart,
			wantStarts: [][]string{{"xdg-open", "/home/a"}},
			wantErr:    errStart,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			programs := &fakePrograms{startErr: test.startErr}
			err := fakeSystem(test.bus, programs).reveal("/home/a/x y.age")
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("reveal returned %v, want %v", err, test.wantErr)
			}
			if !reflect.DeepEqual(programs.starts, test.wantStarts) {
				t.Fatalf("started %q, want %q", programs.starts, test.wantStarts)
			}
		})
	}
}

func TestLinuxSystemReveal_AsksTheFileManager(t *testing.T) {
	bus := &fakeBus{}
	if err := fakeSystem(bus, &fakePrograms{}).reveal("/home/a/x y.age"); err != nil {
		t.Fatal(err)
	}
	want := []fakeCall{{
		destination: fileManagerDestination,
		path:        fileManagerPath,
		method:      showItemsMethod,
		arguments:   []any{[]string{"file:///home/a/x%20y.age"}, ""},
	}}
	if !reflect.DeepEqual(bus.calls, want) {
		t.Fatalf("called %+v, want %+v", bus.calls, want)
	}
	if !bus.isClosed {
		t.Fatal("the bus was left open")
	}
}

func failingCall(*fakeBus, fakeCall) *dbus.Call {
	return &dbus.Call{Err: errors.New("org.freedesktop.DBus.Error.ServiceUnknown")}
}
