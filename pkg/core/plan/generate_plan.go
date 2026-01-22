package plan

import (
	"fmt"
	"os"

	"github.com/algebananazzzzz/planear/pkg/constants"
	"github.com/algebananazzzzz/planear/pkg/core/diff"
	"github.com/algebananazzzzz/planear/pkg/formatters"
	"github.com/algebananazzzzz/planear/pkg/input"
	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/algebananazzzzz/planear/pkg/utils"
)

type GenerateParams[T any] struct {
	CSVPath           string                       // Path to the local CSV directory
	OutputFilePath    string                       // Where to save the generated plan
	FormatRecordFunc  func(T) string               // Formats a record into a display string
	FormatKeyFunc     func(string) string          // Formats the primary key for display
	ExtractKeyFunc    func(T) string               // Extracts the key from a record
	LoadRemoteRecords func() (map[string]T, error) // Function to load remote (DB) records
	ValidateRecord    func(T) error                // Validator for local records
}

func Generate[T any](params GenerateParams[T]) (*types.Plan[T], error) {
	// Validate required function parameters
	if params.ExtractKeyFunc == nil {
		return nil, fmt.Errorf("ExtractKeyFunc is required")
	}
	if params.LoadRemoteRecords == nil {
		return nil, fmt.Errorf("LoadRemoteRecords is required")
	}
	if params.ValidateRecord == nil {
		return nil, fmt.Errorf("ValidateRecord is required")
	}
	if params.FormatRecordFunc == nil {
		return nil, fmt.Errorf("FormatRecordFunc is required")
	}
	if params.FormatKeyFunc == nil {
		return nil, fmt.Errorf("FormatKeyFunc is required")
	}

	localRecords, err := input.LoadCSVDirectoryToMap(params.CSVPath, params.ExtractKeyFunc)
	if err != nil {
		fmt.Printf("%sfailed to load local CSV records: %v%s", constants.ColorRed, err, constants.ColorReset)
		return nil, fmt.Errorf("failed to load local CSV records: %v", err)
	}

	remoteRecords, err := params.LoadRemoteRecords()
	if err != nil {
		fmt.Printf("%sfailed to load remote records: %v%s", constants.ColorRed, err, constants.ColorReset)
		return nil, fmt.Errorf("failed to load remote records: %v", err)
	}

	plan, err := diff.ComputePlanDiff(localRecords, remoteRecords, params.ValidateRecord)
	if err != nil {
		fmt.Printf("%serror generating plan diff: %v%s", constants.ColorRed, err, constants.ColorReset)
		return nil, fmt.Errorf("error generating plan diff: %v", err)
	}

	if plan.IsEmpty() {
		fmt.Printf("%sNo changes required%s\n", constants.ColorGreen, constants.ColorReset)
		if params.OutputFilePath != "" {
			info, err := os.Stat(params.OutputFilePath)
			if err == nil {
				if !info.IsDir() {
					if err := os.Remove(params.OutputFilePath); err != nil {
						fmt.Printf("%sfailed to remove stale plan file: %v%s\n", constants.ColorRed, err, constants.ColorReset)
						return nil, fmt.Errorf("failed to remove stale plan file: %v", err)
					}
					fmt.Printf("%sWarning: removed stale plan file at %s%s\n", constants.ColorYellow, params.OutputFilePath, constants.ColorReset)
				}
			} else if !os.IsNotExist(err) {
				fmt.Printf("%sfailed to check for stale plan file: %v%s\n", constants.ColorRed, err, constants.ColorReset)
				return nil, fmt.Errorf("failed to check for stale plan file: %v", err)
			}
		}
		return &plan, nil
	}

	planDescription := formatters.FormatPlan(plan, params.FormatRecordFunc, params.FormatKeyFunc)
	fmt.Print(planDescription)

	if err := utils.WriteJSONFile(params.OutputFilePath, "plan file", plan); err != nil {
		fmt.Printf("%sfailed to write plan to file: %v%s", constants.ColorRed, err, constants.ColorReset)
		return nil, fmt.Errorf("failed to write plan to file: %v", err)
	}
	return &plan, nil
}
