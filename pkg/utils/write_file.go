package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/algebananazzzzz/planear/pkg/constants"
)

// WriteToFile writes content to the given filePath, creating directories if needed.
func WriteToFile(filePath string, fileDesc string, content []byte) error {
	// If content is empty, prints a skip message
	if len(content) == 0 {
		fmt.Printf("%sSkipping write for empty %s%s\n", constants.ColorYellow, fileDesc, constants.ColorReset)
		return nil
	}

	// Create the directory structure if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}

	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return err
	}
	fmt.Printf("%sSuccessfully written %s to: %s%s\n", constants.ColorGreen, fileDesc, filePath, constants.ColorReset)
	return nil
}

func CreateFileWriter(filePath string) (io.Writer, error) {
	dir := filepath.Dir(filePath)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("create directory %s: %v", dir, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("create file %s: %v", filePath, err)
	}

	return file, nil
}
