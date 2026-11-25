//go:build darwin && cgo

package desktop

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit

#import <AppKit/AppKit.h>
#include <objc/runtime.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

extern void vitaDragEntered(void);
extern void vitaDragExited(void);
extern void vitaDrop(char *paths);

static NSDictionary *fileURLsOnly(void) {
	return @{NSPasteboardURLReadingFileURLsOnlyKey: @YES};
}

static NSDragOperation vitaDraggingEntered(id self, SEL selector, id<NSDraggingInfo> info) {
	if ([[info draggingPasteboard] canReadObjectForClasses:@[[NSURL class]] options:fileURLsOnly()]) {
		vitaDragEntered();
		return NSDragOperationCopy;
	}
	return NSDragOperationNone;
}

static NSDragOperation vitaDraggingUpdated(id self, SEL selector, id<NSDraggingInfo> info) {
	if ([[info draggingPasteboard] canReadObjectForClasses:@[[NSURL class]] options:fileURLsOnly()]) {
		return NSDragOperationCopy;
	}
	return NSDragOperationNone;
}

static void vitaDraggingExited(id self, SEL selector, id<NSDraggingInfo> info) {
	vitaDragExited();
}

static BOOL vitaPrepareForDragOperation(id self, SEL selector, id<NSDraggingInfo> info) {
	return YES;
}

static BOOL vitaPerformDragOperation(id self, SEL selector, id<NSDraggingInfo> info) {
	NSArray<NSURL *> *urls = [[info draggingPasteboard] readObjectsForClasses:@[[NSURL class]] options:fileURLsOnly()];
	NSMutableString *paths = [NSMutableString string];
	for (NSURL *url in urls) {
		if (url.isFileURL && url.path != nil) {
			[paths appendString:url.path];
			[paths appendString:@"\n"];
		}
	}
	vitaDragExited();
	vitaDrop((char *)[paths UTF8String]);
	return YES;
}

static void addMethod(Class viewClass, SEL selector, IMP implementation, const char *returnType,
		const char *argumentType) {
	char types[64];
	snprintf(types, sizeof types, "%s@:%s", returnType, argumentType);
	class_addMethod(viewClass, selector, implementation, types);
}

static void vitaAttachDrop(uintptr_t viewRef) {
	NSView *view = (__bridge NSView *)(void *)viewRef;
	Class viewClass = [view class];
	addMethod(viewClass, @selector(draggingEntered:), (IMP)vitaDraggingEntered, @encode(NSDragOperation), "@");
	addMethod(viewClass, @selector(draggingUpdated:), (IMP)vitaDraggingUpdated, @encode(NSDragOperation), "@");
	addMethod(viewClass, @selector(draggingExited:), (IMP)vitaDraggingExited, @encode(void), "@");
	addMethod(viewClass, @selector(prepareForDragOperation:), (IMP)vitaPrepareForDragOperation, @encode(BOOL), "@");
	addMethod(viewClass, @selector(performDragOperation:), (IMP)vitaPerformDragOperation, @encode(BOOL), "@");
	[view registerForDraggedTypes:@[NSPasteboardTypeFileURL]];
}

// vitaOpenPanel shows an open panel on the main thread and returns the chosen paths separated by newlines, or NULL if
// the person cancelled. The caller frees the result.
static char *vitaOpenPanel(const char *title, bool multiple, bool chooseFolder, const char *startFolder) {
	NSString *message = [NSString stringWithUTF8String:title];
	NSString *start = [NSString stringWithUTF8String:startFolder];
	__block char *result = NULL;
	void (^run)(void) = ^{
		NSOpenPanel *panel = [NSOpenPanel openPanel];
		panel.message = message;
		panel.canChooseFiles = !chooseFolder;
		panel.canChooseDirectories = chooseFolder;
		panel.canCreateDirectories = chooseFolder;
		panel.allowsMultipleSelection = multiple;
		if (start.length > 0) {
			panel.directoryURL = [NSURL fileURLWithPath:start isDirectory:YES];
		}
		if ([panel runModal] != NSModalResponseOK) {
			return;
		}
		NSMutableString *paths = [NSMutableString string];
		for (NSURL *url in panel.URLs) {
			if (url.path != nil) {
				[paths appendString:url.path];
				[paths appendString:@"\n"];
			}
		}
		result = strdup([paths UTF8String]);
	};
	if ([NSThread isMainThread]) {
		run();
		return result;
	}
	// Run the panel from the main run loop rather than the main dispatch queue: the queue is serial, so a modal panel
	// inside a main-queue block would hold back Gio's own main-queue work (redraws, wakeups) until it closed.
	dispatch_semaphore_t done = dispatch_semaphore_create(0);
	CFRunLoopPerformBlock(CFRunLoopGetMain(), kCFRunLoopCommonModes, ^{
		run();
		dispatch_semaphore_signal(done);
	});
	CFRunLoopWakeUp(CFRunLoopGetMain());
	dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
	return result;
}
*/
import "C"

import (
	"os/exec"
	"unsafe"
)

// This file only makes the AppKit calls. Drag events go through dropQueue, in drop_darwin.go.

// CanDrop reports whether files can be dropped on the window.
const CanDrop = true

// SetOwner does nothing on macOS: open panels are modal to the whole app.
func SetOwner(view uintptr) {}

// AttachDrop lets files be dropped on the window's NSView. It must run on the main thread.
func AttachDrop(view uintptr, handler DropHandler) error {
	drops.handler = handler
	C.vitaAttachDrop(C.uintptr_t(view))
	return nil
}

// Reveal shows path selected in a Finder window.
func Reveal(path string) error {
	// open is a fixed system program and path is a single argument to it. No shell is involved.
	return exec.Command("open", "-R", path).Run()
}

func showDialog(dialog Dialog, kind selection) ([]string, error) {
	// dialog.Filters is ignored: an open panel has no filter menu, so restricting it would hide a file with an
	// unexpected name with no way to show it.
	title, startFolder := C.CString(dialog.Title), C.CString(dialog.Folder)
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(startFolder))
	chosen := C.vitaOpenPanel(title, C.bool(kind == multipleFiles), C.bool(kind == singleFolder), startFolder)
	if chosen == nil {
		return nil, nil // cancelled
	}
	defer C.free(unsafe.Pointer(chosen))
	return nonEmptyLines(C.GoString(chosen)), nil
}
