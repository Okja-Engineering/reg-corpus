package parse

import "fmt"

// Tree is one parsed CFR section: the section's DIV8, with its HEAD and CITA
// kept as character data and its labeled paragraphs as nodes.
type Tree struct {
	Section  string  `json:"section"`            // the DIV8's N attribute, e.g. "1024.41"
	Heading  string  `json:"heading"`            // the HEAD's character data
	Citation string  `json:"citation,omitempty"` // the CITA's character data, verbatim
	Children []*Node `json:"children"`           // top-level (letter) nodes, in document order
}

// Node is one labeled unit of a section at any depth.
type Node struct {
	Path     []string `json:"path"`              // label values from the top level down, e.g. ["b", "2", "i", "B"]
	Kind     Kind     `json:"kind"`              // the level kind of the last label
	Heading  string   `json:"heading,omitempty"` // the italic heading right after the label, if any
	Text     string   `json:"text"`              // own text: label and heading included, children excluded, verbatim
	Children []*Node  `json:"children,omitempty"`
}

// Kind is a label's level kind. Depth follows from it: a letter is always
// at the top level, a number under a letter, and so on down to the italic
// number.
type Kind int

// The level kinds, in depth order.
const (
	Letter  Kind = iota // (a)
	Number              // (1)
	Roman               // (i), lowercase roman
	Capital             // (A)
	Italic              // (1) with the number in <I>
)

var kindNames = [...]string{"letter", "number", "roman", "capital", "italic"}

func (k Kind) String() string {
	if k < 0 || int(k) >= len(kindNames) {
		return fmt.Sprintf("Kind(%d)", int(k))
	}
	return kindNames[k]
}

// MarshalText writes a Kind by its name; an unknown Kind is an error.
func (k Kind) MarshalText() ([]byte, error) {
	if k < 0 || int(k) >= len(kindNames) {
		return nil, fmt.Errorf("parse: unknown kind %d", int(k))
	}
	return []byte(kindNames[k]), nil
}

// UnmarshalText reads a Kind by its name; an unknown name is an error.
func (k *Kind) UnmarshalText(b []byte) error {
	for i, name := range kindNames {
		if string(b) == name {
			*k = Kind(i)
			return nil
		}
	}
	return fmt.Errorf("parse: unknown kind %q", b)
}

// Reason names why a parse failed.
type Reason string

// The reasons a parse fails.
const (
	Skipped     Reason = "skipped"
	Repeated    Reason = "repeated"
	OutOfOrder  Reason = "out of order"
	Unplaceable Reason = "unplaceable"
	Unlabeled   Reason = "unlabeled paragraph"
	Malformed   Reason = "malformed"
)

// Error is a parse failure at a location in the XML. Line and Column are
// 1-based; Column counts bytes.
type Error struct {
	Line, Column int
	Reason       Reason
	Label        string // the label as its character data reads, e.g. "(C)"; empty if none
	Detail       string
}

func (e *Error) Error() string {
	if e.Label == "" {
		return fmt.Sprintf("parse: line %d, column %d: %s: %s", e.Line, e.Column, e.Reason, e.Detail)
	}
	return fmt.Sprintf("parse: line %d, column %d: %s label %s: %s", e.Line, e.Column, e.Reason, e.Label, e.Detail)
}
