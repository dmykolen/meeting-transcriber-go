package media

import (
	"math"
	"testing"
)

// tone has enough structure for correlation tests.
func tone(n int, hz float64, level float32) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = level * float32(
			0.7*math.Sin(2*math.Pi*hz*float64(i)/Rate)+
				0.3*math.Sin(2*math.Pi*hz*2.7*float64(i)/Rate))
	}
	return out
}

// hiss avoids the periodic ambiguity of a tone in delay tests.
func hiss(n int, level float32) []float32 {
	out := make([]float32, n)
	seed := uint32(20260909)
	for i := range out {
		seed = seed*1664525 + 1013904223
		out[i] = (float32(seed>>9&0xffff)/32768 - 1) * level
	}
	return out
}

// room simulates acoustic bleed from the far side into the microphone.
func room(system []float32, level float32, delay int) []float32 {
	out := make([]float32, len(system))
	for i := delay; i < len(system); i++ {
		out[i] = level * system[i-delay]
	}
	return out
}

func loudest(a []float32) float64 {
	var top float64
	for _, v := range a {
		top = math.Max(top, math.Abs(float64(v)))
	}
	return top
}

// The far side should survive once rather than once plus echo.
func TestTheFarSideIsHeardOnceAndNotTwice(t *testing.T) {
	n := 3 * Rate
	system := tone(n, 220, 0.35)
	mic := room(system, 0.45, Rate/20) // their voice, 50 ms later, well down

	out := Fold(mic, system)

	var left, theirs float64
	for i := Rate; i < n; i++ { // skip the first second: the fade starts on the mic
		d := float64(out[i] - system[i])
		left += d * d
		theirs += float64(system[i]) * float64(system[i])
	}
	over := 20 * math.Log10(math.Sqrt(left/theirs)+1e-12)
	if over > -20 {
		t.Errorf("a second copy of the far side survives at %.1f dB under it; want under -20", over)
	}
}

// Local-only audio should pass through unchanged.
func TestYourOwnVoiceComesThroughUntouched(t *testing.T) {
	n := 2 * Rate
	mic := tone(n, 160, 0.4)
	system := make([]float32, n) // nobody on the call

	out := Fold(mic, system)
	for i := range out {
		if math.Abs(float64(out[i]-mic[i])) > 1e-6 {
			t.Fatalf("the microphone was altered at sample %d: %v became %v", i, mic[i], out[i])
		}
	}
}

// A brief system chime must not steal the mix from the speaker.
func TestAChimeDoesNotTakeTheMomentFromYou(t *testing.T) {
	n := 2 * Rate
	mic := tone(n, 160, 0.5)
	system := make([]float32, n)
	copy(system[Rate:], tone(Rate/5, 900, 0.05)) // a fifth of a second, quiet

	out := Fold(mic, system)

	var kept, was float64
	for i := Rate; i < Rate+Rate/5; i++ {
		kept += float64(out[i]) * float64(out[i])
		was += float64(mic[i]) * float64(mic[i])
	}
	if lost := 20 * math.Log10(math.Sqrt(kept/was)); lost < -1 {
		t.Errorf("a chime cost the speaker %.1f dB; want no more than 1", -lost)
	}
}

// Channel handover should crossfade rather than click.
func TestTheHandoverIsNotAClick(t *testing.T) {
	n := 4 * Rate
	mic := tone(n, 160, 0.5)
	system := make([]float32, n)
	copy(system[2*Rate:], tone(2*Rate, 300, 0.5)) // they start talking halfway

	out := Fold(mic, system)

	var mixJump, inputJump float64
	for i := 1; i < n; i++ {
		mixJump = math.Max(mixJump, math.Abs(float64(out[i]-out[i-1])))
		inputJump = math.Max(inputJump, math.Abs(float64(mic[i]-mic[i-1])))
		inputJump = math.Max(inputJump, math.Abs(float64(system[i]-system[i-1])))
	}
	if mixJump > inputJump*1.5 {
		t.Errorf("the mix steps by %.4f where its inputs step by at most %.4f — that is a click",
			mixJump, inputJump)
	}
}

// The mix must stay within range.
func TestTheMixNeverClips(t *testing.T) {
	n := 2 * Rate
	mic := tone(n, 160, 0.95)
	system := tone(n, 300, 0.95)

	if top := loudest(Fold(mic, system)); top > 1.0 {
		t.Errorf("the mix reached %.3f; anything over 1 clips", top)
	}
}

// Offset measures tap lateness relative to the microphone channel.
func TestOffsetFindsATapThatWasWrittenLate(t *testing.T) {
	const late = 3680 // 230 ms, which is what a real meeting measured
	n := 20 * Rate

	played := hiss(n, 0.5)
	mic := room(played, 0.5, Rate/200)

	tap := make([]float32, n)
	copy(tap[late:], played)

	want := late - Rate/200
	got := Offset(mic, tap)
	if off := got - want; off < -Rate/500 || off > Rate/500 {
		t.Errorf("Offset found %d samples (%.0f ms), want about %d (%.0f ms)",
			got, float64(got)/Rate*1000, want, float64(want)/Rate*1000)
	}
}

// Already-aligned channels should stay put.
func TestOffsetLeavesAlignedChannelsAlone(t *testing.T) {
	n := 20 * Rate
	system := hiss(n, 0.5)
	mic := room(system, 0.5, Rate/200)

	if got := Offset(mic, system); got > Rate/200 {
		t.Errorf("Offset moved aligned channels by %d samples (%.0f ms)",
			got, float64(got)/Rate*1000)
	}
}
