// Tests that no byte eCFR or govinfo serves reaches fetch's output
// unescaped (worklist round 3, §1): the output is saved as evidence, so
// through every channel an upstream byte can reach a line by, a payload
// that would forge a line of fetch's own, or move the terminal, is written
// on its line, escaped.
package ecfr

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"
)

// forgedLine is the line fetch --verify writes on a match, on the test's
// clock: a payload that put it at the start of a line of the saved output
// would forge the verdict.
const forgedLine = `1024.41: wrote sources/ecfr/title-12/1024.41/meta.yaml: verified_on: "2026-11-25"`

// rawControl says where s, written to a stream, holds a character that
// could end a line or move a terminal: a C0 control but LF, which ends a
// line there, DEL, a C1 control, U+2028 or U+2029, or a byte that isn't
// UTF-8. It is "" when s holds none.
func rawControl(s string) string {
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n == 1 || r < 0x20 && r != '\n' || r == 0x7f || 0x80 <= r && r <= 0x9f || r == 0x2028 || r == 0x2029 {
			return fmt.Sprintf("%q at byte %d", s[i:i+n], i)
		}
		i += n
	}
	return ""
}

// statusServer is an HTTP server that answers every request with status, a
// whole status line as written, and returns its URL.
func statusServer(t *testing.T, status string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				http.ReadRequest(bufio.NewReader(conn))
				fmt.Fprintf(conn, "%s\r\nContent-Length: 2\r\nConnection: close\r\n\r\nno", status)
			}()
		}
	}()
	return "http://" + ln.Addr().String()
}

