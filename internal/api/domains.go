package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

type DNSRecord struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Domain struct {
	ID               string      `json:"id"`
	FQDN             string      `json:"fqdn"`
	Status           string      `json:"status"` // PENDING | ACTIVE | REVOKED
	CreatedAt        string      `json:"created_at"`
	VerifiedAt       string      `json:"verified_at"`
	DNSRecords       []DNSRecord `json:"dns_records"`
	RootRedirect     string      `json:"root_redirect"`
	NotFoundRedirect string      `json:"not_found_redirect"`
}

type DomainPage struct {
	Items    []Domain `json:"items"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
	Total    int      `json:"total"`
	HasNext  bool     `json:"hasNext"`
}

func (c *Client) ListDomains(ctx context.Context) (*DomainPage, error) {
	var out DomainPage
	if err := c.do(ctx, http.MethodGet, "/api/v1/custom-domains", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateDomain(ctx context.Context, fqdn string) (*Domain, error) {
	var out Domain
	if err := c.do(ctx, http.MethodPost, "/api/v1/custom-domains", nil, map[string]string{"fqdn": fqdn}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) VerifyDomain(ctx context.Context, id string) (*Domain, error) {
	var out Domain
	if err := c.do(ctx, http.MethodPost, "/api/v1/custom-domains/"+id+"/verify", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateDomain patches a domain's per-domain routing config. fields
// holds only the keys to change (root_redirect, not_found_redirect,
// custom_robots_txt); a nil value clears that field, an omitted key
// leaves it untouched — the backend distinguishes via model_fields_set.
func (c *Client) UpdateDomain(ctx context.Context, id string, fields map[string]any) (*Domain, error) {
	var out Domain
	if err := c.do(ctx, http.MethodPatch, "/api/v1/custom-domains/"+id, nil, fields, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type DomainDeleteResult struct {
	ID          string `json:"id"`
	FQDN        string `json:"fqdn"`
	Cascade     bool   `json:"cascade"`
	URLsDeleted int    `json:"urls_deleted"`
}

// RevokeDomain stops serving the domain. With cascade, its URLs are
// deleted too; without, they stay in the database but stop resolving.
func (c *Client) RevokeDomain(ctx context.Context, id string, cascade bool) (*DomainDeleteResult, error) {
	q := url.Values{"cascade": {strconv.FormatBool(cascade)}}
	var out DomainDeleteResult
	if err := c.do(ctx, http.MethodDelete, "/api/v1/custom-domains/"+id, q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
