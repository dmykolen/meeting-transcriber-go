package audio

import (
	"fmt"
	"log/slog"
	"sync"
	"unsafe"

	"github.com/gen2brain/malgo"
)

// microphone is the cross-platform capture device.
type microphone struct {
	ctx    *malgo.AllocatedContext
	dev    *malgo.Device
	out    chan []int16
	closed sync.Once
}

func (m *microphone) Samples() <-chan []int16 { return m.out }

func (m *microphone) Close() error {
	m.closed.Do(func() {
		m.dev.Uninit()
		_ = m.ctx.Uninit()
		m.ctx.Free()
		close(m.out)
	})
	return nil
}

func openMicrophone() (Device, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("audio backend: %w", err)
	}

	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = 1
	cfg.SampleRate = SampleRate
	cfg.PeriodSizeInFrames = FrameSize

	mic := &microphone{ctx: ctx, out: make(chan []int16, 128)}
	dev, err := malgo.InitDevice(ctx.Context, cfg, malgo.DeviceCallbacks{
		Data: mic.onData,
		Stop: func() { slog.Warn("microphone stopped") },
	})
	if err != nil {
		_ = ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("microphone: %w", err)
	}
	mic.dev = dev

	if err := dev.Start(); err != nil {
		dev.Uninit()
		_ = ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("start microphone: %w", err)
	}
	slog.Info("microphone open",
		"rate", dev.SampleRate(),
		"native_rate", dev.CaptureInternalSampleRate(),
		"native_channels", dev.CaptureInternalChannels())
	return mic, nil
}

// onData must not block the audio thread.
func (m *microphone) onData(_, in []byte, frames uint32) {
	if frames == 0 || len(in) < int(frames)*2 {
		return
	}
	block := make([]int16, frames)
	copy(block, unsafe.Slice((*int16)(unsafe.Pointer(&in[0])), frames))

	select {
	case m.out <- block:
	default:
		slog.Warn("microphone buffer full; dropped a block")
	}
}
