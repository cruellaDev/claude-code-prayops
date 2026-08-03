package contracts

var AllowedEventAttributeKeys = map[string]struct{}{
	"toolCategory":  {},
	"errorCategory": {},
	"prayerKind":    {},
	"effectSeed":    {},
	"cacheId":       {},
}

// Never persist prompt, command, code, tool_input, tool_output,
// transcript_path, error_details, last_assistant_message, or full image path.
