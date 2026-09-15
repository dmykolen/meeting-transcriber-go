// Package audio captures microphone and optional system audio as one stereo
// stream: left is the microphone, right is whatever the machine is playing.
package audio

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

const (
	// SampleRate is fixed end-to-end.
	SampleRate = 16000

	// FrameSize matches the VAD window.
	FrameSize = 512

	// slack is the largest tolerated lag on the system channel. 125 ms keeps
	// jitter headroom without leaving a persistent audible echo backlog.
	slack = SampleRate / 8
)

// A Device is one 16 kHz mono int16 source.
type Device interface {
	Samples() <-chan []int16
	Close() error
}

// Stream interleaves mic and system audio into stereo frames.
type Stream struct {
	frames chan []int16

	// SystemAudio reports whether the right channel has a real device behind it.
	SystemAudio bool

	closeOnce sync.Once
	devices   []Device
	done      chan struct{}
	wg        sync.WaitGroup
}

// Frames yields interleaved stereo frames, 2*FrameSize samples each.
func (s *Stream) Frames() <-chan []int16 { return s.frames }

// Close stops both devices and drains the mixer.
func (s *Stream) Close() error {
	err := error(nil)
	s.closeOnce.Do(func() {
		close(s.done)
		for _, d := range s.devices {
			err = errors.Join(err, d.Close())
		}
		s.wg.Wait()
	})
	return err
}

// Waiting is how long device open can stay quiet before being reported.
const Waiting = 4 * time.Second

// Open starts capture.
func Open(ctx context.Context, system bool) (*Stream, error) {
	// Opening the microphone can block on an OS permission dialog, so there is
	// deliberately no timeout here.
	// item is already up to show it. An earlier version timed out and retried,
	// which was worse — every abandoned attempt was still blocked inside
	// CoreAudio, and when the permission finally arrived three microphones
	// opened at once and two of them had nobody reading them.
	settled := make(chan struct{})
	defer close(settled)
	go func() {
		select {
		case <-settled:
		case <-time.After(Waiting):
			slog.Warn("still opening the audio devices — macOS is probably asking " +
				"for permission. Look for a dialog on screen, or open System Settings > " +
				"Privacy & Security and enable Microphone and System Audio Recording for Meeting Transcriber")
		}
	}()

	mic, err := openMicrophone()
	if err != nil {
		return nil, err
	}

	var sys Device
	if system {
		if sys, err = openSystemAudio(); err != nil {
			slog.Warn("no system audio; only your own voice will be recorded", "err", err)
			sys = nil
		}
	}

	return newStream(ctx, mic, sys), nil
}

// newStream wires two devices into one stereo stream. Open is the real entry
// point; this is the seam that lets the mixer be driven by fake devices, since
// its whole job is reconciling two clocks that never quite agree.
func newStream(ctx context.Context, mic, sys Device) *Stream {
	s := &Stream{
		frames:      make(chan []int16, 64),
		SystemAudio: sys != nil,
		devices:     []Device{mic},
		done:        make(chan struct{}),
	}
	if sys != nil {
		s.devices = append(s.devices, sys)
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer close(s.frames)
		s.mix(ctx, mic, sys)
	}()
	return s
}

// mix interleaves the two devices, using the microphone as the clock.
//
// The microphone is driven by the audio hardware at exactly the rate we asked
// for, which makes it the honest clock; the system stream is buffered against
// it. Sample-accurate alignment is not needed — the two channels are downmixed
// by the server anyway, and a few milliseconds of skew changes nothing about
// which channel had speech.
func (s *Stream) mix(ctx context.Context, mic, sys Device) {
	var (
		left    []int16
		right   []int16
		sysOpen = sys != nil
		sysCh   <-chan []int16
		dropped int
		said    = time.Now()
	)
	if sysOpen {
		sysCh = sys.Samples()
	}

	for {
		// Take everything the system stream has without waiting for it: it must
		// never hold up the microphone, which is what keeps time.
		for drain := true; drain && sysOpen; {
			select {
			case block, ok := <-sysCh:
				if !ok {
					sysOpen, sysCh = false, nil
					slog.Warn("system audio stopped; right channel goes silent")
					continue
				}
				right = append(right, block...)
			default:
				drain = false
			}
		}
		// One sample, not the whole backlog. Trimming in blocks kept the delay
		// bounded but made it step by tens of milliseconds several times a
		// second — inaudible alone, fatal to anything that needs the channels
		// aligned. One sample per frame drains the same backlog as a few
		// hundred ppm of drift, which a filter can follow.
		if len(right) > slack {
			dropped++
			right = right[1:]
			// Once a minute, not once a frame. Trimming is the normal state of
			// affairs now that the bound is small — the tap delivers in bursts
			// and the backlog is what would otherwise be heard as an echo — so
			// logging every time buries everything else in the file. What is
			// worth knowing is the rate, and whether it is growing.
			if time.Since(said) > time.Minute {
				said = time.Now()
				slog.Info("system audio trimmed to keep the channels together",
					"seconds_dropped", float64(dropped)/SampleRate)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		case block, ok := <-mic.Samples():
			if !ok {
				return
			}
			left = append(left, block...)
		}

		for len(left) >= FrameSize {
			frame := make([]int16, 2*FrameSize)
			for i := range FrameSize {
				frame[2*i] = left[i]
				if i < len(right) {
					frame[2*i+1] = right[i]
				}
			}
			left = left[FrameSize:]
			// A short right channel is padded above rather than stalled: the
			// microphone has already spoken and the frame has to go out on time.
			right = right[min(FrameSize, len(right)):]

			// Blocking, not dropping. The device callbacks already discard
			// blocks when their own buffers fill, so backpressure is expressed
			// once, at the layer that cannot wait. Dropping here as well only
			// produced a burst of warnings every startup, while the consumer
			// was still being wired up.
			select {
			case s.frames <- frame:
			case <-s.done:
				return
			}
		}
	}
}
