// sanity asks gdb and lldb to echo constant values
// back to us. It serves as a sanity test of the
// test system itself.
//
// It is also the only test that actually runs any
// lldb commands, as lldb support in Go is broken.
// See issue 7070.
package main

func main() {
	// BREAKPOINT
	// (gdb) print 1
	// \$1 = 1
	// (lldb) print 2
	// \(int\) \$0 = 2
	_ = 42 // Need at least one statement here, on pain of breakage. This might be a bug.
}
