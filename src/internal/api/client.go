package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Tenant describes an OpenHVX tenant (identified by tenantId).
type Tenant struct {
	TenantID    string          `json:"tenantId,omitempty"`
	Name        string          `json:"name"`
	Status      string          `json:"status,omitempty"`
	Description string          `json:"description,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	Quotas      json.RawMessage `json:"quotas,omitempty"`
}

// UnmarshalJSON normalizes legacy tenant payloads.
func (t *Tenant) UnmarshalJSON(data []byte) error {
	type alias Tenant
	var aux struct {
		alias
		LegacyID    string `json:"id"`
		LegacyMongo string `json:"_id"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*t = Tenant(aux.alias)
	if t.TenantID == "" {
		legacy := strings.TrimSpace(aux.LegacyID)
		if legacy == "" {
			legacy = strings.TrimSpace(aux.LegacyMongo)
		}
		t.TenantID = legacy
	}
	return nil
}

// TenantCreateRequest configures tenant creation via the admin API.
type TenantCreateRequest struct {
	TenantID    string          `json:"tenantId,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Quotas      json.RawMessage `json:"quotas,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	Status      string          `json:"status,omitempty"`
}

// Resource describes an OpenHVX resource.
type Resource struct {
	ID         string              `json:"id"`
	RefID      string              `json:"refId"`
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Kind       string              `json:"kind"`
	State      string              `json:"state"`
	Power      string              `json:"powerState"`
	Status     string              `json:"status"`
	TenantID   string              `json:"tenant_id"`
	TenantID2  string              `json:"tenantId"`
	AgentID    string              `json:"agentId"`
	SizeMB     int                 `json:"sizeMB"`
	HighAvail  bool                `json:"ha"`
	StaleAgent bool                `json:"_staleAgent"`
	AttachedTo *ResourceAttachment `json:"attachedTo"`
}

// ResourceAttachment describes which VM a disk is attached to.
type ResourceAttachment struct {
	RefID      string `json:"refId"`
	AgentID    string `json:"agentId"`
	Name       string `json:"name"`
	AttachedAt string `json:"attachedAt"`
}

// Image describes a VM image.
type Image struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Status      string `json:"status"`
	Scope       string `json:"scope"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Gen         int    `json:"gen"`
	Path        string `json:"path"`
	SizeBytes   int64  `json:"sizeBytes"`
	TenantID    string `json:"tenantId"`
	MTime       string `json:"mtime"`
}

// VMResource describes a VM entry returned by the resource APIs.
type VMResource struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Kind             string  `json:"kind"`
	PowerState       string  `json:"powerState"`
	State            string  `json:"state"`
	CPUUsagePct      float64 `json:"cpuUsagePct"`
	MemoryMB         int     `json:"memoryMb"`
	MemoryAssignedMB int     `json:"memoryAssignedMB"`
	UptimeSec        int     `json:"uptimeSec"`
	TenantID         string  `json:"tenantId"`
	AgentID          string  `json:"agentId"`
	RefID            string  `json:"refId"`
	AutomaticStart   string  `json:"automaticStart"`
	AutomaticStop    string  `json:"automaticStop"`
	CPU              struct {
		VCPUs *int `json:"vcpus"`
	} `json:"cpu"`
}

// Quota describes usage versus limits for a given resource metric.
type Quota struct {
	Metric string  `json:"metric"`
	Name   string  `json:"name"`
	Limit  float64 `json:"limit"`
	Usage  float64 `json:"usage"`
	Unit   string  `json:"unit"`
	Status string  `json:"status"`
}

// TaskTarget describes the target entity for a task.
type TaskTarget struct {
	Kind    string `json:"kind"`
	AgentID string `json:"agentId,omitempty"`
	RefID   string `json:"refId,omitempty"`
}

// TaskMeta specifies task meta capabilities.
type TaskMeta struct {
	Capability string `json:"capability"`
}

// StorageCreateData contains the payload for disk creation.
type StorageCreateData struct {
	Name    string `json:"name"`
	ImageID string `json:"imageId"`
	SizeMB  int    `json:"sizeMB"`
}

// StorageDeleteData contains the payload for disk deletion.
type StorageDeleteData struct {
	RefID string `json:"refId"`
}

// VMCreateData contains the payload for VM creation.
type VMCreateData struct {
	Name       string          `json:"name"`
	DiskID     string          `json:"diskId"`
	CPU        int             `json:"cpu"`
	RAM        string          `json:"ram"`
	Generation int             `json:"generation"`
	HighAvail  bool            `json:"ha"`
	CloudInit  json.RawMessage `json:"cloudInit,omitempty"`
}

// VMDeleteData contains the payload for VM deletion.
type VMDeleteData struct {
	GUID      string `json:"guid"`
	ForceStop bool   `json:"forceStop"`
}

// TenantUserRequest describes the payload for creating a tenant user.
type TenantUserRequest struct {
	Email    string   `json:"email"`
	Password string   `json:"password"`
	TenantID string   `json:"tenantId"`
	Roles    []string `json:"roles"`
}

// LoginRequest represents credentials for PAT retrieval.
type LoginRequest struct {
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password"`
}

