package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

static const void *StripView = &StripView;
static IMP firstMouse, needsKey;

static BOOL stripFirstMouse(id self, SEL cmd, NSEvent *event) {
	return objc_getAssociatedObject(self, StripView) != nil ||
		((BOOL (*)(id, SEL, NSEvent *))firstMouse)(self, cmd, event);
}

static BOOL stripNeedsKey(id self, SEL cmd) {
	return objc_getAssociatedObject(self, StripView) == nil &&
		((BOOL (*)(id, SEL))needsKey)(self, cmd);
}

// swap installs imp for sel on cls and returns what cls answered before,
// whether it defined sel itself or inherited it.
static IMP swap(Class cls, SEL sel, IMP imp) {
	Method m = class_getInstanceMethod(cls, sel);
	IMP before = method_getImplementation(m);
	if (!class_addMethod(cls, sel, imp, method_getTypeEncoding(m))) {
		method_setImplementation(m, imp);
	}
	return before;
}

// untouchable marks the strip's web view so that a click lands on the button
// under it the first time and the panel never becomes key: the keyboard stays
// with the meeting app. Every other web view answers as WebKit always did. The
// view is marked rather than given a class of its own: WebKit observes it with
// KVO, and KVO's generated class must stay its class.
static void untouchable(void *window) {
	WKWebView *view = [(NSWindow *)window valueForKey:@"webView"];
	if (view == nil) {
		return;
	}
	static dispatch_once_t once;
	dispatch_once(&once, ^{
		firstMouse = swap([WKWebView class], @selector(acceptsFirstMouse:), (IMP)stripFirstMouse);
		needsKey = swap([WKWebView class], @selector(needsPanelToBecomeKey), (IMP)stripNeedsKey);
	});
	objc_setAssociatedObject(view, StripView, @YES, OBJC_ASSOCIATION_RETAIN);
}
*/
import "C"

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// untouchable makes the strip take clicks but never the keyboard. WebKit's
// defaults would swallow the first click on an inactive panel, and Wails' own
// notch panel takes the keyboard on hover.
func untouchable(window *application.WebviewWindow) {
	window.OnWindowEvent(events.Mac.WebViewDidFinishNavigation, func(*application.WindowEvent) {
		application.InvokeSync(func() { C.untouchable(window.NativeWindow()) })
	})
}
