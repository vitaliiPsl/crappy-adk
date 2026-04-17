package memory

import (
	"fmt"
	"slices"
	"strings"
)

const (
	instructionTitle    = "User Preferences"
	instructionPreamble = `These are stable user preferences from prior conversations.
Use them as defaults for how you respond and work.
If the current user request conflicts with one of these preferences, follow the current request.`
)

func selectForInjection(memories []Memory, maxMemories int) []Memory {
	if len(memories) == 0 || maxMemories == 0 {
		return nil
	}

	filtered := make([]Memory, 0, len(memories))
	for _, memory := range memories {
		memory.Title = strings.TrimSpace(memory.Title)

		memory.Content = strings.TrimSpace(memory.Content)
		if memory.Title == "" || memory.Content == "" {
			continue
		}

		filtered = append(filtered, memory)
	}

	slices.SortFunc(filtered, func(a, b Memory) int {
		switch {
		case a.UpdatedAt.After(b.UpdatedAt):
			return -1
		case a.UpdatedAt.Before(b.UpdatedAt):
			return 1
		default:
			return strings.Compare(a.ID, b.ID)
		}
	})

	if maxMemories > 0 && len(filtered) > maxMemories {
		filtered = filtered[:maxMemories]
	}

	return filtered
}

func renderInstruction(memories []Memory, maxMemories int) string {
	selected := selectForInjection(memories, maxMemories)
	if len(selected) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", instructionTitle)
	b.WriteString(instructionPreamble)
	b.WriteString("\n\n")

	for _, memory := range selected {
		fmt.Fprintf(&b, "- [%s] %s: %s\n", memory.ID, memory.Title, memory.Content)
	}

	return strings.TrimRight(b.String(), "\n")
}