// LoginResponse captures the data returned by password login endpoints.
type LoginResponse struct {
	AccessToken string         `json:"access_token"`
	TokenType   string         `json:"token_type"`
	ExpiresIn   FlexibleInt    `json:"expires_in"`
	User        map[string]any `json:"user"`
}

// FlexibleInt parses JSON numbers that might be encoded as strings.
type FlexibleInt int

func (fi *FlexibleInt) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		*fi = 0
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if strings.TrimSpace(s) == "" {
			*fi = 0
			return nil
		}
		val, err := strconv.ParseFloat(s, 64)
		if err == nil {
			*fi = FlexibleInt(int(val))
			return nil
		}
		if dur, derr := time.ParseDuration(s); derr == nil {
			*fi = FlexibleInt(int(dur.Seconds()))
			return nil
		}
		return err
	}
	var num float64
	if err := json.Unmarshal(data, &num); err != nil {
		return err
	}
	*fi = FlexibleInt(int(num))
	return nil
}

// PATInfo describes a generated personal access token.
type PATInfo struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	CreatedAt  string `json:"createdAt"`
	ExpiresAt  string `json:"expiresAt"`
	LastUsedAt string `json:"lastUsedAt"`
}

// PATCreateRequest configures PAT creation.
type PATCreateRequest struct {
	Label         string `json:"label,omitempty"`
	ExpiresInDays int    `json:"expiresInDays,omitempty"`
}

// PATCreateResponse provides the token material plus metadata.
type PATCreateResponse struct {
	Token string   `json:"token"`
	PAT   *PATInfo `json:"pat"`
}

// Client is a minimal HTTP client that handles authentication and TLS settings.
type Client struct {
	BaseURL        string
	PAT            string
	HTTP           *http.Client
	IdempotencyKey string
	Context        context.Context
}

// NewClient instantiates a Client with optional insecure TLS configuration.
func NewClient(baseURL, pat string, insecure bool) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, // dev only
		}
	}

	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		PAT:     pat,
		HTTP: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
			// Do not forward credential-bearing operations through redirects.
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (c *Client) requestContext() context.Context {
	if c.Context != nil {
		return c.Context
	}
	return context.Background()
}

// requestURL keeps all requests on an explicitly configured HTTP(S) origin.
func (c *Client) requestURL(path string) (string, error) {
	base, err := url.Parse(c.BaseURL)
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return "", fmt.Errorf("server must be an HTTP(S) URL without embedded credentials, query or fragment")
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "\r\n") {
		return "", fmt.Errorf("invalid API path")
	}
	return strings.TrimRight(c.BaseURL, "/") + path, nil
}

func readResponse(body io.Reader) ([]byte, error) {
	const limit = 16 << 20
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("API response exceeds 16 MiB")
	}
	return data, nil
}

