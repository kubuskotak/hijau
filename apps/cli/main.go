// Command hijau is the CLI for a Hijau localization instance. It talks to the
// REST API with a PAT (see `hijau login`) and is meant for local use and CI.
package main

import (
	"fmt"
	"os"
)

const usage = `hijau — localization CLI

Usage:
  hijau <command> [flags]

Commands:
  login     Authenticate and store a token (--api, --email, --password, or --token; --project)
  status    Show per-language completion (--project, --fail-under)
  pull      Write translations to JSON files (--project, --dir, --state)
  push      Upsert translations from a JSON file (--project, --dir, --lang, --dry-run)
  extract   Scan source for translation keys (--check, --project)

Run 'hijau <command> -h' for flags. Config lives in your user config dir
(override with HIJAU_CONFIG).`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	args := os.Args[2:]
	var err error
	switch os.Args[1] {
	case "login":
		err = cmdLogin(args)
	case "status":
		err = cmdStatus(args)
	case "pull":
		err = cmdPull(args)
	case "push":
		err = cmdPush(args)
	case "extract":
		err = cmdExtract(args)
	case "-h", "--help", "help":
		fmt.Println(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s\n", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
