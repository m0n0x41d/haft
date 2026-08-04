package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project/specflow"
)

var specDraftContractJSON bool

var specDraftContractCmd = &cobra.Command{
	Use:   "draft-contract",
	Short: "Print the profile-independent SpecSection drafting contract",
	Long: `Print the canonical phase, field, value, and check contract needed to
draft target-system, software-system, and term-map carriers.

The contract is profile-independent design-time product knowledge. Reading it
does not establish applicability, activate or approve a section, mutate a
profile, create evidence, or pass a gate. Its validation_call points to the
canonical carrier-first validation surface.`,
	Args: cobra.NoArgs,
	RunE: runSpecDraftContract,
}

func init() {
	specDraftContractCmd.Flags().BoolVar(
		&specDraftContractJSON,
		"json",
		false,
		"print the drafting contract as structured JSON",
	)
	specCmd.AddCommand(specDraftContractCmd)
}

func runSpecDraftContract(cmd *cobra.Command, _ []string) error {
	contract := specflow.CanonicalDraftContract()
	if specDraftContractJSON {
		return writeIndentedJSON(cmd.OutOrStdout(), contract)
	}
	return writeSpecDraftContractSummary(cmd.OutOrStdout(), contract)
}

func writeSpecDraftContractSummary(
	writer io.Writer,
	contract specflow.DraftContract,
) error {
	_, err := fmt.Fprintf(
		writer,
		"Spec draft contract: %s\nAuthority: %s\nPhases: %d\nValidation: %s(action=%q)\n",
		contract.ContractVersion,
		contract.Authority,
		len(contract.Phases),
		contract.ValidationCall.Tool,
		contract.ValidationCall.Arguments["action"],
	)
	return err
}

func encodeDraftContractJSON() (string, error) {
	contract := specflow.CanonicalDraftContract()
	payload, err := json.Marshal(contract)
	if err != nil {
		return "", fmt.Errorf("marshal draft contract: %w", err)
	}
	return string(payload), nil
}
