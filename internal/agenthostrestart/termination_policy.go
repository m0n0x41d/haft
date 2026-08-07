package agenthostrestart

import "fmt"

type applicationTerminationPolicy uint8

const (
	terminationGracefulOnly applicationTerminationPolicy = iota
	terminationAllowSIGTERM
)

func applicationTerminationPolicyForAllowTerm(
	allowTerm bool,
) applicationTerminationPolicy {
	if allowTerm {
		return terminationAllowSIGTERM
	}
	return terminationGracefulOnly
}

func (policy applicationTerminationPolicy) validate() error {
	if policy == terminationGracefulOnly || policy == terminationAllowSIGTERM {
		return nil
	}
	return fmt.Errorf("unknown application termination policy %d", policy)
}

func (policy applicationTerminationPolicy) String() string {
	if policy == terminationAllowSIGTERM {
		return "explicit_sigterm_opt_in"
	}
	return "graceful_only"
}

func (policy applicationTerminationPolicy) appendSupervisorArguments(
	arguments []string,
) []string {
	result := append([]string(nil), arguments...)
	if policy == terminationAllowSIGTERM {
		result = append(result, "--allow-term")
	}
	return result
}