func (c *Client) doRequest(method, path string) (*http.Response, error) {
	requestURL, err := c.requestURL(path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(c.requestContext(), method, requestURL, nil)
	if err != nil {
		return nil, err
	}

	if c.PAT != "" {
		req.Header.Set("Authorization", "Bearer "+c.PAT)
	}
	req.Header.Set("Accept", "application/json")

	return c.HTTP.Do(req)
}

func (c *Client) fetch(path string) ([]byte, error) {
	resp, err := c.doRequest(http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := readResponse(resp.Body)
		return nil, fmt.Errorf("API error: %s: %s", resp.Status, string(b))
	}

	body, err := readResponse(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (c *Client) postJSON(path string, payload any) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return nil, err
	}

	requestURL, err := c.requestURL(path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(c.requestContext(), http.MethodPost, requestURL, &buf)
	if err != nil {
		return nil, err
	}

	if c.PAT != "" {
		req.Header.Set("Authorization", "Bearer "+c.PAT)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if strings.HasSuffix(path, "/tasks") {
		key := strings.TrimSpace(c.IdempotencyKey)
		if len(key) < 1 || len(key) > 200 || strings.ContainsAny(key, "\r\n") {
			return nil, fmt.Errorf("--idempotency-key required for task submission (1..200 characters)")
		}
		req.Header.Set("Idempotency-Key", key)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := readResponse(resp.Body)
		return nil, fmt.Errorf("API error: %s: %s", resp.Status, string(b))
	}

	return readResponse(resp.Body)
}

// Login exchanges credentials for a PAT/token at the given API path.
func (c *Client) Login(path string, creds LoginRequest) (*LoginResponse, error) {
	path = normalizePath(path)
	if path == "" {
		return nil, fmt.Errorf("login path is required")
	}
	username := strings.TrimSpace(creds.Username)
	email := strings.TrimSpace(creds.Email)
	if username == "" {
		username = email
	}
	if email == "" {
		email = username
	}
	if username == "" {
		return nil, fmt.Errorf("username or email is required")
	}
	if creds.Password == "" {
		return nil, fmt.Errorf("password is required")
	}
	payload := LoginRequest{
		Username: username,
		Email:    email,
		Password: creds.Password,
	}
	body, err := c.postJSON(path, payload)
	if err != nil {
		return nil, err
	}
	var resp LoginResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.AccessToken) == "" {
		return nil, fmt.Errorf("login response missing access_token")
	}
	return &resp, nil
}

// CreatePAT exchanges an access token for a PAT.
func (c *Client) CreatePAT(path string, req PATCreateRequest) (*PATCreateResponse, error) {
	path = normalizePath(path)
	if path == "" {
		return nil, fmt.Errorf("pat path is required")
	}
	body, err := c.postJSON(path, req)
	if err != nil {
		return nil, err
	}
	var resp PATCreateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.Token) == "" {
		return nil, fmt.Errorf("pat creation response missing token")
	}
	return &resp, nil
}

func listAndDecode[T any](fetch func(string) ([]byte, error), path string, wrapperKeys ...string) ([]T, error) {
	body, err := fetch(path)
	if err != nil {
		return nil, err
	}
	return decodeSlice[T](body, wrapperKeys...)
}

func (c *Client) listResources(path string) ([]Resource, error) {
	return listAndDecode[Resource](c.fetch, path, "data", "resources")
}

func (c *Client) listQuotas(path string) ([]Quota, error) {
	body, err := c.fetch(path)
	if err != nil {
		return nil, err
	}
	quotas, decodeErr := decodeSlice[Quota](body, "data", "quotas")
	if decodeErr == nil {
		return quotas, nil
	}
	if mapQuotas, mapErr := decodeQuotaMap(body); mapErr == nil {
		return mapQuotas, nil
	}
	return nil, decodeErr
}

func (c *Client) listImages(path string) ([]Image, error) {
	return listAndDecode[Image](c.fetch, path, "data", "images")
}

func (c *Client) getImage(prefix, imageID string) (*Image, error) {
	if imageID == "" {
		return nil, fmt.Errorf("imageID is required")
	}
	body, err := c.fetch(prefix + url.PathEscape(imageID))
	if err != nil {
		return nil, err
	}
	return decodeImageResponse(body)
}

func (c *Client) fetchImageRaw(prefix, imageID string) ([]byte, error) {
	if imageID == "" {
		return nil, fmt.Errorf("imageID is required")
	}
	return c.fetch(prefix + url.PathEscape(imageID))
}

func (c *Client) listVMs(path string) ([]VMResource, error) {
	all, err := listAndDecode[VMResource](c.fetch, path, "data", "resources")
	if err != nil {
		return nil, err
	}
	vms := make([]VMResource, 0, len(all))
	for _, vm := range all {
		if strings.EqualFold(vm.Kind, "vm") {
			vms = append(vms, vm)
		}
	}
	return vms, nil
}

// ListTenantVMs fetches VM resources for a tenant PAT.
func (c *Client) ListTenantVMs(tenantID string) ([]VMResource, error) {
	return c.listVMs(tenantPath("/api/v1/tenant/resources", tenantID))
}

// ListAdminVMs fetches VM resources for a specific tenant via the admin API.
func (c *Client) ListAdminVMs(tenantID string) ([]VMResource, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}
	path := "/api/v1/admin/tenants/" + url.PathEscape(tenantID) + "/resources"
	return c.listVMs(path)
}

