package parse

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// lead is one label of a paragraph's lead, with its heading and own text.
type lead struct {
	label   label
	para    int  // the <P> it is in, from 1
	chained bool // it follows another label of the same lead
	heading string
	text    string // own text: from its label up to the next lead label or the end of the <P>
}

// label is a label as the text shows it, "(" token ")", with its readings.
type label struct {
	text       string    // e.g. "(C)", markup dropped
	start, end int       // its bytes in the paragraph's text
	offset     int       // the XML byte offset of its "("
	readings   []reading // one; for (i), (v) and (x) two, the letter then the roman
}

// reading is a label read as one kind at one value: (C) is (Capital, 3).
type reading struct {
	kind  Kind
	value int
}

// leads finds the labels of a paragraph's lead (ADR 0012): the label at its
// start, then, while the lead goes on, the label right after the last one,
// or past the last one's heading and the one "—" or one space that ends it.
// A heading is the italic text right after a label and one space. The lead
// ends at the first other text; a label after that is running text. A label
// that sits where the lead almost goes on, past any other run of characters
// that are not letters or digits, fails: it is never taken as running text.
func (p *parser) leads(pa *para, n int) ([]lead, error) {
	l, ok, err := p.labelAt(pa, 0)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, p.fail(pa.offset(0), Unlabeled, "", "the paragraph does not start with a label")
	}
	var out []lead
	for {
		ld := lead{label: l, para: n, chained: len(out) > 0}
		pos := l.end
		if pos+1 < len(pa.text) && pa.text[pos] == ' ' && pa.italic[pos+1] {
			h := pos + 1
			if hl, ok, _ := p.labelAt(pa, h); ok {
				return nil, p.fail(hl.offset, Unplaceable, hl.text, "it begins the italic text after "+l.text+", where a heading goes")
			}
			for pos = h; pos < len(pa.text) && pa.italic[pos]; pos++ {
			}
			ld.heading = pa.text[h:pos]
		}
		out = append(out, ld)

		sep := separator(pa.text[pos:])
		next, ok, err := p.labelAt(pa, pos+len(sep))
		if !ok {
			break
		}
		switch {
		case ld.heading == "" && sep == "", ld.heading != "" && (sep == "—" || sep == " "):
		case ld.heading != "":
			return nil, p.fail(next.offset, Unplaceable, next.text,
				`a label follows a heading only past one "—" or one space; here `+describe(sep)+" separates them")
		default:
			return nil, p.fail(next.offset, Unplaceable, next.text,
				"a label follows another only with nothing between them, or past its heading; here "+describe(sep)+" separates them")
		}
		if err != nil {
			return nil, err
		}
		l = next
	}
	for i := range out {
		end := len(pa.text)
		if i+1 < len(out) {
			end = out[i+1].label.start
		}
		out[i].text = pa.text[out[i].label.start:end]
	}
	return out, nil
}

// separator is the run at the start of s of anything but letters, digits
// and "(".
func separator(s string) string {
	for i, r := range s {
		if r == '(' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			return s[:i]
		}
	}
	return s
}

func describe(sep string) string {
	if sep == "" {
		return "nothing"
	}
	return strconv.Quote(sep)
}

