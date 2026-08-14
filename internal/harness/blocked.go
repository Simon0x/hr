package harness

import "strings"

var permissionBlockSignatures = []string{
	"permission denied",
	"permission not granted",
	"requested permissions",
	"requires approval",
	"not authorized",
	"not allowed",
}

func LooksPermissionBlocked(output string) bool {
	lower := strings.ToLower(output)
	for _, sig := range permissionBlockSignatures {
		if strings.Contains(lower, sig) {
			return true
		}
	}
	return false
}
