package utils

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/algebananazzzzz/planear/pkg/constants"
)

func ParseJSONFile(filePath string, fileDesc string, v any) error {
	// Read the existing changelog file
	data, err := os.ReadFile(filePath)

	if os.IsNotExist(err) {
		// file does not exist
		fmt.Printf("%s%s does not exist.. overriding%s\n", constants.ColorPurple, fileDesc, constants.ColorReset)
	} else if err != nil {
		// Error reading changelog file
		return fmt.Errorf("failed to read: %w", err)
	} else {
		// Otherwise, parse the changelog JSON
		if err := json.Unmarshal(data, v); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	}
	return nil
}

func WriteJSONFile(filePath string, fileDesc string, data any) error {
	// Marshal the updated changelog
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := WriteToFile(filePath, fileDesc, out); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}
	return nil
}