// ListTenantResources fetches resources for a tenant PAT.
func (c *Client) ListTenantResources(tenantID string) ([]Resource, error) {
	return c.listResources(tenantPath("/api/v1/tenant/resources", tenantID))
}

// ListTenantResourcesRaw returns the API response body for tenant resources.
func (c *Client) ListTenantResourcesRaw(tenantID string) ([]byte, error) {
	return c.fetch(tenantPath("/api/v1/tenant/resources", tenantID))
}

// ListTenantDisks fetches storage resources (disks) for a tenant PAT.
func (c *Client) ListTenantDisks(tenantID string) ([]Resource, error) {
	return c.listResources(tenantPath("/api/v1/tenant/resources?kind=storage", tenantID))
}

// ListTenantDisksRaw returns the API response body for tenant disks.
func (c *Client) ListTenantDisksRaw(tenantID string) ([]byte, error) {
	return c.fetch(tenantPath("/api/v1/tenant/resources?kind=storage", tenantID))
}

// ListTenantQuotas fetches quota information for a tenant PAT.
func (c *Client) ListTenantQuotas(tenantID string) ([]Quota, error) {
	return c.listQuotas(tenantPath("/api/v1/tenant/quotas", tenantID))
}

// ListTenantQuotasRaw returns the API response body for tenant quotas.
func (c *Client) ListTenantQuotasRaw(tenantID string) ([]byte, error) {
	return c.fetch(tenantPath("/api/v1/tenant/quotas", tenantID))
}

// ListTenantImages fetches VM images accessible in tenant mode.
func (c *Client) ListTenantImages() ([]Image, error) {
	return c.listImages("/api/v1/tenant/images")
}

// ListTenantImagesRaw returns the raw response for tenant images.
func (c *Client) ListTenantImagesRaw() ([]byte, error) {
	return c.fetch("/api/v1/tenant/images")
}

// GetTenantImage fetches a single image in tenant mode.
func (c *Client) GetTenantImage(imageID string) (*Image, error) {
	return c.getImage("/api/v1/tenant/images/", imageID)
}

// GetTenantImageRaw returns the raw response for a single tenant image.
func (c *Client) GetTenantImageRaw(imageID string) ([]byte, error) {
	return c.fetchImageRaw("/api/v1/tenant/images/", imageID)
}

// ResolveTenantImage resolves a tenant image reference.
func (c *Client) ResolveTenantImage(imageID string) ([]byte, error) {
	return c.fetch("/api/v1/tenant/images/" + url.PathEscape(imageID) + "/resolve")
}

// ListAdminResources fetches resources for a specific tenant via the admin API.
func (c *Client) ListAdminResources(tenantID string) ([]Resource, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}
	path := "/api/v1/admin/tenants/" + url.PathEscape(tenantID) + "/resources"
	return c.listResources(path)
}

// ListAdminDisks fetches storage resources for a specific tenant via the admin API.
func (c *Client) ListAdminDisks(tenantID string) ([]Resource, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}
	path := "/api/v1/admin/tenants/" + url.PathEscape(tenantID) + "/resources?kind=storage"
	return c.listResources(path)
}

// ListAdminDisksRaw returns the API response body for admin disk listings.
func (c *Client) ListAdminDisksRaw(tenantID string) ([]byte, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}
	path := "/api/v1/admin/tenants/" + url.PathEscape(tenantID) + "/resources?kind=storage"
	return c.fetch(path)
}

// ListAdminImages fetches VM images from the admin API.
func (c *Client) ListAdminImages() ([]Image, error) {
	return c.listImages("/api/v1/admin/images")
}

// ListAdminImagesRaw returns the raw response for admin images.
func (c *Client) ListAdminImagesRaw() ([]byte, error) {
	return c.fetch("/api/v1/admin/images")
}

