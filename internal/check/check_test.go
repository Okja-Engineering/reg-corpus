// Tests for `regcorpus check --corpus` (C6 per-commit registry state, R2, R5):
// the command contract (exit codes 0/1/2, exact lines, stdout vs stderr), the
// working-tree and git-tree (--tree) snapshots, and corpus-root validation.
package check

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	record   = "id: RX-41-TEST-01\nrev: 1\n"
	goMod    = "module github.com/okja-engineering/reg-corpus\n\ngo 1.27\n"
	notRoot  = "regcorpus check: %s is not a corpus root"
	usageTop = "usage: regcorpus check --corpus [--root DIR] [--tree TREE-ISH]"
)

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func logLine(id, hash string) string {
	return id + ", 1, " + hash + ", tester, 2026-11-21, proposal_review\n"
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, body := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// corpus builds a corpus root (go.mod declaring this module, and stages/)
// holding files, keyed by slash path.
func corpus(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, map[string]string{"go.mod": goMod, "stages/01-fetch-parse/CONTRACT.md": "x\n"})
	write(t, root, files)
	return root
}

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Main(args, &out, &errb)
	return code, out.String(), errb.String()
}

// lines runs check on root and returns exit code and stdout lines; stderr
// must be empty.
func lines(t *testing.T, args ...string) (int, []string) {
	t.Helper()
	code, out, errOut := run(t, args...)
	if errOut != "" {
		t.Fatalf("stderr %q, want empty", errOut)
	}
	return code, strings.Split(strings.TrimSuffix(out, "\n"), "\n")
}

func want(t *testing.T, gotCode int, got []string, wantCode int, wantLines ...string) {
	t.Helper()
	if wantCode == 0 {
		wantLines = []string{"C6 ok"}
	}
	if gotCode != wantCode || strings.Join(got, "\n") != strings.Join(wantLines, "\n") {
		t.Fatalf("exit %d, stdout:\n%s\nwant exit %d, stdout:\n%s",
			gotCode, strings.Join(got, "\n"), wantCode, strings.Join(wantLines, "\n"))
	}
}

const (
	p1        = "C6 registry-state: registry/records/RX-41-TEST-01.yaml: "
	unlogged  = p1 + "no matching verify.log line"
	noLogFile = p1 + "no matching verify.log line (registry/verify.log is missing)"
	onlyRX    = "registry/records/ holds only RX-*.yaml record files"
	layout    = "not in the registry layout: records/RX-*.yaml, verify.log, topics.yaml, inventory/*.yaml (names are case-sensitive)"
)

