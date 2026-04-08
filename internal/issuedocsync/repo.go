package issuedocsync

import (
	"fmt"
	"strings"
)

func ParseRepo(value string) (Repo, error) {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Repo{}, fmt.Errorf("invalid repo %q, want owner/name", value)
	}
	return Repo{Owner: parts[0], Name: parts[1]}, nil
}
