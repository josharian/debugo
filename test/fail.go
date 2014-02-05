package main

func BasicTypes() {
	var i int
	i = 5
	// BREAKPOINT
	// (gdb) printf "%d", i
	// 6
	// (gdb) print i
	// \$. = 7
	_ = i
	var b bool
	// BREAKPOINT
	// (gdb) print b
	// \$. = fals
	_ = b
}

func main() {
	BasicTypes()
}