func TestRegistryStateRules(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		dirs  []string
		code  int
		lines []string
	}{
		{name: "no registry passes"},
		{name: "empty records passes", dirs: []string{"registry/records"}},
		{name: "other registry files pass", files: map[string]string{
			"registry/topics.yaml": "x\n", "registry/inventory/1024.41.yaml": "x\n"}},
		{
			name:  "unlogged record fails",
			files: map[string]string{"registry/records/RX-41-TEST-01.yaml": record, "registry/verify.log": ""},
			code:  1, lines: []string{unlogged},
		},
		{
			name:  "missing verify.log fails",
			files: map[string]string{"registry/records/RX-41-TEST-01.yaml": record},
			code:  1, lines: []string{noLogFile},
		},
		{
			name: "sha256 mismatch fails",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/verify.log":                 logLine("RX-41-TEST-01", sum("other")),
			},
			code: 1, lines: []string{p1 + "sha256 mismatch: file is " + sum(record) +
				"; verify.log has " + sum("other") + " for RX-41-TEST-01"},
		},
		{
			name: "every logged hash is listed on a mismatch",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/verify.log":                 logLine("RX-41-TEST-01", sum("a")) + logLine("RX-41-TEST-01", sum("b")),
			},
			code: 1, lines: []string{p1 + "sha256 mismatch: file is " + sum(record) +
				"; verify.log has " + sum("a") + ", " + sum("b") + " for RX-41-TEST-01"},
		},
		{
			name: "line for another ID does not match",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/verify.log":                 logLine("RX-41-TEST-02", sum(record)),
			},
			code: 1, lines: []string{unlogged},
		},
		{
			name: "matching line passes",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/verify.log":                 logLine("RX-41-TEST-01", sum(record)),
			},
		},
		{
			name: "loose spacing, upper-case hash and CRLF pass",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/verify.log": " RX-41-TEST-01 ,1,  " + strings.ToUpper(sum(record)) +
					" ,tester,2026-11-21 , proposal_review  \r\n",
			},
		},
		{
			name: "a later line matching the current bytes passes",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/verify.log":                 logLine("RX-41-TEST-01", sum("rev 0")) + logLine("RX-41-TEST-01", sum(record)),
			},
		},
		{
			name: "a line over 64 KiB does not stop the scan",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/verify.log":                 strings.Repeat("x", 200_000) + "\n" + logLine("RX-41-TEST-01", sum(record)),
			},
		},
		{
			name: "only six-field lines match; malformed lines are skipped",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/verify.log": "\n,,,,,\n,\nRX-41-TEST-01\n" +
					"RX-41-TEST-01, 1, " + sum(record) + ", tester, 2026-11-21\n" +
					"RX-41-TEST-01, 1, " + sum(record) + ", tester, 2026-11-21, proposal_review, extra\n" +
					"RX-41-TEST-01, 1, " + sum(record) + "\n",
			},
			code: 1, lines: []string{unlogged},
		},
		{
			name:  "non-RX file fails",
			files: map[string]string{"registry/records/notes.txt": "x", "registry/verify.log": ""},
			code:  1, lines: []string{"C6 registry-state: registry/records/notes.txt: " + onlyRX},
		},
		{
			name: "yaml file without the RX- prefix fails",
			files: map[string]string{
				"registry/records/41-TEST-01.yaml": record,
				"registry/verify.log":              logLine("41-TEST-01", sum(record)),
			},
			code: 1, lines: []string{"C6 registry-state: registry/records/41-TEST-01.yaml: " + onlyRX},
		},
		{
			name: "RX- file with the wrong extension fails",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yml": record,
				"registry/verify.log":                logLine("RX-41-TEST-01", sum(record)),
			},
			code: 1, lines: []string{"C6 registry-state: registry/records/RX-41-TEST-01.yml: " + onlyRX},
		},
		{
			name:  "a directory in records fails",
			files: map[string]string{"registry/verify.log": ""},
			dirs:  []string{"registry/records/RX-41-DIR.yaml"},
			code:  1, lines: []string{"C6 registry-state: registry/records/RX-41-DIR.yaml: " + onlyRX},
		},
		{
			name: "a record under a case variant of records/ is reported, logged or not",
			files: map[string]string{
				"registry/Records/RX-41-TEST-01.yaml": record,
				"registry/verify.log":                 logLine("RX-41-TEST-01", sum(record)),
			},
			code: 1, lines: []string{
				"C6 registry-state: registry/Records: " + layout,
				"C6 registry-state: registry/Records/RX-41-TEST-01.yaml: " + layout,
			},
		},
		{
			name:  "a top-level name that folds to registry is the registry",
			files: map[string]string{"Registry/records/RX-41-TEST-01.yaml": record},
			code:  1, lines: []string{
				"C6 registry-state: Registry: " + layout,
				"C6 registry-state: Registry/records: " + layout,
				"C6 registry-state: Registry/records/RX-41-TEST-01.yaml: " + layout,
			},
		},
		{
			name: "files outside records/, verify.log, topics.yaml and inventory/*.yaml are reported",
			files: map[string]string{
				"registry/RX-41-TEST-01.yaml":      record,
				"registry/notes.txt":               "x\n",
				"registry/inventory/1024.41.yml":   "x\n",
				"registry/inventory/sub/1024.yaml": "x\n",
				"registry/records/sub/RX-1.yaml":   record,
				"registry/inventory/1024.41.yaml":  "x\n",
				"registry/topics.yaml":             "x\n",
				"registry/verify.log":              "",
			},
			code: 1, lines: []string{
				"C6 registry-state: registry/RX-41-TEST-01.yaml: " + layout,
				"C6 registry-state: registry/inventory/1024.41.yml: " + layout,
				"C6 registry-state: registry/inventory/sub: " + layout,
				"C6 registry-state: registry/inventory/sub/1024.yaml: " + layout,
				"C6 registry-state: registry/notes.txt: " + layout,
				"C6 registry-state: registry/records/sub: " + onlyRX,
				"C6 registry-state: registry/records/sub/RX-1.yaml: " + layout,
			},
		},
		{
			name: "a newline in a name stays on the violation's one line, quoted",
			files: map[string]string{
				"registry/records/RX-41-TEST-01\nC6 ok.yaml": record,
				"registry/verify.log":                        "",
			},
			code: 1, lines: []string{`C6 registry-state: "registry/records/RX-41-TEST-01\nC6 ok.yaml": no matching verify.log line`},
		},
		{
			name: "one line per violation, sorted by path",
			files: map[string]string{
				"registry/records/RX-41-TEST-01.yaml": record,
				"registry/records/RX-41-TEST-02.yaml": "id: RX-41-TEST-02\n",
				"registry/records/readme.md":          "x",
				"registry/verify.log":                 logLine("RX-41-TEST-01", sum(record)),
			},
			code: 1, lines: []string{
				"C6 registry-state: registry/records/RX-41-TEST-02.yaml: no matching verify.log line",
				"C6 registry-state: registry/records/readme.md: " + onlyRX,
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
			code, got := lines(t, "--corpus", "--root", root)
			want(t, code, got, tc.code, tc.lines...)
		})
	}
}

