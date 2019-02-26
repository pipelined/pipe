package audio

import (
	"io"

	"github.com/pipelined/signal"
)

// Track is a sequence of pipes which are executed one after another.
type Track struct {
	numChannels int
	sampleRate  int

	start   *link
	end     *link
	current *link

	// newIndex is a channel to receive new index
	newIndex chan int
	// last sent index
	nextIndex int
}

// stream is a sequence of Clips in track.
// It uses double-linked list structure.
type link struct {
	At int
	Clip
	Next *link
	Prev *link
}

// End returns an end index of link.
func (l *link) End() int {
	if l == nil {
		return -1
	}
	return l.At + l.Len
}

// NewTrack creates a new track in a session.
func NewTrack(sampleRate int, numChannels int) (t *Track) {
	t = &Track{
		nextIndex:   0,
		sampleRate:  sampleRate,
		numChannels: numChannels,
	}
	return
}

// Pump implements track pump with a sequence of not overlapped clips.
func (t *Track) Pump(sourceID string, bufferSize int) (func() ([][]float64, error), int, int, error) {
	// bufferSize = bufferSize
	t.newIndex = make(chan int)
	t.nextIndex = 0
	return func() ([][]float64, error) {
		if t.nextIndex >= t.endIndex() {
			return nil, io.EOF
		}
		b := t.bufferAt(t.nextIndex, bufferSize)
		t.nextIndex += bufferSize
		return b, nil
	}, t.sampleRate, t.numChannels, nil
}

// Reset flushes all links from track.
func (t *Track) Reset() {
	t.current = nil
	t.start = nil
	t.end = nil
}

func (t *Track) bufferAt(index, bufferSize int) (result signal.Float64) {
	if t.current == nil {
		t.current = t.linkAfter(index)
	}
	var buf signal.Float64
	bufferEnd := index + bufferSize
	for bufferSize > result.Size() {
		// if current link starts after frame then append empty buffer
		if t.current == nil || t.current.At >= bufferEnd {
			result = result.Append(signal.EmptyFloat64(t.numChannels, bufferSize-result.Size()))
		} else {
			// if link starts in current frame
			if t.current.At >= index {
				// calculate offset buffer size
				// offset buffer is needed to align a link start within a buffer
				offsetBufSize := t.current.At - index
				result = result.Append(signal.EmptyFloat64(t.numChannels, offsetBufSize))
				if bufferEnd >= t.current.End() {
					buf = t.current.data.Slice(t.current.Start, t.current.Len)
				} else {
					buf = t.current.data.Slice(t.current.Start, bufferSize-result.Size())
				}
			} else {
				start := index - t.current.At + t.current.Start
				if bufferEnd >= t.current.End() {
					buf = t.current.data.Slice(start, t.current.End()-index)
				} else {
					buf = t.current.data.Slice(start, bufferSize)
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

// linkAfter searches for a first link after passed index.
func (t *Track) linkAfter(index int) *link {
	slice := t.start
	for slice != nil {
		if slice.At >= index {
			return slice
		}
		slice = slice.Next
	}
	return nil
}

// endIndex returns index of last value of last link.
func (t *Track) endIndex() int {
	if t.end == nil {
		return -1
	}
	return t.end.At + t.end.Len
}

// AddClip assigns a frame to a track.
func (t *Track) AddClip(at int, c Clip) {
	if c.Asset == nil {
		return
	}
	t.current = nil
	l := &link{
		At:   at,
		Clip: c,
	}

	if t.start == nil {
		t.start = l
		t.end = l
		return
	}

	var next, prev *link
	if next = t.linkAfter(at); next != nil {
		prev = next.Prev
		next.Prev = l
	} else {
		prev = t.end
		t.end = l
	}

	if prev != nil {
		prev.Next = l
	} else {
		t.start = l
	}
	l.Next = next
	l.Prev = prev

	t.resolveOverlaps(l)
}

// resolveOverlaps resolves overlaps
func (t *Track) resolveOverlaps(l *link) {
	t.alignNextLink(l)
	t.alignPrevLink(l)
}

func (t *Track) alignNextLink(l *link) {
	next := l.Next
	if next == nil {
		return
	}
	overlap := l.At - next.At + l.Len
	if overlap > 0 {
		if next.Len > overlap {
			// shorten next
			next.Start = next.Start + overlap
			next.Len = next.Len - overlap
			next.At = next.At + overlap
		} else {
			// remove next
			l.Next = next.Next
			if l.Next != nil {
				l.Next.Prev = l
			} else {
				t.end = l
			}
			t.alignNextLink(l)
		}
	}
}

func (t *Track) alignPrevLink(l *link) {
	prev := l.Prev
	if prev == nil {
		return
	}
	overlap := prev.At - l.At + prev.Len
	if overlap > 0 {
		prev.Len = prev.Len - overlap
		if overlap > l.Len {
			at := l.At + l.Len
			start := overlap + l.Len + l.At - prev.At
			len := overlap - l.Len
			t.AddClip(at, prev.Asset.Clip(start, len))
		}
	}
}
