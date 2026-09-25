// compose-plan previews, records and rolls back Docker Compose deploys.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/Ali-932/compose-plan/internal/cli"
	"github.com/spf13/pflag"
)

func main() {
	var opts cli.Options
	pflag.StringSliceVarP(&opts.Files, "file", "f", nil, "compose file (repeatable)")
	pflag.StringSliceVar(&opts.EnvFiles, "env-file", nil, "variables file (repeatable)")
	pflag.StringVarP(&opts.Name, "project-name", "p", "", "project name")
	pflag.Usage = func() { fmt.Fprint(os.Stderr, cli.Usage) }
	pflag.Parse()

	cmd, args := pflag.Arg(0), pflag.Args()[min(1, pflag.NArg()):]
	entries, err := checkInput(cmd, args, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "compose-plan:", err)
		fmt.Fprint(os.Stderr, cli.Usage)
		os.Exit(2)
	}

	ctx := context.Background()
	switch cmd {
	case "plan":
		_, _, err = cli.RunPlan(ctx, opts)
	case "apply":
		err = cli.RunApply(ctx, opts, "")
	case "history":
		err = cli.RunHistory(ctx, opts)
	case "diff":
		err = cli.RunDiff(ctx, opts, entries[0], entries[1])
	case "rollback":
		err = cli.RollBack(ctx, opts, entries[0])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "compose-plan:", err)
		os.Exit(1)
	}
}

// argCount is how many arguments each command takes after its name.
var argCount = map[string]int{"plan": 0, "apply": 0, "history": 0, "diff": 2, "rollback": 1}

// checkInput rejects bad input before any command runs, and returns the
// entry numbers diff and rollback were given.
func checkInput(cmd string, args []string, opts cli.Options) ([]int, error) {
	want, known := argCount[cmd]
	switch {
	case cmd == "":
		return nil, fmt.Errorf("a command is required")
	case !known:
		return nil, fmt.Errorf("unknown command %q", cmd)
	case len(args) != want:
		return nil, fmt.Errorf("%s takes %d argument(s), got %d", cmd, want, len(args))
	}

	var entries []int
	for _, a := range args {
		n, err := strconv.Atoi(a)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("%q is not an entry number", a)
		}
		entries = append(entries, n)
	}

	for _, f := range opts.Files {
		if _, err := os.Stat(f); err != nil {
			return nil, fmt.Errorf("compose file not found: %s", f)
		}
	}
	for _, f := range opts.EnvFiles {
		if _, err := os.Stat(f); err != nil {
			return nil, fmt.Errorf("env file not found: %s", f)
		}
	}
	return entries, nil
}