func mklink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func TestNonRegularEntriesFail(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "verify.log")
	write(t, filepath.Dir(outside), map[string]string{"verify.log": logLine("RX-41-TEST-01", sum(record))})

	t.Run("symlinked verify.log", func(t *testing.T) {
		root := corpus(t, map[string]string{"registry/records/RX-41-TEST-01.yaml": record})
		mklink(t, outside, filepath.Join(root, "registry", "verify.log"))
		code, got := lines(t, "--corpus", "--root", root)
		want(t, code, got, 1,
			unlogged,
			"C6 registry-state: registry/verify.log: is a symlink; registry/ holds only regular files")
	})
	t.Run("symlinked record", func(t *testing.T) {
		root := corpus(t, map[string]string{"registry/verify.log": "", "registry/records/RX-41-TEST-02.yaml": "x", "staged.yaml": record})
		mklink(t, filepath.Join(root, "staged.yaml"), filepath.Join(root, "registry", "records", "RX-41-TEST-01.yaml"))
		code, got := lines(t, "--corpus", "--root", root)
		want(t, code, got, 1, p1+"is a symlink; registry/ holds only regular files",
			"C6 registry-state: registry/records/RX-41-TEST-02.yaml: no matching verify.log line")
	})
	t.Run("dangling records symlink", func(t *testing.T) {
		root := corpus(t, map[string]string{"registry/verify.log": ""})
		mklink(t, filepath.Join(root, "nowhere"), filepath.Join(root, "registry", "records"))
		code, got := lines(t, "--corpus", "--root", root)
		want(t, code, got, 1, "C6 registry-state: registry/records: is a symlink; registry/ holds only regular files")
	})
	t.Run("symlinked registry", func(t *testing.T) {
		root := corpus(t, nil)
		elsewhere := corpus(t, map[string]string{"registry/verify.log": ""})
		mklink(t, filepath.Join(elsewhere, "registry"), filepath.Join(root, "registry"))
		code, got := lines(t, "--corpus", "--root", root)
		want(t, code, got, 1, "C6 registry-state: registry: is a symlink; registry/ holds only regular files")
	})
}

func TestUnreadableRecordNamesRuleAndFile(t *testing.T) {
	root := corpus(t, map[string]string{"registry/records/RX-41-TEST-01.yaml": record, "registry/verify.log": ""})
	f := filepath.Join(root, "registry", "records", "RX-41-TEST-01.yaml")
	if err := os.Chmod(f, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(f, 0o644) })
	if _, err := os.ReadFile(f); err == nil {
		t.Skip("running with privileges that read a mode-0 file")
	}
	code, got := lines(t, "--corpus", "--root", root)
	if code != 1 || len(got) != 1 || !strings.HasPrefix(got[0], p1+"cannot read: ") {
		t.Fatalf("exit %d, stdout %q; want 1 and one %q line", code, got, p1+"cannot read: ...")
	}
}

func TestUnlistableDirectoryNamesRuleAndPath(t *testing.T) {
	root := corpus(t, map[string]string{"registry/inventory/1024.41.yaml": "x\n"})
	d := filepath.Join(root, "registry", "inventory")
	if err := os.Chmod(d, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(d, 0o755) })
	if _, err := os.ReadDir(d); err == nil {
		t.Skip("running with privileges that list a mode-0 directory")
	}
	code, got := lines(t, "--corpus", "--root", root)
	if code != 1 || len(got) != 1 || !strings.HasPrefix(got[0], "C6 registry-state: registry/inventory: cannot read: ") {
		t.Fatalf("exit %d, stdout %q; want 1 and one cannot-read line for registry/inventory", code, got)
	}
}

