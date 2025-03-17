package hcloud

import (
	"bytes"
	"context"
	"encoding/json"
)

func getRequest[Schema any](ctx context.Context, client *Client, url string) (*Schema, *Response, error) {
	req, err := client.NewRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	var respBody Schema
	resp, err := client.Do(req, &respBody)
	if err != nil {
		return nil, resp, err
	}

	return &respBody, resp, nil
}

func postRequest[Schema any](ctx context.Context, client *Client, url string, reqBody any) (*Schema, *Response, error) {
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, err
	}

	req, err := client.NewRequest(ctx, "POST", url, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, nil, err
	}

	var respBody Schema
	resp, err := client.Do(req, &respBody)
	if err != nil {
		return nil, resp, err
	}

	return &respBody, resp, nil
}

func putRequest[Schema any](ctx context.Context, client *Client, url string, reqBody any) (*Schema, *Response, error) {
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, err
	}

	req, err := client.NewRequest(ctx, "PUT", url, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, nil, err
	}

	var respBody Schema
	resp, err := client.Do(req, &respBody)
	if err != nil {
		return nil, resp, err
	}

	return &respBody, resp, nil
}

func deleteRequest[Schema any](ctx context.Context, client *Client, url string) (*Schema, *Response, error) {
	req, err := client.NewRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return nil, nil, err
	}

	var respBody Schema
	resp, err := client.Do(req, &respBody)
	if err != nil {
		return nil, resp, err
	}

	return &respBody, resp, nil
}

func deleteRequestNoResult(ctx context.Context, client *Client, url string) (*Response, error) {
	req, err := client.NewRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return nil, err
	}

	return client.Do(req, nil)
}
