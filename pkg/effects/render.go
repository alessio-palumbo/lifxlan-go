package effects

import "time"

// Render produces timestamped frames from effect using a fixed deterministic step.
func Render(effect Effect, step, duration time.Duration) []FrameAt {
	if effect == nil || step <= 0 || duration <= 0 {
		return nil
	}

	effect.Reset()

	frames := make([]FrameAt, 0, duration/step)
	for at := time.Duration(0); at < duration; at += step {
		frame, ok := effect.Next(step)
		if !ok {
			break
		}
		frames = append(frames, FrameAt{At: at, Frame: frame})
	}

	return frames
}
