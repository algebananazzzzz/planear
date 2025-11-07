package main

import (
	"os"

	"github.com/algebananazzzzz/planear/examples/lib"
	"github.com/algebananazzzzz/planear/pkg/core/plan"
)

func main() {
	params := plan.GenerateParams[lib.UserRecord]{
		CSVPath:           lib.CsvDir,
		OutputFilePath:    lib.OutputPlanPath,
		FormatRecordFunc:  lib.FormatRecord,
		FormatKeyFunc:     lib.FormatKey,
		ExtractKeyFunc:    lib.ExtractKey,
		LoadRemoteRecords: lib.LoadRemoteRecords,
		ValidateRecord:    lib.ValidateUserRecord,
	}

	_, err := plan.Generate(params)
	if err != nil {
		// Error is already printed by plan.Generate
		// Exit gracefully with code 1 after all logs are printed
		os.Exit(1)
	}
}
