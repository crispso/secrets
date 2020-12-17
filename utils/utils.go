package utils

import (
	"errors"
	"fmt"
	"os"
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

func IsIgnoredFolder(path string) bool {
	_, ok := ignoreFolders[path]
	return ok
}

func Remove(slice []string, s int) []string {
	return append(slice[:s], slice[s+1:]...)
}

func PopCommand(args []string) (string, []string, error) {
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

func PopFiles(args []string) ([]string, []string, error) {
	var (
		file string
		err  error
	)
	files := make([]string, 0, 1)

	for {
		file, os.Args, err = PopCommand(os.Args)
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

func NoopDebugln(format string, a ...interface{}) error {
	return nil
}

func ErrPrintln(format string, a ...interface{}) error {
	_, err := fmt.Fprintf(os.Stderr, format+"\n", a...)
	return err
}
