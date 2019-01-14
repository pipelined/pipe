package track

import (
	"github.com/pipelined/phono"
)

// Track is a sequence of pipes which are executed one after another.
type Track struct {
	phono.UID
	bs phono.BufferSize
	phono.NumChannels

	start   *clip
	end     *clip
	current *clip

	// newIndex is a channel to receive new index
	newIndex chan int64
	// last sent index
	nextIndex int64
}

// Clip is a phono.Clip in track.
// It uses double-linked list structure.
type clip struct {
	At int64
	phono.Clip
	Next *clip
	Prev *clip
}

// End returns an end index of Clip.
func (c *clip) End() int64 {
	if c == nil {
		return -1
	}
	return c.At + int64(c.Len)
}

// New creates a new track in a session.
func New(bs phono.BufferSize, nc phono.NumChannels) (t *Track) {
	t = &Track{
		UID:         phono.NewUID(),
		nextIndex:   0,
		bs:          bs,
		NumChannels: nc,
	}
	return
}

// BufferSizeParam pushes new limit value for pump.
func (t *Track) BufferSizeParam(bs phono.BufferSize) phono.Param {
	return phono.Param{
		ID: t.ID(),
		Apply: func() {
			t.bs = bs
		},
	}
}

// Pump implements track pump with a sequence of not overlapped clips.
func (t *Track) Pump(string) (phono.PumpFunc, error) {
	t.newIndex = make(chan int64)
	t.nextIndex = 0
	return func() (phono.Buffer, error) {
		if t.nextIndex >= t.clipsEnd() {
			return nil, phono.ErrEOP
		}
		b := t.bufferAt(t.nextIndex)
		t.nextIndex += int64(t.bs)
		return b, nil
	}, nil
}

// Reset flushes all clips from track.
func (t *Track) Reset() {
	t.start = nil
	t.end = nil
}

func (t *Track) bufferAt(index int64) (result phono.Buffer) {
	if t.current == nil {
		t.current = t.clipAfter(index)
	}
	var buf phono.Buffer
	bufferEnd := index + int64(t.bs)
	for t.bs > result.Size() {
		// if current clip starts after frame then append empty buffer
		if t.current == nil || t.current.At >= bufferEnd {
			result = result.Append(phono.EmptyBuffer(t.NumChannels, t.bs-result.Size()))
		} else {
			// if clip starts in current frame
			if t.current.At >= index {
				// calculate offset buffer size
				// offset buffer is needed to align a clip start within a buffer
				offsetBufSize := phono.BufferSize(t.current.At - index)
				result = result.Append(phono.EmptyBuffer(t.NumChannels, offsetBufSize))
				if bufferEnd >= t.current.End() {
					buf = t.current.Slice(t.current.Start, t.current.Len)
				} else {
					buf = t.current.Slice(t.current.Start, int(t.bs-result.Size()))
				}
			} else {
				start := index - t.current.At + int64(t.current.Start)
				if bufferEnd >= t.current.End() {
					buf = t.current.Slice(start, int(t.current.End()-index))
				} else {
					buf = t.current.Slice(start, int(t.bs))
				}
			}
			index += int64(buf.Size())
			result = result.Append(buf)
			if index >= t.current.End() {
				t.current = t.current.Next
			}
		}
	}
	return result
}

// clipAfter searches for a first clip after passed index.
// returns start position of clip and index in clip.
func (t *Track) clipAfter(index int64) *clip {
	slice := t.start
	for slice != nil {
		if slice.At >= index {
			return slice
		}
		slice = slice.Next
	}
	return nil
}

// clipsEnd returns index of last value of last clip.
func (t *Track) clipsEnd() int64 {
	if t.end == nil {
		return -1
	}
	return t.end.At + int64(t.end.Len)
}

// AddClip assigns a frame to a track.
func (t *Track) AddClip(at int64, f phono.Clip) {
	t.current = nil
	c := &clip{
		At:   at,
		Clip: f,
	}

	if t.start == nil {
		t.start = c
		t.end = c
		return
	}

	var next, prev *clip
	if next = t.clipAfter(at); next != nil {
		prev = next.Prev
		next.Prev = c
	} else {
		prev = t.end
		t.end = c
	}

	if prev != nil {
		prev.Next = c
	} else {
		t.start = c
	}
	c.Next = next
	c.Prev = prev

	t.resolveOverlaps(c)
}

// resolveOverlaps resolves overlaps
func (t *Track) resolveOverlaps(c *clip) {
	t.alignNextClip(c)
	t.alignPrevClip(c)
}

func (t *Track) alignNextClip(c *clip) {
	next := c.Next
	if next == nil {
		return
	}
	overlap := int(c.At-next.At) + c.Len
	if overlap > 0 {
		if next.Len > overlap {
			// shorten next
			next.Start = next.Start + int64(overlap)
			next.Len = next.Len - int(overlap)
			next.At = next.At + int64(overlap)
		} else {
			// remove next
			c.Next = next.Next
			if c.Next != nil {
				c.Next.Prev = c
			} else {
				t.end = c
			}
			t.alignNextClip(c)
		}
	}
}

func (t *Track) alignPrevClip(c *clip) {
	prev := c.Prev
	if prev == nil {
		return
	}
	overlap := int(prev.At-c.At) + prev.Len
	if overlap > 0 {
		prev.Len = prev.Len - int(overlap)
		if int(overlap) > c.Len {
			at := c.At + int64(c.Len)
			start := int64(overlap+c.Len) + c.At - prev.At
			len := overlap - c.Len
			t.AddClip(at, prev.Buffer.Clip(start, len))
		}
	}
}
