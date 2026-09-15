//go:build !darwin

package audio

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/gen2brain/malgo"
)

// Outside macOS the system stream is another miniaudio capture device. Windows
// uses loopback; Linux looks for a PipeWire/PulseAudio monitor source.
type systemAudio struct {
	ctx    *malgo.AllocatedContext
	dev    *malgo.Device
	out    chan []int16
	closed sync.Once
}

func (s *systemAudio) Samples() <-chan []int16 { return s.out }

func (s *systemAudio) Close() error {
	s.closed.Do(func() {
		s.dev.Uninit()
		_ = s.ctx.Uninit()
		s.ctx.Free()
		close(s.out)
	})
	return nil
}

func openSystemAudio() (Device, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("audio backend: %w", err)
	}

	cfg := malgo.DefaultDeviceConfig(malgo.Loopback)
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = 1
	cfg.SampleRate = SampleRate
	cfg.PeriodSizeInFrames = FrameSize

	if runtime.GOOS != "windows" {
		monitor, err := findMonitor(ctx)
		if err != nil {
			_ = ctx.Uninit()
			ctx.Free()
			return nil, err
		}
		// Miniaudio loopback is WASAPI-only; Linux monitor sources are plain captures.
		cfg = malgo.DefaultDeviceConfig(malgo.Capture)
		cfg.Capture.Format = malgo.FormatS16
		cfg.Capture.Channels = 1
		cfg.SampleRate = SampleRate
		cfg.PeriodSizeInFrames = FrameSize
		cfg.Capture.DeviceID = unsafe.Pointer(&monitor.ID[0])
		slog.Info("system audio via monitor source", "device", monitor.Name())
	}

	sys := &systemAudio{ctx: ctx, out: make(chan []int16, 128)}
	dev, err := malgo.InitDevice(ctx.Context, cfg, malgo.DeviceCallbacks{
		Data: sys.onData,
		Stop: func() { slog.Warn("system audio stopped") },
	})
	if err != nil {
		_ = ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("system audio: %w", err)
	}
	sys.dev = dev

	if err := dev.Start(); err != nil {
		dev.Uninit()
		_ = ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("start system audio: %w", err)
	}
	slog.Info("system audio open", "rate", dev.SampleRate())
	return sys, nil
}

// findMonitor prefers the default sink's monitor when several exist.
func findMonitor(ctx *malgo.AllocatedContext) (malgo.DeviceInfo, error) {
	devices, err := ctx.Devices(malgo.Capture)
	if err != nil {
		return malgo.DeviceInfo{}, fmt.Errorf("list capture devices: %w", err)
	}
	var fallback *malgo.DeviceInfo
	for i := range devices {
		if !strings.Contains(strings.ToLower(devices[i].Name()), "monitor") {
			continue
		}
		if devices[i].IsDefault != 0 {
			return devices[i], nil
		}
		if fallback == nil {
			fallback = &devices[i]
		}
	}
	if fallback != nil {
		return *fallback, nil
	}
	return malgo.DeviceInfo{}, errors.New("no monitor source; PipeWire or PulseAudio is needed for system audio")
}

func (s *systemAudio) onData(_, in []byte, frames uint32) {
	if frames == 0 || len(in) < int(frames)*2 {
		return
	}
	block := make([]int16, frames)
	copy(block, unsafe.Slice((*int16)(unsafe.Pointer(&in[0])), frames))

	select {
	case s.out <- block:
	default:
		slog.Warn("system audio buffer full; dropped a block")
	}
}
