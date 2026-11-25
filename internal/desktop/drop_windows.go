package desktop

import (
	"sync/atomic"

	"golang.org/x/sys/windows"
)

// DROPEFFECT values, which tell the source of a drag what the window will do with it.
const (
	dropEffectNone = 0 // DROPEFFECT_NONE
	dropEffectCopy = 1 // DROPEFFECT_COPY
)

// The interface IDs that the drop target answers to in QueryInterface.
var (
	iidUnknown    = windows.GUID{Data1: 0x00000000, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}} // IID_IUnknown
	iidDropTarget = windows.GUID{Data1: 0x00000122, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}} // IID_IDropTarget
)

// dropTarget is a COM IDropTarget implemented in Go. Its first field is the COM method table, which
// desktop_windows.go fills with callbacks that call the methods below. The methods run one at a time, on the window's
// thread.
type dropTarget struct {
	methods     *[7]uintptr
	references  atomic.Uint32
	handler     DropHandler
	isAccepting bool // whether the drag now over the window carries files
}

// queryInterface is IUnknown::QueryInterface: the target answers to IUnknown and IDropTarget.
func (target *dropTarget) queryInterface(interfaceID *windows.GUID, object **dropTarget) uintptr {
	if *interfaceID != iidUnknown && *interfaceID != iidDropTarget {
		*object = nil
		return hresultNoInterface
	}
	*object = target
	target.references.Add(1)
	return hresultOK
}

// addReference is IUnknown::AddRef.
func (target *dropTarget) addReference() uintptr { return uintptr(target.references.Add(1)) }

// release is IUnknown::Release. The target is never freed, so the count is only for COM's benefit. Adding
// ^uint32(0) subtracts one, as the sync/atomic documentation describes.
func (target *dropTarget) release() uintptr { return uintptr(target.references.Add(^uint32(0))) }

// enter is IDropTarget::DragEnter, given whether the drag carries files.
func (target *dropTarget) enter(hasFiles bool, effect *uint32) {
	target.isAccepting = hasFiles
	*effect &= target.allowedEffect()
	if target.isAccepting && target.handler.Enter != nil {
		target.handler.Enter()
	}
}

// over is IDropTarget::DragOver.
func (target *dropTarget) over(effect *uint32) { *effect &= target.allowedEffect() }

// leave is IDropTarget::DragLeave.
func (target *dropTarget) leave() {
	if target.isAccepting && target.handler.Leave != nil {
		target.handler.Leave()
	}
	target.isAccepting = false
}

// drop is IDropTarget::Drop, given the dropped paths.
func (target *dropTarget) drop(paths []string, effect *uint32) {
	if len(paths) == 0 {
		*effect = dropEffectNone
	} else {
		*effect &= dropEffectCopy
	}
	target.leave()
	if len(paths) > 0 && target.handler.Drop != nil {
		target.handler.Drop(paths)
	}
}

func (target *dropTarget) allowedEffect() uint32 {
	if target.isAccepting {
		return dropEffectCopy
	}
	return dropEffectNone
}

// droppedFileList is a dropped HDROP, read with DragQueryFileW.
type droppedFileList interface {
	count() uint32
	length(index uint32) uint32 // in UTF-16 code units, not counting the terminating NUL
	read(index uint32, buffer []uint16)
}

// maxPathLength is the longest path Windows allows, in UTF-16 code units.
const maxPathLength = 32767

// droppedPaths reads every path in files, skipping any that are empty or longer than a path can be.
func droppedPaths(files droppedFileList) []string {
	var paths []string
	for index := range files.count() {
		length := files.length(index)
		if length == 0 || length > maxPathLength {
			continue
		}
		buffer := make([]uint16, length+1)
		files.read(index, buffer)
		paths = append(paths, windows.UTF16ToString(buffer))
	}
	return paths
}
