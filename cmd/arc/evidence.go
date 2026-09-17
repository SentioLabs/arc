// Evidence records use captured work versions even when the server head has moved.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/sentiolabs/arc/internal/types"
	"github.com/spf13/cobra"
)

func init() { rootCmd.AddCommand(newEvidenceCommand()) }

// newEvidenceCommand requires a phase, source artifact, and explicit stdin input.
// The evidence payload is separate from the captured governance preconditions.
func newEvidenceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evidence ISSUE",
		Short: "Record phase evidence using the original work context",
		Args:  cobra.ExactArgs(1),
		RunE:  runEvidence,
	}
	cmd.Flags().String("phase", "", "build, review, or verify")
	cmd.Flags().String("context", "", "ExpectedGovernance JSON captured at work start")
	cmd.Flags().Bool("stdin", false, "Read evidence from stdin")
	return cmd
}

// runEvidence validates the stored capture before sending completion provenance.
// It sends evidence bytes unchanged and leaves stale conflicts visible to the caller.
// No preliminary governance read can turn stale work into evidence for a newer pin.
func runEvidence(cmd *cobra.Command, args []string) error {
	if err := requirePlanFlags(cmd, "phase", "context", "stdin"); err != nil {
		return err
	}
	if !boolFlag(cmd, "stdin") {
		return errors.New("--stdin is required")
	}
	expected, err := readWorkContext(stringFlag(cmd, "context"))
	if err != nil {
		return err
	}
	evidence, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	c, p, err := planClient()
	if err != nil {
		return err
	}
	record, err := c.RecordExecutionEvidence(
		p,
		args[0],
		types.ExecutionEvidenceRequest{
			Expected: expected,
			Phase:    stringFlag(cmd, "phase"),
			Evidence: string(evidence),
		},
	)
	if err != nil {
		return err
	}
	if outputJSON {
		outputResult(record)
	} else {
		fmt.Printf("Evidence: %s phase %s, contract version %d\n",
			record.ID, record.Phase, record.Expected.ContractVersion,
		)
		outputResult(record.Expected)
	}
	return nil
}
