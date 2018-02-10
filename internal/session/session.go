package session

//Session is a top level abstraction
type Session struct {
	SampleRate float64
	BufferSize int64
}

//New creates a new session
func New() *Session {
	return &Session{}
}

//Option of a session
type Option func(s *Session) Option

//WithOptions allows to set multiple options at once
func (s *Session) WithOptions(options ...Option) *Session {
	for _, option := range options {
		option(s)
	}
	return s
}

//SampleRate defines sample rate
func SampleRate(sampleRate float64) Option {
	return func(s *Session) Option {
		previous := s.SampleRate
		s.SampleRate = sampleRate
		return SampleRate(previous)
	}
}

//BufferSize defines sample rate
func BufferSize(bufferSize int64) Option {
	return func(s *Session) Option {
		previous := s.BufferSize
		s.BufferSize = bufferSize
		return BufferSize(previous)
	}
}
