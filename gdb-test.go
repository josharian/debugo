// gdb-test runs automated tests of Go's gdb integration.
//
// TODO: Nice high-level description of how this works.
package main

// TODO:
// * better input/output parsing -- multiline output, etc.
// * spot check against lldb

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	verbose = flag.Bool("v", false, "verbose")
)

const usageFooter = `
gdb-test runs automated tests of Go's gdb integration.

TODO: Describe the format of the automated tests.
`

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [args] <test-cases>\n\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprint(os.Stderr, usageFooter)
		os.Exit(2)
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
	}

	// Make sure all our tools are available
	goTool, err := exec.LookPath("go")
	if err != nil {
		fatal(err)
	}
	gdb, err := exec.LookPath("gdb")
	if err != nil {
		fatal(err)
	}

	// Set up temp dir
	tempDir, err := ioutil.TempDir("", "gdb-test")
	if err != nil {
		fatal(err)
	}
	if *verbose {
		fmt.Println("Using temp dir", tempDir)
	}
	defer func() {
		if *verbose {
			fmt.Println("Removing temp dir", tempDir)
		}
		err := os.RemoveAll(tempDir)
		if err != nil {
			fmt.Println("Failed to clean up temp dir", tempDir, err)
		}
	}()

	for _, source := range flag.Args() {
		if !strings.HasSuffix(source, ".go") {
			fmt.Printf("Skipping test %s: Does not have .go suffix\n", source)
			continue
		}

		if *verbose {
			fmt.Println("Running test %s", source)
		}

		f, err := os.Open(source)
		if err != nil {
			fatal(err)
		}

		// Parse test cases from source
		bps, err := Parse(f, source)
		if err != nil {
			fatal(err)
		}
		if *verbose {
			fmt.Println("Parsed breakpoints: ", bps)
		}
		f.Close()

		// Build executable
		executable := filepath.Join(tempDir, source[:len(source)-len(".go")])
		cmd := exec.Command(goTool, "build", "-o", executable, "-gcflags", "-N -l", source)
		if *verbose {
			fmt.Println("Running", cmd)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Run(); err != nil {
			fatal(err)
		}
		if *verbose {
			fmt.Println("Built executable:", executable)
		}

		// Figure out GOROOT
		cmd = exec.Command(goTool, "env", "GOROOT")
		goRootBuf := new(bytes.Buffer)
		cmd.Stdout = goRootBuf
		if *verbose {
			fmt.Println("Running", cmd)
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Run(); err != nil {
			fatal(err)
		}
		goRoot := strings.TrimSpace(goRootBuf.String())

		// Construct gdb script
		scriptPath := filepath.Join(tempDir, "script.gdb")
		script, err := os.Create(scriptPath)
		if err != nil {
			fatal(err)
		}
		fmt.Fprintf(script, "add-auto-load-safe-path %s/src/pkg/runtime/runtime-gdb.py\n", goRoot)

		prolog := `python
import re
def test(command, want, context):
    out = gdb.execute(command, False, True)
    err = "Breakpoint {context} want regex {want} have {out}".format(**locals())
    match = re.match(want, out)
    assert match is not None, err
end
`

		fmt.Fprint(script, prolog)
		for _, bp := range bps {
			fmt.Fprintf(script, "tbreak %s:%d\n", bp.Filename, bp.Line)
			fmt.Fprintln(script, "commands")
			if !*verbose {
				fmt.Fprintln(script, "silent")
			}
			for i, command := range bp.Commands {
				fmt.Fprintf(script, "python test(%q, %q, \"%s:%d\")\n", command, "^"+bp.Want[i]+"$", bp.Filename, bp.Line)
			}
			fmt.Fprintln(script, "continue")
			fmt.Fprintln(script, "end")
		}
		fmt.Fprintln(script, "run")
		script.Close()
		if *verbose {
			fmt.Println("Script:")
			all, _ := ioutil.ReadFile(scriptPath)
			fmt.Println(string(all))
		}

		// Run gdb, hope for the best
		// TODO: Check gdb version?
		batch := "--batch-silent"
		if *verbose {
			batch = "--batch"
		}
		cmd = exec.Command(gdb, executable,
			batch,
			"--return-child-result",
			"--command", scriptPath,
			"--nx", // ignore .gdbinit
			"--quiet",
		)
		gdbOut := new(bytes.Buffer)
		cmd.Stderr = gdbOut
		if *verbose {
			fmt.Println("Running", cmd)
			cmd.Stdout = os.Stdout
		}
		err = cmd.Run()
		if err == nil {
			fmt.Printf("PASS %v\n", source)
			continue
		}

		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			if *verbose {
				fmt.Printf("gdb output:\n\n%s\n\n", gdbOut.String())
			}
			fmt.Printf("err %T %v", err, err)
			fatal(err)
		}

		// gdbDump := gdbOut.String()
		// TODO: How to check for a bug in this code vs. a test failure?
		if !exitErr.Success() {
			// Scan through looking for assertion errors that we've triggered
			scan := bufio.NewScanner(gdbOut)
			for scan.Scan() {
				line := scan.Text()
				line = strings.TrimSpace(line)
				// fmt.Printf("EXAMINING %q\n", line)
				switch {
				case strings.HasPrefix(line, "AssertionError: Breakpoint"):
					line = line[len("AssertionError: "):]
					fmt.Println(line) // TODO: Gussy up more?
					// case strings.HasPrefix(line, "Traceback"):
					// 	fmt.Println("Oops, something went wrong. Maybe you have an unreachable breakpoint?")
					// 	fmt.Printf("\n--- RAW GDB OUTPUT ---\n:\n\n%s\n\n------\n", gdbDump)
				}
			}
			if err := scan.Err(); err != nil {
				fatal(err)
			}
		}
	}
}

func fatal(e error) {
	fmt.Println("HI")
	fmt.Println(e)
	os.Exit(1)
}
