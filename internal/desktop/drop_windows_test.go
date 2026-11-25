package desktop

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// dropEffectMove is DROPEFFECT_MOVE, which a drag source may offer besides copying.
const dropEffectMove = 2

// recordingTarget is a dropTarget whose handler records each call as a line in events.
func recordingTarget(events *[]string) *dropTarget {
	return &dropTarget{handler: DropHandler{
		Enter: func() { *events = append(*events, "enter") },
		Leave: func() { *events = append(*events, "leave") },
		Drop:  func(paths []string) { *events = append(*events, "drop "+strings.Join(paths, ",")) },
	}}
}

func TestDropTargetQueryInterface(t *testing.T) {
	otherInterface := windows.GUID{Data1: 0x12345678}
	tests := []struct {
		name           string
		interfaceID    windows.GUID
		wantResult     uintptr
		wantIsTarget   bool
		wantReferences uint32
	}{
		{
			name:           "IUnknown is answered",
			interfaceID:    iidUnknown,
			wantResult:     hresultOK,
			wantIsTarget:   true,
			wantReferences: 1,
		},
		{
			name:           "IDropTarget is answered",
			interfaceID:    iidDropTarget,
			wantResult:     hresultOK,
			wantIsTarget:   true,
			wantReferences: 1,
		},
		{name: "another interface is refused", interfaceID: otherInterface, wantResult: hresultNoInterface},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := &dropTarget{}
			object := &dropTarget{}
			result := target.queryInterface(&test.interfaceID, &object)
			var wantObject *dropTarget
			if test.wantIsTarget {
				wantObject = target
			}
			if result != test.wantResult || object != wantObject {
				t.Fatalf("queryInterface = %#x with object %p; want %#x", result, object, test.wantResult)
			}
			if references := target.references.Load(); references != test.wantReferences {
				t.Fatalf("%d references, want %d", references, test.wantReferences)
			}
		})
	}
}

func TestDropTargetReferences(t *testing.T) {
	target := &dropTarget{}
	if count := target.addReference(); count != 1 {
		t.Fatalf("AddRef returned %d, want 1", count)
	}
	if count := target.addReference(); count != 2 {
		t.Fatalf("AddRef returned %d, want 2", count)
	}
	if count := target.release(); count != 1 {
		t.Fatalf("Release returned %d, want 1", count)
	}
}

func TestDropTargetDrag(t *testing.T) {
	tests := []struct {
		name            string
		hasFiles        bool
		wantEnterEffect uint32
		wantOverEffect  uint32
		wantEvents      []string
	}{
		{
			name:            "a drag with files is copied",
			hasFiles:        true,
			wantEnterEffect: dropEffectCopy,
			wantOverEffect:  dropEffectCopy,
			wantEvents:      []string{"enter", "leave"},
		},
		{
			name:            "a drag without files is refused",
			hasFiles:        false,
			wantEnterEffect: dropEffectNone,
			wantOverEffect:  dropEffectNone,
			wantEvents:      nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var events []string
			target := recordingTarget(&events)
			effect := uint32(dropEffectCopy | dropEffectMove)
			target.enter(test.hasFiles, &effect)
			if effect != test.wantEnterEffect {
				t.Fatalf("DragEnter effect %d, want %d", effect, test.wantEnterEffect)
			}
			effect = dropEffectCopy | dropEffectMove
			target.over(&effect)
			if effect != test.wantOverEffect {
				t.Fatalf("DragOver effect %d, want %d", effect, test.wantOverEffect)
			}
			target.leave()
			target.leave()
			if !reflect.DeepEqual(events, test.wantEvents) {
				t.Fatalf("events %q, want %q", events, test.wantEvents)
			}
		})
	}
}

func TestDropTargetDrop(t *testing.T) {
	tests := []struct {
		name       string
		paths      []string
		wantEffect uint32
		wantEvents []string
	}{
		{
			name:       "dropped files are copied and handed over",
			paths:      []string{`C:\a\x.age`, `C:\a\y.age`},
			wantEffect: dropEffectCopy,
			wantEvents: []string{"enter", "leave", `drop C:\a\x.age,C:\a\y.age`},
		},
		{
			name:       "a drop without readable files is refused",
			paths:      nil,
			wantEffect: dropEffectNone,
			wantEvents: []string{"enter", "leave"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var events []string
			target := recordingTarget(&events)
			effect := uint32(dropEffectCopy | dropEffectMove)
			target.enter(true, &effect)
			effect = dropEffectCopy | dropEffectMove
			target.drop(test.paths, &effect)
			if effect != test.wantEffect {
				t.Fatalf("Drop effect %d, want %d", effect, test.wantEffect)
			}
			if !reflect.DeepEqual(events, test.wantEvents) {
				t.Fatalf("events %q, want %q", events, test.wantEvents)
			}
		})
	}
}

func TestDropTarget_WithoutAHandlerDoesNothing(t *testing.T) {
	target := &dropTarget{}
	effect := uint32(dropEffectCopy)
	target.enter(true, &effect)
	target.drop([]string{`C:\a\x.age`}, &effect)
	if effect != dropEffectCopy {
		t.Fatalf("Drop effect %d, want %d", effect, dropEffectCopy)
	}
}

// fakeDroppedFiles is a droppedFileList holding paths. lengths overrides the length reported for an index.
type fakeDroppedFiles struct {
	paths   []string
	lengths map[uint32]uint32
}

func (files fakeDroppedFiles) count() uint32 {
	return uint32(len(files.paths)) //nolint:gosec // a test holds a few paths
}

func (files fakeDroppedFiles) length(index uint32) uint32 {
	if length, ok := files.lengths[index]; ok {
		return length
	}
	return uint32(len(utf16.Encode([]rune(files.paths[index])))) //nolint:gosec // a test path is short
}

func (files fakeDroppedFiles) read(index uint32, buffer []uint16) {
	copy(buffer, utf16.Encode([]rune(files.paths[index])))
}

func TestDroppedPaths(t *testing.T) {
	tests := []struct {
		name      string
		files     fakeDroppedFiles
		wantPaths []string
	}{
		{name: "no files have no paths", files: fakeDroppedFiles{}, wantPaths: nil},
		{
			name:      "every path is read",
			files:     fakeDroppedFiles{paths: []string{`C:\a\x.age`, `C:\a\ü 😀.age`}},
			wantPaths: []string{`C:\a\x.age`, `C:\a\ü 😀.age`},
		},
		{
			name:      "an empty path is skipped",
			files:     fakeDroppedFiles{paths: []string{"", `C:\a\x.age`}},
			wantPaths: []string{`C:\a\x.age`},
		},
		{
			name: "a path longer than Windows allows is skipped",
			files: fakeDroppedFiles{
				paths:   []string{`C:\a\x.age`, `C:\a\y.age`},
				lengths: map[uint32]uint32{0: maxPathLength + 1},
			},
			wantPaths: []string{`C:\a\y.age`},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if paths := droppedPaths(test.files); !reflect.DeepEqual(paths, test.wantPaths) {
				t.Fatalf("droppedPaths = %q, want %q", paths, test.wantPaths)
			}
		})
	}
}
