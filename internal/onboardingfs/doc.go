// Package onboardingfs owns the exact filesystem carrier for the non-binding
// structured-memory "Not now" disposition.
//
// Writers serialize through an advisory lock and hold no-follow directory
// identities across namespace effects. Install never overwrites different
// bytes. Reopen removes only the exact expected carrier. Failures after a
// linearizing link or unlink become closed outcome-unknown results whose only
// recovery is an exact-same retry.
package onboardingfs
