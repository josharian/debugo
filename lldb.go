package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/template"
)

const lldbScriptTemplate = `
import json
import re
import os
import socket
import sys

import lldb

sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
sock.connect("{{.Sock}}")

def send_result(status, msg=None, filename=None, lineno=None):
	res = {"status": status}
	if msg is not None:
		res["msg"] = str(msg)
	if filename is not None:
		res["file"] = filename
	if lineno is not None:
		res["line"] = lineno
	dump = json.dumps(res) + "\n"
	enc = dump.encode('ascii')
	sock.sendall(enc)

debugger = lldb.SBDebugger.Create()
debugger.SkipLLDBInitFiles(True)
debugger.SetAsync(False)  # pause script execution when running lldb commands

target = debugger.CreateTargetWithFileAndArch({{.Executable | printf "%q"}}, lldb.LLDB_ARCH_DEFAULT)

if not target:
	send_result("ERROR", "failed to create target")
	sys.exit(1)	

bps = {}
{{range $bp := .Breakpoints}}
{{if .Tests}}
filename = {{$bp.Filename | printf "%q"}}
lineno = {{$bp.Line}}
bp = target.BreakpointCreateByLocation(filename, lineno)
if bp.GetNumLocations() != 1:
	send_result("ERROR", "failed to resolve breakpoint; see golang.org/issue/7070", filename, lineno)
	sys.exit(1)
#bp.SetOneShot(True)
tests = []
{{range $test := .Tests}}
{{if eq $test.Debugger "lldb" }}
tests.append(({{$test.Command | printf "%q"}}, {{$test.Want | joinn | printf "%q"}}, filename, {{$test.Line}}))
{{end}}
{{end}}
bps[bp.GetID()] = (bp, tests)
{{end}}
{{end}}

process = target.LaunchSimple(None, None, os.getcwd())

if not process:
	send_result("ERROR", "failed to launch process")
	sys.exit(1)	

while True:
	state = process.GetState()
	if state == lldb.eStateExited:
		# process has exited; we're done
		sys.exit(0)

	if state != lldb.eStateStopped:
		send_result("ERROR", "unexpected process state: " + str(state))
		sys.exit(1)

	# find the current breakpoint
	bp_id = None
	for t in process:
		if t.GetStopReason() == lldb.eStopReasonBreakpoint:
			bp_id = t.GetStopReasonDataAtIndex(0)
			break

	if bp_id is None:
		send_result("ERROR", "stopped but not on a breakpoint")
		sys.exit(1)

	bp_tests = bps.get(bp_id)
	if bp_tests is None:
		send_result("ERROR", "stopped at an unrecognized breakpoint")
		sys.exit(1)

	# Run the commands, check the results
	bp, tests = bp_tests
	for test in tests:
		cmd, want, filename, lineno = test

		send_result("RUNNING", cmd, filename, lineno)
		ret = lldb.SBCommandReturnObject()
		debugger.GetCommandInterpreter().HandleCommand(cmd, ret)
		if not ret.Succeeded():
			send_result("ERROR", "command " + cmd + " failed: " + ret.GetError().strip(), filename, lineno)
			continue

		out = ret.GetOutput()
		match = re.match("^" + want + "$", out)
		if match is None:
			msg = "want regex {want} have {out}".format(**locals())
			send_result("FAIL", msg, filename, lineno)
		else:
			send_result("PASS", None, filename, lineno)

	process.Continue()
`

// Lldb is all lldb-related context.
type Lldb struct {
	Path      string // path to lldb
	Python    string // path to python
	PythonMod string // path to the lldb python module
	Template  *template.Template
}

func (l *Lldb) Init() error {
	path, err := exec.LookPath("lldb")
	if err != nil {
		return err
	}
	l.Path = path

	python, err := exec.LookPath("python")
	if err != nil {
		return err
	}
	l.Python = python

	pymodBuf := new(bytes.Buffer)
	cmd := exec.Command(path, "--python-path")
	cmd.Stdout = pymodBuf
	if *debug {
		cmd.Stderr = os.Stderr
		fmt.Println("Running", cmd)
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	l.PythonMod = strings.TrimSpace(pymodBuf.String())

	funcMap := template.FuncMap{
		"joinn": func(v interface{}) (string, error) {
			slice, ok := v.([]string)
			if !ok {
				return "", fmt.Errorf("expected []string, got %v (%T)", v, v)
			}
			return strings.Join(slice, "\n"), nil
		},
	}

	l.Template = template.Must(template.New("script").Funcs(funcMap).Parse(lldbScriptTemplate))

	// TODO: Check lldb version
	return nil
}

func (l *Lldb) Run(executable string, scriptPath string) error {
	cmd := exec.Command(l.Python, scriptPath)
	// TODO: Preserve environ(?)
	// env := os.Environ()
	// env = append()
	cmd.Env = []string{"PYTHONPATH=" + l.PythonMod + ":" + os.Getenv("PYTHONPATH")}
	if *debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Println("Running", cmd)
	}
	err := cmd.Run()
	// Using non-system-provided Python causes crash on importing lldb
	// due to binary mismatch. The message looks like:
	// Fatal Python error: PyThreadState_Get: no current thread
	// Try to catch this here and help out unsuspecting users.
	if exitErr, ok := err.(*exec.ExitError); ok {
		if ws, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus); ok && ws == 0x6 {
			fmt.Printf("Failed to import Python lldb module using Python executable %v.\n", l.Python)
			fmt.Println("This is likely due to not using the system-provided Python, usually at /usr/bin/python.")
			fmt.Println("Try adjusting your PATH or virtualenv.")
		}
	}
	return err
}

func (l *Lldb) ScriptTemplate() *template.Template { return l.Template }
func (l *Lldb) Name() string                       { return "lldb" }
