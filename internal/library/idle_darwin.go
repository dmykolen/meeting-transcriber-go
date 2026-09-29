package library

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>

// property reads a number from the first IOKit service of a class, from inside
// one of its dictionaries when table is set; -1 when there is none. Neither
// read below needs a privacy permission.
static double property(const char *class, CFStringRef table, CFStringRef key) {
	io_service_t service = IOServiceGetMatchingService(kIOMainPortDefault, IOServiceMatching(class));
	if (!service) return -1;
	CFTypeRef value = IORegistryEntryCreateCFProperty(service, table ? table : key, kCFAllocatorDefault, 0);
	IOObjectRelease(service);
	if (!value) return -1;
	CFTypeRef number = value;
	if (table) number = CFGetTypeID(value) == CFDictionaryGetTypeID() ? CFDictionaryGetValue(value, key) : NULL;
	double out = -1;
	if (number && CFGetTypeID(number) == CFNumberGetTypeID()) CFNumberGetValue(number, kCFNumberDoubleType, &out);
	CFRelease(value);
	return out;
}

static double sinceInput(void) { return property("IOHIDSystem", NULL, CFSTR("HIDIdleTime")) / 1e9; }
static double gpuBusy(void) { return property("IOAccelerator", CFSTR("PerformanceStatistics"), CFSTR("Device Utilization %")); }
static double loadAverage(void) { double load; return getloadavg(&load, 1) == 1 ? load : -1; }
*/
import "C"

import "runtime"

// occupied says why the Mac is not free for heavy work: "user" while somebody
// has touched the keyboard, mouse or trackpad in the last five minutes, "busy"
// while other work keeps half its processors or its GPU busy, and nothing when
// it is free. The GPU figure is a moment, not an average: a spike only delays
// the next look by one pass of the queue.
func occupied() string {
	switch {
	case C.sinceInput() < 5*60:
		return "user"
	case float64(C.loadAverage()) >= float64(runtime.NumCPU())/2, C.gpuBusy() >= 50:
		return "busy"
	}
	return ""
}
