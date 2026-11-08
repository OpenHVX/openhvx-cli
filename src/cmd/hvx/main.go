package main

import (
	"errors"
	"fmt"
	"os"

	"hvx-cli/internal/cli"
	"hvx-cli/internal/config"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Print(cli.Usage())
		return
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error loading config:", err)
		os.Exit(1)
	}

	app := cli.NewApp(cfg, os.Stdout, os.Stderr)
	if err := app.Execute(os.Args[1:]); err != nil {
		if errors.Is(err, cli.ErrUsage) {
			fmt.Fprint(os.Stderr, cli.Usage())
		} else {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}
