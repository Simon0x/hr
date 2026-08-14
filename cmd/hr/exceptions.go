package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Simon0x/hr/internal/exceptions"
	"github.com/Simon0x/hr/internal/persistence"
)

func exceptionsActor() string {
	if a := os.Getenv("HR_ACTOR"); a != "" {
		return a
	}
	return exceptions.DefaultActor
}

func cmdExceptions(root string, args []string) int {
	if hasFlag(args, "json") {
		return exceptionsJSON(root)
	}
	return exceptionsInteractive(root)
}

func exceptionsJSON(root string) int {
	open, err := exceptions.Open(context.Background(), persistence.File{Root: root}, root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	b, err := json.MarshalIndent(open, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fmt.Println(string(b))
	return 0
}

func exceptionsInteractive(root string) int {
	reader := bufio.NewReader(os.Stdin)
	actor := exceptionsActor()

	for {
		open, err := exceptions.Open(context.Background(), persistence.File{Root: root}, root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		if len(open) == 0 {
			fmt.Println("no open exceptions.")
			return 0
		}

		printExceptionList(open)
		fmt.Print("\nselect a number to view, or press enter to quit: ")
		line, ok := readLine(reader)
		if !ok || line == "" {
			return 0
		}

		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > len(open) {
			fmt.Println("not a valid selection.")
			continue
		}

		resolveOne(reader, root, actor, open[n-1])
	}
}

func printExceptionList(open []exceptions.Exception) {
	fmt.Println()
	for i, e := range open {
		deadline := ""
		if e.Deadline != "" {
			deadline = "  deadline " + e.Deadline
		}
		fmt.Printf("[%d] %-2s  %-21s %-12s  %s%s\n",
			i+1, e.Consequence, e.Class, e.Department, truncate(e.Because, 50), deadline)
	}
}

func resolveOne(reader *bufio.Reader, root, actor string, exc exceptions.Exception) {
	fmt.Println()
	fmt.Printf("%s — %s (%s)\n", exc.Department, exc.Class, exc.Consequence)
	fmt.Printf("because          %s\n", exc.Because)
	fmt.Printf("recommendation   %s\n", exc.Recommendation)
	if exc.Uncertainty != "" {
		fmt.Printf("uncertainty      %s\n", exc.Uncertainty)
	}
	if exc.Deadline != "" {
		fmt.Printf("deadline         %s\n", exc.Deadline)
	}
	if exc.RequiredAuthority != "" {
		fmt.Printf("requires         %s\n", exc.RequiredAuthority)
	}

	for {
		fmt.Println()
		for i, o := range exc.Options {
			fmt.Printf("  [%d] %s\n", i+1, o)
		}
		fmt.Print("\nchoose an option, 'b' to go back, or press enter to quit: ")
		line, ok := readLine(reader)
		if !ok || line == "" {
			return
		}
		if line == "b" {
			return
		}

		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > len(exc.Options) {
			fmt.Println("not a valid selection.")
			continue
		}
		option := exc.Options[n-1]

		fmt.Printf("resolve %s with %q? [y/N] ", exc.Department, option)
		confirm, ok := readLine(reader)
		if !ok || strings.ToLower(confirm) != "y" {
			continue
		}

		if err := exceptions.Resolve(context.Background(), persistence.File{Root: root}, root, actor, exc, option); err != nil {
			fmt.Fprintln(os.Stderr, "resolve failed:", err)
			return
		}
		fmt.Println("recorded.")
		return
	}
}

func readLine(reader *bufio.Reader) (string, bool) {
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", false
	}
	return strings.TrimSpace(line), true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
