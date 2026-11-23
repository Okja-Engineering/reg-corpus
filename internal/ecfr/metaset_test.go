// Tests for `regcorpus meta set` (R1, plan step 3; ADRs 0010 and 0011): it
// sets version_effective_date or version_fr_citations in a stored source's
// meta.yaml through internal/meta and changes nothing else; it sets no other
// field; each citation must be in the citation note (<CITA>) of the raw XML
// meta.yaml describes; the exit codes are 0 ok, 1 refused by what is stored,
// 2 usage or a file it can't read or write. Offline.
package ecfr

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/meta"
)

// citedBody is a stand-in for 1024.41's raw XML with a citation note, and
// Federal Register citations outside it: in an XREF and in running text.
const citedBody = "<?xml version=\"1.0\" encoding=\"UTF-8\"?> \n<DIV8 N=\"1024.41\" TYPE=\"SECTION\">\n" +
	"<HEAD>§ 1024.41 Stand-in.</HEAD>\n" +
	"<XREF ID=\"20250516\" REFID=\"35\" AMDINSN=\"4\">Link to an amendment published at 90 FR 20799, May 16, 2025.</XREF>\n" +
	"<P>(a) Stand-in text that mentions 89 FR 11111 in running text.</P>\n" +
	"<CITA TYPE=\"N\">[78 FR 10876, Feb. 14, 2013, as amended at 81 FR 72373, Oct. 19, 2016; 90 FR 20792, May 16, 2025]\n</CITA>\n" +
	"</DIV8>\n"

// citedCorpus returns a corpus root where 1024.41 holds citedBody and the
// meta.yaml fetch writes for it.
func citedCorpus(t *testing.T) string {
	t.Helper()
	root := corpusRoot(t)
	writeFile(t, root, rawPath("1024.41"), citedBody)
	writeFile(t, root, metaPath("1024.41"), fetchMeta("2025-07-15", sha(citedBody)))
	return root
}

func metaAt(t *testing.T, root string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = metaMain(root, args, &out, &errb)
	return code, out.String(), errb.String()
}

// withLines is fetchMeta for citedBody with step 3's lines in place.
func withLines(effective, citations string) string {
	m := fetchMeta("2025-07-15", sha(citedBody))
	add := ""
	if effective != "" {
		add += "version_effective_date: " + effective + "\n"
	}
	if citations != "" {
		add += "version_fr_citations: " + citations + "\n"
	}
	return strings.Replace(m, "retrieved_at:", add+"retrieved_at:", 1)
}

// TestMetaSetWritesOnlyTheFieldAsked: each run sets the one field named, in
// the spec's place and form, and keeps every other line as it was.
func TestMetaSetWritesOnlyTheFieldAsked(t *testing.T) {
	root := citedCorpus(t)
	mp := metaPath("1024.41")
	for _, step := range []struct {
		args       []string
		line, want string
	}{
		{[]string{"version_effective_date", "2025-07-15"}, `version_effective_date: "2025-07-15"`,
			withLines(`"2025-07-15"`, "")},
		{[]string{"version_fr_citations", "90 FR 20792"}, `version_fr_citations: ["90 FR 20792"]`,
			withLines(`"2025-07-15"`, `["90 FR 20792"]`)},
		{[]string{"version_fr_citations", "78 FR 10876", "90 FR 20792"}, `version_fr_citations: ["78 FR 10876", "90 FR 20792"]`,
			withLines(`"2025-07-15"`, `["78 FR 10876", "90 FR 20792"]`)},
		{[]string{"version_effective_date", "2025-07-14"}, `version_effective_date: "2025-07-14"`,
			withLines(`"2025-07-14"`, `["78 FR 10876", "90 FR 20792"]`)},
	} {
		code, stdout, stderr := metaAt(t, root, append([]string{"set", "1024.41"}, step.args...)...)
		if want := "1024.41: wrote " + mp + ": " + step.line + "\n"; code != 0 || stdout != want || stderr != "" {
			t.Fatalf("%q: exit %d, stdout %q, stderr %q; want 0, %q", step.args, code, stdout, stderr, want)
		}
		if got := readFile(t, root, mp); got != step.want {
			t.Fatalf("%q: %s:\n%s\nwant:\n%s", step.args, mp, got, step.want)
		}
	}
}

