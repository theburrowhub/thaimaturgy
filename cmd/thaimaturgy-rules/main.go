// Command thaimaturgy-rules manages external rules bundles separately from
// adventure content.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/rules/bundlestore"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(arguments []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("thaimaturgy-rules", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDirectory := flags.String("data-dir", strings.TrimSpace(os.Getenv("THAIM_DATA_DIR")), "thAImaturgy data directory")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: thaimaturgy-rules [--data-dir PATH] <install FILE|list|path>")
		flags.PrintDefaults()
	}
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	commandArguments := flags.Args()
	if len(commandArguments) == 0 {
		flags.Usage()
		return 2
	}

	var applicationStore *storage.Storage
	var err error
	if strings.TrimSpace(*dataDirectory) != "" {
		applicationStore, err = storage.NewWithPath(*dataDirectory)
	} else {
		applicationStore, err = storage.New()
	}
	if err != nil {
		fmt.Fprintln(stderr, "storage:", err)
		return 1
	}
	rulesStore, err := bundlestore.New(filepath.Join(applicationStore.BasePath(), bundlestore.DirectoryName), nil)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	switch commandArguments[0] {
	case "install":
		if len(commandArguments) != 2 {
			fmt.Fprintln(stderr, "install requires exactly one .rules.zip path")
			return 2
		}
		installed, err := rulesStore.InstallFile(context.Background(), commandArguments[1])
		if err != nil {
			fmt.Fprintln(stderr, "install:", err)
			return 1
		}
		lock := installed.Loaded.Artifact.Lock()
		fmt.Fprintf(stdout, "installed %s@%s\n  digest: %s\n  path: %s\n", lock.ID, lock.Version, lock.Digest, installed.Path)
		return 0

	case "list":
		if len(commandArguments) != 1 {
			fmt.Fprintln(stderr, "list takes no arguments")
			return 2
		}
		report := rulesStore.Discover(context.Background())
		for _, bundle := range report.Bundles {
			lock := bundle.Loaded.Artifact.Lock()
			fmt.Fprintf(stdout, "%s@%s  %s  %s\n", lock.ID, lock.Version, lock.Digest, bundle.Path)
		}
		for _, failure := range report.Failures {
			fmt.Fprintln(stderr, "invalid:", failure.Error())
		}
		if len(report.Failures) > 0 {
			return 1
		}
		return 0

	case "path":
		if len(commandArguments) != 1 {
			fmt.Fprintln(stderr, "path takes no arguments")
			return 2
		}
		fmt.Fprintln(stdout, rulesStore.Root())
		return 0

	default:
		fmt.Fprintf(stderr, "unknown command %q\n", commandArguments[0])
		flags.Usage()
		return 2
	}
}
