package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// TODO: multiple line output?
// TODO: clean up code -- use a real state machine instead of this goop
// TODO: support (gdb), (lldb), (*db)
// TODO: Handle lldb vs gdb properly
// TODO: Better error messages
// TODO: Use go/ast to find comments instead of //-prefix checking
// TODO: Use nested comments (// //) to allow comments in tests?

type Test struct {
	Filename string
	Line     int
	Command  string // debugger command to run
	Want     string // regex desired response
}

type Breakpoint struct {
	Filename  string
	Line      int
	GdbTests  []Test
	LldbTests []Test
}

func Parse(r io.Reader, filename string) ([]Breakpoint, error) {
	var bps []Breakpoint
	var commandNext, wantNext bool
	var lineno int
	var bp Breakpoint
	var t Test
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		lineno++
		line := scan.Text()
		line = strings.TrimSpace(line)

		if commandNext {
			// TODO: This is ugly
			if strings.HasPrefix(line, "// //") {
				continue
			}
			if !strings.HasPrefix(line, "// (gdb) ") {
				if len(bp.GdbTests) == 0 {
					return nil, fmt.Errorf("%s:%d expected // (gdb) command, got %q", filename, lineno, line)
				} else {
					// No further commands; we're done with this breakpoint
					bps = append(bps, bp)
					commandNext = false
					wantNext = false
					continue
				}
			}
			t = Test{Command: strings.TrimSpace(line[len("// (gdb) "):]), Line: lineno}
			commandNext = false
			wantNext = true
			continue
		}

		if wantNext {
			// TODO: This is ugly
			if strings.HasPrefix(line, "// //") {
				continue
			}
			if !strings.HasPrefix(line, "//") {
				return nil, fmt.Errorf("%s:%d expected //-prefixed regex, got %q", filename, lineno, line)
			}
			t.Want = strings.TrimSpace(line[2:])
			if !strings.HasPrefix(t.Want, "^") {
				t.Want = "^" + t.Want
			}
			if !strings.HasSuffix(t.Want, "$") {
				t.Want = t.Want + "$"
			}
			bp.GdbTests = append(bp.GdbTests, t)
			bp.LldbTests = append(bp.LldbTests, t)
			t = Test{}
			commandNext = true
			wantNext = false
			continue
		}

		if line == "// BREAKPOINT" {
			commandNext = true
			bp = Breakpoint{Filename: filename, Line: lineno}
		}
	}

	if err := scan.Err(); err != nil {
		return nil, err
	}

	return bps, nil
}
