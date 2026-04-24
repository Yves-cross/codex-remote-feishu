package preview

import (
	"fmt"
	"strconv"
	"strings"
)

func parseTurnDiffUnifiedDiff(diff string) []turnDiffParsedFile {
	lines := splitTurnDiffRawLines(diff)
	if len(lines) == 0 {
		return nil
	}

	files := make([]turnDiffParsedFile, 0, 8)
	var current *turnDiffParsedFile
	var currentHunk *turnDiffParsedHunk

	flushHunk := func() {
		if current == nil || currentHunk == nil {
			return
		}
		current.Hunks = append(current.Hunks, *currentHunk)
		currentHunk = nil
	}
	flushFile := func() {
		if current == nil {
			return
		}
		flushHunk()
		normalizeTurnDiffParsedFile(current)
		files = append(files, *current)
		current = nil
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			flushFile()
			file := newTurnDiffParsedFile(len(files), line)
			current = &file
			continue
		}
		if current == nil {
			continue
		}
		current.RawLines = append(current.RawLines, line)

		if currentHunk != nil {
			if nextHunk, ok := parseTurnDiffHunkHeader(line); ok {
				flushHunk()
				currentHunk = &nextHunk
				continue
			}
			if line == `\ No newline at end of file` {
				continue
			}
			if parsedLine, ok := parseTurnDiffHunkLine(line); ok {
				currentHunk.Lines = append(currentHunk.Lines, parsedLine)
				continue
			}
			flushHunk()
		}

		if hunk, ok := parseTurnDiffHunkHeader(line); ok {
			currentHunk = &hunk
			continue
		}
		current.HeaderLines = append(current.HeaderLines, line)
		applyTurnDiffHeaderLine(current, line)
	}
	flushFile()

	if len(files) == 0 && strings.TrimSpace(diff) != "" {
		return []turnDiffParsedFile{newTurnDiffFallbackFile(diff)}
	}
	return files
}

func newTurnDiffParsedFile(index int, diffLine string) turnDiffParsedFile {
	file := turnDiffParsedFile{
		Index:    index,
		RawLines: []string{diffLine},
	}
	if oldPath, newPath, ok := parseTurnDiffGitHeaderPaths(diffLine); ok {
		file.OldPath = oldPath
		file.NewPath = newPath
	}
	return file
}

func newTurnDiffFallbackFile(diff string) turnDiffParsedFile {
	lines := splitTurnDiffRawLines(diff)
	return turnDiffParsedFile{
		RawLines:    append([]string(nil), lines...),
		HeaderLines: append([]string(nil), lines...),
		ChangeKind:  "raw_diff",
	}
}

func parseTurnDiffGitHeaderPaths(line string) (string, string, bool) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "diff --git "))
	if rest == "" {
		return "", "", false
	}
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return normalizeTurnDiffMarkerPath(parts[0]), normalizeTurnDiffMarkerPath(parts[1]), true
}

func applyTurnDiffHeaderLine(file *turnDiffParsedFile, line string) {
	if file == nil {
		return
	}
	switch {
	case strings.HasPrefix(line, "rename from "):
		file.ChangeKind = "rename"
		file.OldPath = strings.TrimSpace(strings.TrimPrefix(line, "rename from "))
	case strings.HasPrefix(line, "rename to "):
		file.ChangeKind = "rename"
		file.NewPath = strings.TrimSpace(strings.TrimPrefix(line, "rename to "))
	case strings.HasPrefix(line, "copy from "):
		file.ChangeKind = "copy"
		file.OldPath = strings.TrimSpace(strings.TrimPrefix(line, "copy from "))
	case strings.HasPrefix(line, "copy to "):
		file.ChangeKind = "copy"
		file.NewPath = strings.TrimSpace(strings.TrimPrefix(line, "copy to "))
	case strings.HasPrefix(line, "new file mode "):
		file.ChangeKind = "add"
	case strings.HasPrefix(line, "deleted file mode "):
		file.ChangeKind = "delete"
	case strings.HasPrefix(line, "Binary files "):
		file.Binary = true
	case strings.HasPrefix(line, "index "):
		oldBlobID, newBlobID, ok := parseTurnDiffIndexLine(line)
		if ok {
			file.OldBlobID = oldBlobID
			file.NewBlobID = newBlobID
		}
	case strings.HasPrefix(line, "--- "):
		file.OldPath = parseTurnDiffPathMarkerLine(line)
	case strings.HasPrefix(line, "+++ "):
		file.NewPath = parseTurnDiffPathMarkerLine(line)
	}
}

