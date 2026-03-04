package core

import "fmt"

// WorkflowProfileType represents built-in orchestration profiles in V2.
type WorkflowProfileType string

const (
	WorkflowProfileNormal      WorkflowProfileType = "normal"
	WorkflowProfileStrict      WorkflowProfileType = "strict"
	WorkflowProfileFastRelease WorkflowProfileType = "fast_release"
)

const (
	MinWorkflowProfileSLAMinutes = 1
	MaxWorkflowProfileSLAMinutes = 60
)

var validWorkflowProfileTypes = map[WorkflowProfileType]struct{}{
	WorkflowProfileNormal:      {},
	WorkflowProfileStrict:      {},
	WorkflowProfileFastRelease: {},
}

// Validate checks whether the profile type is one of supported values.
func (t WorkflowProfileType) Validate() error {
	if _, ok := validWorkflowProfileTypes[t]; !ok {
		return fmt.Errorf("invalid workflow profile type %q", t)
	}
	return nil
}

// WorkflowProfile defines orchestration behavior and SLA timeout policy.
type WorkflowProfile struct {
	Type       WorkflowProfileType `json:"type"`
	SLAMinutes int                 `json:"sla_minutes"`
}

// Validate checks whether the workflow profile contains valid V2 settings.
func (p WorkflowProfile) Validate() error {
	if err := p.Type.Validate(); err != nil {
		return err
	}
	if p.SLAMinutes < MinWorkflowProfileSLAMinutes || p.SLAMinutes > MaxWorkflowProfileSLAMinutes {
		return fmt.Errorf(
			"sla_minutes must be between %d and %d",
			MinWorkflowProfileSLAMinutes,
			MaxWorkflowProfileSLAMinutes,
		)
	}
	return nil
}
