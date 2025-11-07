package main

import (
	"os"

	"github.com/algebananazzzzz/planear/examples/lib"
	"github.com/algebananazzzzz/planear/pkg/core/apply"
)

func main() {
	params := apply.RunParams[lib.UserRecord]{
		PlanFilePath: lib.OutputPlanPath,
		FormatRecord: lib.FormatRecord,
		FormatKey:    lib.FormatKey,
		OnAdd:        lib.AddUser,
		OnUpdate:     lib.UpdateUser,
		OnDelete:     lib.DeleteUser,
		OnFinalize:   lib.Finalize,
	}

	if err := apply.Run(params); err != nil {
		// Error is already printed by apply.Run
		// Exit gracefully with code 1 after all logs are printed
		os.Exit(1)
	}
}
