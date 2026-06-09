package service

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildGeminiAPIKeyUpstreamRequestProfiles(t *testing.T) {
	t.Parallel()

	body := []byte(`{"tools":[{"googleSearch":{}}],"contents":[{"parts":[{"text":"hi"}]}]}`)

	t.Run("default google ai studio", func(t *testing.T) {
		t.Parallel()

		account := &Account{
			Platform: PlatformGemini,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key": "gemini-key",
			},
		}
		req, requestIDHeader, err := buildGeminiAPIKeyUpstreamRequest(context.Background(), account, "https://generativelanguage.googleapis.com", geminiAPIKeyUpstreamRequestOptions{
			Model:    "gemini-2.5-flash",
			Action:   "streamGenerateContent",
			Stream:   true,
			Body:     body,
			BodyMode: geminiAPIKeyBodyNormalizeAIStudio,
		})
		require.NoError(t, err)
		require.Equal(t, geminiAPIKeyRequestIDHeader, requestIDHeader)
		require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse", req.URL.String())
		require.Equal(t, "gemini-key", req.Header.Get("x-goog-api-key"))
		require.Empty(t, req.Header.Get("Authorization"))

		sent, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Contains(t, string(sent), "google_search")
		require.NotContains(t, string(sent), "googleSearch")
	})

	t.Run("qiniu vertex bypass", func(t *testing.T) {
		t.Parallel()

		account := &Account{
			Platform: PlatformGemini,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key":                 "qiniu-key",
				"gemini_upstream_profile": "qiniu_vertex_bypass",
			},
		}
		req, requestIDHeader, err := buildGeminiAPIKeyUpstreamRequest(context.Background(), account, "https://api.qnaigc.com/bypass/vertex/", geminiAPIKeyUpstreamRequestOptions{
			Model:    "dj-gemini-3.1-pro-preview",
			Action:   "streamGenerateContent",
			Stream:   true,
			Body:     body,
			BodyMode: geminiAPIKeyBodyNormalizeAIStudio,
		})
		require.NoError(t, err)
		require.Equal(t, geminiAPIKeyRequestIDHeader, requestIDHeader)
		require.Equal(t, "https://api.qnaigc.com/bypass/vertex/v1/models/dj-gemini-3.1-pro-preview:streamGenerateContent?alt=sse", req.URL.String())
		require.Equal(t, "Bearer qiniu-key", req.Header.Get("Authorization"))
		require.Empty(t, req.Header.Get("x-goog-api-key"))
		require.Equal(t, "application/json", req.Header.Get("Accept"))

		sent, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Contains(t, string(sent), "googleSearch")
		require.NotContains(t, string(sent), "google_search")
	})
}

func TestBuildGeminiAPIKeyUpstreamRequestQiniuRejectsCountTokens(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":                 "qiniu-key",
			"gemini_upstream_profile": "qiniu_vertex_bypass",
		},
	}
	_, _, err := buildGeminiAPIKeyUpstreamRequest(context.Background(), account, "https://api.qnaigc.com/bypass/vertex", geminiAPIKeyUpstreamRequestOptions{
		Model:    "dj-gemini-3.1-pro-preview",
		Action:   "countTokens",
		Body:     []byte(`{}`),
		BodyMode: geminiAPIKeyBodyNative,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "qiniu_vertex_bypass")
	require.Contains(t, err.Error(), "countTokens")
}

func TestQiniuGeminiProfileDoesNotSupportAIStudioGETEndpoints(t *testing.T) {
	t.Parallel()

	qiniuAccount := &Account{
		ID:       301,
		Platform: PlatformGemini,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":                 "qiniu-key",
			"gemini_upstream_profile": "qiniu_vertex_bypass",
		},
	}
	require.False(t, qiniuAccount.SupportsGeminiAIStudioGETEndpoints())

	googleAccount := &Account{
		ID:       302,
		Platform: PlatformGemini,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "gemini-key",
		},
	}
	require.True(t, googleAccount.SupportsGeminiAIStudioGETEndpoints())

	svc := &GeminiMessagesCompatService{}
	_, err := svc.ForwardAIStudioGET(context.Background(), qiniuAccount, "/v1beta/models")
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "does not support ai studio get")
}
