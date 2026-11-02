package library

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	lastInput   = windows.NewLazySystemDLL("user32.dll").NewProc("GetLastInputInfo")
	systemTimes = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemTimes")
)

// occupied says why the PC is not free for heavy work: "user" while somebody
// has touched the keyboard or mouse in the last five minutes, "busy" while
// other work keeps half its processors busy, and nothing when it is free.
// Windows keeps no load average and shows the GPU only through performance
// counters, so the processors are measured over one second and the GPU is not.
func occupied() string {
	input := struct{ size, at uint32 }{size: 8}
	if ok, _, _ := lastInput.Call(uintptr(unsafe.Pointer(&input))); ok != 0 &&
		uint32(windows.DurationSinceBoot().Milliseconds())-input.at < 5*60*1000 {
		return "user"
	}
	idle, total := cpuTimes()
	time.Sleep(time.Second)
	idle2, total2 := cpuTimes()
	if total2 > total && float64(idle2-idle)/float64(total2-total) <= 0.5 {
		return "busy"
	}
	return ""
}

// cpuTimes is the processors' idle time and all their time since boot, in 100
// ns ticks; kernel time already holds the idle time.
func cpuTimes() (idle, total uint64) {
	var i, k, u windows.Filetime
	systemTimes.Call(uintptr(unsafe.Pointer(&i)), uintptr(unsafe.Pointer(&k)), uintptr(unsafe.Pointer(&u)))
	ticks := func(f windows.Filetime) uint64 { return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime) }
	return ticks(i), ticks(k) + ticks(u)
}
