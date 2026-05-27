package integration

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	api "github.com/ogen-go/ogen/internal/integration/test_additional_operations"
)

type additionalOperationsTestServer struct {
	api.UnimplementedHandler
}

func (s *additionalOperationsTestServer) Echo(ctx context.Context, req api.EchoReq) (r api.EchoOK, err error) {
	return api.EchoOK{Data: req}, nil
}

func (s *additionalOperationsTestServer) QueryEcho(ctx context.Context, req api.QueryEchoReq) (r api.QueryEchoOK, err error) {
	return api.QueryEchoOK{Data: req}, nil
}

func (s *additionalOperationsTestServer) UnlinkEcho(ctx context.Context, req api.UnlinkEchoReq) (r api.UnlinkEchoOK, err error) {
	return api.UnlinkEchoOK{Data: req}, nil
}

func TestAdditionalOperations(t *testing.T) {
	srv, err := api.NewServer(&additionalOperationsTestServer{})
	require.NoError(t, err)

	s := httptest.NewServer(srv)
	defer s.Close()

	client, err := api.NewClient(s.URL)
	require.NoError(t, err)

	const text = "Hello, 世界"

	// Each case covers one custom HTTP method on /echo: raw HTTP round-trip
	// followed by a generated-client round-trip.
	for _, tc := range []struct {
		name   string
		method string
		call   func(ctx context.Context) (io.Reader, error)
	}{
		{
			name:   "LINK_additionalOperation",
			method: "LINK",
			call: func(ctx context.Context) (io.Reader, error) {
				return client.Echo(ctx, api.EchoReq{Data: strings.NewReader(text)})
			},
		},
		{
			name:   "QUERY_fixedField",
			method: "QUERY",
			call: func(ctx context.Context) (io.Reader, error) {
				return client.QueryEcho(ctx, api.QueryEchoReq{Data: strings.NewReader(text)})
			},
		},
		{
			name:   "UNLINK_additionalOperation",
			method: "UNLINK",
			call: func(ctx context.Context) (io.Reader, error) {
				return client.UnlinkEcho(ctx, api.UnlinkEchoReq{Data: strings.NewReader(text)})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, s.URL+"/echo", strings.NewReader(text))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "text/plain")

			resp, err := s.Client().Do(req)
			require.NoError(t, err)

			respText, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, text, string(respText))
			require.Equal(t, http.StatusOK, resp.StatusCode)

			clientResp, err := tc.call(t.Context())
			require.NoError(t, err)
			clientRespText, err := io.ReadAll(clientResp)
			require.NoError(t, err)
			require.Equal(t, text, string(clientRespText))
		})
	}
}
