package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type CodeupProviderConfig struct {
	Token          string
	Domain         string
	OrganizationID string
}

type CodeupProvider struct {
	token          string
	domain         string
	organizationID string
	httpClient     *http.Client
}

func NewCodeupProvider(cfg CodeupProviderConfig) *CodeupProvider {
	return &CodeupProvider{
		token:          strings.TrimSpace(cfg.Token),
		domain:         normalizeCodeupDomain(cfg.Domain),
		organizationID: strings.TrimSpace(cfg.OrganizationID),
	}
}

func (p *CodeupProvider) Kind() string { return "codeup" }

func (p *CodeupProvider) Detect(_ context.Context, originURL string) (ChangeRequestRepo, bool, error) {
	remote, err := parseCodeupRemote(originURL)
	if err != nil {
		return ChangeRequestRepo{}, false, nil
	}
	if !p.matchesHost(remote.Host) {
		return ChangeRequestRepo{}, false, nil
	}
	return ChangeRequestRepo{
		Kind:      p.Kind(),
		Host:      remote.Host,
		Namespace: remote.Namespace,
		Name:      remote.Repo,
	}, true, nil
}

func (p *CodeupProvider) EnsureOpen(ctx context.Context, repo ChangeRequestRepo, input EnsureOpenInput) (ChangeRequest, bool, error) {
	if strings.TrimSpace(p.token) == "" {
		return ChangeRequest{}, false, errors.New("codeup provider token is required")
	}

	reqInfo, err := p.resolveRequest(repo, input.Extra)
	if err != nil {
		return ChangeRequest{}, false, err
	}
	head := strings.TrimSpace(input.Head)
	base := strings.TrimSpace(input.Base)
	title := strings.TrimSpace(input.Title)
	if head == "" || base == "" || title == "" {
		return ChangeRequest{}, false, errors.New("codeup ensure open requires title/head/base")
	}

	existing, found, err := p.findOpenChangeRequest(ctx, reqInfo, head, base, title)
	if err != nil {
		return ChangeRequest{}, false, err
	}
	if found {
		return existing, false, nil
	}

	payload := map[string]any{
		"title":           title,
		"description":     strings.TrimSpace(input.Body),
		"sourceBranch":    head,
		"targetBranch":    base,
		"sourceProjectId": reqInfo.SourceProjectID,
		"targetProjectId": reqInfo.TargetProjectID,
	}
	if reviewers := stringSliceFromAny(input.Extra["reviewer_user_ids"]); len(reviewers) > 0 {
		payload["reviewerUserIds"] = reviewers
	}
	if trigger, ok := boolFromAny(input.Extra["trigger_ai_review_run"]); ok {
		payload["triggerAIReviewRun"] = trigger
	}
	if workItems := strings.TrimSpace(stringFromAny(input.Extra["work_item_ids"])); workItems != "" {
		payload["workItemIds"] = workItems
	}

	var created codeupChangeRequest
	if err := p.doJSON(ctx, http.MethodPost, reqInfo.createURL(), payload, &created); err != nil {
		return ChangeRequest{}, false, fmt.Errorf("codeup create change request failed: %w", err)
	}
	cr := created.toChangeRequest()
	if cr.Number <= 0 {
		return ChangeRequest{}, false, errors.New("codeup create change request returned empty localId")
	}
	cr.Metadata = reqInfo.metadata()
	return cr, true, nil
}

