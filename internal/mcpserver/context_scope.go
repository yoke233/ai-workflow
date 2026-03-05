package mcpserver

import "strings"

// ContextScope defines the URI prefixes an agent role can read/write.
type ContextScope struct {
	ReadPrefixes  []string // viking:// URI prefixes allowed for reading
	WritePrefixes []string // viking:// URI prefixes allowed for writing
}

// ResolveContextScope returns the context access scope for a given role.
// projectID and issueID are substituted into URI templates.
func ResolveContextScope(role, projectID, issueID string) ContextScope {
	r := strings.NewReplacer("{pid}", projectID, "{iid}", issueID)

	sub := func(templates []string) []string {
		out := make([]string, len(templates))
		for i, t := range templates {
			out[i] = r.Replace(t)
		}
		return out
	}

	switch role {
	case "team_leader":
		return ContextScope{
			ReadPrefixes:  sub([]string{"viking://resources/{pid}/docs/", "viking://resources/shared/"}),
			WritePrefixes: sub([]string{"viking://resources/{pid}/specs/", "viking://resources/{pid}/docs/"}),
		}
	case "reviewer":
		return ContextScope{
			ReadPrefixes: sub([]string{"viking://resources/{pid}/specs/{iid}/", "viking://resources/{pid}/docs/"}),
		}
	case "decomposer":
		return ContextScope{
			ReadPrefixes:  sub([]string{"viking://resources/{pid}/specs/{iid}/", "viking://resources/{pid}/docs/"}),
			WritePrefixes: sub([]string{"viking://resources/{pid}/specs/"}),
		}
	case "worker":
		return ContextScope{
			ReadPrefixes: sub([]string{"viking://resources/{pid}/specs/{iid}/"}),
		}
	case "aggregator":
		return ContextScope{
			ReadPrefixes:  sub([]string{"viking://resources/{pid}/specs/", "viking://resources/{pid}/archive/"}),
			WritePrefixes: sub([]string{"viking://resources/{pid}/specs/", "viking://resources/{pid}/archive/"}),
		}
	default:
		return ContextScope{}
	}
}
