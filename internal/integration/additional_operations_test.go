package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdditionalOperations(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "LINK" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		buf := new(strings.Builder)
		_, _ = io.Copy(buf, r.Body)
		w.Write([]byte(buf.String()))
	})

	s := httptest.NewServer(mux)
	defer s.Close()

	const text = "Hello, 世界"

	req, err := http.NewRequest("LINK", s.URL+"/echo", strings.NewReader(text))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := s.Client().Do(req)
	require.NoError(t, err)

	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, text, buf.String())
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
