// You can edit this code!
// Click here and start typing.
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
)

var ignore = struct{}{}

var ignoreFolders = map[string]struct{}{
	".git":         ignore,
	"node_modules": ignore,
	"mongo-data":   ignore,
}

const (
	encryptCmd string = "seal"
	decryptCmd string = "open"
)

var verbose bool
var dryRun bool
var projectRoot string
var key string

/*
	secrets open <root folder>
	secrets open :: take root folder from pwd
	secrets seal
*/

func isIgnoredFolder(path string) bool {
	_, ok := ignoreFolders[path]
	// fmt.Println(path, value, ok)
	return ok
}

func findEncryptedFiles(root string) ([]string, error) {
	return findFiles(root, *regexp.MustCompile(`\.enc$`))
}

func findUnencryptedFiles(root string) ([]string, error) {
	return findFiles(root, *regexp.MustCompile(`secret\.(yaml|yml)$`))
}

func findFiles(root string, re regexp.Regexp) ([]string, error) {
	result := make([]string, 0, 1)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if isIgnoredFolder(info.Name()) {
			return filepath.SkipDir
		}

		if !info.IsDir() && re.MatchString(path) {
			absolutePath, _ := filepath.Abs(path)
			result = append(result, absolutePath)
		}

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}

	return result, nil
}

func printIf(str string) {
	if len(str) > 0 {
		fmt.Println(str)
	}
}

func callKms(operation string, keyName string, plaintextFile string, ciphertextFile string) error {
	if dryRun {
		return nil
	}
	cmd := exec.Command(
		"gcloud",
		"kms",
		operation,
		"--location", "global",
		"--keyring", "immi-project-secrets",
		"--key", keyName,
		"--plaintext-file", plaintextFile,
		"--ciphertext-file", ciphertextFile,
	)
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	err := cmd.Run()
	if err != nil {
		if strings.Contains(stdErr.String(), "NOT_FOUND: ") {
			err := createKey(keyName)
			if err != nil {
				return err
			}
			return callKms(operation, keyName, plaintextFile, ciphertextFile)
		}
		printIf(fmt.Sprintf("out: %s", stdOut.String()))
		printIf(fmt.Sprintf("err: %s", stdErr.String()))
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}
	return nil
}

func createKey(keyName string) error {
	fmt.Printf("creating key for the project %s\n", keyName)
	if dryRun {
		return nil
	}
	cmd := exec.Command(
		"gcloud",
		"kms",
		"keys",
		"create", keyName,
		"--purpose", "encryption",
		"--rotation-period", "100d",
		"--next-rotation-time", "+p100d",
		"--location", "global",
		"--keyring", "immi-project-secrets",
	)
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	err := cmd.Run()
	if err != nil {
		printIf(fmt.Sprintf("out: %s", stdOut.String()))
		printIf(fmt.Sprintf("err: %s", stdErr.String()))
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return err
	}
	return nil
}

func encrypt(keyName string, plaintextFile string) {
	callKms("encrypt", keyName, plaintextFile, plaintextFile+".enc")
}

func decrypt(keyName string, ciphertextFile string) {
	re := regexp.MustCompile(`\.enc$`)
	plaintextFile := re.ReplaceAllString(ciphertextFile, "")
	if plaintextFile == ciphertextFile {
		fmt.Fprintf(os.Stderr, "Not a .enc file: %s\n", ciphertextFile)
		os.Exit(1)
	}
	callKms("decrypt", keyName, plaintextFile, ciphertextFile)
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

func remove(slice []string, s int) []string {
	return append(slice[:s], slice[s+1:]...)
}

func popCommand(args []string) (string, []string, error) {
	for i, a := range args {
		if i == 0 {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			return a, remove(args, i), nil
		}
	}
	return "", args, errors.New("command not found")
}

func isGitTracked(projectRoot string, filePath string) (bool, error) {
	// fmt.Println("is", filePath, "tracked in", projectRoot)
	cmd := exec.Command(
		"git",
		"-C", projectRoot,
		"ls-files", "--error-unmatch", filePath,
	)
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	err := cmd.Run()
	if err != nil {
		// printIf(fmt.Sprintf("out: %s", stdOut.String()))
		// printIf(fmt.Sprintf("err: %s", stdErr.String()))
		return false, err
	}
	return true, nil
}

func isGitIgnored(projectRoot string, filePath string) (bool, error) {
	// fmt.Println("is", filePath, "ignored in", projectRoot)
	cmd := exec.Command(
		"git",
		"-C", projectRoot,
		"check-ignore", filePath,
	)
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	err := cmd.Run()
	if err != nil {
		printIf(fmt.Sprintf("out: %s", stdOut.String()))
		printIf(fmt.Sprintf("err: %s", stdErr.String()))
		return false, err
	}
	return (strings.TrimSpace(stdOut.String()) == filePath), nil
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

func addGitIgnore(projectRoot string, fileToIgnore string) error {
	isTracked, err := isGitTracked(projectRoot, fileToIgnore)
	if isTracked {
		// fmt.Println("NOT appending", fileToIgnore, "to gitignore because it's already tracked")
		return errors.New("file already tracked")
	}
	isIgnored, err := isGitIgnored(projectRoot, fileToIgnore)
	// fmt.Println(isIgnored, err)
	if isIgnored {
		// fmt.Println("NOT appending", fileToIgnore, "to gitignore because it's already ignored")
		return nil
	}
	relativePath, err := filepath.Rel(projectRoot, fileToIgnore)
	if err != nil {
		return err
	}
	return appendToFile(path.Join(projectRoot, ".gitignore"), relativePath)
}

func main() {
	// fmt.Println(isGitTracked("/home/rauno/projects/go/secrets", "test/pipeline"))
	// fmt.Println(addGitIgnore("/home/rauno/projects/@jobbatical/analytics", "/home/rauno/projects/@jobbatical/analytics/env.run"))
	// os.Exit(0)
	flag.BoolVar(&verbose, "verbose", false, "Log debug info")
	flag.BoolVar(&dryRun, "dry-run", false, "Skip calls to GCP")
	flag.StringVar(&projectRoot, "root", "", "Project root folder(name will be used as key name)")
	flag.StringVar(&key, "key", "", "Key to use")
	var (
		cmd string
		err error
	)
	cmd, os.Args, err = popCommand(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	if verbose {
		fmt.Println(os.Args)
	}

	flag.Parse()

	if projectRoot == "" {
		projectRoot, _ = findProjectRoot(".")
	}

	if key == "" {
		key = filepath.Base(projectRoot)
	}

	if verbose {
		fmt.Printf("dry run: %t\n", dryRun)
		fmt.Printf("key: %s\n", key)
		fmt.Printf("project root: %s\n", projectRoot)
		fmt.Printf("cmd: %s\n", cmd)
	}

	if cmd == encryptCmd {
		files, _ := findUnencryptedFiles(projectRoot)
		for _, path := range files {
			encrypt(key, path)
			addGitIgnore(projectRoot, path)
			fmt.Printf("%s encrypted\n", path)
		}
		os.Exit(0)
	}
	if cmd == decryptCmd {
		files, _ := findEncryptedFiles(projectRoot)
		for _, path := range files {
			decrypt(key, path)
			fmt.Printf("%s decrypted\n", path)
		}
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "Unknown command: %s\nUsage: secrets [open|seal] [--dry-run] [--verbose] [--root <project root>] [--key <encryption key name>]\n", cmd)
	os.Exit(1)
}
