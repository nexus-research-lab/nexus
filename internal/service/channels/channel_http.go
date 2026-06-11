package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func doChannelJSON(
	ctx context.Context,
	client *http.Client,
	method string,
	endpoint string,
	body any,
	headers map[string]string,
) (*http.Response, error) {
	if client == nil {
		client = defaultChannelHTTPClient
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	if body != nil && request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return client.Do(request)
}

func doChannelJSONExpectSuccess(
	ctx context.Context,
	client *http.Client,
	method string,
	endpoint string,
	body any,
	headers map[string]string,
) error {
	response, err := doChannelJSON(ctx, client, method, endpoint, body, headers)
	if err != nil {
		return err
	}
	return expectSuccess(response)
}

func doChannelJSONExpectSuccessDecode(
	ctx context.Context,
	client *http.Client,
	method string,
	endpoint string,
	body any,
	headers map[string]string,
	output any,
) error {
	response, err := doChannelJSON(ctx, client, method, endpoint, body, headers)
	if err != nil {
		return err
	}
	return expectSuccessDecode(response, output)
}

func expectSuccess(response *http.Response) error {
	defer response.Body.Close()
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("delivery request failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(body)))
}

func expectSuccessDecode(response *http.Response, output any) error {
	defer response.Body.Close()
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		if output == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			return nil
		}
		if err := json.NewDecoder(response.Body).Decode(output); err != nil && err != io.EOF {
			return err
		}
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("delivery request failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(body)))
}
