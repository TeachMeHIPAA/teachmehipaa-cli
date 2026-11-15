package resource

import (
	"context"
	"errors"
	"fmt"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/api"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
)

// waitForOperation prints the Operation once it is done, or saves an
// export's file with -o. A failed or timed-out Operation is still printed,
// so a script has its name.
func waitForOperation(ctx context.Context, f *cmdutil.Factory, client *api.Client, body []byte, in *input) error {
	operation, err := api.ParseOperation(body)
	if err != nil {
		return err
	}
	operation, err = client.WaitOperation(ctx, operation, in.waitTimeout)
	var failed *api.OperationFailedError
	var timedOut *api.OperationTimeoutError
	switch {
	case errors.As(err, &timedOut):
		return errors.Join(f.PrintJSON(operation.Body),
			fmt.Errorf("%w; check on it with `tmh operations retrieve %s --wait`", err, operation.ID()))
	case errors.As(err, &failed):
		return errors.Join(f.PrintJSON(operation.Body), err)
	case err != nil:
		return err
	}
	if in.output == "" {
		return f.PrintJSON(operation.Body)
	}
	export, err := api.ExportOf(operation)
	if err != nil {
		return err
	}
	if err := f.WriteOutput(in.output, export.Data, false); err != nil {
		return err
	}
	fmt.Fprintf(f.Stderr, "Saved %s (%d rows) to %s\n", export.Filename, export.RowCount, cmdutil.OutputName(in.output))
	return nil
}
