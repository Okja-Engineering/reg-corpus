package sink

import "io"

// Writer is stdout or stderr as regcorpus writes them: each Write one line.
type Writer struct{ w io.Writer }

// New returns the Writer that writes w's lines.
func New(w io.Writer) *Writer { return &Writer{w} }

// Write writes p, one line. Red: a stub that writes p as it is.
func (s *Writer) Write(p []byte) (int, error) { return s.w.Write(p) }

// Text is regcorpus's own text of one or more lines, a help text.
type Text string

// Print writes t to w. Red: a stub that writes t in one Write.
func Print(w io.Writer, t Text) { io.WriteString(w, string(t)) }
