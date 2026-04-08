package control

import "strings"

// LegacyActionKey normalizes known removed compat actions so different ingress
// aliases can share one user-facing migration path.
func LegacyActionKey(text string) string {
	trimmed := strings.TrimSpace(text)
	switch strings.ToLower(trimmed) {
	case "/newinstance", "newinstance", "new_instance":
		return "newinstance"
	case "/killinstance", "killinstance", "kill_instance":
		return "killinstance"
	case "resume_headless_thread":
		return "resume_headless_thread"
	default:
		return trimmed
	}
}

// LegacyActionCommand returns the user-facing command that best explains a
// removed compat action.
func LegacyActionCommand(text string) string {
	switch LegacyActionKey(text) {
	case "newinstance", "resume_headless_thread":
		return "/newinstance"
	case "killinstance":
		return "/killinstance"
	default:
		return strings.TrimSpace(text)
	}
}
