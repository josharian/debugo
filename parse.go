package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

type Test struct {
	Line     int      // line the command occurred on
	Debugger string   // Which debugger is this a test for? "gdb" or "lldb"
	Command  string   // debugger command to run
	Want     []string // regex desired response
}

type Breakpoint struct {
	Filename string
	Line     int    // line the breakpoint is set at
	Tests    []Test // tests to run when this breakpoint is hit
}

func Parse(filename string) ([]Breakpoint, error) {
	var bps []Breakpoint

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	for _, cg := range f.Comments {
		if cg.List[0].Text != "// BREAKPOINT" {
			continue
		}
		bp, err := parseBreakpoint(fset, filename, cg)
		if err != nil {
			return nil, err
		}
		bps = append(bps, bp)
	}

	return bps, nil
}

func parseBreakpoint(fset *token.FileSet, filename string, cg *ast.CommentGroup) (Breakpoint, error) {
	var t Test
	bp := Breakpoint{Filename: filename, Line: fset.Position(cg.Pos()).Line}

	appendTest := func() {
		if t.Debugger != "" {
			bp.Tests = append(bp.Tests, t)
		}
	}

	for _, comment := range cg.List[1:] {
		line := strings.TrimSpace(comment.Text)
		lineno := fset.Position(comment.Pos()).Line

		if !strings.HasPrefix(line, "// ") {
			continue
		}
		line = strings.TrimSpace(line[len("//"):])

		// Check whether this is a new test. If so,
		// save the previous test and start a new one.
		switch {
		case strings.HasPrefix(line, "(gdb) "):
			appendTest()
			t = Test{
				Debugger: "gdb",
				Command:  strings.TrimSpace(line[len("(gdb)"):]),
				Line:     lineno,
			}
			continue
		case strings.HasPrefix(line, "(lldb) "):
			appendTest()
			t = Test{
				Debugger: "lldb",
				Command:  strings.TrimSpace(line[len("(lldb)"):]),
				Line:     lineno,
			}
			continue
		}

		// Not a new test; must be a Want from the current test.

		if t.Debugger == "" {
			// Oops, no current test
			return bp, fmt.Errorf("%s:%d expected a (gdb) or (lldb) command", filename, lineno)
		}

		t.Want = append(t.Want, line)
	}

	// Save the last test.
	appendTest()
	return bp, nil
}