func (p *CodeupProvider) Merge(ctx context.Context, repo ChangeRequestRepo, number int, input MergeInput) error {
	if strings.TrimSpace(p.token) == "" {
		return errors.New("codeup provider token is required")
	}
	if number <= 0 {
		return errors.New("codeup merge requires a positive change request number")
	}

	reqInfo, err := p.resolveRequest(repo, input.Extra)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"mergeType":          codeupMergeMethod(input.Method),
		"removeSourceBranch": false,
	}
	if removeSourceBranch, ok := boolFromAny(input.Extra["remove_source_branch"]); ok {
		payload["removeSourceBranch"] = removeSourceBranch
	}
	mergeMessage := strings.TrimSpace(input.CommitMessage)
	if mergeMessage == "" {
		mergeMessage = strings.TrimSpace(input.CommitTitle)
	}
	if mergeMessage != "" {
		payload["mergeMessage"] = mergeMessage
	}

	err = p.doJSON(ctx, http.MethodPost, reqInfo.mergeURL(number), payload, nil)
	if err == nil {
		return nil
	}

	current, getErr := p.getChangeRequest(ctx, reqInfo, number)
	if getErr == nil && current.isMerged() {
		return nil
	}

	mergeErr := &MergeError{
		Provider: p.Kind(),
		Repo:     repo,
		Number:   number,
		Message:  err.Error(),
	}
	if current != nil {
		mergeErr.URL = current.webURL()
		mergeErr.MergeableState = strings.TrimSpace(current.Status)
		mergeErr.AlreadyMerged = current.isMerged()
	}
	return mergeErr
}

type codeupRequestInfo struct {
	BaseURL         string
	OrganizationID  string
	RepositoryID    string
	SourceProjectID int64
	TargetProjectID int64
}

func (i codeupRequestInfo) createURL() string {
	return strings.TrimRight(i.BaseURL, "/") + fmt.Sprintf("/oapi/v1/codeup/organizations/%s/repositories/%s/changeRequests",
		url.PathEscape(i.OrganizationID), url.PathEscape(i.RepositoryID))
}

func (i codeupRequestInfo) mergeURL(number int) string {
	return strings.TrimRight(i.BaseURL, "/") + fmt.Sprintf("/oapi/v1/codeup/organizations/%s/repositories/%s/changeRequests/%d/merge",
		url.PathEscape(i.OrganizationID), url.PathEscape(i.RepositoryID), number)
}

func (i codeupRequestInfo) getURL(number int) string {
	return strings.TrimRight(i.BaseURL, "/") + fmt.Sprintf("/oapi/v1/codeup/organizations/%s/repositories/%s/changeRequests/%d",
		url.PathEscape(i.OrganizationID), url.PathEscape(i.RepositoryID), number)
}

func (i codeupRequestInfo) listURL() string {
	u := strings.TrimRight(i.BaseURL, "/") + fmt.Sprintf("/oapi/v1/codeup/organizations/%s/changeRequests",
		url.PathEscape(i.OrganizationID))
	values := url.Values{}
	if i.TargetProjectID > 0 {
		values.Set("projectIds", strconv.FormatInt(i.TargetProjectID, 10))
	} else if i.SourceProjectID > 0 {
		values.Set("projectIds", strconv.FormatInt(i.SourceProjectID, 10))
	}
	values.Set("state", "opened")
	values.Set("page", "1")
	values.Set("perPage", "100")
	return u + "?" + values.Encode()
}

func (i codeupRequestInfo) metadata() map[string]any {
	out := map[string]any{
		"provider":          "codeup",
		"organization_id":   i.OrganizationID,
		"repository_id":     i.RepositoryID,
		"source_project_id": i.SourceProjectID,
		"target_project_id": i.TargetProjectID,
	}
	return out
}

type codeupChangeRequest struct {
	LocalID         int64  `json:"localId"`
	Title           string `json:"title"`
	Status          string `json:"status"`
	WebURL          string `json:"webUrl"`
	DetailURL       string `json:"detailUrl"`
	SourceBranch    string `json:"sourceBranch"`
	TargetBranch    string `json:"targetBranch"`
	SourceProjectID int64  `json:"sourceProjectId"`
	TargetProjectID int64  `json:"targetProjectId"`
	SHA             string `json:"sha"`
}

func (c codeupChangeRequest) webURL() string {
	if v := strings.TrimSpace(c.WebURL); v != "" {
		return v
	}
	return strings.TrimSpace(c.DetailURL)
}

