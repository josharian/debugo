package main

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	filename := "testdata/parsable.go"
	bps, err := Parse(filename)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	want := []Breakpoint{
		// Basic
		Breakpoint{Filename: filename, Line: 4,
			Tests: []Test{
				Test{Line: 5, Debugger: "gdb", Command: "cmd1", Want: []string{"want1"}},
				Test{Line: 7, Debugger: "gdb", Command: "cmd2", Want: []string{"want2a", "want2b"}},
				Test{Line: 10, Debugger: "lldb", Command: "cmd3", Want: []string{"want3"}},
			},
		},
		// InlineComments
		Breakpoint{Filename: filename, Line: 15,
			Tests: []Test{
				Test{Line: 17, Debugger: "gdb", Command: "cmd4", Want: []string{"want4a", "want4b"}},
			},
		},
	}

	if !reflect.DeepEqual(bps, want) {
		t.Errorf("parsed incorrectly: got %v, want %v", bps, want)
	}
}
