package main

import (
	"fmt"
	"io/ioutil"
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
def test(command, want, filename, line):
	sock.send(json.dumps({"status": "RUNNING", "file": filename, "line": line}) + "\n")
	out = gdb.execute(command, False, True)
	match = re.match(want, out)
	if match is None:
		msg = "want regex {want} have {out}".format(**locals())
		sock.send(json.dumps({"status": "FAIL", "file": filename, "line": line, "msg": msg}) + "\n")
	else:
		sock.send(json.dumps({"status": "PASS", "file": filename, "line": line}) + "\n")
end

{{range $bp := .Breakpoints}}
tbreak {{$bp.Filename}}:{{$bp.Line}}
commands
silent
{{range $test := .GdbTests}}
python test({{$test.Command | printf "%q"}}, {{$test.Want | printf "%q"}}, {{$bp.Filename | printf "%q"}}, {{$test.Line}})
{{end}}
continue
end
{{end}}
run
`

// Gdb is all gdb-related context.
type Gdb struct {
	Path     string // path to gdb
	Template *template.Template
}

func NewGdb() (*Gdb, error) {
	path, err := exec.LookPath("gdb")
	if err != nil {
		return nil, err
	}
	gdb := &Gdb{Path: path}
	gdb.Template = template.Must(template.New("script").Parse(gdbScriptTemplate))

	// TODO: Check gdb version
	return gdb, nil
}

func (g *Gdb) WriteScript(scriptPath string, dot ScriptContext) error {
	script, err := os.Create(scriptPath)
	if err != nil {
		return err
	}
	defer script.Close()
	if err := g.Template.Execute(script, dot); err != nil {
		return err
	}
	if *debug {
		fmt.Println("Script:")
		if all, err := ioutil.ReadFile(scriptPath); err == nil {
			fmt.Println("----")
			fmt.Println(string(all))
			fmt.Println("----")
		} else {
			fmt.Println(err)
		}
	}
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
