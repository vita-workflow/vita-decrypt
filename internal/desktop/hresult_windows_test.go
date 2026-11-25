package desktop

import (
	"errors"
	"syscall"
	"testing"
)

const (
	// upperHalf is what may be left in the upper half of a 64-bit return register. It is 0 on 32-bit Windows.
	upperHalf = ^uintptr(0) &^ lowHalf
	// hresultFail is E_FAIL, a failure that nothing allows.
	hresultFail = 0x80004005
)

func TestIsHRESULT(t *testing.T) {
	tests := []struct {
		name    string
		result  uintptr
		code    uintptr
		wantIs  bool
		wantErr bool
	}{
		{name: "the same code matches", result: hresultOK, code: hresultOK, wantIs: true},
		{name: "the upper half is ignored", result: upperHalf | hresultCancelled, code: hresultCancelled, wantIs: true},
		{name: "another code doesn't match", result: hresultFalse, code: hresultOK, wantIs: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if is := isHRESULT(test.result, test.code); is != test.wantIs {
				t.Fatalf("isHRESULT(%#x, %#x) = %v, want %v", test.result, test.code, is, test.wantIs)
			}
		})
	}
}

func TestCheckHRESULT(t *testing.T) {
	tests := []struct {
		name        string
		result      uintptr
		wantErr     error
		wantErrText string
	}{
		{name: "S_OK is success", result: hresultOK},
		{name: "S_FALSE is success", result: hresultFalse},
		{
			name:        "a failure says what was being done",
			result:      upperHalf | hresultFail,
			wantErr:     syscall.Errno(hresultFail),
			wantErrText: "opening: " + syscall.Errno(hresultFail).Error(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := checkHRESULT(test.result, "opening")
			if !errors.Is(err, test.wantErr) || (err == nil) != (test.wantErr == nil) {
				t.Fatalf("checkHRESULT(%#x) = %v, want %v", test.result, err, test.wantErr)
			}
			if err != nil && err.Error() != test.wantErrText {
				t.Fatalf("the error says %q, want %q", err, test.wantErrText)
			}
		})
	}
}

func TestCheckHRESULTAllowing(t *testing.T) {
	tests := []struct {
		name    string
		result  uintptr
		wantErr error
	}{
		{name: "success is success", result: hresultOK},
		{name: "the allowed failure is success", result: hresultAlreadyRegistered},
		{name: "another failure is an error", result: hresultFail, wantErr: syscall.Errno(hresultFail)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := checkHRESULTAllowing(test.result, hresultAlreadyRegistered, "registering")
			if !errors.Is(err, test.wantErr) || (err == nil) != (test.wantErr == nil) {
				t.Fatalf("checkHRESULTAllowing(%#x) = %v, want %v", test.result, err, test.wantErr)
			}
		})
	}
}

func TestCheckCOMStart(t *testing.T) {
	errOther := errors.New("not an HRESULT")
	tests := []struct {
		name           string
		err            error
		wantWasStarted bool
		wantErr        error
	}{
		{name: "no error started COM", err: nil, wantWasStarted: true},
		{name: "S_FALSE started COM again", err: syscall.Errno(hresultFalse), wantWasStarted: true},
		{name: "another mode is used as it is", err: syscall.Errno(hresultChangedMode), wantWasStarted: false},
		{name: "a failure is an error", err: syscall.Errno(hresultFail), wantErr: syscall.Errno(hresultFail)},
		{name: "an error without a code is an error", err: errOther, wantErr: errOther},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wasStarted, err := checkCOMStart(test.err)
			isSameErr := errors.Is(err, test.wantErr) && (err == nil) == (test.wantErr == nil)
			if wasStarted != test.wantWasStarted || !isSameErr {
				t.Fatalf("checkCOMStart(%v) = %v, %v; want %v, %v",
					test.err, wasStarted, err, test.wantWasStarted, test.wantErr)
			}
		})
	}
}