// GetAdminImage fetches a single image via the admin API.
func (c *Client) GetAdminImage(imageID string) (*Image, error) {
	return c.getImage("/api/v1/admin/images/", imageID)
}

// GetAdminImageRaw returns the raw response for a single admin image.
func (c *Client) GetAdminImageRaw(imageID string) ([]byte, error) {
	return c.fetchImageRaw("/api/v1/admin/images/", imageID)
}

// ResolveAdminImage resolves an image reference via the admin API.
func (c *Client) ResolveAdminImage(imageID string) ([]byte, error) {
	return c.fetch("/api/v1/admin/images/" + url.PathEscape(imageID) + "/resolve")
}

// ListAdminQuotas fetches quota information for a specific tenant via the admin API.
func (c *Client) ListAdminQuotas(tenantID string) ([]Quota, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}
	path := "/api/v1/admin/tenants/" + url.PathEscape(tenantID) + "/quotas"
	return c.listQuotas(path)
}

// ListAdminQuotasRaw returns the API response body for admin tenant quotas.
func (c *Client) ListAdminQuotasRaw(tenantID string) ([]byte, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}
	path := "/api/v1/admin/tenants/" + url.PathEscape(tenantID) + "/quotas"
	return c.fetch(path)
}

// ListAdminResourcesRaw returns the API response body for admin tenant resources.
func (c *Client) ListAdminResourcesRaw(tenantID string) ([]byte, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}
	path := "/api/v1/admin/tenants/" + url.PathEscape(tenantID) + "/resources"
	return c.fetch(path)
}

// RecalculateQuota triggers a quota recalculation for the given tenant.
func (c *Client) RecalculateQuota(tenantID string) error {
	if tenantID == "" {
		return fmt.Errorf("tenantID is required")
	}
	path := "/api/v1/admin/tenants/" + url.PathEscape(tenantID) + "/quotas/recalculate"
	resp, err := c.doRequest(http.MethodPost, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := readResponse(resp.Body)
		return fmt.Errorf("API error: %s: %s", resp.Status, string(b))
	}

	io.Copy(io.Discard, resp.Body)
	return nil
}

// ListTenants fetches the tenant list.
func (c *Client) ListTenants() ([]Tenant, error) {
	return listAndDecode[Tenant](c.fetch, "/api/v1/admin/tenants", "data", "tenants")
}

// ListTenantsRaw returns the API response body for the tenant list.
func (c *Client) ListTenantsRaw() ([]byte, error) {
	return c.fetch("/api/v1/admin/tenants")
}

// CreateTenant provisions a tenant via the admin API.
func (c *Client) CreateTenant(req TenantCreateRequest) (*Tenant, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.Description = strings.TrimSpace(req.Description)
	req.Status = strings.TrimSpace(req.Status)
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenantId is required")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	body, err := c.postJSON("/api/v1/admin/tenants", req)
	if err != nil {
		return nil, err
	}
	return decodeTenantResponse(body)
}

// DeleteTenant removes a tenant via the admin API.
func (c *Client) DeleteTenant(tenantID string) error {
	if tenantID == "" {
		return fmt.Errorf("tenantID is required")
	}
	path := "/api/v1/admin/tenants/" + url.PathEscape(tenantID)
	resp, err := c.doRequest(http.MethodDelete, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := readResponse(resp.Body)
		return fmt.Errorf("API error: %s: %s", resp.Status, string(b))
	}

	io.Copy(io.Discard, resp.Body)
	return nil
}

func normalizeStorageCreateData(data StorageCreateData) (StorageCreateData, error) {
	data.Name = strings.TrimSpace(data.Name)
	if data.Name == "" {
		return data, fmt.Errorf("name is required")
	}
	data.ImageID = strings.TrimSpace(data.ImageID)
	if data.ImageID == "" {
		return data, fmt.Errorf("imageId is required")
	}
	if data.SizeMB <= 0 {
		return data, fmt.Errorf("sizeMB must be greater than zero")
	}
	return data, nil
}