func TestNoUpstreamByteReachesTheOutputUnescaped(t *testing.T) {
	// The payloads, each written as the channel can carry it: an LF and
	// forgedLine, an escape sequence, a CR, a next line (U+0085) and a
	// line separator (U+2028), the last two from their code points.
	nel, ls := string(rune(0x85)), string(rune(0x2028))
	const esc = "\x1b[2K\x1b[1A"
	// What shows the payload reached the output, as the output writes it:
	// forgedLine's text, on a line of its own or not, or an escape.
	const forgedText = "1024.41: wrote sources/ecfr/title-12/1024.41/meta.yaml: verified_on: "
	const escText = `\x1b[2K`
	nelText, lsText := `\`+"u0085", `\`+"u2028"
	raw := string(repoFile(t, rawPath("1024.41")))
	gran := string(repoFile(t, granule41))
	before := func(s, at, what string) string { // what inserted before the first at in s
		i := strings.Index(s, at)
		if i < 0 {
			t.Fatalf("no %q to insert before", at)
		}
		return s[:i] + what + s[i:]
	}
	// An attribute value holding an LF, forgedLine and a line separator,
	// written as references.
	attr := "&#10;" + strings.ReplaceAll(forgedLine, `"`, "&quot;") + "&#x2028;"
	issued, _ := json.Marshal("\n" + forgedLine + esc + ls)
	for _, tc := range []struct {
		name  string
		set   func(f *fakeECFR)
		url   string   // a server that answers everything, for the client's both hosts
		args  []string // fetch's; "--verify 1024.41" when nil
		reach string
	}{
		{name: "a granule's XML declaration", reach: forgedText, set: func(f *fakeECFR) {
			f.set(govContent41, reply{body: before(gran, "?>", "\n"+forgedLine+"\n")})
		}},
		{name: "a granule's DOCTYPE", reach: forgedText, set: func(f *fakeECFR) {
			f.set(govContent41, reply{body: before(gran, "<CFRGRANULE", "<!DOCTYPE CFRGRANULE\n"+forgedLine+"\n>\n")})
		}},
		{name: "a PI named XML inside a granule's root", reach: forgedText, set: func(f *fakeECFR) {
			f.set(govContent41, reply{body: before(gran, "<SECTION>", "<?XML x\n"+forgedLine+"\n?>")})
		}},
		{name: "a PI inside a granule's root", reach: forgedText, set: func(f *fakeECFR) {
			f.set(govContent41, reply{body: before(gran, "<SECTION>", "<?pi\n"+forgedLine+"\n?>")})
		}},
		{name: "a name in a granule holding a line separator", reach: lsText, set: func(f *fakeECFR) {
			f.set(govContent41, reply{body: strings.Replace(gran, "<SECTION>", "<SECTION"+ls+"x>", 1)})
		}},
		{name: "a name in a granule holding a next line", reach: nelText, set: func(f *fakeECFR) {
			f.set(govContent41, reply{body: strings.Replace(gran, "<SECTION>", "<SECTION"+nel+"x>", 1)})
		}},
		{name: "a fresh eCFR XML's declaration", reach: forgedText, set: func(f *fakeECFR) {
			f.set(xmlURI("2025-07-15", "1024.41"), reply{body: before(raw, "?>", "\n"+forgedLine+"\n")})
		}},
		{name: "a name in a fresh eCFR XML holding a line separator", reach: lsText, set: func(f *fakeECFR) {
			f.set(xmlURI("2025-07-15", "1024.41"), reply{body: strings.Replace(raw, "<HEAD>", "<HEAD"+ls+"x>", 1)})
		}},
		{name: "an eCFR root's N", reach: forgedText, set: func(f *fakeECFR) {
			f.set(xmlURI("2025-07-15", "1024.41"), reply{body: strings.Replace(raw, `N="1024.41"`, `N="1024.41`+attr+`"`, 1)})
		}},
		{name: "an eCFR root's TYPE", reach: forgedText, set: func(f *fakeECFR) {
			f.set(xmlURI("2025-07-15", "1024.41"), reply{body: strings.Replace(raw, `TYPE="SECTION"`, `TYPE="SECTION`+attr+`"`, 1)})
		}},
		{name: "an eCFR root's namespace", reach: forgedText, set: func(f *fakeECFR) {
			ns := strings.Replace(raw, `<DIV8 N="1024.41"`, `<x:DIV8 xmlns:x="`+attr+`" N="1024.41"`, 1)
			f.set(xmlURI("2025-07-15", "1024.41"), reply{body: strings.Replace(ns, "</DIV8>", "</x:DIV8>", 1)})
		}},
		{name: "an issue_date", reach: escText, args: []string{"--latest", "1024.41"}, set: func(f *fakeECFR) {
			f.set(versionsURI("1024.41"), reply{body: strings.Replace(versions41, `],"meta"`,
				`,{"date":"2025-07-15","issue_date":`+string(issued)+`,"identifier":"1024.41","removed":false}],"meta"`, 1)})
		}},
		{name: "a redirect's URL", reach: lsText, set: func(f *fakeECFR) {
			odd := govContent41 + "?x=" + ls + nel
			f.set(govLink41, govRedirect(odd))
			f.set(odd, reply{body: gran})
		}},
		// A CR, an escape sequence, and 0x9b, a byte that isn't UTF-8 but
		// is a control sequence introducer to a terminal reading bytes as
		// C1 controls.
		{name: "an HTTP reason phrase", reach: escText, url: "HTTP/1.1 404 Not\rFound" + esc + "\x9b2K\r" + forgedLine},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := verifyFake(t)
			if tc.set != nil {
				tc.set(f)
			}
			if tc.url != "" {
				u := statusServer(t, tc.url)
				c = &Client{BaseURL: u + apiPath, Govinfo: u}
			}
			args := tc.args
			if args == nil {
				args = []string{"--verify", "1024.41"}
			}
			root := corpusRoot(t)
			stored41(t, root, nil)
			code, stdout, stderr := fetchAt(t, root, c, "2026-11-25", args...)
			for _, s := range []struct{ name, out string }{{"stdout", stdout}, {"stderr", stderr}} {
				if at := rawControl(s.out); at != "" {
					t.Errorf("%s holds %s, unescaped: %q", s.name, at, s.out)
				}
				for _, l := range strings.Split(s.out, "\n") {
					if strings.HasPrefix(l, forgedText) {
						t.Errorf("%s has a forged line %q", s.name, l)
					}
				}
			}
			if !strings.Contains(stdout+stderr, tc.reach) {
				t.Errorf("exit %d, stdout %q, stderr %q: the payload, %q, never reached the output", code, stdout, stderr, tc.reach)
			}
		})
	}
}
