package sandbox

import (
	"context"
	"fmt"
)

// SandboxPaginator iterates through sandbox list pages.
type SandboxPaginator struct {
	client    *Client
	params    ListParams
	hasNext   bool
	nextToken *string
}

// NewSandboxPaginator creates a paginator for sandboxes.
func (c *Client) NewSandboxPaginator(params *ListParams) *SandboxPaginator {
	p := ListParams{}
	if params != nil {
		p = *params
	}
	return &SandboxPaginator{
		client:    c,
		params:    p,
		hasNext:   true,
		nextToken: p.NextToken,
	}
}

// HasNext reports whether another page can be fetched.
func (p *SandboxPaginator) HasNext() bool { return p.hasNext }

// NextToken returns the cursor for the next page, if any.
func (p *SandboxPaginator) NextToken() *string { return p.nextToken }

// NextItems fetches the next sandbox page.
func (p *SandboxPaginator) NextItems(ctx context.Context) ([]ListedSandbox, error) {
	if !p.hasNext {
		return nil, fmt.Errorf("no more items to fetch")
	}
	p.params.NextToken = p.nextToken
	items, next, err := p.client.ListPage(ctx, &p.params)
	if err != nil {
		return nil, err
	}
	p.nextToken = next
	p.hasNext = next != nil && *next != ""
	return items, nil
}

// SnapshotPaginator iterates through snapshot list pages.
type SnapshotPaginator struct {
	client    *Client
	params    SnapshotListParams
	hasNext   bool
	nextToken *string
}

// NewSnapshotPaginator creates a paginator for snapshots.
func (c *Client) NewSnapshotPaginator(params *SnapshotListParams) *SnapshotPaginator {
	p := SnapshotListParams{}
	if params != nil {
		p = *params
	}
	return &SnapshotPaginator{
		client:    c,
		params:    p,
		hasNext:   true,
		nextToken: p.NextToken,
	}
}

// HasNext reports whether another page can be fetched.
func (p *SnapshotPaginator) HasNext() bool { return p.hasNext }

// NextToken returns the cursor for the next page, if any.
func (p *SnapshotPaginator) NextToken() *string { return p.nextToken }

// NextItems fetches the next snapshot page.
func (p *SnapshotPaginator) NextItems(ctx context.Context) ([]SnapshotInfo, error) {
	if !p.hasNext {
		return nil, fmt.Errorf("no more items to fetch")
	}
	p.params.NextToken = p.nextToken
	items, next, err := p.client.ListSnapshots(ctx, &p.params)
	if err != nil {
		return nil, err
	}
	p.nextToken = next
	p.hasNext = next != nil && *next != ""
	return items, nil
}
