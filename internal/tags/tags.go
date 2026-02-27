package tags

const (
	TagCLI       = "cli"
	TagStoreMode = "cli-store-mode"
	TagSchema    = "cli-schema"
	TagFlatten   = "cli-flatten"
	TagOwner     = "cli-owner"

	TagCLIValue    = "bundr"
	TagSchemaValue = "v1"

	StoreModeRaw  = "raw"
	StoreModeJSON = "json"
)

// ManagedTags returns the required tags for a managed key.
func ManagedTags(storeMode string) map[string]string {
	return map[string]string{
		TagCLI:       TagCLIValue,
		TagStoreMode: storeMode,
		TagSchema:    TagSchemaValue,
	}
}
