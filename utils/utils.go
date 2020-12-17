// `utils` has various utilities. Should only contain pure functions not dependant on `options`.
package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

var ignore = struct{}{}

var ignoreFolders = map[string]struct{}{
	".git":         ignore,
	"node_modules": ignore,
	"mongo-data":   ignore,
}

func IsIgnoredFolder(path string) bool {
	_, ok := ignoreFolders[path]
	return ok
}

func FindEncryptedFiles(openAll bool, root string) ([]string, error) {
	var rgx string
	if openAll {
		rgx = `\.enc$`
	} else {
		rgx = `secret\.(yaml|yml)\.enc$`
	}
	return FindFiles(root, *regexp.MustCompile(rgx))
}

func FindUnencryptedFiles(root string) ([]string, error) {
	return FindFiles(root, *regexp.MustCompile(`secret\.(yaml|yml)$`))
}

func FindFiles(root string, re regexp.Regexp) ([]string, error) {
	result := make([]string, 0, 1)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if IsIgnoredFolder(info.Name()) {
			return filepath.SkipDir
		}

		if !info.IsDir() && re.MatchString(path) {
			absolutePath, _ := filepath.Abs(path)
			result = append(result, absolutePath)
		}

		return nil
	})

	if err != nil {
		ErrPrintln("%s", err)
	}

	return result, nil
}

func RunCommand(name string, arg ...string) (*exec.Cmd, string, string, error) {
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
		ErrPrintln("command failed: %s", cmd)
		ErrPrintln("%s", stdErr.String())
	}
	return cmd, stdOut.String(), stdErr.String(), err
}

func NoopDebugln(format string, a ...interface{}) error {
	return nil
}

func ErrPrintln(format string, a ...interface{}) error {
	_, err := fmt.Fprintf(os.Stderr, format+"\n", a...)
	return err
}

func ExitIfError(err error) {
	if err != nil {
		ErrPrintln("Error: %s", err)
		os.Exit(1)
	}
}
