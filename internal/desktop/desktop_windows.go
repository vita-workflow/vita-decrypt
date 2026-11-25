package desktop

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This file only makes the COM and Win32 calls. The logic is in dialog_windows.go, drop_windows.go, and
// hresult_windows.go.
//
// The functions here that pass pointers to Windows are marked //nolint:gosec, because gosec flags every use of unsafe
// (G103) and unsafe.Pointer is the only way to make those calls. Each one is a few lines that only make the calls.

// CanDrop reports whether files can be dropped on the window.
const CanDrop = true

var (
	ole32   = windows.NewLazySystemDLL("ole32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procCoCreateInstance            = ole32.NewProc("CoCreateInstance")
	procOleInitialize               = ole32.NewProc("OleInitialize")
	procRegisterDragDrop            = ole32.NewProc("RegisterDragDrop")
	procReleaseStgMedium            = ole32.NewProc("ReleaseStgMedium")
	procDragQueryFileW              = shell32.NewProc("DragQueryFileW")
	procSHCreateItemFromParsingName = shell32.NewProc("SHCreateItemFromParsingName")
	procILCreateFromPathW           = shell32.NewProc("ILCreateFromPathW")
	procILFree                      = shell32.NewProc("ILFree")
	procSHOpenFolderAndSelectItems  = shell32.NewProc("SHOpenFolderAndSelectItems")

	clsidFileOpenDialog = windows.GUID{
		Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE,
		Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7},
	}
	iidFileOpenDialog = windows.GUID{
		Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768,
		Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60},
	}
	iidShellItem = windows.GUID{
		Data1: 0x43826D1E, Data2: 0xE718, Data3: 0x42EE,
		Data4: [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE},
	}

	// owner is the window that dialogs are modal to, set by SetOwner.
	owner atomic.Uintptr
)

// Method numbers in COM method tables, counting IUnknown's three methods first. Each is named after its interface and
// method.
const (
	unknownRelease          = 2  // IUnknown::Release
	fileDialogShow          = 3  // IModalWindow::Show
	fileDialogSetFileTypes  = 4  // IFileDialog::SetFileTypes
	fileDialogSetOptions    = 9  // IFileDialog::SetOptions
	fileDialogGetOptions    = 10 // IFileDialog::GetOptions
	fileDialogSetFolder     = 12 // IFileDialog::SetFolder
	fileDialogSetTitle      = 17 // IFileDialog::SetTitle
	fileDialogGetResult     = 20 // IFileDialog::GetResult
	fileDialogGetResults    = 27 // IFileOpenDialog::GetResults
	shellItemGetDisplayName = 5  // IShellItem::GetDisplayName
	itemArrayGetCount       = 7  // IShellItemArray::GetCount
	itemArrayGetItemAt      = 8  // IShellItemArray::GetItemAt
	dataObjectGetData       = 3  // IDataObject::GetData
	dataObjectQueryGetData  = 5  // IDataObject::QueryGetData
)

// Win32 values passed to the calls below. The names in the comments are the Win32 ones.
const (
	displayNameFileSystemPath   = 0x80058000 // SIGDN_FILESYSPATH
	classContextInProcessServer = 1          // CLSCTX_INPROC_SERVER
	clipboardFormatHDROP        = 15         // CF_HDROP
	aspectContent               = 1          // DVASPECT_CONTENT
	mediumHGLOBAL               = 1          // TYMED_HGLOBAL
	allDroppedFiles             = 0xFFFFFFFF // the index that makes DragQueryFileW return the number of files
)

// comObject is any COM interface pointer: a pointer to a method table.
type comObject struct{ methods *[32]uintptr }

// call calls a method. Like syscall.Proc.Call, it keeps anything its arguments point to alive and in place until it
// returns, so callers may pass uintptr(unsafe.Pointer(&local)).
//
//go:uintptrescapes
//nolint:gosec // a COM method takes a pointer to its object first
func (object *comObject) call(method int, arguments ...uintptr) uintptr {
	result, _, _ := syscall.SyscallN(object.methods[method],
		append([]uintptr{uintptr(unsafe.Pointer(object))}, arguments...)...)
	return result
}

func (object *comObject) release() {
	if object != nil {
		object.call(unknownRelease)
	}
}

// SetOwner makes dialogs modal to the window with this HWND.
func SetOwner(view uintptr) { owner.Store(view) }

// withCOM runs action on a locked thread with COM started for single-threaded use, as shell dialogs require.
func withCOM(action func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	wasStarted, err := checkCOMStart(
		windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE))
	if err != nil {
		return err
	}
	if wasStarted {
		defer windows.CoUninitialize()
	}
	return action()
}