// labelAt reads a label at index i of a paragraph's text: "(", ASCII
// letters or digits, ")". ok reports that shape. err is set for a shape
// with no reading: italic parentheses, a token partly italic, an italic
// token that is not a number, or a token that is no label.
func (p *parser) labelAt(pa *para, i int) (l label, ok bool, err error) {
	s := pa.text
	if i >= len(s) || s[i] != '(' {
		return label{}, false, nil
	}
	j := i + 1
	for j < len(s) && isAlnum(s[j]) {
		j++
	}
	if j == i+1 || j >= len(s) || s[j] != ')' {
		return label{}, false, nil
	}
	l = label{text: s[i : j+1], start: i, end: j + 1, offset: pa.offset(i)}
	italic := pa.italic[i+1]
	var why string
	switch {
	case pa.italic[i] || pa.italic[j]:
		why = "its parentheses are italic"
	case slices.Contains(pa.italic[i+1:j], !italic):
		why = "part of the label is italic"
	default:
		l.readings, why = readings(s[i+1:j], italic)
	}
	if why != "" {
		return l, true, p.fail(l.offset, Unplaceable, l.text, why)
	}
	return l, true, nil
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// readings gives a label token's readings, or why it has none. Only (i),
// (v) and (x) read both as a letter and as a roman numeral (R1a); (c), (d),
// (l) and (m) are letters.
func readings(tok string, italic bool) ([]reading, string) {
	if n, err := strconv.Atoi(tok); err == nil && tok[0] != '0' {
		if italic {
			return []reading{{Italic, n}}, ""
		}
		return []reading{{Number, n}}, ""
	}
	if italic {
		return nil, "an italic label that is not a number"
	}
	if len(tok) == 1 && tok[0] >= 'A' && tok[0] <= 'Z' {
		return []reading{{Capital, int(tok[0]-'A') + 1}}, ""
	}
	if len(tok) == 1 && tok[0] >= 'a' && tok[0] <= 'z' {
		rs := []reading{{Letter, int(tok[0]-'a') + 1}}
		if tok == "i" || tok == "v" || tok == "x" {
			r, _ := roman(tok)
			rs = append(rs, reading{Roman, r})
		}
		return rs, ""
	}
	if r, ok := roman(tok); ok {
		return []reading{{Roman, r}}, ""
	}
	return nil, "not a letter, number, roman numeral or capital letter"
}

var romanDigits = []struct {
	value int
	text  string
}{{1000, "m"}, {900, "cm"}, {500, "d"}, {400, "cd"}, {100, "c"}, {90, "xc"},
	{50, "l"}, {40, "xl"}, {10, "x"}, {9, "ix"}, {5, "v"}, {4, "iv"}, {1, "i"}}

func toRoman(n int) string {
	var b strings.Builder
	for _, d := range romanDigits {
		for ; n >= d.value; n -= d.value {
			b.WriteString(d.text)
		}
	}
	return b.String()
}

// roman reads a lowercase roman numeral written the standard way.
func roman(tok string) (int, bool) {
	n, rest := 0, tok
	for _, d := range romanDigits {
		for strings.HasPrefix(rest, d.text) {
			n, rest = n+d.value, rest[len(d.text):]
		}
	}
	return n, n > 0 && rest == "" && toRoman(n) == tok
}

// labelText writes a value as a label of a kind, e.g. (Capital, 2) is "(B)".
func labelText(k Kind, v int) string {
	switch k {
	case Letter:
		return "(" + string(rune('a'+v-1)) + ")"
	case Roman:
		return "(" + toRoman(v) + ")"
	case Capital:
		return "(" + string(rune('A'+v-1)) + ")"
	default:
		return "(" + strconv.Itoa(v) + ")"
	}
}

// levels is the open levels: levels[k] is the value of the last label of
// Kind k. Depth comes from the kind: a label of Kind k is a child of the
// open level k-1 and a sibling of the open level k, which closes the levels
// below it. The open levels are the tree's last node at each depth.
type levels []int

func (v levels) path() string {
	var b strings.Builder
	for k, val := range v {
		b.WriteString(labelText(Kind(k), val))
	}
	return b.String()
}

// fit says why a reading can't be placed on the open levels, or returns ""
// if it can. A chained label, one after another in a lead, opens the level
// below the label before it, which is the last open level.
func (v levels) fit(r reading, chained bool) (Reason, string) {
	k := int(r.kind)
	switch {
	case k > len(v):
		open := "no level is open"
		if len(v) > 0 {
			open = "the open levels are " + v.path()
		}
		return Unplaceable, fmt.Sprintf("%s labels sit under %s labels; %s", r.kind, Kind(k-1), open)
	case chained && k < len(v):
		return Unplaceable, "in a paragraph's lead it follows " + v.path() + ", so it opens the level below it"
	case k == len(v):
		if r.value == 1 {
			return "", ""
		}
		under := "at the top level"
		if k > 0 {
			under = "under " + v.path()
		}
		return Skipped, fmt.Sprintf("the first %s label %s is %s", r.kind, under, labelText(r.kind, 1))
	}
	last, at := v[k], v[:k+1].path()
	switch {
	case r.value == last+1:
		return "", ""
	case r.value == last:
		return Repeated, "it repeats " + at
	case r.value < last:
		return OutOfOrder, "it comes after " + at
	}
	return Skipped, fmt.Sprintf("after %s the next %s label is %s", at, r.kind, labelText(r.kind, last+1))
}

func (v levels) fits(l lead, r reading) bool {
	reason, _ := v.fit(r, l.chained)
	return reason == ""
}

// with is the open levels after a reading that fits is placed.
func (v levels) with(r reading) levels {
	return append(slices.Clone(v[:r.kind]), r.value)
}

// place puts each lead's label in the tree, in document order.
func (p *parser) place(leads []lead) error {
	for i, ld := range leads {
		r, err := p.choose(leads, i)
		if err != nil {
			return err
		}
		if reason, why := p.open.fit(r, ld.chained); reason != "" {
			return p.fail(ld.label.offset, reason, ld.label.text, why)
		}
		p.open = p.open.with(r)
		n := &Node{Kind: r.kind, Para: ld.para, Heading: ld.heading, Text: ld.text}
		for k, val := range p.open {
			n.Path = append(n.Path, strings.Trim(labelText(Kind(k), val), "()"))
		}
		siblings := &p.tree.Children
		for range len(p.open) - 1 {
			last := (*siblings)[len(*siblings)-1]
			siblings = &last.Children
		}
		*siblings = append(*siblings, n)
	}
	return nil
}

// choose picks the reading of the i-th label. (i), (v) or (x) is a letter
// only when the last letter-level sibling directly precedes it, that is
// when the letter reading fits; otherwise it is roman. When the roman
// reading fits too, the next label in document order decides (ADR 0012):
// the reading after which it fits is taken. If no label follows, or the
// next fits after neither reading, the parse fails. No label fits after
// both. Say the letter reading is the letter L. After it, the next label
// fits only as the letter L+1 or a first number; after the roman reading,
// under (L-1)(n), only as the letter L, the number n+1 (not 1), the next
// roman or a first capital. A next (i), (v) or (x) fits after the letter
// reading only as L+1, which is (j), (w) or (y), none of them (i), (v) or
// (x).
func (p *parser) choose(leads []lead, i int) (reading, error) {
	ld := leads[i]
	if len(ld.label.readings) == 1 {
		return ld.label.readings[0], nil
	}
	letter, roman := ld.label.readings[0], ld.label.readings[1]
	if !p.open.fits(ld, letter) {
		return roman, nil
	}
	if !p.open.fits(ld, roman) {
		return letter, nil
	}
	both := fmt.Sprintf("it could be %s or %s", p.open.with(letter).path(), p.open.with(roman).path())
	if i+1 == len(leads) {
		return reading{}, p.fail(ld.label.offset, Unplaceable, ld.label.text, both+", and no label follows to decide")
	}
	next := leads[i+1]
	var took []reading
	for _, r := range ld.label.readings {
		after := p.open.with(r)
		if slices.ContainsFunc(next.label.readings, func(nr reading) bool { return after.fits(next, nr) }) {
			took = append(took, r)
		}
	}
	if len(took) != 1 {
		return reading{}, p.fail(ld.label.offset, Unplaceable, ld.label.text,
			both+", and the next label, "+next.label.text+", fits after neither")
	}
	return took[0], nil
}
