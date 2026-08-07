package cli

var initPi bool

const (
	piPackageRelDir = ".haft/pi/haft-pi"
	// Project-scope local package paths in .pi/settings.json are resolved by
	// pi relative to the .pi directory itself, not the project root — hence
	// the leading "..". A root-relative "./..." entry is silently skipped.
	piSettingsEntry = "../" + piPackageRelDir
	// Written by earlier haft builds; resolved to .pi/.haft/... and never
	// loaded. The typed legacy registry migrates only this exact scalar.
	piLegacySettingsEntry = "./" + piPackageRelDir
)

func init() {
	initCmd.Flags().BoolVar(&initPi, "pi", false, "Configure for Pi — materializes the bundled @haft/pi package and registers it in .pi/settings.json")
}
