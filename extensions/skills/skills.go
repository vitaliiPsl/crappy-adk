package skills

import (
	"context"
	"fmt"
	"strings"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/x/tool"
)

const (
	readName        = "read_skill"
	readDescription = "Load the full instructions for a skill. Use this when you identify a task that matches a skill in the catalog."

	readReferenceName        = "read_skill_reference"
	readReferenceDescription = "Load a reference document from a skill. Use this to access supporting documents related to a skill."

	instructionTemplate = `# Skills

You have access to specialized skills. Use read_skill to load full instructions when a task matches a skill. Use read_skill_reference to access supporting documents.

Available skills:
%s`
)

// Skill is a named, loadable set of instructions the agent can use on demand.
type Skill struct {
	// Unique name of the skill.
	Name string
	// Short summary shown to the agent in the skill catalog.
	Description string
	// Full instructions loaded when the agent picks this skill.
	Content string
	// Names of reference documents available alongside this skill.
	References []string
	// TODO: Add scripts and assets
}

// Store is the source of skills for the agent.
type Store interface {
	// List returns all available skills.
	List(ctx context.Context) ([]Skill, error)
	// Get returns a skill by name.
	Get(ctx context.Context, name string) (*Skill, error)
	// GetReference returns the content of a reference document belonging to a skill.
	GetReference(ctx context.Context, skill string, reference string) (string, error)
}

// ReadArgs are the arguments for the read_skill tool.
type ReadArgs struct {
	Skill string `json:"skill" jsonschema:"Name of the skill to load"`
}

// RefArgs are the arguments for the read_skill_reference tool.
type RefArgs struct {
	Skill     string `json:"skill" jsonschema:"Name of the skill"`
	Reference string `json:"reference" jsonschema:"Name of the reference document"`
}

// WithSkills returns a set of [agent.Option] values that give the agent access to a skill catalog.
// It registers two tools — read_skill and read_skill_reference — and injects a system prompt
// listing all available skills so the agent knows when to use them.
// Use with [agent.WithExtension] to apply all options at once.
func WithSkills(store Store) []agent.Option {
	readSkill := tool.MustFunction(
		readName,
		readDescription,
		func(ctx context.Context, args ReadArgs) (string, error) {
			s, err := store.Get(ctx, args.Skill)
			if err != nil {
				return "", err
			}

			return s.Content, nil
		},
	)

	readRef := tool.MustFunction(
		readReferenceName,
		readReferenceDescription,
		func(ctx context.Context, args RefArgs) (string, error) {
			return store.GetReference(ctx, args.Skill, args.Reference)
		},
	)

	return []agent.Option{
		agent.WithTools(readSkill, readRef),
		agent.WithInstructions(func(ctx context.Context) (string, error) {
			skills, err := store.List(ctx)
			if err != nil {
				return "", err
			}

			var catalog strings.Builder
			for _, s := range skills {
				fmt.Fprintf(&catalog, "- %s: %s\n", s.Name, s.Description)
			}

			return fmt.Sprintf(instructionTemplate, catalog.String()), nil
		}),
	}
}
