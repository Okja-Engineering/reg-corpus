// Tests for the per-commit C6 registry-state rule and the `check` command
// surface (C6, R2, R5).
package check

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const record = "id: RX-41-TEST-01\nrev: 1\n"

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// corpus builds a root dir. files maps slash paths to contents; a nil map
// leaves the root empty.
func corpus(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for p, body := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func logLine(id, hash string) string {
	return id + ", 1, " + hash + ", tester, 2026-11-21, proposal_review\n"
}

func TestCorpusRegistryState(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		dirs  []string
		want  []string // substrings, one per expected violation line; empty = clean
	}{
		{name: "no registry passes"},
		{name: "empty registry passes", dirs: []string{"registry/records"}},
		{
			name:  "unlogged record fails",
			files: map[string]string{"registry/records/RX-41-TEST-01.yaml": record, "registry/verify.log": ""},
			want:  []string{"C6 registry-state: registry/records/RX-41-TEST-01.yaml: no matching verify.log line"},
		},
		{
			name:  "missing verify.log with records fails",
			files: map[string]string{"registry/records/RX-41-TEST-01.yaml": record},
			want:  []string{"C6 registry-state: registry/records/RX-41-TEST-01.yaml: no matching verify.log line"},
		},
		{
			name: "sha mismatch fails",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/verify.log":                 logLine("RX-41-TEST-01", sum("something else")),
			},
			want: []string{"C6 registry-state: registry/records/RX-41-TEST-01.yaml: sha256"},
		},
		{
			name: "line for another ID does not match",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/verify.log":                 logLine("RX-41-TEST-02", sum(record)),
			},
			want: []string{"C6 registry-state: registry/records/RX-41-TEST-01.yaml: no matching verify.log line"},
		},
		{
			name: "matching line passes",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/verify.log":                 logLine("RX-41-TEST-01", sum(record)),
			},
		},
		{
			name: "matching line with loose spacing passes",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/verify.log": "RX-41-TEST-01 ,1,  " + sum(record) +
					" ,tester,2026-11-21 , proposal_review  \n",
			},
		},
		{
			name: "a later line matching the current bytes passes",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/verify.log": logLine("RX-41-TEST-01", sum("rev 0")) +
					logLine("RX-41-TEST-01", sum(record)),
			},
		},
		{
			name: "non-RX-*.yaml file fails",
			files: map[string]string{
				"registry/records/notes.txt": "x",
				"registry/verify.log":        "",
			},
			want: []string{"C6 registry-state: registry/records/notes.txt: "},
		},
		{
			name: "yaml file without the RX- prefix fails",
			files: map[string]string{
				"registry/records/41-TEST-01.yaml": record,
				"registry/verify.log":              logLine("41-TEST-01", sum(record)),
			},
			want: []string{"C6 registry-state: registry/records/41-TEST-01.yaml: "},
		},
		{
			name: "RX- file with the wrong extension fails",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yml": record,
				"registry/verify.log":                logLine("RX-41-TEST-01", sum(record)),
			},
			want: []string{"C6 registry-state: registry/records/RX-41-TEST-01.yml: "},
		},
		{
			name: "one line per violation",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/records/RX-41-TEST-02.yaml": "id: RX-41-TEST-02\n",
				"registry/records/readme.md":          "x",
				"registry/verify.log":                 logLine("RX-41-TEST-01", sum(record)),
			},
			want: []string{
				"C6 registry-state: registry/records/RX-41-TEST-02.yaml: no matching verify.log line",
				"C6 registry-state: registry/records/readme.md: ",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := corpus(t, tc.files)
			for _, d := range tc.dirs {
				if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Corpus(root)
			if err != nil {
				t.Fatalf("Corpus: unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d violations %q, want %d %q", len(got), got, len(tc.want), tc.want)
			}
			for i, w := range tc.want {
				if !strings.HasPrefix(got[i].String(), w) {
					t.Errorf("violation %d = %q, want prefix %q", i, got[i].String(), w)
				}
			}
		})
	}
}

func TestCorpusSubdirectoryInRecordsFails(t *testing.T) {
	root := corpus(t, map[string]string{"registry/verify.log": ""})
	if err := os.MkdirAll(filepath.Join(root, "registry", "records", "RX-41-DIR.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Corpus(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0].String(), "C6 registry-state: registry/records/RX-41-DIR.yaml: ") {
		t.Fatalf("got %q, want one violation for the directory", got)
	}
}

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Main(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestMainCleanCorpus(t *testing.T) {
	root := corpus(t, nil)
	code, out, _ := run(t, "--corpus", "--root", root)
	if code != 0 || out != "C6 ok\n" {
		t.Fatalf("exit %d, stdout %q; want 0 and %q", code, out, "C6 ok\n")
	}
}

func TestMainViolationsExitOne(t *testing.T) {
	root := corpus(t, map[string]string{"registry/records/RX-41-TEST-01.yaml": record})
	code, out, _ := run(t, "--corpus", "--root", root)
	want := "C6 registry-state: registry/records/RX-41-TEST-01.yaml: no matching verify.log line"
	if code != 1 || !strings.Contains(out, want) {
		t.Fatalf("exit %d, stdout %q; want 1 and a line %q", code, out, want)
	}
	if strings.Contains(out, "C6 ok") {
		t.Fatalf("stdout %q reports ok despite a violation", out)
	}
}

func TestMainRootDefaultsToCwd(t *testing.T) {
	root := corpus(t, map[string]string{"registry/records/RX-41-TEST-01.yaml": record})
	t.Chdir(root)
	code, out, _ := run(t, "--corpus")
	if code != 1 || !strings.Contains(out, "RX-41-TEST-01") {
		t.Fatalf("exit %d, stdout %q; want 1 naming RX-41-TEST-01 from cwd", code, out)
	}
}

func TestMainWithoutCorpusExitsTwo(t *testing.T) {
	code, _, errOut := run(t)
	if code != 2 || !strings.Contains(errOut, "consumer checks C1–C5 arrive in plan step 22") {
		t.Fatalf("exit %d, stderr %q; want 2 and the plan step 22 message", code, errOut)
	}
}

func TestMainBadUsageExitsTwo(t *testing.T) {
	for _, args := range [][]string{{"--bogus"}, {"--corpus", "extra"}} {
		if code, _, _ := run(t, args...); code != 2 {
			t.Errorf("args %q: exit %d, want 2", args, code)
		}
	}
}
