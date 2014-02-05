// basictypes tests the debuggers' ability
// to interpret basic types.
package main

func Stack() {
	var i int
	var b bool
	// BREAKPOINT
	// (gdb) info locals
	// b = false
	// i = 0
	i = 5
	// BREAKPOINT
	// (gdb) print i
	// \$1 = 5
	_ = i
	b = true
	// BREAKPOINT
	// (gdb) print b
	// \$2 = true
	_ = b
}

func Heap() (*int, *bool) {
	i := 5
	b := false
	/* BROKEN, SKIPPED:
	// BREAKPOINT
	// (gdb) print i
	// \$[0-9]+ = 5
	// (gdb) print b
	// \$[0-9]+ = false
	*/
	return &i, &b
}

func main() {
	Stack()
	Heap()
}
