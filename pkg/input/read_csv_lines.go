package input

import (
	"encoding/csv"
	"fmt"
	"os"
)

// ReadCSVLines reads a CSV file and returns all rows as a slice of string slices.
func ReadCSVLines(filePath string) ([][]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file %s: %w", filePath, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV file %s: %w", filePath, err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("empty CSV file: %s", filePath)
	}

	return records, nil
}
