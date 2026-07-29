package entitycontract

import "github.com/m0n0x41d/haft/internal/typedmemorywire"

const (
	Version                 = "haft.entity.v1"
	ActionEstablish         = "establish"
	EntityReferenceKindID   = "U.EntityRef"
	MaximumAliases          = typedmemorywire.MaximumChanges - 1
	ExplicitOperatorRequest = "explicit_operator_request"
	NamedReceivingUse       = "named_receiving_use"
)
