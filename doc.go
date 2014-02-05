// debugo runs automated tests of Go's gdb and lldb support.
//
//
// How to write tests
//
// Tests are regular Go programs with special inline comments.
// The comments indicate where to set breakpoints, what debugger
// commands to run when that breakpoint is hit, and what response
// the debugger should provide. For example:
//
// func Simple() (int, bool) {
// 	i := 5
// 	b := false
// 	// BREAKPOINT
// 	// (gdb) print i
// 	// \$1 = 5
// 	// (gdb) info locals
// 	// b = false
// 	// i = 5
// 	// (lldb) print i
// 	// \(int\) \$0 = 5
// 	return i, b
// }
//
// The test parser looks for comment groups beginning with "// BREAKPOINT".
// Breakpoints get set at that line in the code. Breakpoints are temporary;
// any given breakpoint will trigger exactly once.
//
// Commands are prefaced with "(gdb)" or "(lldb)", depending on which
// debugger they are to be run with. Commands for different debuggers
// can be intermingled freely.
//
// The expected output is interpreted as a Python regular expression, thus the
// escaping of the dollar signs and parens in the example above.
//
// The test parser ignores /* */ comments. If you need to add commentary into the
// middle of a test, you can do so by using /* */.
//
//
// How it works, at a high level:
//
// 1. Gather environment info: Where is the Go command? Where is GOROOT? Are lldb
//    and gdb available?
// 2. Compile the source file into a temp directory.
// 3. Parse the source file, extracting breakpoints and associated tests.
// 4. Generate a script to be fed to gdb/lldb. The gdb script is a sequence of
//    gdb commands, dropping down to Python as needed. The lldb script is a
//    Python script, which uses the Python lldb module to drive lldb.
// 5. Listen on a socket to receive test results. (This proved to be much easier
//    and more robust than trying to directly parse the output from gdb or lldb.)
// 6. Execute the test script, gathering results.
// 7. Repeat as needed.
//
package main