// TestMetaSetKeepsTheFieldsOtherStepsSet: a meta.yaml holding every field
// keeps each of them byte for byte when one step 3 field is set.
func TestMetaSetKeepsTheFieldsOtherStepsSet(t *testing.T) {
	root := citedCorpus(t)
	all := strings.Replace(withLines(`"2025-07-15"`, `["90 FR 20792"]`), "sha256_raw:", "verified_on: null\nsha256_raw:", 1) +
		"sha256_text: \"" + strings.Repeat("a", 64) + "\"\n"
	writeFile(t, root, metaPath("1024.41"), all)
	if code, _, stderr := metaAt(t, root, "set", "1024.41", "version_effective_date", "2025-07-14"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	want := strings.Replace(all, `version_effective_date: "2025-07-15"`, `version_effective_date: "2025-07-14"`, 1)
	if got := readFile(t, root, metaPath("1024.41")); got != want {
		t.Errorf("meta.yaml:\n%s\nwant:\n%s", got, want)
	}
}

// TestMetaSetIsANoOpWhenTheValueIsSet: setting the value a field already has
// touches nothing and says so.
func TestMetaSetIsANoOpWhenTheValueIsSet(t *testing.T) {
	root := citedCorpus(t)
	writeFile(t, root, metaPath("1024.41"), withLines(`"2025-07-15"`, `["90 FR 20792"]`))
	before := tree(t, root)
	for _, args := range [][]string{
		{"version_effective_date", "2025-07-15"},
		{"version_fr_citations", "90 FR 20792"},
	} {
		code, stdout, stderr := metaAt(t, root, append([]string{"set", "1024.41"}, args...)...)
		want := "1024.41: unchanged " + metaPath("1024.41") + ": " + args[0] + ": "
		if code != 0 || !strings.HasPrefix(stdout, want) || stderr != "" {
			t.Errorf("%q: exit %d, stdout %q, stderr %q; want 0, %q...", args, code, stdout, stderr, want)
		}
	}
	if !equalTrees(before, tree(t, root)) {
		t.Errorf("files changed")
	}
}

// TestMetaSetTakesOnlyCitationsTheStoredXMLCites: each citation must be one
// the citation note (<CITA>) of the raw XML cites, whole: not one cited only
// elsewhere in the XML, and not a prefix of one it cites (ADR 0011). One
// refused citation refuses the run, and nothing is written.
func TestMetaSetTakesOnlyCitationsTheStoredXMLCites(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cites []string
		code  int
		want  []string // in stderr when refused
	}{
		{"the last citation in the note", []string{"90 FR 20792"}, 0, nil},
		{"an earlier citation in the note", []string{"81 FR 72373"}, 0, nil},
		{"two in the note", []string{"78 FR 10876", "90 FR 20792"}, 0, nil},
		{"one cited only in an XREF", []string{"90 FR 20799"}, 1, []string{`"90 FR 20799"`, rawPath("1024.41"), "<CITA>"}},
		{"one cited only in running text", []string{"89 FR 11111"}, 1, []string{`"89 FR 11111"`}},
		{"a prefix of a cited page", []string{"90 FR 2079"}, 1, []string{`"90 FR 2079"`}},
		{"a cited page in another volume", []string{"91 FR 20792"}, 1, []string{`"91 FR 20792"`}},
		{"one cited and one not", []string{"90 FR 20792", "90 FR 20799"}, 1, []string{`"90 FR 20799"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := citedCorpus(t)
			before := tree(t, root)
			code, stdout, stderr := metaAt(t, root, append([]string{"set", "1024.41", "version_fr_citations"}, tc.cites...)...)
			if code != tc.code {
				t.Fatalf("exit %d, stdout %q, stderr %q; want %d", code, stdout, stderr, tc.code)
			}
			if tc.code == 0 {
				return
			}
			if stdout != "" || !strings.HasPrefix(stderr, "regcorpus meta: 1024.41: ") {
				t.Errorf("stdout %q, stderr %q; want nothing on stdout, an error for 1024.41", stdout, stderr)
			}
			for _, w := range tc.want {
				if !strings.Contains(stderr, w) {
					t.Errorf("stderr %q lacks %q", stderr, w)
				}
			}
			if strings.Contains(stderr, `"90 FR 20792" is`) {
				t.Errorf("stderr %q refuses a cited citation", stderr)
			}
			if !equalTrees(before, tree(t, root)) {
				t.Errorf("files changed")
			}
		})
	}
}

// TestMetaSetChecksCitationsAgainstTheRawFileMetaYAMLDescribes: the raw XML
// read is raw/<source_version_date>.xml with sha256_raw's bytes, a regular
// file through real directories; anything else is refused (exit 1) and
// nothing is written.
func TestMetaSetChecksCitationsAgainstTheRawFileMetaYAMLDescribes(t *testing.T) {
	older := "sources/ecfr/title-12/1024.41/raw/2025-05-16.xml"
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, root string)
		cite  string
		want  []string
	}{
		{"the raw file is missing", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(rawPath("1024.41")))); err != nil {
				t.Fatal(err)
			}
		}, "90 FR 20792", []string{rawPath("1024.41")}},
		{"the raw file's bytes aren't sha256_raw's", func(t *testing.T, root string) {
			writeFile(t, root, rawPath("1024.41"), strings.Replace(citedBody, "Stand-in text", "Other text", 1))
		}, "90 FR 20792", []string{rawPath("1024.41"), "sha256_raw"}},
		{"the raw file is a symlink", func(t *testing.T, root string) {
			p := filepath.Join(root, filepath.FromSlash(rawPath("1024.41")))
			writeFile(t, root, "elsewhere.xml", citedBody)
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("../../../../../elsewhere.xml", p); err != nil {
				t.Fatal(err)
			}
		}, "90 FR 20792", []string{rawPath("1024.41"), "not a regular file"}},
		{"raw/ is a symlink", func(t *testing.T, root string) {
			dir := filepath.Join(root, filepath.FromSlash("sources/ecfr/title-12/1024.41/raw"))
			out := t.TempDir()
			writeFile(t, out, "2025-07-15.xml", citedBody)
			if err := os.RemoveAll(dir); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(out, dir); err != nil {
				t.Fatal(err)
			}
		}, "90 FR 20792", []string{"sources/ecfr/title-12/1024.41/raw", "not a real directory"}},
		{"another raw file cites it", func(t *testing.T, root string) {
			writeFile(t, root, older, strings.Replace(citedBody, "90 FR 20792, May 16, 2025]", "90 FR 20792, May 16, 2025; 91 FR 1, Jan. 2, 2026]", 1))
		}, "91 FR 1", []string{older, rawPath("1024.41"), "one raw file"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := citedCorpus(t)
			tc.setup(t, root)
			before := tree(t, root)
			code, stdout, stderr := metaAt(t, root, "set", "1024.41", "version_fr_citations", tc.cite)
			if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "regcorpus meta: 1024.41: ") {
				t.Errorf("exit %d, stdout %q, stderr %q; want 1 for 1024.41", code, stdout, stderr)
			}
			for _, w := range tc.want {
				if !strings.Contains(stderr, w) {
					t.Errorf("stderr %q lacks %q", stderr, w)
				}
			}
			if !equalTrees(before, tree(t, root)) {
				t.Errorf("files changed")
			}
		})
	}
}

// TestMetaSetRefusesAMetaYAMLItCantTrust: meta set changes only a meta.yaml
// fetch made, in the spec's schema, a regular file through real directories
// (exit 1 otherwise, nothing written).
func TestMetaSetRefusesAMetaYAMLItCantTrust(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, root string)
		want  []string
	}{
		{"no meta.yaml", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(metaPath("1024.41")))); err != nil {
				t.Fatal(err)
			}
		}, []string{metaPath("1024.41"), "regcorpus fetch"}},
		{"no source directory", func(t *testing.T, root string) {
			if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash("sources/ecfr/title-12/1024.41"))); err != nil {
				t.Fatal(err)
			}
		}, []string{metaPath("1024.41"), "regcorpus fetch"}},
		{"a meta.yaml not in the schema", func(t *testing.T, root string) {
			writeFile(t, root, metaPath("1024.41"), fetchMeta("2025-07-15", sha(citedBody))+"note: \"x\"\n")
		}, []string{metaPath("1024.41"), `"note"`}},
		{"meta.yaml a symlink", func(t *testing.T, root string) {
			p := filepath.Join(root, filepath.FromSlash(metaPath("1024.41")))
			writeFile(t, root, "elsewhere.yaml", fetchMeta("2025-07-15", sha(citedBody)))
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("../../../../elsewhere.yaml", p); err != nil {
				t.Fatal(err)
			}
		}, []string{metaPath("1024.41"), "not a regular file"}},
		{"the source directory a symlink", func(t *testing.T, root string) {
			dir := filepath.Join(root, filepath.FromSlash("sources/ecfr/title-12/1024.41"))
			out := t.TempDir()
			if err := os.Rename(dir, filepath.Join(out, "1024.41")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(out, "1024.41"), dir); err != nil {
				t.Fatal(err)
			}
		}, []string{"sources/ecfr/title-12/1024.41", "not a real directory"}},
		{"the source directory a symlink, the value already set", func(t *testing.T, root string) {
			writeFile(t, root, metaPath("1024.41"), withLines(`"2025-07-15"`, ""))
			dir := filepath.Join(root, filepath.FromSlash("sources/ecfr/title-12/1024.41"))
			out := t.TempDir()
			if err := os.Rename(dir, filepath.Join(out, "1024.41")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(out, "1024.41"), dir); err != nil {
				t.Fatal(err)
			}
		}, []string{"sources/ecfr/title-12/1024.41", "not a real directory"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := citedCorpus(t)
			tc.setup(t, root)
			before := tree(t, root)
			code, stdout, stderr := metaAt(t, root, "set", "1024.41", "version_effective_date", "2025-07-15")
			if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "regcorpus meta: 1024.41: ") {
				t.Errorf("exit %d, stdout %q, stderr %q; want 1 for 1024.41", code, stdout, stderr)
			}
			for _, w := range tc.want {
				if !strings.Contains(stderr, w) {
					t.Errorf("stderr %q lacks %q", stderr, w)
				}
			}
			if !equalTrees(before, tree(t, root)) {
				t.Errorf("files changed")
			}
		})
	}
}

// TestMetaSetUsageErrorsExitTwo: meta set takes a known section and one of
// step 3's two fields with values of its type. Every other field belongs to
// the command that computes it. A usage error writes nothing.
func TestMetaSetUsageErrorsExitTwo(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string // in stderr
	}{
		{"no subcommand", nil, []string{"usage: regcorpus meta"}},
		{"an unknown subcommand", []string{"get", "1024.41", "version_effective_date"}, []string{`"get"`, "usage: regcorpus meta"}},
		{"set alone", []string{"set"}, []string{"usage: regcorpus meta"}},
		{"no field", []string{"set", "1024.41"}, []string{"usage: regcorpus meta"}},
		{"no value", []string{"set", "1024.41", "version_effective_date"}, []string{"version_effective_date", "usage: regcorpus meta"}},
		{"no citation", []string{"set", "1024.41", "version_fr_citations"}, []string{"version_fr_citations", "usage: regcorpus meta"}},
		{"two dates", []string{"set", "1024.41", "version_effective_date", "2025-07-15", "2025-07-16"}, []string{"version_effective_date", "one"}},
		{"a date that isn't one", []string{"set", "1024.41", "version_effective_date", "2025-02-30"}, []string{`"2025-02-30"`}},
		{"a citation that isn't one", []string{"set", "1024.41", "version_fr_citations", "90 FR"}, []string{`"90 FR"`}},
		{"a citation with its date", []string{"set", "1024.41", "version_fr_citations", "90 FR 20792, May 16, 2025"}, []string{"90 FR 20792, May 16, 2025"}},
		{"an unknown section", []string{"set", "1024.99", "version_effective_date", "2025-07-15"}, []string{`unknown section "1024.99"`}},
		{"an unknown field", []string{"set", "1024.41", "version_date", "2025-07-15"}, []string{`"version_date"`}},
		{"a flag", []string{"set", "--root", "x", "1024.41", "version_effective_date", "2025-07-15"}, []string{`"--root"`, "takes no flags"}},
		{"a flag after the field", []string{"set", "1024.41", "version_effective_date", "--force", "2025-07-15"}, []string{`"--force"`, "takes no flags"}},
		{"a citation twice", []string{"set", "1024.41", "version_fr_citations", "90 FR 20792", "78 FR 10876", "90 FR 20792"},
			[]string{"version_fr_citations", `"90 FR 20792"`, "twice"}},
	}
	for _, f := range meta.Fields() {
		if f == "version_effective_date" || f == "version_fr_citations" {
			continue
		}
		cases = append(cases, struct {
			name string
			args []string
			want []string
		}{"the field " + f, []string{"set", "1024.41", f, "x"},
			[]string{f, "version_effective_date", "version_fr_citations", "written by the command that computes it"}})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := citedCorpus(t)
			before := tree(t, root)
			code, stdout, stderr := metaAt(t, root, tc.args...)
			if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "regcorpus meta: ") {
				t.Errorf("exit %d, stdout %q, stderr %q; want 2 and an error", code, stdout, stderr)
			}
			for _, w := range tc.want {
				if !strings.Contains(stderr, w) {
					t.Errorf("stderr %q lacks %q", stderr, w)
				}
			}
			if !equalTrees(before, tree(t, root)) {
				t.Errorf("files changed")
			}
		})
	}
	t.Run("not the corpus root", func(t *testing.T) {
		root := t.TempDir()
		code, stdout, stderr := metaAt(t, root, "set", "1024.41", "version_effective_date", "2025-07-15")
		if code != 2 || stdout != "" || !strings.Contains(stderr, "not a corpus root") {
			t.Errorf("exit %d, stdout %q, stderr %q; want 2, not a corpus root", code, stdout, stderr)
		}
		if after := tree(t, root); len(after) != 0 {
			t.Errorf("wrote %v", keysOf(after))
		}
	})
}

// TestMetaHelpGivesTheExitCodes: -h prints the usage, with the exit codes,
// on stdout and exits 0.
func TestMetaHelpGivesTheExitCodes(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"-help"}, {"set", "-h"}} {
		code, stdout, stderr := metaAt(t, t.TempDir(), args...)
		if code != 0 || !strings.HasPrefix(stdout, "usage: regcorpus meta set") || stderr != "" {
			t.Errorf("%q: exit %d, stdout %q, stderr %q; want 0 and usage", args, code, stdout, stderr)
		}
		for _, w := range []string{"Exit 0", "Exit 1", "Exit 2", "<CITA>"} {
			if !strings.Contains(stdout, w) {
				t.Errorf("%q: usage lacks %q", args, w)
			}
		}
		// Exit 1 names each refusal by what is stored, raw XML the strict
		// parse refuses too (TestCitationsComeOnlyFromWellFormedXML).
		_, exit1, _ := strings.Cut(stdout, "Exit 1")
		exit1, _, _ = strings.Cut(exit1, "Exit 2")
		for _, w := range []string{"run regcorpus fetch first", "spec's schema", "another source", "raw/", "its bytes", "symlink", "well-formed", "doesn't cite"} {
			if !strings.Contains(exit1, w) {
				t.Errorf("%q: the Exit 1 list lacks %q", args, w)
			}
		}
	}
}

// TestMetaSetReplacesMetaYAMLWhole: the new meta.yaml is world-readable like
// every stored file, and no temporary file is left beside it.
func TestMetaSetReplacesMetaYAMLWhole(t *testing.T) {
	root := citedCorpus(t)
	if code, _, stderr := metaAt(t, root, "set", "1024.41", "version_effective_date", "2025-07-15"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(metaPath("1024.41"))))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("meta.yaml mode %v, want 0644", info.Mode().Perm())
	}
	ents, err := os.ReadDir(filepath.Join(root, filepath.FromSlash("sources/ecfr/title-12/1024.41")))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	if got := strings.Join(names, " "); got != ".gitkeep meta.yaml raw" {
		t.Errorf("source directory holds %q, want .gitkeep, meta.yaml and raw/ only", got)
	}
}

// TestMetaSetWriteFailureExitsTwo: a meta.yaml that can't be replaced is a
// local file error (exit 2), and it is left as it was.
func TestMetaSetWriteFailureExitsTwo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running with privileges that write into a mode-0555 directory")
	}
	root := citedCorpus(t)
	dir := filepath.Join(root, filepath.FromSlash("sources/ecfr/title-12/1024.41"))
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	before := tree(t, root)
	code, stdout, stderr := metaAt(t, root, "set", "1024.41", "version_effective_date", "2025-07-15")
	if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "regcorpus meta: 1024.41: ") {
		t.Errorf("exit %d, stdout %q, stderr %q; want 2 for 1024.41", code, stdout, stderr)
	}
	if !equalTrees(before, tree(t, root)) {
		t.Errorf("files changed")
	}
}

// TestCommittedSourcesCarryStep3Fields: each committed source's meta.yaml
// has version_effective_date and version_fr_citations (R1, plan step 3), and
// setting them again on a copy is a no-op: each citation is in the citation
// note of the committed raw XML meta.yaml describes. Needs the repository.
func TestCommittedSourcesCarryStep3Fields(t *testing.T) {
	top := repoTop(t)
	for _, s := range sources {
		t.Run(s.Name, func(t *testing.T) {
			root := corpusRoot(t)
			b, err := os.ReadFile(filepath.Join(top, filepath.FromSlash(s.dir()), "meta.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			m, err := meta.Parse(b)
			if err != nil {
				t.Fatalf("%s/meta.yaml: %v", s.dir(), err)
			}
			if m.VersionEffectiveDate == "" || m.VersionFRCitations == nil {
				t.Fatalf("%s/meta.yaml lacks version_effective_date or version_fr_citations", s.dir())
			}
			raw := "raw/" + string(m.SourceVersionDate) + ".xml"
			body, err := os.ReadFile(filepath.Join(top, filepath.FromSlash(s.dir()), filepath.FromSlash(raw)))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, root, s.dir()+"/meta.yaml", string(b))
			writeFile(t, root, s.dir()+"/"+raw, string(body))
			before := tree(t, root)
			cites := make([]string, len(m.VersionFRCitations))
			for i, c := range m.VersionFRCitations {
				cites[i] = string(c)
			}
			for _, args := range [][]string{
				{"version_effective_date", string(m.VersionEffectiveDate)},
				append([]string{"version_fr_citations"}, cites...),
			} {
				code, stdout, stderr := metaAt(t, root, append([]string{"set", s.Name}, args...)...)
				if code != 0 || !strings.HasPrefix(stdout, s.Name+": unchanged ") {
					t.Errorf("%q: exit %d, stdout %q, stderr %q; want 0, unchanged", args, code, stdout, stderr)
				}
			}
			if !equalTrees(before, tree(t, root)) {
				t.Errorf("files changed")
			}
		})
	}
}

// TestEveryMetaWriterChecksTheStoredState: before writing anything, every
// command that writes a meta.yaml (fetch on a re-run, meta set for either
// field) checks one thing: meta.yaml is in the spec's schema; its cfr, tier,
// role, source and official are the source table's; and raw/ holds exactly
// raw/<source_version_date>.xml, a regular file through real directories,
// with sha256_raw's bytes. Anything else exits 1 and changes nothing.
func TestEveryMetaWriterChecksTheStoredState(t *testing.T) {
	const older = "sources/ecfr/title-12/1024.41/raw/2025-05-16.xml"
	good := fetchMeta("2025-07-15", sha(citedBody))
	edit := func(old, new string) func(*testing.T, string) {
		return func(t *testing.T, root string) {
			if !strings.Contains(good, old) {
				t.Fatalf("fixture lacks %q", old)
			}
			writeFile(t, root, metaPath("1024.41"), strings.Replace(good, old, new, 1))
		}
	}
	rawFull := func(root string) string { return filepath.Join(root, filepath.FromSlash(rawPath("1024.41"))) }
	writers := []struct {
		name string
		run  func(t *testing.T, root string) (code int, stdout, stderr string)
	}{
		{"fetch", func(t *testing.T, root string) (int, string, string) {
			f, c := newFake(t)
			f.set(xmlURI("2025-07-15", "1024.41"), reply{body: citedBody})
			return fetchAt(t, root, c, "2026-11-22", "1024.41")
		}},
		{"meta set version_effective_date", func(t *testing.T, root string) (int, string, string) {
			return metaAt(t, root, "set", "1024.41", "version_effective_date", "2025-07-15")
		}},
		{"meta set version_fr_citations", func(t *testing.T, root string) (int, string, string) {
			return metaAt(t, root, "set", "1024.41", "version_fr_citations", "90 FR 20792")
		}},
	}
	for _, d := range []struct {
		name  string
		setup func(*testing.T, string)
		want  []string // in stderr
	}{
		{"cfr names another section", edit("12 CFR 1024.41", "12 CFR 1024.30"), []string{metaPath("1024.41"), "12 CFR 1024.30"}},
		{"another tier", edit("tier: law", "tier: regulator_interpretation"), []string{metaPath("1024.41"), "regulator_interpretation"}},
		{"another role", edit("role: inventoried", "role: cite_only"), []string{metaPath("1024.41"), "cite_only"}},
		{"official: true", edit("official: false", "official: true"), []string{metaPath("1024.41"), "official"}},
		{"a field the schema doesn't have", func(t *testing.T, root string) {
			writeFile(t, root, metaPath("1024.41"), good+"note: \"x\"\n")
		}, []string{metaPath("1024.41"), `"note"`}},
		{"another source_version_date", edit(`source_version_date: "2025-07-15"`, `source_version_date: "2025-05-16"`),
			[]string{metaPath("1024.41"), "2025-05-16"}},
		{"another sha256_raw", edit(sha(citedBody), strings.Repeat("d", 64)), []string{strings.Repeat("d", 64)}},
		{"the raw file missing", func(t *testing.T, root string) {
			if err := os.Remove(rawFull(root)); err != nil {
				t.Fatal(err)
			}
		}, []string{"git restore"}},
		{"the raw file changed", func(t *testing.T, root string) {
			writeFile(t, root, rawPath("1024.41"), strings.Replace(citedBody, "Stand-in text", "Other text", 1))
		}, []string{rawPath("1024.41"), "sha256"}},
		{"a second raw file", func(t *testing.T, root string) {
			writeFile(t, root, older, citedBody)
		}, []string{older}},
		{"the raw file a symlink", func(t *testing.T, root string) {
			writeFile(t, root, "elsewhere.xml", citedBody)
			if err := os.Remove(rawFull(root)); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("../../../../../elsewhere.xml", rawFull(root)); err != nil {
				t.Fatal(err)
			}
		}, []string{rawPath("1024.41"), "not a regular file"}},
		{"raw/ a symlink", func(t *testing.T, root string) {
			dir := filepath.Dir(rawFull(root))
			out := t.TempDir()
			writeFile(t, out, "2025-07-15.xml", citedBody)
			if err := os.RemoveAll(dir); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(out, dir); err != nil {
				t.Fatal(err)
			}
		}, []string{"sources/ecfr/title-12/1024.41/raw", "not a real directory"}},
	} {
		for _, w := range writers {
			t.Run(d.name+", "+w.name, func(t *testing.T) {
				root := citedCorpus(t)
				d.setup(t, root)
				before := tree(t, root)
				code, stdout, stderr := w.run(t, root)
				if code != 1 || stdout != "" {
					t.Errorf("exit %d, stdout %q, stderr %q; want 1 and nothing on stdout", code, stdout, stderr)
				}
				for _, s := range d.want {
					if !strings.Contains(stderr, s) {
						t.Errorf("stderr %q lacks %q", stderr, s)
					}
				}
				if !equalTrees(before, tree(t, root)) {
					t.Errorf("files changed")
				}
			})
		}
	}
	// The same stored state, untouched, is accepted by each.
	for _, w := range writers {
		t.Run("as fetch stored it, "+w.name, func(t *testing.T) {
			root := citedCorpus(t)
			if code, _, stderr := w.run(t, root); code != 0 {
				t.Errorf("exit %d, stderr %q; want 0", code, stderr)
			}
		})
	}
}

// TestConcurrentMetaSetsKeepEachOthersField: two meta set runs on one
// source at once, each setting a different field, both exit 0 and both
// fields are in meta.yaml after them: neither replaces meta.yaml with a copy
// read before the other wrote (no lost update).
func TestConcurrentMetaSetsKeepEachOthersField(t *testing.T) {
	want := withLines(`"2025-07-15"`, `["90 FR 20792"]`)
	for i := range 20 {
		root := citedCorpus(t)
		start := make(chan struct{})
		var wg sync.WaitGroup
		codes := make([]int, 2)
		stderrs := make([]string, 2)
		for j, args := range [][]string{
			{"set", "1024.41", "version_effective_date", "2025-07-15"},
			{"set", "1024.41", "version_fr_citations", "90 FR 20792"},
		} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				var out, errb bytes.Buffer
				codes[j] = metaMain(root, args, &out, &errb)
				stderrs[j] = errb.String()
			}()
		}
		close(start)
		wg.Wait()
		if codes[0] != 0 || codes[1] != 0 {
			t.Fatalf("run %d: exits %v, stderr %q; want 0 and 0", i, codes, stderrs)
		}
		if got := readFile(t, root, metaPath("1024.41")); got != want {
			t.Fatalf("run %d: meta.yaml:\n%s\nwant both fields:\n%s", i, got, want)
		}
	}
}

// notedCorpus returns a corpus root where 1024.41's raw XML holds notes, the
// markup of its citation notes (<CITA> elements and what is between them),
// and the meta.yaml fetch writes for it.
func notedCorpus(t *testing.T, notes string) string {
	t.Helper()
	body := "<?xml version=\"1.0\" encoding=\"UTF-8\"?> \n<DIV8 N=\"1024.41\" TYPE=\"SECTION\">\n" +
		"<HEAD>§ 1024.41 Stand-in.</HEAD>\n<P>(a) Stand-in text.</P>\n" + notes + "\n</DIV8>\n"
	root := corpusRoot(t)
	writeFile(t, root, rawPath("1024.41"), body)
	writeFile(t, root, metaPath("1024.41"), fetchMeta("2025-07-15", sha(body)))
	return root
}

// TestCitationsAreReadFromEachNoteAsWritten: a citation is "<vol> FR
// <page>" within one run of a citation note's text, its parts separated by
// spaces, tabs, line breaks or no-break spaces; a shorthand page after a
// citation ("78 FR 63004, 63005") is cited too, up to the next citation.
// Text in two notes, or on either side of an element inside a note, never
// joins into one.
func TestCitationsAreReadFromEachNoteAsWritten(t *testing.T) {
	for _, tc := range []struct {
		name, notes string
		cite        string
		code        int
	}{
		{"parts across a line break", "<CITA TYPE=\"N\">[78 FR 10876, Feb. 14, 2013; 90\nFR\n20792, May 16, 2025]\n</CITA>", "90 FR 20792", 0},
		{"parts across no-break spaces", "<CITA TYPE=\"N\">[90&#xA0;FR 20792, May 16, 2025]</CITA>", "90 FR 20792", 0},
		{"a citation after an element in the note", "<CITA TYPE=\"N\">[<E T=\"03\">See</E> 90 FR 20792, May 16, 2025]</CITA>", "90 FR 20792", 0},
		{"a citation inside an element in the note", "<CITA TYPE=\"N\">[<E T=\"03\">90 FR 20792</E>, May 16, 2025]</CITA>", "90 FR 20792", 0},
		{"the first of a shorthand pair", "<CITA TYPE=\"N\">[78 FR 63004, 63005, Oct. 23, 2013]</CITA>", "78 FR 63004", 0},
		{"the second of a shorthand pair", "<CITA TYPE=\"N\">[78 FR 63004, 63005, Oct. 23, 2013]</CITA>", "78 FR 63005", 0},
		{"the third of a shorthand run", "<CITA TYPE=\"N\">[78 FR 63004, 63005, 63006, Oct. 23, 2013]</CITA>", "78 FR 63006", 0},
		{"a year is no page", "<CITA TYPE=\"N\">[78 FR 60437, Oct. 1, 2013]</CITA>", "78 FR 2013", 1},
		{"parts split by an element", "<CITA TYPE=\"N\">[90 FR <E T=\"03\">20792</E>, May 16, 2025]</CITA>", "90 FR 20792", 1},
		{"parts split by an element's end", "<CITA TYPE=\"N\">[<E T=\"03\">90 FR</E> 20792, May 16, 2025]</CITA>", "90 FR 20792", 1},
		{"a citation after another, past only a comma", "<CITA TYPE=\"N\">[78 FR 6856, 81 FR 72370, Oct. 19, 2016]</CITA>", "81 FR 72370", 0},
		{"no shorthand page taken from the next citation", "<CITA TYPE=\"N\">[78 FR 6856, 81 FR 72370, Oct. 19, 2016]</CITA>", "78 FR 81", 1},
		{"a citation in the first of two notes", "<CITA TYPE=\"N\">[78 FR 10876, Feb. 14, 2013]</CITA>\n<P>(b) Text.</P>\n<CITA TYPE=\"N\">[90 FR 20792, May 16, 2025]</CITA>", "78 FR 10876", 0},
		{"a citation in the second of two notes", "<CITA TYPE=\"N\">[78 FR 10876, Feb. 14, 2013]</CITA>\n<P>(b) Text.</P>\n<CITA TYPE=\"N\">[90 FR 20792, May 16, 2025]</CITA>", "90 FR 20792", 0},
		{"parts split across two notes", "<CITA TYPE=\"N\">[90 FR</CITA>\n<CITA TYPE=\"N\">20792, May 16, 2025]</CITA>", "90 FR 20792", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := notedCorpus(t, tc.notes)
			code, stdout, stderr := metaAt(t, root, "set", "1024.41", "version_fr_citations", tc.cite)
			if code != tc.code {
				t.Errorf("exit %d, stdout %q, stderr %q; want %d", code, stdout, stderr, tc.code)
			}
		})
	}
	// A refusal lists exactly the citations the note cites, each once.
	root := notedCorpus(t, "<CITA TYPE=\"N\">[78 FR 10876, Feb. 14, 2013, as amended at 78 FR 63004, 63005, Oct. 23, 2013; "+
		"78 FR 10876, Feb. 14, 2013; 90 FR 20792, May 16, 2025]</CITA>")
	code, _, stderr := metaAt(t, root, "set", "1024.41", "version_fr_citations", "90 FR 1")
	if want := "which cites 78 FR 10876, 78 FR 63004, 78 FR 63005, 90 FR 20792;"; code != 1 || !strings.Contains(stderr, want) {
		t.Errorf("exit %d, stderr %q; want 1 and %q", code, stderr, want)
	}
	root = notedCorpus(t, "<CITA TYPE=\"N\">[78 FR 6856, 81 FR 72370, Oct. 19, 2016]</CITA>")
	code, _, stderr = metaAt(t, root, "set", "1024.41", "version_fr_citations", "90 FR 1")
	if want := "which cites 78 FR 6856, 81 FR 72370;"; code != 1 || !strings.Contains(stderr, want) {
		t.Errorf("exit %d, stderr %q; want 1 and %q", code, stderr, want)
	}
}

// TestCitationsComeOnlyFromWellFormedXML: raw XML a strict parse refuses
// (here an unquoted attribute) is refused (exit 1, naming the raw file), even
// with sha256_raw's bytes; no citation is read from it.
func TestCitationsComeOnlyFromWellFormedXML(t *testing.T) {
	root := notedCorpus(t, "<CITA TYPE=N>[90 FR 20792, May 16, 2025]</CITA>")
	before := tree(t, root)
	code, stdout, stderr := metaAt(t, root, "set", "1024.41", "version_fr_citations", "90 FR 20792")
	if code != 1 || stdout != "" || !strings.Contains(stderr, rawPath("1024.41")) || !strings.Contains(stderr, "well-formed") {
		t.Errorf("exit %d, stdout %q, stderr %q; want 1 naming the raw file as not well-formed", code, stdout, stderr)
	}
	if !equalTrees(before, tree(t, root)) {
		t.Errorf("files changed")
	}
}

// TestEffectiveDateIsTakenAsGiven: version_effective_date is set as given,
// whatever its relation to source_version_date (2025-07-15 here): only its
// form is checked. eCFR also dates a version by a rule's publication, before
// the rule takes effect, so no bound between the two dates holds.
func TestEffectiveDateIsTakenAsGiven(t *testing.T) {
	for _, date := range []string{"2025-07-15", "2013-02-14", "2025-07-16", "2026-01-01"} {
		t.Run(date, func(t *testing.T) {
			root := citedCorpus(t)
			code, stdout, stderr := metaAt(t, root, "set", "1024.41", "version_effective_date", date)
			if code != 0 || stderr != "" {
				t.Fatalf("exit %d, stdout %q, stderr %q; want 0", code, stdout, stderr)
			}
			if got, want := readFile(t, root, metaPath("1024.41")), withLines(`"`+date+`"`, ""); got != want {
				t.Errorf("meta.yaml:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
