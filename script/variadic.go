/*
Unless explicitly stated otherwise all files in this repository are licensed
under the MIT License.
This product includes software developed at Datadog (https://www.datadoghq.com/).
Copyright 2018 Datadog, Inc.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	python "github.com/hanzoai/cpy3"
)

var (
	output     = flag.String("o", "variadic.c", "output file")
	caseNumber = flag.Int("n", python.MaxVariadicLength, "number of case in the switch statement")
)

func main() {
	flag.Parse()

	out, err := os.Create(*output)
	if err != nil {
		fmt.Printf("Error opening file %s: %s", *output, err)
		os.Exit(1)
	}
	defer out.Close()
	_, err = out.WriteString(`/*
Unless explicitly stated otherwise all files in this repository are licensed
under the MIT License.
This product includes software developed at Datadog (https://www.datadoghq.com/).
Copyright 2018 Datadog, Inc.
*/

`)
	if err != nil {
		fmt.Printf("Error writing to file %s: %s", *output, err)
		os.Exit(1)
	}

	out.WriteString("#include \"Python.h\"\n\n")
	out.WriteString(renderTemplate(*caseNumber, "PyObject_CallFunctionObjArgs", "callable"))
	out.WriteString(renderTemplate(*caseNumber, "PyObject_CallMethodObjArgs", "obj", "name"))

}

func renderTemplate(n int, functionName string, pyArgs ...string) string {
	template :=
		`PyObject* _go_%s(%sint argc, PyObject **argv) {

    PyObject *result = NULL;
    switch (argc) {
%s    }
    return result;
}
`
	var args strings.Builder
	for _, arg := range pyArgs {
		fmt.Fprintf(&args, "PyObject *%s, ", arg)
	}
	var switchTemplate strings.Builder
	for i := 0; i < n+1; i++ {
		fmt.Fprintf(&switchTemplate, "        case %d:\n", i)
		var args strings.Builder
		for _, arg := range pyArgs {
			fmt.Fprintf(&args, "%s, ", arg)
		}
		for j := 0; j < i; j++ {
			fmt.Fprintf(&args, "argv[%d], ", j)
		}
		args.WriteString("NULL")
		fmt.Fprintf(&switchTemplate, "            return %s(%s);\n", functionName, args.String())
	}
	return fmt.Sprintf(template, functionName, args.String(), switchTemplate.String())
}