func parseTurnDiffIndexLine(line string) (string, string, bool) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "index "))
	if rest == "" {
		return "", "", false
	}
	firstField := strings.Fields(rest)
	if len(firstField) == 0 {
		return "", "", false
	}
	parts := strings.SplitN(firstField[0], "..", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	oldBlobID, oldOK := normalizeTurnDiffBlobID(parts[0])
	newBlobID, newOK := normalizeTurnDiffBlobID(parts[1])
	if !oldOK || !newOK {
		return "", "", false
	}
	return oldBlobID, newBlobID, true
}

func normalizeTurnDiffBlobID(value string) (string, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", false
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return "", false
		}
	}
	return value, true
}

func normalizeTurnDiffParsedFile(file *turnDiffParsedFile) {
	if file == nil {
		return
	}
	file.OldPath = normalizeTurnDiffPath(file.OldPath)
	file.NewPath = normalizeTurnDiffPath(file.NewPath)
	switch {
	case file.ChangeKind != "":
	case file.OldPath == "" && file.NewPath != "":
		file.ChangeKind = "add"
	case file.OldPath != "" && file.NewPath == "":
		file.ChangeKind = "delete"
	case file.OldPath != "" && file.NewPath != "" && file.OldPath != file.NewPath:
		file.ChangeKind = "rename"
	default:
		file.ChangeKind = "modify"
	}
}

func parseTurnDiffPathMarkerLine(line string) string {
	value := strings.TrimSpace(line[4:])
	if idx := strings.IndexByte(value, '\t'); idx >= 0 {
		value = value[:idx]
	}
	return normalizeTurnDiffMarkerPath(value)
}

func normalizeTurnDiffMarkerPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/dev/null" {
		return ""
	}
	path = strings.Trim(path, `"`)
	switch {
	case strings.HasPrefix(path, "a/"), strings.HasPrefix(path, "b/"):
		return path[2:]
	default:
		return path
	}
}

func normalizeTurnDiffPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/dev/null" {
		return ""
	}
	return strings.Trim(path, `"`)
}

func parseTurnDiffHunkHeader(line string) (turnDiffParsedHunk, bool) {
	if !strings.HasPrefix(line, "@@ ") && !strings.HasPrefix(line, "@@-") && !strings.HasPrefix(line, "@@ -") {
		return turnDiffParsedHunk{}, false
	}
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "@@") {
		return turnDiffParsedHunk{}, false
	}
	end := strings.Index(trimmed[2:], "@@")
	if end < 0 {
		return turnDiffParsedHunk{}, false
	}
	rangeText := strings.TrimSpace(trimmed[2 : end+2])
	parts := strings.Fields(rangeText)
	if len(parts) < 2 {
		return turnDiffParsedHunk{}, false
	}
	oldStart, oldLines, ok := parseTurnDiffRangeToken(parts[0], '-')
	if !ok {
		return turnDiffParsedHunk{}, false
	}
	newStart, newLines, ok := parseTurnDiffRangeToken(parts[1], '+')
	if !ok {
		return turnDiffParsedHunk{}, false
	}
	return turnDiffParsedHunk{
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
		RawLine:  line,
	}, true
}

func parseTurnDiffRangeToken(token string, prefix byte) (int, int, bool) {
	token = strings.TrimSpace(token)
	if token == "" || token[0] != prefix {
		return 0, 0, false
	}
	token = token[1:]
	parts := strings.SplitN(token, ",", 2)
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	lines := 1
	if len(parts) == 2 {
		lines, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, false
		}
	}
	return start, lines, true
}

func parseTurnDiffHunkLine(line string) (turnDiffParsedLine, bool) {
	if line == "" {
		return turnDiffParsedLine{}, false
	}
	switch line[0] {
	case ' ', '+', '-':
		return turnDiffParsedLine{
			Kind: line[0],
			Text: line[1:],
		}, true
	default:
		return turnDiffParsedLine{}, false
	}
}

func splitTurnDiffRawLines(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.SplitAfter(text, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		line := strings.TrimSuffix(part, "\n")
		line = strings.TrimSuffix(line, "\r")
		lines = append(lines, line)
	}
	return lines
}

