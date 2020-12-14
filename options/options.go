// `options` handles reading in user input. Does not have logic for detecting smart defaults.
package options

import (
	"errors"
	"flag"
	"jobbatical/secrets/utils"
	"os"
	"path/filepath"
	"strings"
)

const Usage string = "Usage secrets <open|seal> [<file path>...] [--dry-run] [--verbose] [--root <project root>] [--key <encryption key name>] [--open-all]"
const ExpectedOrganization string = "crispso"
const ExpectedRepoHost string = "github.com"
const KeyRing string = "crisp-project-secrets"
const Location string = "global"
const EncryptCmd string = "seal"
const DecryptCmd string = "open"

var DryRun bool
var Key string
var OpenAll bool
var ProjectRoot string
var Verbose bool
var Cmd string
var Files []string

func Remove(slice []string, s int) []string {
	return append(slice[:s], slice[s+1:]...)
}

func popCommand(args []string) (string, []string, error) {
	for i, a := range args {
		if i == 0 {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			return a, Remove(args, i), nil
		} else {
			break
		}
	}
	return "", args, errors.New("command not found")
}

func popFiles(args []string) ([]string, []string, error) {
	var (
		file string
		err  error
	)
	files := make([]string, 0, 1)

	for {
		file, os.Args, err = popCommand(os.Args)
		if err != nil {
			break
		}
		absolutePath, err := filepath.Abs(file)
		if err != nil {
			return files, os.Args, err
		}
		files = append(files, absolutePath)
	}

	return files, os.Args, nil
}

func init() {
	var err error

	Cmd, os.Args, err = popCommand(os.Args)
	if err != nil {
		utils.ErrPrintln("Error: %s\n%s", err, Usage)
		os.Exit(1)
	}

	Files, os.Args, err = popFiles(os.Args)
	utils.ExitIfError(err)

	flag.BoolVar(&DryRun, "dry-run", false, "Skip calls to GCP")
	flag.StringVar(&Key, "key", "", "Key to use")
	flag.BoolVar(&OpenAll, "open-all", false, "Opens all .enc files within the repository")
	flag.StringVar(&ProjectRoot, "root", "", "Project root folder(name will be used as key name)")
	flag.BoolVar(&Verbose, "verbose", false, "Log debug info")

	flag.Parse()
}