func (c codeupChangeRequest) isMerged() bool {
	state := strings.ToLower(strings.TrimSpace(c.Status))
	return state == "merged" || state == "merge_success"
}

func (c codeupChangeRequest) toChangeRequest() ChangeRequest {
	return ChangeRequest{
		Number:  int(c.LocalID),
		URL:     c.webURL(),
		HeadSHA: strings.TrimSpace(c.SHA),
		Metadata: map[string]any{
			"provider": "codeup",
		},
	}
}

func (p *CodeupProvider) resolveRequest(repo ChangeRequestRepo, extra map[string]any) (codeupRequestInfo, error) {
	if strings.TrimSpace(repo.Host) == "" {
		return codeupRequestInfo{}, errors.New("codeup repo host is required")
	}
	orgID := strings.TrimSpace(stringFromAny(extra["organization_id"]))
	if orgID == "" {
		orgID = codeupOrganizationFromNamespace(repo.Namespace)
	}
	if orgID == "" {
		orgID = p.organizationID
	}
	if orgID == "" {
		return codeupRequestInfo{}, errors.New("codeup organization_id is required (step config or AI_WORKFLOW_CODEUP_ORGANIZATION_ID)")
	}

	repositoryID := strings.TrimSpace(stringFromAny(extra["repository_id"]))
	if repositoryID == "" {
		repositoryID = codeupRepositoryPathFromRepo(repo)
	}
	if repositoryID == "" {
		return codeupRequestInfo{}, errors.New("codeup repository_id is required")
	}

	projectID, _ := toInt64(extra["project_id"])
	sourceProjectID, ok := toInt64(extra["source_project_id"])
	if !ok || sourceProjectID <= 0 {
		sourceProjectID = projectID
	}
	targetProjectID, ok := toInt64(extra["target_project_id"])
	if !ok || targetProjectID <= 0 {
		targetProjectID = projectID
	}

	baseURL := "https://" + repo.Host
	if p.domain != "" {
		baseURL = p.domain
	}

	return codeupRequestInfo{
		BaseURL:         strings.TrimRight(baseURL, "/"),
		OrganizationID:  orgID,
		RepositoryID:    repositoryID,
		SourceProjectID: sourceProjectID,
		TargetProjectID: targetProjectID,
	}, nil
}

func (p *CodeupProvider) findOpenChangeRequest(ctx context.Context, reqInfo codeupRequestInfo, head string, base string, title string) (ChangeRequest, bool, error) {
	if reqInfo.SourceProjectID <= 0 && reqInfo.TargetProjectID <= 0 {
		return ChangeRequest{}, false, nil
	}
	var list []codeupChangeRequest
	if err := p.doJSON(ctx, http.MethodGet, reqInfo.listURL(), nil, &list); err != nil {
		return ChangeRequest{}, false, nil
	}
	for _, item := range list {
		if strings.TrimSpace(item.SourceBranch) != head {
			continue
		}
		if strings.TrimSpace(item.TargetBranch) != base {
			continue
		}
		if item.SourceProjectID > 0 && reqInfo.SourceProjectID > 0 && item.SourceProjectID != reqInfo.SourceProjectID {
			continue
		}
		if item.TargetProjectID > 0 && reqInfo.TargetProjectID > 0 && item.TargetProjectID != reqInfo.TargetProjectID {
			continue
		}
		if strings.TrimSpace(item.Title) != "" && strings.TrimSpace(title) != "" && strings.TrimSpace(item.Title) != strings.TrimSpace(title) {
			continue
		}
		cr := item.toChangeRequest()
		cr.Metadata = reqInfo.metadata()
		return cr, true, nil
	}
	return ChangeRequest{}, false, nil
}

