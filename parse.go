package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type Breakpoint struct {
	Filename string
	Line     int
	Commands []string // gdb command to run; commands correspond by index to entries in want
	Want     []string // regular expressions indicating the desired reply
}

func Parse(r io.Reader, filename string) ([]Breakpoint, error) {
	var bps []Breakpoint
	var commandNext, wantNext bool
	var lineno int
	var bp Breakpoint
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		lineno++
		line := scan.Text()
		line = strings.TrimSpace(line)

		if commandNext {
			if !strings.HasPrefix(line, "//") {
				if len(bp.Commands) == 0 {
					return nil, fmt.Errorf("%s:%d expected //-prefixed command, got %q", filename, lineno, line)
				} else {
					// No further commands; we're done with this breakpoint
					bps = append(bps, bp)
					commandNext = false
					wantNext = false
					continue
				}
			}
			bp.Commands = append(bp.Commands, strings.TrimSpace(line[2:]))
			commandNext = false
			wantNext = true
			continue
		}

		if wantNext {
			if !strings.HasPrefix(line, "//") {
				return nil, fmt.Errorf("%s:%d expected //-prefixed regex, got %q", filename, lineno, line)
			}
			bp.Want = append(bp.Want, strings.TrimSpace(line[2:]))
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
