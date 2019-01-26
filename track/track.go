package track

import (
	"io"

	"github.com/pipelined/phono"
)

// Track is a sequence of pipes which are executed one after another.
type Track struct {
	bufferSize  int
	numChannels int

	start   *clip
	end     *clip
	current *clip

	// newIndex is a channel to receive new index
	newIndex chan int
	// last sent index
	nextIndex int
}

// Clip is a phono.Clip in track.
// It uses double-linked list structure.
type clip struct {
	At int
	phono.Clip
	Next *clip
	Prev *clip
}

// End returns an end index of Clip.
func (c *clip) End() int {
	if c == nil {
		return -1
	}
	return c.At + c.Len
}

// New creates a new track in a session.
func New(bufferSize int, numChannels int) (t *Track) {
	t = &Track{
		nextIndex:   0,
		bufferSize:  bufferSize,
		numChannels: numChannels,
	}
	return
}

// BufferSizeParam pushes new limit value for pump.
func (t *Track) BufferSizeParam(bufferSize int) func() {
	return func() {
		t.bufferSize = bufferSize
	}
}

// Pump implements track pump with a sequence of not overlapped clips.
func (t *Track) Pump(string) (func() (phono.Buffer, error), error) {
	t.newIndex = make(chan int)
	t.nextIndex = 0
	return func() (phono.Buffer, error) {
		if t.nextIndex >= t.clipsEnd() {
			return nil, io.EOF
		}
		b := t.bufferAt(t.nextIndex)
		t.nextIndex += t.bufferSize
		return b, nil
	}, nil
}

// Reset flushes all clips from track.
func (t *Track) Reset() {
	t.start = nil
	t.end = nil
}

func (t *Track) bufferAt(index int) (result phono.Buffer) {
	if t.current == nil {
		t.current = t.clipAfter(index)
	}
	var buf phono.Buffer
	bufferEnd := index + t.bufferSize
	for t.bufferSize > result.Size() {
		// if current clip starts after frame then append empty buffer
		if t.current == nil || t.current.At >= bufferEnd {
			result = result.Append(phono.EmptyBuffer(t.numChannels, t.bufferSize-result.Size()))
		} else {
			// if clip starts in current frame
			if t.current.At >= index {
				// calculate offset buffer size
				// offset buffer is needed to align a clip start within a buffer
				offsetBufSize := t.current.At - index
				result = result.Append(phono.EmptyBuffer(t.numChannels, offsetBufSize))
				if bufferEnd >= t.current.End() {
					buf = t.current.Slice(t.current.Start, t.current.Len)
				} else {
					buf = t.current.Slice(t.current.Start, t.bufferSize-result.Size())
				}
			} else {
				start := index - t.current.At + t.current.Start
				if bufferEnd >= t.current.End() {
					buf = t.current.Slice(start, t.current.End()-index)
				} else {
					buf = t.current.Slice(start, t.bufferSize)
				}
			}
			index += buf.Size()
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
func (t *Track) clipAfter(index int) *clip {
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
func (t *Track) clipsEnd() int {
	if t.end == nil {
		return -1
	}
	return t.end.At + t.end.Len
}

// AddClip assigns a frame to a track.
func (t *Track) AddClip(at int, f phono.Clip) {
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
	overlap := c.At - next.At + c.Len
	if overlap > 0 {
		if next.Len > overlap {
			// shorten next
			next.Start = next.Start + overlap
			next.Len = next.Len - overlap
			next.At = next.At + overlap
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
	overlap := prev.At - c.At + prev.Len
	if overlap > 0 {
		prev.Len = prev.Len - overlap
		if overlap > c.Len {
			at := c.At + c.Len
			start := overlap + c.Len + c.At - prev.At
			len := overlap - c.Len
			t.AddClip(at, prev.Buffer.Clip(start, len))
		}
	}
}