func splitTurnDiffTextLines(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.SplitAfter(text, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		line := strings.TrimSuffix(part, "\n")
		line = strings.TrimSuffix(line, "\r")
		lines = append(lines, line)
	}
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func buildTurnDiffMergedView(file turnDiffParsedFile, anchorText string, anchorIsAfter bool) ([]turnDiffPreviewLine, []turnDiffPreviewHunk, error) {
	if len(file.Hunks) == 0 {
		return nil, nil, fmt.Errorf("turn diff file has no hunks")
	}
	anchorLines := splitTurnDiffTextLines(anchorText)
	lines := make([]turnDiffPreviewLine, 0, len(anchorLines)+32)
	hunks := make([]turnDiffPreviewHunk, 0, len(file.Hunks))
	oldCursor := 1
	newCursor := 1

	appendContextGap := func(targetCursor int) error {
		if anchorIsAfter {
			if targetCursor < newCursor {
				return fmt.Errorf("turn diff new cursor regressed")
			}
			if targetCursor-1 > len(anchorLines) {
				return fmt.Errorf("turn diff new cursor overflow")
			}
			for _, line := range anchorLines[newCursor-1 : targetCursor-1] {
				lines = append(lines, turnDiffPreviewLine{
					Kind: "context",
					Old:  turnDiffDisplayLineNumber(oldCursor),
					Now:  turnDiffDisplayLineNumber(newCursor),
					Text: line,
				})
				oldCursor++
				newCursor++
			}
			return nil
		}
		if targetCursor < oldCursor {
			return fmt.Errorf("turn diff old cursor regressed")
		}
		if targetCursor-1 > len(anchorLines) {
			return fmt.Errorf("turn diff old cursor overflow")
		}
		for _, line := range anchorLines[oldCursor-1 : targetCursor-1] {
			lines = append(lines, turnDiffPreviewLine{
				Kind: "context",
				Old:  turnDiffDisplayLineNumber(oldCursor),
				Now:  turnDiffDisplayLineNumber(newCursor),
				Text: line,
			})
			oldCursor++
			newCursor++
		}
		return nil
	}

	for _, hunk := range file.Hunks {
		targetCursor := hunk.NewStart
		if !anchorIsAfter {
			targetCursor = hunk.OldStart
		}
		if err := appendContextGap(targetCursor); err != nil {
			return nil, nil, err
		}

		hunkStartIndex := -1
		hunkEndIndex := -1
		added := 0
		removed := 0
		for _, patchLine := range hunk.Lines {
			switch patchLine.Kind {
			case ' ':
				lines = append(lines, turnDiffPreviewLine{
					Kind: "context",
					Old:  turnDiffDisplayLineNumber(oldCursor),
					Now:  turnDiffDisplayLineNumber(newCursor),
					Text: patchLine.Text,
				})
				oldCursor++
				newCursor++
			case '+':
				lines = append(lines, turnDiffPreviewLine{
					Kind: "add",
					Old:  "",
					Now:  turnDiffDisplayLineNumber(newCursor),
					Text: patchLine.Text,
				})
				if hunkStartIndex < 0 {
					hunkStartIndex = len(lines) - 1
				}
				hunkEndIndex = len(lines) - 1
				added++
				newCursor++
			case '-':
				lines = append(lines, turnDiffPreviewLine{
					Kind: "remove",
					Old:  turnDiffDisplayLineNumber(oldCursor),
					Now:  "",
					Text: patchLine.Text,
				})
				if hunkStartIndex < 0 {
					hunkStartIndex = len(lines) - 1
				}
				hunkEndIndex = len(lines) - 1
				removed++
				oldCursor++
			}
		}
		if hunkStartIndex < 0 {
			hunkStartIndex = maxTurnDiffInt(len(lines)-1, 0)
			hunkEndIndex = hunkStartIndex
		}
		hunks = append(hunks, turnDiffPreviewHunk{
			Title:    formatTurnDiffHunkTitle(hunk),
			Subtitle: formatTurnDiffHunkSubtitle(added, removed),
			Start:    hunkStartIndex,
			End:      hunkEndIndex,
		})
	}

	if anchorIsAfter {
		if err := appendContextGap(len(anchorLines) + 1); err != nil {
			return nil, nil, err
		}
	} else {
		if err := appendContextGap(len(anchorLines) + 1); err != nil {
			return nil, nil, err
		}
	}
	return lines, hunks, nil
}

func buildTurnDiffPatchOnlyView(file turnDiffParsedFile) ([]turnDiffPreviewLine, []turnDiffPreviewHunk) {
	if len(file.Hunks) == 0 {
		lines := make([]turnDiffPreviewLine, 0, len(file.RawLines))
		for _, raw := range file.RawLines {
			lines = append(lines, turnDiffPreviewLine{
				Kind: "context",
				Old:  "",
				Now:  "",
				Text: raw,
			})
		}
		if len(lines) == 0 {
			return nil, nil
		}
		return lines, []turnDiffPreviewHunk{{
			Title:    fallbackTurnDiffHunkTitle(file),
			Subtitle: formatTurnDiffHunkSubtitleFromStats(turnDiffFileStats(file)),
			Start:    0,
			End:      len(lines) - 1,
		}}
	}

	lines := make([]turnDiffPreviewLine, 0, 64)
	hunks := make([]turnDiffPreviewHunk, 0, len(file.Hunks))
	for _, hunk := range file.Hunks {
		oldLine := hunk.OldStart
		newLine := hunk.NewStart
		hunkStartIndex := len(lines)
		added := 0
		removed := 0
		for _, patchLine := range hunk.Lines {
			switch patchLine.Kind {
			case ' ':
				lines = append(lines, turnDiffPreviewLine{
					Kind: "context",
					Old:  turnDiffDisplayLineNumber(oldLine),
					Now:  turnDiffDisplayLineNumber(newLine),
					Text: patchLine.Text,
				})
				oldLine++
				newLine++
			case '+':
				lines = append(lines, turnDiffPreviewLine{
					Kind: "add",
					Old:  "",
					Now:  turnDiffDisplayLineNumber(newLine),
					Text: patchLine.Text,
				})
				added++
				newLine++
			case '-':
				lines = append(lines, turnDiffPreviewLine{
					Kind: "remove",
					Old:  turnDiffDisplayLineNumber(oldLine),
					Now:  "",
					Text: patchLine.Text,
				})
				removed++
				oldLine++
			}
		}
		hunks = append(hunks, turnDiffPreviewHunk{
			Title:    formatTurnDiffHunkTitle(hunk),
			Subtitle: formatTurnDiffHunkSubtitle(added, removed),
			Start:    hunkStartIndex,
			End:      len(lines) - 1,
		})
	}
	return lines, hunks
}

func turnDiffFileStats(file turnDiffParsedFile) turnDiffPreviewStats {
	stats := turnDiffPreviewStats{}
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			switch line.Kind {
			case '+':
				stats.Added++
			case '-':
				stats.Removed++
			}
		}
	}
	return stats
}

