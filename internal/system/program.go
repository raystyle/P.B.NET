package system

import (
	"fmt"
	"os"
	"path/filepath"
)

// ExecutableName is used to get the executable file name.
func ExecutableName() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Base(path), nil
}

// ChangeCurrentDirectory is used to changed path for service program
// and prevent to get invalid path when running test.
func ChangeCurrentDirectory() error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	return os.Chdir(filepath.Dir(path))
}

// CheckError is used to check error is nil, if err is not nil,
// it will print error and exit program with code 1.
func CheckError(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// PrintError is used to print error and exit program with code 1.
func PrintError(err string) {
	fmt.Println(err)
	os.Exit(1)
}