func showDialog(dialog Dialog, kind selection) ([]string, error) {
	var paths []string
	err := withCOM(func() error {
		object, err := createFileDialog()
		if err != nil {
			return err
		}
		defer object.release()
		paths, err = runFileDialog(shellFileDialog{object}, dialog, kind, owner.Load())
		return err
	})
	return paths, err
}

//nolint:gosec // passing pointers to Windows
func createFileDialog() (*comObject, error) {
	var object *comObject
	result, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidFileOpenDialog)), 0,
		classContextInProcessServer, uintptr(unsafe.Pointer(&iidFileOpenDialog)), uintptr(unsafe.Pointer(&object)))
	return object, checkHRESULT(result, "creating the file dialog")
}

// shellFileDialog is the shell's IFileOpenDialog. Its methods make the COM calls that runFileDialog asks for.
type shellFileDialog struct{ object *comObject }

//nolint:gosec // passing a pointer to Windows
func (dialog shellFileDialog) options() uint32 {
	var options uint32
	dialog.object.call(fileDialogGetOptions, uintptr(unsafe.Pointer(&options)))
	return options
}

func (dialog shellFileDialog) setOptions(options uint32) {
	dialog.object.call(fileDialogSetOptions, uintptr(options))
}

//nolint:gosec // passing a pointer to Windows
func (dialog shellFileDialog) setTitle(title *uint16) {
	dialog.object.call(fileDialogSetTitle, uintptr(unsafe.Pointer(title)))
}

//nolint:gosec // passing a pointer to Windows
func (dialog shellFileDialog) setFileTypes(fileTypes []filterSpec) {
	dialog.object.call(fileDialogSetFileTypes, uintptr(len(fileTypes)), uintptr(unsafe.Pointer(&fileTypes[0])))
	runtime.KeepAlive(fileTypes)
}

// setFolder starts the dialog in the folder at path. If the folder can't be found, the dialog starts where it would
// have anyway.
//
//nolint:gosec // passing pointers to Windows
func (dialog shellFileDialog) setFolder(path *uint16) {
	var item *comObject
	result, _, _ := procSHCreateItemFromParsingName.Call(uintptr(unsafe.Pointer(path)), 0,
		uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&item)))
	if isFailure(result) {
		return
	}
	defer item.release()
	dialog.object.call(fileDialogSetFolder, uintptr(unsafe.Pointer(item)))
}

func (dialog shellFileDialog) show(owner uintptr) uintptr {
	return dialog.object.call(fileDialogShow, owner)
}

//nolint:gosec // passing pointers to Windows
func (dialog shellFileDialog) results() ([]string, error) {
	var items *comObject
	result := dialog.object.call(fileDialogGetResults, uintptr(unsafe.Pointer(&items)))
	if err := checkHRESULT(result, readingChoice); err != nil {
		return nil, err
	}
	defer items.release()
	var count uint32
	items.call(itemArrayGetCount, uintptr(unsafe.Pointer(&count)))
	return collectPaths(count, func(index uint32) (string, error) {
		var item *comObject
		result := items.call(itemArrayGetItemAt, uintptr(index), uintptr(unsafe.Pointer(&item)))
		if err := checkHRESULT(result, readingChoice); err != nil {
			return "", err
		}
		defer item.release()
		return itemPath(item)
	})
}

//nolint:gosec // passing a pointer to Windows
func (dialog shellFileDialog) result() (string, error) {
	var item *comObject
	result := dialog.object.call(fileDialogGetResult, uintptr(unsafe.Pointer(&item)))
	if err := checkHRESULT(result, readingChoice); err != nil {
		return "", err
	}
	defer item.release()
	return itemPath(item)
}

// itemPath is the file system path of an IShellItem.
//
//nolint:gosec // passing a pointer to Windows, and freeing the memory it returns
func itemPath(item *comObject) (string, error) {
	var path *uint16
	result := item.call(shellItemGetDisplayName, displayNameFileSystemPath, uintptr(unsafe.Pointer(&path)))
	if err := checkHRESULT(result, readingChoice); err != nil {
		return "", err
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(path))
	return windows.UTF16PtrToString(path), nil
}

// Reveal opens File Explorer at path's folder with path selected.
func Reveal(path string) error {
	pathUTF16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- withCOM(func() error { return openFolderAndSelect(path, pathUTF16) }) }()
	return <-done
}

//nolint:gosec // passing a pointer to Windows
func openFolderAndSelect(path string, pathUTF16 *uint16) error {
	itemList, _, _ := procILCreateFromPathW.Call(uintptr(unsafe.Pointer(pathUTF16)))
	if itemList == 0 {
		return fmt.Errorf("can't find %s", path)
	}
	defer func() { _, _, _ = procILFree.Call(itemList) }() // ILFree returns nothing
	result, _, _ := procSHOpenFolderAndSelectItems.Call(itemList, 0, 0, 0)
	return checkHRESULT(result, "opening File Explorer")
}

