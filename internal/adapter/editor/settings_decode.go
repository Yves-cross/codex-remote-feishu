package editor

import "encoding/json"

func decodeVSCodeSettings(raw []byte) (map[string]any, error) {
	settings := map[string]any{}
	if len(raw) == 0 {
		return settings, nil
	}
	normalized := stripVSCodeJSONTrailingCommas(stripVSCodeJSONComments(raw))
	if err := json.Unmarshal(normalized, &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

func stripVSCodeJSONComments(raw []byte) []byte {
	out := make([]byte, 0, len(raw))
	inString := false
	escape := false
	lineComment := false
	blockComment := false

	for i := 0; i < len(raw); i++ {
		current := raw[i]
		if lineComment {
			if current == '\n' || current == '\r' {
				lineComment = false
				out = append(out, current)
			}
			continue
		}
		if blockComment {
			if current == '\n' || current == '\r' {
				out = append(out, current)
				continue
			}
			if current == '*' && i+1 < len(raw) && raw[i+1] == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if inString {
			out = append(out, current)
			if escape {
				escape = false
				continue
			}
			switch current {
			case '\\':
				escape = true
			case '"':
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			out = append(out, current)
			continue
		}
		if current == '/' && i+1 < len(raw) {
			switch raw[i+1] {
			case '/':
				lineComment = true
				i++
				continue
			case '*':
				blockComment = true
				i++
				continue
			}
		}
		out = append(out, current)
	}
	return out
}

func stripVSCodeJSONTrailingCommas(raw []byte) []byte {
	out := make([]byte, 0, len(raw))
	inString := false
	escape := false

	for _, current := range raw {
		if inString {
			out = append(out, current)
			if escape {
				escape = false
				continue
			}
			switch current {
			case '\\':
				escape = true
			case '"':
				inString = false
			}
			continue
		}

		switch current {
		case '"':
			inString = true
			out = append(out, current)
		case '}', ']':
			for i := len(out) - 1; i >= 0; i-- {
				if isVSCodeJSONWhitespace(out[i]) {
					continue
				}
				if out[i] == ',' {
					out = append(out[:i], out[i+1:]...)
				}
				break
			}
			out = append(out, current)
		default:
			out = append(out, current)
		}
	}
	return out
}

func isVSCodeJSONWhitespace(value byte) bool {
	switch value {
	case ' ', '\n', '\r', '\t':
		return true
	default:
		return false
	}
}
