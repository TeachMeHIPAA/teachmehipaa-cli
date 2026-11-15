package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/api"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/output"
)

// Page is one page of a list, as every list method answers.
type Page struct {
	Items []json.RawMessage `json:"items"`
	// NextCursor is null on the last page.
	NextCursor *string `json:"nextCursor"`
}

// FetchPage sends a list request and reads the page it answers with.
func FetchPage(ctx context.Context, client *api.Client, request api.Request) (Page, error) {
	response, err := client.Do(ctx, request)
	if err != nil {
		return Page{}, err
	}
	var page Page
	if err := json.Unmarshal(response.Body, &page); err != nil {
		return Page{}, fmt.Errorf("reading the list page: %w", err)
	}
	return page, nil
}

// listAll requests page after page with the same filters and page size
// until nextCursor is null. A short or empty page doesn't end the list:
// some filters apply per page (api-sdk-surface.md §4).
func listAll(ctx context.Context, client *api.Client, request api.Request, pagination *Pagination, items *output.Items) error {
	for {
		current, err := FetchPage(ctx, client, request)
		if err != nil {
			return err
		}
		for _, item := range current.Items {
			if err := items.Write(item); err != nil {
				return err
			}
		}
		if current.NextCursor == nil {
			return items.Close()
		}
		next, err := nextPage(pagination, current)
		if err != nil {
			return err
		}
		request.Query = maps.Clone(request.Query)
		if request.Query == nil {
			request.Query = url.Values{}
		}
		request.Query.Set(pagination.Param, next)
	}
}

func nextPage(pagination *Pagination, current Page) (string, error) {
	if pagination.ItemID == "" || len(current.Items) == 0 {
		return *current.NextCursor, nil
	}
	var last map[string]json.RawMessage
	var id string
	if json.Unmarshal(current.Items[len(current.Items)-1], &last) != nil ||
		json.Unmarshal(last[pagination.ItemID], &id) != nil || id == "" {
		return "", fmt.Errorf("the last item of the page has no %s to continue from", pagination.ItemID)
	}
	return id, nil
}
