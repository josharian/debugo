package main

// TODO:
// * Improve naming, docs, tests
// * Better socket handling? Kind crude and in-place right now.
// * Better output formatting, more output formatting options (summary?).
// * Check to make sure that all breakpoints got hit; fail if not.
// * Invoke gdb/lldb only once, load/unload targets in turn?
//   Should be faster, but complicated scripts, and takes longer
//   to get to first failure.
// * Run multiple tests in parallel? Should be ok in theory, but
//   muddles output.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	verbose = flag.Bool("v", false, "verbose")
	debug   = flag.Bool("d", false, "print lots of debug goop")
	noGdb   = flag.Bool("no-gdb", false, "skip gdb")
	noLldb  = flag.Bool("no-lldb", false, "skip lldb")
)

const usageFooter = `
debugo runs automated tests of Go's gdb and lldb integration.
`

// ScriptContext is all the information needed to
// generate a debugger test script from a template.
// TODO: Better name.
type ScriptContext struct {
	GoRoot      string
	Sock        string // socket path for sending replies to
	Breakpoints []Breakpoint
	Executable  string
}

// TestResult represents something that happened while running a test.
// TODO: Better naming
type TestResult struct {
	Status string `json:"status"` // "RUNNING", "PASS", "FAIL", "ERROR", "INFO"
	File   string `json:"file"`
	Line   int    `json:"line"`
	Msg    string `json:"msg"`
}

func (tr TestResult) String() string {
	return fmt.Sprintf("%s:%d %s %s", tr.File, tr.Line, tr.Status, tr.Msg)
}

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
	cmd := exec.Command(goTool, "env", "GOROOT")
	goRootBuf := new(bytes.Buffer)
	cmd.Stdout = goRootBuf
	if err := cmd.Run(); err != nil {
		fatal(err)
	}
	goRoot := strings.TrimSpace(goRootBuf.String())

	debuggers := make([]Debugger, 0, 2)
	if !*noGdb {
		gdb := new(Gdb)
		if err := gdb.Init(); err != nil {
			fmt.Printf("SKIPPING gdb: %v\n", err)
		} else {
			debuggers = append(debuggers, gdb)
		}
	}
	if !*noLldb {
		lldb := new(Lldb)
		if err := lldb.Init(); err != nil {
			fmt.Printf("SKIPPING lldb: %v\n", err)
		} else {
			debuggers = append(debuggers, lldb)
		}
	}
	if len(debuggers) == 0 {
		fatal("No debuggers available.")
	}

	// Set up temp dir
	tempDir, err := ioutil.TempDir("", "debugo")
	if err != nil {
		fatal(err)
	}
	if *debug {
		fmt.Println("Using temp dir", tempDir)
		fmt.Println("**Not** cleaning up temp dir on exit")
	} else {
		defer func() {
			err := os.RemoveAll(tempDir)
			if err != nil {
				fmt.Println("Failed to clean up temp dir", tempDir, err)
			}
		}()
	}

	// Set up socket for receiving replies
	sock := filepath.Join(tempDir, "status.sock")
	listener, err := net.Listen("unix", sock)
	if err != nil {
		fatal(err)
	}
	defer listener.Close()

	for _, source := range flag.Args() {
		if !strings.HasSuffix(source, ".go") {
			fmt.Printf("SKIPPING test %s: Does not have .go suffix\n", source)
			continue
		}

		if *debug {
			fmt.Printf("Running test %s\n", source)
		}

		// Build executable
		executable := filepath.Join(tempDir, source[:len(source)-len(".go")])
		cmd := exec.Command(goTool, "build", "-o", executable, "-gcflags", "-N -l", source)
		buildErr := new(bytes.Buffer)
		cmd.Stderr = buildErr
		if *debug {
			fmt.Println("Running", cmd)
		}
		if err := cmd.Run(); err != nil {
			fmt.Printf("Failed to build %s:\n", source)
			fmt.Println(buildErr.String())
			fatal(err)
		}

		// Parse the code to extract test cases
		bps, err := Parse(source)
		if err != nil {
			fmt.Printf("SKIPPING test %s: Failed to parse: %v\n", source, err)
			continue
		}

		// Test with all debuggers
		for _, d := range debuggers {

			// Listen for replies and parse them
			replyc := make(chan TestResult)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					fatal(err)
				}
				scan := bufio.NewScanner(conn)
				for scan.Scan() {
					line := scan.Bytes()
					var res TestResult
					err := json.Unmarshal(line, &res)
					if err != nil {
						fmt.Println("Failed to unmarshal JSON:", string(line))
						fatal(err)
					}
					replyc <- res
				}
				if err := scan.Err(); err != nil {
					fatal(err)
				}
			}()

			go func() {
				// TODO: Count to make sure we ran the expected number of tests
				// TODO: better print of info/error messages w/ no file/lineno
				for reply := range replyc {
					if reply.Status == "FAIL" || *verbose {
						fmt.Printf("[%s] %v\n", d.Name(), reply)
					}
				}
			}()

			// Run debugger
			scriptPath := filepath.Join(tempDir, "script."+d.Name())
			dot := ScriptContext{GoRoot: goRoot, Sock: sock, Breakpoints: bps, Executable: executable}
			if err := writeScript(d, scriptPath, dot); err != nil {
				fatal(err)
			}
			if err := d.Run(executable, scriptPath); err != nil {
				fatal(err)
			}

			close(replyc)
		}
	}
}

func fatal(e interface{}) {
	fmt.Println(e)
	os.Exit(1)
}

func writeScript(d Debugger, scriptPath string, dot ScriptContext) error {
	script, err := os.Create(scriptPath)
	if err != nil {
		return err
	}
	defer script.Close()
	if err := d.ScriptTemplate().Execute(script, dot); err != nil {
		return err
	}
	if *debug {
		fmt.Println("Script:")
		if all, err := ioutil.ReadFile(scriptPath); err == nil {
			fmt.Println("----")
			fmt.Println(string(all))
			fmt.Println("----")
		} else {
			fmt.Println(err)
		}
	}
	return nil
}