func TestRootMustBeACorpus(t *testing.T) {
	good := corpus(t, map[string]string{"internal/check/x.go": "package check\n"})
	otherMod := corpus(t, map[string]string{"go.mod": "module example.com/other\n"})
	noStages := t.TempDir()
	write(t, noStages, map[string]string{"go.mod": goMod})
	quoted := corpus(t, map[string]string{"go.mod": "// c\nmodule \"github.com/okja-engineering/reg-corpus\" // x\n"})
	noMod := "(no go.mod declaring module github.com/okja-engineering/reg-corpus)\n"
	for _, tc := range []struct {
		name, root string
		code       int
		reason     string
	}{
		{"missing directory", filepath.Join(good, "typo"), 2, noMod},
		{"empty directory", t.TempDir(), 2, noMod},
		{"a subdirectory of a corpus", filepath.Join(good, "internal"), 2, noMod},
		{"another module", otherMod, 2, noMod},
		{"no stages/", noStages, 2, "(no stages/ directory)\n"},
		{"a corpus with no registry", good, 0, ""},
		{"quoted module path and comments", quoted, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errOut := run(t, "--corpus", "--root", tc.root)
			if code != tc.code {
				t.Fatalf("exit %d (stdout %q, stderr %q), want %d", code, out, errOut, tc.code)
			}
			if tc.code == 2 {
				want := strings.Replace(notRoot, "%s", tc.root, 1) + " " + tc.reason
				if out != "" || errOut != want {
					t.Fatalf("stdout %q, stderr %q; want nothing on stdout and %q on stderr", out, errOut, want)
				}
			}
		})
	}
}

func TestRootDefaultsToCwd(t *testing.T) {
	root := corpus(t, map[string]string{"registry/records/RX-41-TEST-01.yaml": record})
	t.Chdir(root)
	code, got := lines(t, "--corpus")
	want(t, code, got, 1, noLogFile)
}

// gitT runs git in dir and returns its trimmed stdout.
func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// repo builds a corpus with files and commits it in a new git repo.
func repo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := corpus(t, files)
	gitT(t, root, "init", "-q")
	gitT(t, root, "add", "-A")
	gitT(t, root, "commit", "-q", "-m", "base")
	return root
}

func TestTreeChecksCommittedBytesNotTheWorkingTree(t *testing.T) {
	root := repo(t, map[string]string{
		"registry/records/RX-41-TEST-01.yaml": record,
		"registry/verify.log":                 logLine("RX-41-TEST-01", sum(record)),
	})
	write(t, root, map[string]string{"registry/records/RX-41-TEST-01.yaml": "edited, not committed\n"})

	code, got := lines(t, "--corpus", "--root", root, "--tree", "HEAD")
	want(t, code, got, 0)
	code, got = lines(t, "--corpus", "--root", root)
	want(t, code, got, 1, p1+"sha256 mismatch: file is "+sum("edited, not committed\n")+
		"; verify.log has "+sum(record)+" for RX-41-TEST-01")

	tree := gitT(t, root, "write-tree")
	code, got = lines(t, "--corpus", "--root", filepath.Join(root, "stages"), "--tree", tree)
	want(t, code, got, 0)
}

