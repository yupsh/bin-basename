package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestRun(t *testing.T) {
	cases := []struct {
		name       string
		version    string
		wantOut    string
		wantErrSub string
		args       []string
		wantCode   int
	}{
		{
			name:    "plain path strips directory",
			args:    []string{"basename", "/usr/local/bin/script.sh"},
			wantOut: "script.sh\n",
		},
		{
			name:    "suffix operand is removed",
			args:    []string{"basename", "/usr/local/bin/script.sh", ".sh"},
			wantOut: "script\n",
		},
		{
			name:    "suffix equal to name is kept",
			args:    []string{"basename", ".txt", ".txt"},
			wantOut: ".txt\n",
		},
		{
			name:    "trailing slash strips to last component",
			args:    []string{"basename", "/path/to/dir/"},
			wantOut: "dir\n",
		},
		{
			name:    "root stays root",
			args:    []string{"basename", "/"},
			wantOut: "/\n",
		},
		{
			name:    "multiple flag prints every name",
			args:    []string{"basename", "-a", "/x/y", "/p/q.txt"},
			wantOut: "y\nq.txt\n",
		},
		{
			name:    "suffix flag implies multiple",
			args:    []string{"basename", "-s", ".txt", "a.txt", "/b/c.txt"},
			wantOut: "a\nc\n",
		},
		{
			name:    "suffix long flag",
			args:    []string{"basename", "--suffix", ".log", "/var/log/app.log"},
			wantOut: "app\n",
		},
		{
			name:    "multiple long flag",
			args:    []string{"basename", "--multiple", "/a/b", "/c/d"},
			wantOut: "b\nd\n",
		},
		{
			name:    "zero flag separates with NUL",
			args:    []string{"basename", "-az", "/x/y", "/p/q"},
			wantOut: "y\x00q\x00",
		},
		{
			name:    "zero long flag",
			args:    []string{"basename", "--zero", "/x/y"},
			wantOut: "y\x00",
		},
		{
			name:    "version flag reports injected version",
			version: "1.2.3",
			args:    []string{"basename", "--version"},
			wantOut: "basename version 1.2.3\n",
		},
		{
			name:       "no operand errors",
			args:       []string{"basename"},
			wantCode:   1,
			wantErrSub: "basename: missing operand",
		},
		{
			name:       "missing operand under suffix flag",
			args:       []string{"basename", "-s", ".sh"},
			wantCode:   1,
			wantErrSub: "basename: missing operand",
		},
		{
			name:       "extra bare operand errors",
			args:       []string{"basename", "a", "b", "c"},
			wantCode:   1,
			wantErrSub: "basename: extra operand",
		},
		{
			name:       "unknown flag errors",
			args:       []string{"basename", "--nope"},
			wantCode:   1,
			wantErrSub: "basename:",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := run(tc.version, tc.args, strings.NewReader(""), &out, &errOut, afero.NewMemMapFs())

			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr=%q)", code, tc.wantCode, errOut.String())
			}
			if tc.wantErrSub == "" && out.String() != tc.wantOut {
				t.Fatalf("stdout = %q, want %q", out.String(), tc.wantOut)
			}
			if tc.wantErrSub != "" && !strings.Contains(errOut.String(), tc.wantErrSub) {
				t.Fatalf("stderr = %q, want substring %q", errOut.String(), tc.wantErrSub)
			}
		})
	}
}

// TestEmitWriteError exercises the output write-failure path: emit must surface
// the writer's error rather than silently dropping it.
func TestEmitWriteError(t *testing.T) {
	err := emit(failWriter{}, [][]byte{[]byte("x")}, '\n')
	if !errors.Is(err, errWrite) {
		t.Fatalf("emit error = %v, want %v", err, errWrite)
	}
}

// TestStripPipelineError covers the pipeline-failure path: when the underlying
// gloo collect fails, run must report it on stderr and exit non-zero.
func TestStripPipelineError(t *testing.T) {
	orig := collect
	t.Cleanup(func() { collect = orig })
	collect = func([]string, string) (any, error) { return nil, errCollect }

	var out, errOut bytes.Buffer
	code := run("", []string{"basename", "/a/b"}, strings.NewReader(""), &out, &errOut, afero.NewMemMapFs())

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "basename: collect failed") {
		t.Fatalf("stderr = %q, want it to mention the collect failure", errOut.String())
	}
}

const errCollect Error = "collect failed"

const errWrite Error = "write failed"

// failWriter fails every Write, modelling a broken stdout (e.g. a closed pipe).
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errWrite }

func Test_main(t *testing.T) {
	origExit, origRun := osExit, runCLI
	t.Cleanup(func() { osExit, runCLI = origExit, origRun })

	gotCode := -1
	osExit = func(code int) { gotCode = code }
	runCLI = func(string, []string, io.Reader, io.Writer, io.Writer, afero.Fs) int { return 7 }

	main()

	if gotCode != 7 {
		t.Fatalf("main propagated exit code %d, want 7", gotCode)
	}
}
