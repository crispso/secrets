package main

import (
	"errors"
	"fmt"
	"jobbatical/secrets/git"
	"jobbatical/secrets/kms"
	"jobbatical/secrets/log"
	"jobbatical/secrets/options"
	"jobbatical/secrets/utils"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var verbose bool = options.Verbose
var projectRoot string = options.ProjectRoot
var key string = options.Key

func isProjectRoot(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir()
}

func findProjectRoot(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	nextPath := filepath.Join(path, "..")
	if path == nextPath {
		return path, errors.New("not in project. Run the script inside a project folder(git repo) or provide it as an argument")
	}
	if isProjectRoot(path) {
		return path, nil
	}
	return findProjectRoot(nextPath)
}

func getProjectRepo(projectRoot string) (string, error) {
	_, stdOut, _, err := utils.RunCommand("git", "-C", projectRoot, "remote", "-v")
	if err != nil {
		return "", err
	}
	example := fmt.Sprintf("git@%s:%s/<project name>.git", options.ExpectedRepoHost, options.ExpectedOrganization)
	re := regexp.MustCompile("(?i)" + options.ExpectedRepoHost + `:([^/]*)/([^/\.]*)\.git`)
	matches := re.FindStringSubmatch(stdOut)
	if len(matches) == 3 {
		org := matches[1]
		project := matches[2]

		if strings.ToLower(org) == options.ExpectedOrganization {
			return project, nil
		}

		return "", fmt.Errorf(
			`%s not a %s project in %s: expecting a remote %s, got %s in %s`,
			projectRoot,
			options.ExpectedOrganization,
			options.ExpectedRepoHost,
			example,
			project,
			org,
		)
	}
	return "", fmt.Errorf(
		`%s not a project in %s: expecting a remote %s`,
		projectRoot,
		options.ExpectedRepoHost,
		example,
	)
}

func getKeyName(projectRoot string) string {
	repo, err := getProjectRepo(projectRoot)
	if err == nil {
		return repo
	}
	return filepath.Base(projectRoot)
}

func main() {
	log.PrintDebugln("%s", os.Args)

	if projectRoot == "" {
		projectRoot, _ = findProjectRoot(".")
	}

	if key == "" {
		key = getKeyName(projectRoot)
	}

	log.PrintDebugln("dry run: %t", options.DryRun)
	log.PrintDebugln("options.ExpectedOrganization: %s", options.ExpectedOrganization)
	log.PrintDebugln("options.ExpectedRepoHost: %s", options.ExpectedRepoHost)
	log.PrintDebugln("keyRing: %s", options.KeyRing)
	log.PrintDebugln("key: %s", key)
	log.PrintDebugln("project root: %s", projectRoot)
	log.PrintDebugln("cmd: %s", options.Cmd)
	log.PrintDebugln("files: %s (%d)", options.Files, len(options.Files))

	if options.Cmd == options.EncryptCmd {
		if len(options.Files) == 0 {
			options.Files, _ = utils.FindUnencryptedFiles(projectRoot)
		}
		for _, path := range options.Files {
			fmt.Printf("encrypting %s\n", path)
			utils.ExitIfError(kms.Encrypt(key, path))
			err := git.AddToIgnored(projectRoot, path)
			if err == git.ErrFileAlreadyTracked {
				utils.ErrPrintln("Warning: plain-text file already checked in: %s", path)
				continue
			}
			utils.ExitIfError(err)
		}
		os.Exit(0)
	}
	if options.Cmd == options.DecryptCmd {
		if len(options.Files) == 0 {
			options.Files, _ = utils.FindEncryptedFiles(options.OpenAll, projectRoot)
		}
		for _, path := range options.Files {
			fmt.Printf("decrypting %s\n", path)
			err := kms.Decrypt(key, path)
			utils.ExitIfError(err)
		}
		os.Exit(0)
	}
	utils.ErrPrintln("Unknown command: %s\n%s", options.Cmd, options.Usage)
	os.Exit(1)
}
