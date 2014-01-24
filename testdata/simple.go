package main

func BasicTypes() {
	var i int
	i = 5
	// BREAKPOINT
	// (gdb) printf "%d", i
	// 5
	// (gdb) print i
	// \$. = 5
	_ = i
	var b bool
	// BREAKPOINT
	// (gdb) print b
	// \$. = false
	_ = b
}

func main() {
	BasicTypes()
}
