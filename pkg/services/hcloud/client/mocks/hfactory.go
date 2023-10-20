// Package mocks implements important mocking interface of Hcloud.
package mocks

import hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"

type hcloudFactory struct {
	client *Client
}

// NewHcloudFactory returns the hcloud factory interface.
func NewHcloudFactory(client *Client) hcloudclient.Factory {
	return &hcloudFactory{client: client}
}

var _ = hcloudclient.Factory(&hcloudFactory{})

func (f *hcloudFactory) NewClient(_ string) hcloudclient.Client {
	return f.client
}
