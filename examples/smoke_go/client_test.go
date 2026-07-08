package keeper

import "testing"

// TestClientConstructs verifies the openapi_go_client codegen produced a usable
// Go client: NewClient + *Client are part of oapi-codegen's guaranteed surface,
// so if the generate → go_library pipeline broke, this fails to build.
func TestClientConstructs(t *testing.T) {
	c, err := NewClient("https://example.com")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient returned a nil client")
	}
}