func TestTreeRules(t *testing.T) {
	t.Run("unlogged committed record", func(t *testing.T) {
		root := repo(t, map[string]string{"registry/records/RX-41-TEST-01.yaml": record})
		code, got := lines(t, "--corpus", "--root", root, "--tree", "HEAD")
		want(t, code, got, 1, noLogFile)
	})
	t.Run("executable record is a regular file", func(t *testing.T) {
		root := corpus(t, map[string]string{"registry/verify.log": logLine("RX-41-TEST-01", sum(record))})
		write(t, root, map[string]string{"registry/records/RX-41-TEST-01.yaml": record})
		os.Chmod(filepath.Join(root, "registry", "records", "RX-41-TEST-01.yaml"), 0o755)
		gitT(t, root, "init", "-q")
		gitT(t, root, "add", "-A")
		code, got := lines(t, "--corpus", "--root", root, "--tree", gitT(t, root, "write-tree"))
		want(t, code, got, 0)
	})
	t.Run("symlinked verify.log", func(t *testing.T) {
		root := corpus(t, map[string]string{"registry/records/RX-41-TEST-01.yaml": record})
		mklink(t, "/tmp/elsewhere.log", filepath.Join(root, "registry", "verify.log"))
		gitT(t, root, "init", "-q")
		gitT(t, root, "add", "-A")
		code, got := lines(t, "--corpus", "--root", root, "--tree", gitT(t, root, "write-tree"))
		want(t, code, got, 1, unlogged,
			"C6 registry-state: registry/verify.log: is a symlink; registry/ holds only regular files")
	})
	t.Run("symlinked registry", func(t *testing.T) {
		root := corpus(t, nil)
		mklink(t, "/tmp/elsewhere", filepath.Join(root, "registry"))
		gitT(t, root, "init", "-q")
		gitT(t, root, "add", "-A")
		code, got := lines(t, "--corpus", "--root", root, "--tree", gitT(t, root, "write-tree"))
		want(t, code, got, 1, "C6 registry-state: registry: is a symlink; registry/ holds only regular files")
	})
	t.Run("gitlink under registry", func(t *testing.T) {
		root := repo(t, map[string]string{"registry/verify.log": ""})
		head := gitT(t, root, "rev-parse", "HEAD")
		gitT(t, root, "update-index", "--add", "--cacheinfo", "160000,"+head+",registry/inventory")
		code, got := lines(t, "--corpus", "--root", root, "--tree", gitT(t, root, "write-tree"))
		want(t, code, got, 1,
			"C6 registry-state: registry/inventory: is a gitlink (submodule); registry/ holds only regular files")
	})
	t.Run("case variants of registry/ and records/ beside the real ones", func(t *testing.T) {
		root := repo(t, map[string]string{
			"registry/records/RX-41-TEST-01.yaml": record,
			"registry/verify.log":                 logLine("RX-41-TEST-01", sum(record)),
		})
		blob := gitT(t, root, "rev-parse", "HEAD:registry/records/RX-41-TEST-01.yaml")
		for _, p := range []string{
			"registry/Records/RX-41-TEST-02.yaml",
			"Registry/records/RX-41-TEST-03.yaml",
			"regiſtry/records/RX-41-TEST-04.yaml", // long s: folds to s, as APFS does
		} {
			gitT(t, root, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+p)
		}
		code, got := lines(t, "--corpus", "--root", root, "--tree", gitT(t, root, "write-tree"))
		want(t, code, got, 1,
			"C6 registry-state: Registry: "+layout,
			"C6 registry-state: Registry/records: "+layout,
			"C6 registry-state: Registry/records/RX-41-TEST-03.yaml: "+layout,
			"C6 registry-state: registry/Records: "+layout,
			"C6 registry-state: registry/Records/RX-41-TEST-02.yaml: "+layout,
			"C6 registry-state: regiſtry: "+layout,
			"C6 registry-state: regiſtry/records: "+layout,
			"C6 registry-state: regiſtry/records/RX-41-TEST-04.yaml: "+layout,
		)
	})
	t.Run("a directory in records is listed and fails", func(t *testing.T) {
		root := repo(t, map[string]string{
			"registry/verify.log":                       "",
			"registry/records/RX-41-DIR.yaml/RX-1.yaml": record,
		})
		code, got := lines(t, "--corpus", "--root", root, "--tree", "HEAD")
		want(t, code, got, 1,
			"C6 registry-state: registry/records/RX-41-DIR.yaml: "+onlyRX,
			"C6 registry-state: registry/records/RX-41-DIR.yaml/RX-1.yaml: "+layout)
	})
	t.Run("replace refs don't change the bytes checked", func(t *testing.T) {
		root := repo(t, map[string]string{
			"registry/records/RX-41-TEST-01.yaml": record,
			"registry/verify.log":                 logLine("RX-41-TEST-01", sum("logged\n")),
		})
		write(t, root, map[string]string{"logged.txt": "logged\n"})
		logged := gitT(t, root, "hash-object", "-w", "logged.txt")
		gitT(t, root, "replace", gitT(t, root, "rev-parse", "HEAD:registry/records/RX-41-TEST-01.yaml"), logged)
		code, got := lines(t, "--corpus", "--root", root, "--tree", "HEAD")
		want(t, code, got, 1, p1+"sha256 mismatch: file is "+sum(record)+
			"; verify.log has "+sum("logged\n")+" for RX-41-TEST-01")
	})
	t.Run("tree that is not a corpus", func(t *testing.T) {
		root := repo(t, nil)
		gitT(t, root, "rm", "-q", "go.mod")
		tree := gitT(t, root, "write-tree")
		code, out, errOut := run(t, "--corpus", "--root", root, "--tree", tree)
		if code != 2 || out != "" || errOut != "regcorpus check: tree "+tree+
			" is not a corpus root (no go.mod declaring module github.com/okja-engineering/reg-corpus)\n" {
			t.Fatalf("exit %d, stdout %q, stderr %q; want 2 and a not-a-corpus message", code, out, errOut)
		}
	})
	t.Run("unknown tree-ish", func(t *testing.T) {
		root := repo(t, nil)
		code, out, errOut := run(t, "--corpus", "--root", root, "--tree", "no-such-ref")
		if code != 2 || out != "" || !strings.HasPrefix(errOut, "regcorpus check: ") {
			t.Fatalf("exit %d, stdout %q, stderr %q; want 2 and an error on stderr", code, out, errOut)
		}
	})
	t.Run("option-like tree-ish", func(t *testing.T) {
		root := repo(t, nil)
		code, _, errOut := run(t, "--corpus", "--root", root, "--tree", "--output=/tmp/x")
		if code != 2 || !strings.HasPrefix(errOut, "regcorpus check: ") {
			t.Fatalf("exit %d, stderr %q; want 2", code, errOut)
		}
	})
	t.Run("empty tree-ish is an error, not the working tree", func(t *testing.T) {
		root := repo(t, nil)
		code, out, errOut := run(t, "--corpus", "--root", root, "--tree", "")
		if code != 2 || out != "" || !strings.HasPrefix(errOut, "regcorpus check: ") {
			t.Fatalf("exit %d, stdout %q, stderr %q; want 2 and an error on stderr", code, out, errOut)
		}
	})
	t.Run("rev:path tree-ish names a subtree", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, map[string]string{
			"sub/go.mod": goMod, "sub/stages/01-fetch-parse/CONTRACT.md": "x\n",
			"sub/registry/records/RX-41-TEST-01.yaml": record,
		})
		gitT(t, root, "init", "-q")
		gitT(t, root, "add", "-A")
		gitT(t, root, "commit", "-q", "-m", "base")
		code, got := lines(t, "--corpus", "--root", root, "--tree", "HEAD:sub")
		want(t, code, got, 1, noLogFile)
	})
	t.Run("not a git repo", func(t *testing.T) {
		code, _, errOut := run(t, "--corpus", "--root", corpus(t, nil), "--tree", "HEAD")
		if code != 2 || !strings.HasPrefix(errOut, "regcorpus check: ") {
			t.Fatalf("exit %d, stderr %q; want 2", code, errOut)
		}
	})
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"-help"}} {
		code, out, errOut := run(t, args...)
		if code != 0 || !strings.HasPrefix(out, usageTop) || errOut != "" {
			t.Errorf("%q: exit %d, stdout %q, stderr %q; want 0 and usage on stdout", args, code, out, errOut)
		}
	}
	if code, _, errOut := run(t, "--corpus", "extra"); code != 2 ||
		!strings.HasPrefix(errOut, "regcorpus check: unexpected argument \"extra\"\n"+usageTop) {
		t.Errorf("extra argument: exit %d, stderr %q; want 2 naming \"extra\" then usage", code, errOut)
	}
	for _, args := range [][]string{{"--bogus"}, {"--corpus", "extra"}, {"--corpus", "--root"}} {
		code, out, errOut := run(t, args...)
		if code != 2 || out != "" || !strings.Contains(errOut, usageTop) {
			t.Errorf("%q: exit %d, stdout %q, stderr %q; want 2 and usage on stderr", args, code, out, errOut)
		}
	}
	code, out, errOut := run(t)
	if code != 2 || out != "" ||
		errOut != "regcorpus check: only --corpus exists; consumer checks C1–C5 arrive in plan step 22\n" {
		t.Errorf("no args: exit %d, stdout %q, stderr %q; want 2 and the plan step 22 message", code, out, errOut)
	}
}
