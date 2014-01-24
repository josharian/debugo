package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// TODO: multiple output?
// TODO: clean up code
// TODO: support (gdb), (lldb), (*db)

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
