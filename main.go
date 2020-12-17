package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"jobbatical/secrets/utils"
)

const (
	expectedOrganization string = "jobbatical"
	expectedRepoHost     string = "github.com"
	usage                string = "Usage secrets <open|seal> [<file path>...] [--dry-run] [--verbose] [--root <project root>] [--key <encryption key name>] [--open-all]"
	encryptCmd           string = "seal"
	decryptCmd           string = "open"
	keyRing              string = "immi-project-secrets"
	location             string = "global"
)

var errFileAlreadyTracked = errors.New("file already tracked")
var verbose bool
var dryRun bool
var projectRoot string
var key string
var openAll bool
var printDebugln = utils.ErrPrintln

type gcloudError struct {
	err    error
	stdErr string
}

func (e *gcloudError) Error() string {
	return fmt.Sprintf("gcloud command failed: %s", e.stdErr)
}

func runCommand(name string, arg ...string) (*exec.Cmd, string, string, error) {
	cmd := exec.Command(
		name,
		arg...,
	)
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	err := cmd.Run()
	if err != nil {
		printDebugln("command failed: %s", cmd)
		printDebugln("%s", stdErr.String())
	}
	return cmd, stdOut.String(), stdErr.String(), err
}

func callKms(operation string, keyName string, plaintextFile string, ciphertextFile string) error {
	if dryRun {
		return nil
	}
	_, _, stdErr, err := runCommand(
		"gcloud",
		"kms",
		operation,
		"--location", location,
		"--keyring", keyRing,
		"--key", keyName,
		"--plaintext-file", plaintextFile,
		"--ciphertext-file", ciphertextFile,
	)
	if err != nil {
		if strings.Contains(stdErr, "NOT_FOUND: ") {
			err := createKey(keyName)
			if err != nil {
				return err
			}
			return callKms(operation, keyName, plaintextFile, ciphertextFile)
		}
		return &gcloudError{err, stdErr}
	}
	return nil
}

func createKey(keyName string) error {
	printDebugln("creating key for the project %s", keyName)
	if dryRun {
		return nil
	}
	_, _, stdErr, err := runCommand(
		"gcloud",
		"kms",
		"keys",
		"create", keyName,
		"--purpose", "encryption",
		"--rotation-period", "100d",
		"--next-rotation-time", "+p100d",
		"--location", location,
		"--keyring", keyRing,
	)
	if err != nil {
		return &gcloudError{err, stdErr}
	}
	return nil
}

func encrypt(keyName string, plaintextFile string) error {
	return callKms("encrypt", keyName, plaintextFile, plaintextFile+".enc")
}

func decrypt(keyName string, ciphertextFile string) error {
	re := regexp.MustCompile(`\.enc$`)
	plaintextFile := re.ReplaceAllString(ciphertextFile, "")
	if plaintextFile == ciphertextFile {
		utils.ErrPrintln("Not a .enc file: %s", ciphertextFile)
		os.Exit(1)
	}
	return callKms("decrypt", keyName, plaintextFile, ciphertextFile)
}

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

func isGitTracked(projectRoot string, filePath string) (bool, error) {
	_, _, _, err := runCommand(
		"git",
		"-C", projectRoot,
		"ls-files", "--error-unmatch", filePath,
	)
	if err != nil {
		return false, err
	}
	return true, nil
}

func isGitIgnored(projectRoot string, filePath string) (bool, error) {
	_, stdOut, _, err := runCommand(
		"git",
		"-C", projectRoot,
		"check-ignore", filePath,
	)
	if err != nil {
		return false, err
	}
	return (strings.TrimSpace(stdOut) == filePath), nil
}

func appendToFile(filePath string, line string) error {
	f, err := os.OpenFile(filePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		return err
	}
	return nil
}

