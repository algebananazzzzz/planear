package input

import (
	"io/fs"
	"path/filepath"
)

// LoadCSVDirectoryToMap loads all `.csv` files from a directory, decodes records of type T,
// and builds a map using a user-defined key extractor.
//
// It expects each CSV file in the directory to have headers matching the csv struct tags on T.
// The keyFunc extracts the map key from each decoded T value.
// All CSV files are merged into a single result map. If a key appears more than once across files,
// the later record will overwrite the earlier one.
//
// Returns an error if any CSV file is unreadable or improperly formatted.
//
// Example usage:
//
//	usersByID, err := LoadCSVDirectoryToMap[User, string](
//	    "./data",
//	    func(u User) string { return u.ID },
//	)
func LoadCSVDirectoryToMap[T any, K comparable](dirPath string, keyFunc func(T) K) (map[K]T, error) {
	records := make(map[K]T)

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".csv" {
			return nil // skip directories and non-csv files
		}

		fileRecords, err := DecodeCSVFile[T](path)
		if err != nil {
			return err
		}

		// Merge fileRecords into map with keys extracted by keyFunc
		for _, rec := range fileRecords {
			k := keyFunc(rec)
			records[k] = rec
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return records, nil
}
