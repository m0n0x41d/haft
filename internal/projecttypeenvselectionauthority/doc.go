// Package projecttypeenvselectionauthority owns the pure durable source and
// resolution algebra for selecting one exact project TypeEnv head. The default
// branch records lower-assurance dedicated-CLI ingress under an exact project
// policy without pretending the kernel observed h-decide. The strict branch
// binds reviewed content, one actual human SpeechAct/Work record, its
// content-addressed U.Commitment(MAY) Permission, and a transaction-time
// authority judgement.
//
// The semantic contract is source-adapter-neutral. A resolver-policy edition
// separately pins a concrete verified adapter (the controlling-terminal
// adapter is one option). This package does not make that strict adapter the
// default product authorization mode and does not model an explicit h-decide
// invocation as a SpeechAct that the kernel did not observe.
//
// The Permission subject is a separate HaftSoftwareSystem RoleAssignment, not
// the human SpeechAct performer. A closed built-in policy-edition registry
// derives its ProjectGovernanceSubstrate assignment from independently
// content-addressed A.1 system admission, A.2 role admission, A.2.1
// justification, and provenance records. New writes use only the current
// edition while its exact FPF and product-spec source pins remain current;
// historical editions remain decodeable for replay.
//
// All records here are replayable data. This package deliberately exports no
// live authority-use constructor or capability. The effect service must reread
// the exact current policy, mint and consume a nonserializable single-use
// capability, and perform compare-and-swap mutation. Until that boundary is
// present, this package is not a complete public head-selection service.
package projecttypeenvselectionauthority
