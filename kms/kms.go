package kms

import (
	"fmt"
	"jobbatical/secrets/log"
	"jobbatical/secrets/options"
	"jobbatical/secrets/utils"
	"os"
	"regexp"
	"strings"
)

type gcloudError struct {
	err    error
	stdErr string
}

func (e *gcloudError) Error() string {
	return fmt.Sprintf("gcloud command failed: %s", e.stdErr)
}

func callKms(operation string, keyName string, plaintextFile string, ciphertextFile string) error {
	if options.DryRun {
		return nil
	}
	_, _, stdErr, err := utils.RunCommand(
		"gcloud",
		"kms",
		operation,
		"--location", options.Location,
		"--keyring", options.KeyRing,
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
	log.PrintDebugln("creating key for the project %s", keyName)
	if options.DryRun {
		return nil
	}
	_, _, stdErr, err := utils.RunCommand(
		"gcloud",
		"kms",
		"keys",
		"create", keyName,
		"--purpose", "encryption",
		"--rotation-period", "100d",
		"--next-rotation-time", "+p100d",
		"--location", options.Location,
		"--keyring", options.KeyRing,
	)
	if err != nil {
		return &gcloudError{err, stdErr}
	}
	return nil
}

func Encrypt(keyName string, plaintextFile string) error {
	return callKms("encrypt", keyName, plaintextFile, plaintextFile+".enc")
}

func Decrypt(keyName string, ciphertextFile string) error {
	re := regexp.MustCompile(`\.enc$`)
	plaintextFile := re.ReplaceAllString(ciphertextFile, "")
	if plaintextFile == ciphertextFile {
		utils.ErrPrintln("Not a .enc file: %s", ciphertextFile)
		os.Exit(1)
	}
	return callKms("decrypt", keyName, plaintextFile, ciphertextFile)
}
