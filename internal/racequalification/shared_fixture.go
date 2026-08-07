package racequalification

// SharedFixtureDirectoryEnvironment names the runner-owned, invocation-local
// directory that race test processes may use to publish immutable derived test
// fixtures. The runner remains the sole owner of the directory lifecycle.
const SharedFixtureDirectoryEnvironment = "HAFT_RACE_SHARED_FIXTURE_DIRECTORY"
