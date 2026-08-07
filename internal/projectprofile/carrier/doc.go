// Package carrier reads the legacy haft.project-profile/v1 YAML carrier.
//
// The legacy carrier is a compatibility input only. It is neither the
// canonical project-profile projection nor an authority or applicability
// proof. New project-profile state must be admitted through the durable
// profile-admission boundary and projected by the current projection package.
// This package deliberately exposes no encoder or write operation.
package carrier