func formatTurnDiffHunkTitle(hunk turnDiffParsedHunk) string {
	start := hunk.NewStart
	count := hunk.NewLines
	if count <= 0 {
		start = hunk.OldStart
		count = hunk.OldLines
	}
	if count <= 0 {
		count = 1
	}
	end := start + count - 1
	if start <= 0 {
		start = 1
	}
	if end < start {
		end = start
	}
	if start == end {
		return "第 " + strconv.Itoa(start) + " 行"
	}
	return "第 " + strconv.Itoa(start) + "-" + strconv.Itoa(end) + " 行"
}

func formatTurnDiffHunkSubtitle(added, removed int) string {
	return fmt.Sprintf("+%d / -%d", added, removed)
}

func formatTurnDiffHunkSubtitleFromStats(stats turnDiffPreviewStats) string {
	return formatTurnDiffHunkSubtitle(stats.Added, stats.Removed)
}

func fallbackTurnDiffHunkTitle(file turnDiffParsedFile) string {
	name := turnDiffDisplayName(file)
	if name == "" {
		return "变更"
	}
	return name
}

func turnDiffDisplayName(file turnDiffParsedFile) string {
	switch {
	case strings.TrimSpace(file.NewPath) != "":
		return strings.TrimSpace(file.NewPath)
	case strings.TrimSpace(file.OldPath) != "":
		return strings.TrimSpace(file.OldPath)
	default:
		return "变更"
	}
}

func turnDiffDisplayLineNumber(value int) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func joinTurnDiffLinesText(lines []turnDiffPreviewLine, includeKinds ...string) string {
	if len(lines) == 0 {
		return ""
	}
	allowed := map[string]bool{}
	for _, kind := range includeKinds {
		allowed[strings.TrimSpace(kind)] = true
	}
	var parts []string
	for _, line := range lines {
		if len(allowed) != 0 && !allowed[line.Kind] {
			continue
		}
		parts = append(parts, line.Text)
	}
	return strings.Join(parts, "\n")
}

func maxTurnDiffInt(value, fallback int) int {
	if value > fallback {
		return value
	}
	return fallback
}
