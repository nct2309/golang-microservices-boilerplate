package types

import (
	"golang-microservices-boilerplate/pkg/core/entity"
)

// FilterOptions contains common filtering, pagination, and sorting options using Limit/Offset
type FilterOptions struct {
	Limit          int                    `json:"limit"`           // Maximum number of items to return
	Offset         int                    `json:"offset"`          // Number of items to skip
	SortBy         string                 `json:"sort_by"`         // Field to sort by
	SortDesc       bool                   `json:"sort_desc"`       // True for descending order
	Filters        map[string]interface{} `json:"filters"`         // Key-value pairs for filtering
	IncludeDeleted bool                   `json:"include_deleted"` // Whether to include soft-deleted records
}

// DefaultFilterOptions returns a default set of filter options using Limit/Offset
func DefaultFilterOptions() FilterOptions {
	return FilterOptions{
		Limit:          50, // Default limit
		Offset:         0,  // Default offset
		SortBy:         "created_at",
		SortDesc:       true,
		Filters:        make(map[string]interface{}),
		IncludeDeleted: false,
	}
}

// PaginationResult represents a paginated result containing entity pointers using Limit/Offset.
// We use type parameter E constrained by entity.Entity here.
type PaginationResult[E entity.Entity] struct {
	Items      []*E  `json:"items"`       // Slice of entity pointers (*E)
	TotalItems int64 `json:"total_items"` // Total number of items matching the query
	Limit      int   `json:"limit"`       // The limit used for this query
	Offset     int   `json:"offset"`      // The offset used for this query
}