func (p *CodeupProvider) getChangeRequest(ctx context.Context, reqInfo codeupRequestInfo, number int) (*codeupChangeRequest, error) {
	var out codeupChangeRequest
	if err := p.doJSON(ctx, http.MethodGet, reqInfo.getURL(number), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *CodeupProvider) doJSON(ctx context.Context, method string, rawURL string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("x-yunxiao-token", p.token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("status %d: %s", resp.StatusCode, msg)
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (p *CodeupProvider) client() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return http.DefaultClient
}

func (p *CodeupProvider) matchesHost(host string) bool {
	normalized := strings.ToLower(strings.TrimSpace(host))
	if normalized == "" {
		return false
	}
	if p.domain != "" {
		u, err := url.Parse(p.domain)
		if err == nil && strings.ToLower(strings.TrimSpace(u.Host)) == normalized {
			return true
		}
	}
	return strings.Contains(normalized, "rdc.aliyuncs.com") || strings.Contains(normalized, "codeup.aliyun.com")
}

type codeupRemote struct {
	Host      string
	Namespace string
	Repo      string
}

func parseCodeupRemote(raw string) (codeupRemote, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return codeupRemote{}, errors.New("remote url is required")
	}

	if strings.Contains(trimmed, "://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return codeupRemote{}, err
		}
		return buildCodeupRemote(strings.ToLower(strings.TrimSpace(u.Host)), u.Path)
	}

	atIndex := strings.Index(trimmed, "@")
	colonIndex := strings.LastIndex(trimmed, ":")
	if atIndex <= 0 || colonIndex <= atIndex+1 || colonIndex >= len(trimmed)-1 {
		return codeupRemote{}, errors.New("unsupported remote url")
	}
	host := strings.ToLower(strings.TrimSpace(trimmed[atIndex+1 : colonIndex]))
	return buildCodeupRemote(host, trimmed[colonIndex+1:])
}

func buildCodeupRemote(host string, rawPath string) (codeupRemote, error) {
	cleanPath := strings.Trim(strings.TrimSpace(rawPath), "/")
	cleanPath = strings.TrimSuffix(cleanPath, ".git")
	if cleanPath == "" {
		return codeupRemote{}, errors.New("remote repository path is empty")
	}
	segments := strings.Split(cleanPath, "/")
	if len(segments) < 2 {
		return codeupRemote{}, errors.New("remote repository path must include namespace/repo")
	}
	repo := strings.TrimSpace(segments[len(segments)-1])
	namespace := strings.Trim(strings.Join(segments[:len(segments)-1], "/"), "/")
	if repo == "" || namespace == "" {
		return codeupRemote{}, errors.New("remote repository path must include namespace/repo")
	}
	return codeupRemote{
		Host:      strings.TrimSpace(host),
		Namespace: path.Clean(namespace),
		Repo:      repo,
	}, nil
}

func codeupOrganizationFromNamespace(namespace string) string {
	trimmed := strings.Trim(strings.TrimSpace(namespace), "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func codeupRepositoryPathFromRepo(repo ChangeRequestRepo) string {
	namespace := strings.Trim(strings.TrimSpace(repo.Namespace), "/")
	name := strings.TrimSpace(repo.Name)
	if namespace == "" {
		return name
	}
	if name == "" {
		return namespace
	}
	return namespace + "/" + name
}

func normalizeCodeupDomain(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "://") {
		return strings.TrimRight(trimmed, "/")
	}
	return "https://" + strings.TrimRight(trimmed, "/")
}

func codeupMergeMethod(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "", "merge":
		return "no-fast-forward"
	case "squash":
		return "squash"
	case "rebase":
		return "rebase"
	case "ff-only", "no-fast-forward":
		return strings.ToLower(strings.TrimSpace(method))
	default:
		return "no-fast-forward"
	}
}

func stringFromAny(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	default:
		return ""
	}
}

func boolFromAny(v any) (bool, bool) {
	switch value := v.(type) {
	case bool:
		return value, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err == nil {
			return parsed, true
		}
	}
	return false, false
}

func stringSliceFromAny(v any) []string {
	switch value := v.(type) {
	case []string:
		return append([]string(nil), value...)
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if s := strings.TrimSpace(stringFromAny(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
