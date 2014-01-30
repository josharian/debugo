package main

import (
	"fmt"
	"os"
	"os/exec"
	"text/template"
)

const gdbScriptTemplate = `
add-auto-load-safe-path {{.GoRoot}}/src/pkg/runtime/runtime-gdb.py

python
import json
import re
import socket

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

def test(command, want, filename, lineno):
	send_result("RUNNING", command, filename, lineno)
	out = gdb.execute(command, False, True)
	match = re.match(want, out)
	if match is None:
		msg = "want regex {want} have {out}".format(**locals())
		send_result("FAIL", msg, filename, lineno)
	else:
		send_result("PASS", None, filename, lineno)
end

{{range $bp := .Breakpoints}}
{{if .GdbTests}}
tbreak {{$bp.Filename}}:{{$bp.Line}}
commands
silent
{{range $test := .GdbTests}}
python test({{$test.Command | printf "%q"}}, {{$test.Want | printf "%q"}}, {{$bp.Filename | printf "%q"}}, {{$test.Line}})
{{end}}
continue
end
{{end}}
{{end}}
run
`

// Gdb is all gdb-related context.
type Gdb struct {
	Path     string // path to gdb
	Template *template.Template
}

func (g *Gdb) Init() error {
	path, err := exec.LookPath("gdb")
	if err != nil {
		return err
	}
	g.Path = path
	g.Template = template.Must(template.New("script").Parse(gdbScriptTemplate))

	// TODO: Check gdb version
	return nil
}

func (g *Gdb) Run(executable string, scriptPath string) error {
	cmd := exec.Command(g.Path, executable,
		"--batch",
		"--return-child-result",
		"--command", scriptPath,
		"--nx", // ignore .gdbinit
	)
	if *debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Println("Running", cmd)
	}
	return cmd.Run()
}

func (g *Gdb) ScriptTemplate() *template.Template { return g.Template }
func (g *Gdb) Name() string                       { return "gdb" }
