package git

import (
	"errors"
	"jobbatical/secrets/log"
	"jobbatical/secrets/utils"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var ErrFileAlreadyTracked = errors.New("file already tracked")

func isTracked(projectRoot string, filePath string) (bool, error) {
	_, _, _, err := utils.RunCommand(
		"git",
		"-C", projectRoot,
		"ls-files", "--error-unmatch", filePath,
	)
	if err != nil {
		return false, err
	}
	return true, nil
}

func isIgnored(projectRoot string, filePath string) (bool, error) {
	_, stdOut, _, err := utils.RunCommand(
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

func AddToIgnored(projectRoot string, fileToIgnore string) error {
	relativePath, err := filepath.Rel(projectRoot, fileToIgnore)
	if err != nil {
		return err
	}

	isTracked, err := isTracked(projectRoot, relativePath)
	if isTracked {
		log.PrintDebugln("NOT appending %s to gitignore because it's already tracked", fileToIgnore)
		return ErrFileAlreadyTracked
	}
	isIgnored, err := isIgnored(projectRoot, fileToIgnore)
	if isIgnored {
		log.PrintDebugln("NOT appending %s to gitignore because it's already ignored", fileToIgnore)
		return nil
	}
	return appendToFile(path.Join(projectRoot, ".gitignore"), relativePath)
}
