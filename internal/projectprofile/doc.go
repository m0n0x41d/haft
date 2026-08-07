// Package projectprofile defines the project-profile data used for read-only
// legacy carrier compatibility and the additive final-v1 admission algebra.
//
// The legacy Auto/Declared carrier is deliberately orientation-only: parsing
// its receipt-shaped YAML cannot establish Required or NotApplicable and the
// package exposes no legacy authorization or declaration-write seam. A later
// canonical resolver must consume the final-v1 durable admission record before
// binding capability applicability becomes reachable.
//
// The additive final-v1 algebra keeps its candidate payload cycle-free, models
// onboarding as a dated Work occurrence, and exposes only non-binding admission
// inputs. Final receipts, Declared v1 profiles, and admission records have
// package-private concrete forms and finalization.
package projectprofile
