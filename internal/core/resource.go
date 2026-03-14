package core

import "time"

// ResourceKind constants.
const ResourceKindAttachment = "attachment"

// ResourceBinding links a Project to an external resource (git repo, local dir, S3, etc.)
// or a WorkItem to an attachment (kind=attachment).
type ResourceBinding struct {
	ID        int64          `json:"id"`
	ProjectID int64          `json:"project_id"`
	IssueID   *int64         `json:"issue_id,omitempty"`
	Kind      string         `json:"kind"` // "git" | "local_fs" | "s3" | "attachment" | ...
	URI       string         `json:"uri"`
	Config    map[string]any `json:"config,omitempty"`
	Label     string         `json:"label,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// NewAttachmentBinding creates a ResourceBinding that represents a work-item file attachment.
func NewAttachmentBinding(issueID int64, fileName, filePath, mimeType string, size int64) *ResourceBinding {
	return &ResourceBinding{
		IssueID: &issueID,
		Kind:    ResourceKindAttachment,
		URI:     filePath,
		Label:   fileName,
		Config: map[string]any{
			"mime_type": mimeType,
			"size":      size,
		},
	}
}

// AttachmentFileName returns the display name for an attachment binding.
func (rb *ResourceBinding) AttachmentFileName() string { return rb.Label }

// AttachmentFilePath returns the on-disk path for an attachment binding.
func (rb *ResourceBinding) AttachmentFilePath() string { return rb.URI }

// AttachmentMimeType extracts the MIME type from an attachment binding's Config.
func (rb *ResourceBinding) AttachmentMimeType() string {
	if rb.Config == nil {
		return ""
	}
	s, _ := rb.Config["mime_type"].(string)
	return s
}

// AttachmentSize extracts the file size from an attachment binding's Config.
func (rb *ResourceBinding) AttachmentSize() int64 {
	if rb.Config == nil {
		return 0
	}
	switch v := rb.Config["size"].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	}
	return 0
}