// CreateStorageTask submits a storage.create task for the specified tenant (admin API).
func (c *Client) CreateStorageTask(tenantID string, data StorageCreateData) ([]byte, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}
	payloadData, err := normalizeStorageCreateData(data)
	if err != nil {
		return nil, err
	}
	payload := struct {
		TenantID string            `json:"tenantId"`
		Target   TaskTarget        `json:"target"`
		Action   string            `json:"action"`
		Data     StorageCreateData `json:"data"`
		Meta     TaskMeta          `json:"meta"`
	}{
		TenantID: tenantID,
		Target: TaskTarget{
			Kind: "storage",
		},
		Action: "storage.create",
		Data:   payloadData,
		Meta: TaskMeta{
			Capability: "storage.create",
		},
	}
	return c.postJSON("/api/v1/admin/tasks", payload)
}

// DeleteStorageTask submits a storage.delete task for the specified tenant.
func (c *Client) DeleteStorageTask(tenantID, refID, agentID string) ([]byte, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}
	refID = strings.TrimSpace(refID)
	if refID == "" {
		return nil, fmt.Errorf("refID is required")
	}
	data := StorageDeleteData{RefID: refID}
	payload := struct {
		TenantID string            `json:"tenantId"`
		Target   TaskTarget        `json:"target"`
		Action   string            `json:"action"`
		Data     StorageDeleteData `json:"data"`
		Meta     TaskMeta          `json:"meta"`
	}{
		TenantID: tenantID,
		Target: TaskTarget{
			Kind:    "storage",
			RefID:   refID,
			AgentID: agentID,
		},
		Action: "storage.delete",
		Data:   data,
		Meta: TaskMeta{
			Capability: "storage.delete",
		},
	}
	return c.postJSON("/api/v1/admin/tasks", payload)
}

// CreateVMTask submits a vm.create task for the specified tenant.
func (c *Client) CreateVMTask(tenantID string, data VMCreateData, agentID string) ([]byte, error) {
	if strings.TrimSpace(data.Name) == "" || strings.TrimSpace(data.RAM) == "" || data.CPU <= 0 || strings.TrimSpace(data.DiskID) == "" {
		return nil, fmt.Errorf("missing required VM data")
	}
	payload := struct {
		TenantID string       `json:"tenantId,omitempty"`
		Target   TaskTarget   `json:"target"`
		Action   string       `json:"action"`
		Data     VMCreateData `json:"data"`
		Meta     TaskMeta     `json:"meta"`
	}{
		TenantID: tenantID,
		Target: TaskTarget{
			Kind:    "vm",
			AgentID: agentID,
		},
		Action: "vm.create",
		Data:   data,
		Meta: TaskMeta{
			Capability: "vm.create",
		},
	}
	return c.postJSON("/api/v1/admin/tasks", payload)
}

// DeleteVMTask submits a vm.delete task for the specified tenant.
func (c *Client) DeleteVMTask(tenantID, agentID, refID string, data VMDeleteData) ([]byte, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}
	if refID == "" {
		return nil, fmt.Errorf("refID is required")
	}
	if !validGUID(refID) || !strings.EqualFold(refID, data.GUID) {
		return nil, fmt.Errorf("--ref-id and --guid must identify the same canonical Hyper-V GUID")
	}
	refID = strings.ToLower(refID)
	data.GUID = refID
	payload := struct {
		TenantID string       `json:"tenantId"`
		Target   TaskTarget   `json:"target"`
		Action   string       `json:"action"`
		Data     VMDeleteData `json:"data"`
		Meta     TaskMeta     `json:"meta"`
	}{
		TenantID: tenantID,
		Target: TaskTarget{
			Kind:    "vm",
			AgentID: agentID,
			RefID:   refID,
		},
		Action: "vm.delete",
		Data:   data,
		Meta: TaskMeta{
			Capability: "vm.delete",
		},
	}
	return c.postJSON("/api/v1/admin/tasks", payload)
}

// RegisterTenantUser creates a tenant user via the admin API.
func (c *Client) RegisterTenantUser(req TenantUserRequest) ([]byte, error) {
	if req.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if req.Password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenantId is required")
	}
	if len(req.Roles) == 0 {
		return nil, fmt.Errorf("at least one role is required")
	}
	return c.postJSON("/api/v1/admin/auth/tenant/register", req)
}

// GetAdminTask fetches an admin task payload by id.
func (c *Client) GetAdminTask(taskID string) ([]byte, error) {
	if taskID == "" {
		return nil, fmt.Errorf("taskID is required")
	}
	return c.fetch("/api/v1/admin/tasks/" + url.PathEscape(taskID))
}

