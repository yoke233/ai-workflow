package planning

// GeneratedStep is the planner output for a single step in a generated DAG.
type GeneratedStep struct {
	Name                 string   `json:"name"`
	Type                 string   `json:"type"`
	DependsOn            []string `json:"depends_on,omitempty"`
	AgentRole            string   `json:"agent_role,omitempty"`
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`
	AcceptanceCriteria   []string `json:"acceptance_criteria,omitempty"`
	Description          string   `json:"description,omitempty"`
}

// GeneratedDAG is the planner output for the full DAG.
type GeneratedDAG struct {
	Steps []GeneratedStep `json:"steps"`
}
