package main

import "text/template"

// Debugger is the interface shared between Gdb and Lldb.
type Debugger interface {
	Init() error // if non-nil return, do not use
	Name() string
	ScriptTemplate() *template.Template
	Run(executable string, scriptPath string) error
}

// TODO: DRY up some of lldb, gdb: python boilerplate, funcMap
