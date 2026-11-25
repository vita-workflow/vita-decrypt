package desktop

import (
	"errors"
	"fmt"
	"syscall"
)

// HRESULT values this package checks for. The names in the comments are the Win32 ones.
const (
	hresultOK                = 0          // S_OK
	hresultFalse             = 1          // S_FALSE: COM was already started on this thread, the same way
	hresultNoInterface       = 0x80004002 // E_NOINTERFACE
	hresultAlreadyRegistered = 0x80040101 // DRAGDROP_E_ALREADYREGISTERED
	hresultChangedMode       = 0x80010106 // RPC_E_CHANGED_MODE: COM was already started on this thread, another way
	hresultCancelled         = 0x800704C7 // HRESULT_FROM_WIN32(ERROR_CANCELLED)
)

const (
	// lowHalf masks the 32-bit value a Windows function returns. Its return register is 64 bits wide on amd64 and
	// arm64, and the upper half is undefined.
	lowHalf = 0xFFFFFFFF
	// severityBit is set in every failure HRESULT.
	severityBit = 0x80000000
)

// isHRESULT reports whether a call's raw result is the HRESULT code.
func isHRESULT(result, code uintptr) bool { return result&lowHalf == code }

// isFailure reports whether a call's raw result is a failure HRESULT.
func isFailure(result uintptr) bool { return result&severityBit != 0 }

// checkHRESULT turns a failure HRESULT into an error that says what was being done.
func checkHRESULT(result uintptr, operation string) error {
	if !isFailure(result) {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, syscall.Errno(result&lowHalf))
}

// checkHRESULTAllowing is checkHRESULT, except that the failure code allowed counts as success.
func checkHRESULTAllowing(result, allowed uintptr, operation string) error {
	if isHRESULT(result, allowed) {
		return nil
	}
	return checkHRESULT(result, operation)
}

// checkCOMStart reads the error from CoInitializeEx. wasStarted says whether this call started COM, so it must be
// stopped with CoUninitialize.
func checkCOMStart(err error) (wasStarted bool, startErr error) {
	var errno syscall.Errno
	switch {
	case err == nil:
		return true, nil
	case !errors.As(err, &errno):
		return false, fmt.Errorf("starting COM: %w", err)
	case isHRESULT(uintptr(errno), hresultFalse):
		return true, nil
	case isHRESULT(uintptr(errno), hresultChangedMode):
		// Already started differently on this thread; use it as it is.
		return false, nil
	default:
		return false, fmt.Errorf("starting COM: %w", err)
	}
}