func getProjectRepo(projectRoot string) (string, error) {
	_, stdOut, _, err := runCommand("git", "-C", projectRoot, "remote", "-v")
	if err != nil {
		return "", err
	}
	example := fmt.Sprintf("git@%s:%s/<project name>.git", expectedRepoHost, expectedOrganization)
	re := regexp.MustCompile("(?i)" + expectedRepoHost + `:([^/]*)/([^/\.]*)\.git`)
	matches := re.FindStringSubmatch(stdOut)
	if len(matches) == 3 {
		org := matches[1]
		project := matches[2]

		if strings.ToLower(org) == expectedOrganization {
			return project, nil
		}

		return "", fmt.Errorf(
			`%s not a %s project in %s: expecting a remote %s, got %s in %s`,
			projectRoot,
			expectedOrganization,
			expectedRepoHost,
			example,
			project,
			org,
		)
	}
	return "", fmt.Errorf(
		`%s not a project in %s: expecting a remote %s`,
		projectRoot,
		expectedRepoHost,
		example,
	)
}

func addGitIgnore(projectRoot string, fileToIgnore string) error {
	relativePath, err := filepath.Rel(projectRoot, fileToIgnore)
	if err != nil {
		return err
	}

	isTracked, err := isGitTracked(projectRoot, relativePath)
	if isTracked {
		printDebugln("NOT appending %s to gitignore because it's already tracked", fileToIgnore)
		return errFileAlreadyTracked
	}
	isIgnored, err := isGitIgnored(projectRoot, fileToIgnore)
	if isIgnored {
		printDebugln("NOT appending %s to gitignore because it's already ignored", fileToIgnore)
		return nil
	}
	return appendToFile(path.Join(projectRoot, ".gitignore"), relativePath)
}

func exitIfError(err error) {
	if err != nil {
		utils.ErrPrintln("Error: %s", err)
		os.Exit(1)
	}
}

func getKeyName(projectRoot string) string {
	repo, err := getProjectRepo(projectRoot)
	if err == nil {
		return repo
	}
	return filepath.Base(projectRoot)
}

func main() {
	var (
		cmd   string
		files []string
		err   error
	)

	cmd, os.Args, err = utils.PopCommand(os.Args)
	if err != nil {
		utils.ErrPrintln("Error: %s\n%s", err, usage)
		os.Exit(1)
	}

	files, os.Args, err = utils.PopFiles(os.Args)
	exitIfError(err)

	flag.BoolVar(&verbose, "verbose", false, "Log debug info")
	flag.BoolVar(&dryRun, "dry-run", false, "Skip calls to GCP")
	flag.BoolVar(&openAll, "open-all", false, "Opens all .enc files within the repository")
	flag.StringVar(&projectRoot, "root", "", "Project root folder(name will be used as key name)")
	flag.StringVar(&key, "key", "", "Key to use")

	flag.Parse()

	if (!verbose) {
		printDebugln = utils.NoopDebugln
	}

	printDebugln("%s", os.Args)

	if projectRoot == "" {
		projectRoot, _ = findProjectRoot(".")
	}

	if key == "" {
		key = getKeyName(projectRoot)
	}

	printDebugln("dry run: %t", dryRun)
	printDebugln("expectedOrganization: %s", expectedOrganization)
	printDebugln("expectedRepoHost: %s", expectedRepoHost)
	printDebugln("keyRing: %s", keyRing)
	printDebugln("key: %s", key)
	printDebugln("project root: %s", projectRoot)
	printDebugln("cmd: %s", cmd)
	printDebugln("files: %s (%d)", files, len(files))

	if cmd == encryptCmd {
		if len(files) == 0 {
			files, _ = utils.FindUnencryptedFiles(projectRoot)
		}
		for _, path := range files {
			fmt.Printf("encrypting %s\n", path)
			exitIfError(encrypt(key, path))
			err := addGitIgnore(projectRoot, path)
			if err == errFileAlreadyTracked {
				utils.ErrPrintln("Warning: plain-text file already checked in: %s", path)
				continue
			}
			exitIfError(err)
		}
		os.Exit(0)
	}
	if cmd == decryptCmd {
		if len(files) == 0 {
			files, _ = utils.FindEncryptedFiles(openAll, projectRoot)
		}
		for _, path := range files {
			fmt.Printf("decrypting %s\n", path)
			err := decrypt(key, path)
			exitIfError(err)
		}
		os.Exit(0)
	}
	utils.ErrPrintln("Unknown command: %s\n%s", cmd, usage)
	os.Exit(1)
}