// GetTenantTask fetches a tenant task payload by id.
func (c *Client) GetTenantTask(taskID string) ([]byte, error) {
	if taskID == "" {
		return nil, fmt.Errorf("taskID is required")
	}
	return c.fetch("/api/v1/tenant/tasks/" + url.PathEscape(taskID))
}

func decodeSlice[T any](body []byte, wrapperKeys ...string) ([]T, error) {
	var direct []T
	if err := json.Unmarshal(body, &direct); err == nil {
		return direct, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err == nil {
		for _, key := range wrapperKeys {
			if raw, ok := obj[key]; ok {
				var data []T
				if err := json.Unmarshal(raw, &data); err == nil {
					return data, nil
				}
			}
		}
	}

	if len(body) > 300 {
		body = body[:300]
	}
	return nil, fmt.Errorf("unable to decode response body: %s", string(body))
}

func decodeQuotaMap(body []byte) ([]Quota, error) {
	var wrapper struct {
		Data map[string]struct {
			Name   string  `json:"name"`
			Limit  float64 `json:"limit"`
			Used   float64 `json:"used"`
			Usage  float64 `json:"usage"`
			Unit   string  `json:"unit"`
			Status string  `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, err
	}
	if len(wrapper.Data) == 0 {
		return nil, fmt.Errorf("quota map response missing data")
	}
	quotas := make([]Quota, 0, len(wrapper.Data))
	for metric, entry := range wrapper.Data {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = metric
		}
		usage := entry.Used
		if usage == 0 && entry.Usage != 0 {
			usage = entry.Usage
		}
		quotas = append(quotas, Quota{
			Metric: metric,
			Name:   name,
			Limit:  entry.Limit,
			Usage:  usage,
			Unit:   entry.Unit,
			Status: entry.Status,
		})
	}
	return quotas, nil
}

func decodeTenantResponse(body []byte) (*Tenant, error) {
	var tenant Tenant
	if err := json.Unmarshal(body, &tenant); err == nil {
		if tenant.TenantID != "" || tenant.Name != "" {
			return &tenant, nil
		}
	}

	var wrapper struct {
		Tenant json.RawMessage `json:"tenant"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil {
		for _, raw := range []json.RawMessage{wrapper.Data, wrapper.Tenant} {
			if len(raw) == 0 {
				continue
			}
			if tenant, err := decodeTenantResponse(raw); err == nil && tenant != nil {
				return tenant, nil
			}
		}
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}
	if len(body) > 300 {
		body = body[:300]
	}
	return nil, fmt.Errorf("unable to decode tenant response: %s", strings.TrimSpace(string(body)))
}

func decodeImageResponse(body []byte) (*Image, error) {
	var img Image
	if err := json.Unmarshal(body, &img); err == nil {
		if strings.TrimSpace(img.ID) != "" || strings.TrimSpace(img.Name) != "" {
			return &img, nil
		}
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(body, &arr); err == nil {
		for _, raw := range arr {
			if image, err := decodeImageResponse(raw); err == nil && image != nil {
				return image, nil
			}
		}
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err == nil {
		for _, key := range []string{"image", "data"} {
			if raw, ok := obj[key]; ok {
				if image, err := decodeImageResponse(raw); err == nil && image != nil {
					return image, nil
				}
			}
		}
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}
	if len(body) > 300 {
		body = body[:300]
	}
	return nil, fmt.Errorf("unable to decode image response: %s", strings.TrimSpace(string(body)))
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func tenantPath(base, tenantID string) string {
	// Tenant context is derived from the PAT/JWT; avoid adding tenantId query params.
	_ = tenantID
	return base
}

func validGUID(value string) bool {
	return regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(value)
}

// SubmitTask sends the HTTP task payload; AMQP envelopes are built by the backend.
func (c *Client) SubmitTask(mode string, payload json.RawMessage) ([]byte, error) {
	if mode != "tenant" && mode != "admin" {
		return nil, fmt.Errorf("invalid task API mode")
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal(payload, &input); err != nil || input == nil {
		return nil, fmt.Errorf("task must be a JSON object")
	}
	if mode == "tenant" && input["tenantId"] != nil {
		return nil, fmt.Errorf("tenantId must be derived from tenant credentials")
	}
	return c.postJSON("/api/v1/"+mode+"/tasks", payload)
}