// windowDropTarget is the window's drop target. COM holds a pointer to it that the garbage collector can't see, so
// this variable keeps it alive for as long as the program runs.
var windowDropTarget = &dropTarget{methods: &dropTargetMethods}

// dropTargetMethods is the IDropTarget method table. The methods take a POINTL by value, declared here as one uintptr
// (point). That matches the amd64 and arm64 calling conventions, where the 8-byte struct fits one register; on 386
// it would take two slots and shift effect.
var dropTargetMethods = [7]uintptr{
	windows.NewCallback(func(target *dropTarget, interfaceID *windows.GUID, object **dropTarget) uintptr {
		return target.queryInterface(interfaceID, object)
	}),
	windows.NewCallback(func(target *dropTarget) uintptr { return target.addReference() }),
	windows.NewCallback(func(target *dropTarget) uintptr { return target.release() }),
	windows.NewCallback(func(target *dropTarget, data *comObject, keyState, point uintptr, effect *uint32) uintptr {
		target.enter(hasFiles(data), effect)
		return hresultOK
	}),
	windows.NewCallback(func(target *dropTarget, keyState, point uintptr, effect *uint32) uintptr {
		target.over(effect)
		return hresultOK
	}),
	windows.NewCallback(func(target *dropTarget) uintptr {
		target.leave()
		return hresultOK
	}),
	windows.NewCallback(func(target *dropTarget, data *comObject, keyState, point uintptr, effect *uint32) uintptr {
		target.drop(pathsDroppedIn(data), effect)
		return hresultOK
	}),
}

// AttachDrop lets files be dropped on the window with this HWND. It must run on the window's thread.
//
//nolint:gosec // passing a pointer to Windows; windowDropTarget is never moved or freed
func AttachDrop(view uintptr, handler DropHandler) error {
	windowDropTarget.handler = handler
	result, _, _ := procOleInitialize.Call(0)
	if err := checkHRESULTAllowing(result, hresultChangedMode, "starting OLE"); err != nil {
		return err
	}
	result, _, _ = procRegisterDragDrop.Call(view, uintptr(unsafe.Pointer(windowDropTarget)))
	return checkHRESULTAllowing(result, hresultAlreadyRegistered, "accepting dropped files")
}

// formatEtc is a FORMATETC, which names a format of dragged data.
type formatEtc struct {
	format       uint16
	targetDevice uintptr
	aspect       uint32
	index        int32
	mediumType   uint32
}

// storageMedium is a STGMEDIUM, which holds dragged data.
type storageMedium struct {
	mediumType       uint32
	data             uintptr
	releaseInterface uintptr
}

// hdropFormat asks for dragged files as an HDROP.
var hdropFormat = formatEtc{format: clipboardFormatHDROP, aspect: aspectContent, index: -1, mediumType: mediumHGLOBAL}

//nolint:gosec // passing a pointer to Windows
func hasFiles(data *comObject) bool {
	if data == nil {
		return false
	}
	return isHRESULT(data.call(dataObjectQueryGetData, uintptr(unsafe.Pointer(&hdropFormat))), hresultOK)
}

//nolint:gosec // passing pointers to Windows
func pathsDroppedIn(data *comObject) []string {
	if data == nil {
		return nil
	}
	var medium storageMedium
	result := data.call(dataObjectGetData, uintptr(unsafe.Pointer(&hdropFormat)), uintptr(unsafe.Pointer(&medium)))
	if !isHRESULT(result, hresultOK) {
		return nil
	}
	defer func() { _, _, _ = procReleaseStgMedium.Call(uintptr(unsafe.Pointer(&medium))) }() // returns nothing
	return droppedPaths(hdrop(medium.data))
}

// hdrop is a handle to dropped files. Its methods call DragQueryFileW.
type hdrop uintptr

func (files hdrop) count() uint32 { return files.query(allDroppedFiles, 0, 0) }

func (files hdrop) length(index uint32) uint32 { return files.query(uintptr(index), 0, 0) }

//nolint:gosec // passing a pointer to Windows
func (files hdrop) read(index uint32, buffer []uint16) {
	files.query(uintptr(index), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
}

//go:uintptrescapes
func (files hdrop) query(index, buffer, size uintptr) uint32 {
	result, _, _ := procDragQueryFileW.Call(uintptr(files), index, buffer, size)
	return uint32(result) //nolint:gosec // DragQueryFileW returns a 32-bit UINT; the register's upper half is undefined
}
