package main

func Basic() {
	// BREAKPOINT
	// (gdb) cmd1
	// want1
	// (gdb) cmd2
	// want2a
	// want2b
	// (lldb) cmd3
	// want3
}

func InlineComments() {
	// BREAKPOINT
	/* inline comment */
	// (gdb) cmd4
	/* inline comment */
	// want4a
	/* inline comment */
	// want4b
	/* inline comment */
}

func main() {
	// Non-breakpoint comment.
}
