// Code generated by ifacemaker; DO NOT EDIT.

package hcloud

import (
	"context"
)

// IResourceActionClient ...
type IResourceActionClient interface {
	// GetByID retrieves an action by its ID. If the action does not exist, nil is returned.
	GetByID(ctx context.Context, id int64) (*Action, *Response, error)
	// List returns a list of actions for a specific page.
	//
	// Please note that filters specified in opts are not taken into account
	// when their value corresponds to their zero value or when they are empty.
	List(ctx context.Context, opts ActionListOpts) ([]*Action, *Response, error)
	// All returns all actions for the given options.
	All(ctx context.Context, opts ActionListOpts) ([]*Action, error)
}
