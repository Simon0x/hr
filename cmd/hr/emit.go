package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Simon0x/hr/internal/emit"
	"github.com/Simon0x/hr/internal/persistence"
)

func cmdEmit(root string, args []string) int {
	var file string
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			file = a
			break
		}
	}

	var raw []byte
	var err error
	if file != "" {
		raw, err = os.ReadFile(file)
	} else {
		raw, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if len(raw) == 0 || len(strings.TrimSpace(string(raw))) == 0 {
		fmt.Fprintln(os.Stderr, "nothing on stdin and no file given")
		return 2
	}

	actor := os.Getenv("HR_ACTOR")

	result, problems, err := emit.Emit(context.Background(), persistence.File{Root: root}, root, raw, actor)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, p)
		}
		fmt.Fprintln(os.Stderr, "\nnot stored — the artifact does not satisfy its contract")
		return 1
	}

	if actor == "" || strings.HasSuffix(actor, "/unattributed") {
		fmt.Fprintln(os.Stderr, "warning: HR_ACTOR unset, recorded as unattributed — an event with no identity cannot answer who acted")
	}

	fmt.Fprintf(os.Stderr, "stored %s (unsigned — CI attests)\n", result.Rel)
	fmt.Println(result.Rel)
	return 0
}
