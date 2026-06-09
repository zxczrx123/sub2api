package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	GeminiUpstreamProfileCredentialKey = "gemini_upstream_profile"

	GeminiUpstreamProfileGoogleAIStudio    = "google_aistudio"
	GeminiUpstreamProfileQiniuVertexBypass = "qiniu_vertex_bypass"
)

type geminiAPIKeyBodyMode string

const (
	geminiAPIKeyBodyNative            geminiAPIKeyBodyMode = "native"
	geminiAPIKeyBodyNormalizeAIStudio geminiAPIKeyBodyMode = "normalize_aistudio"
	geminiAPIKeyRequestIDHeader       string               = "x-request-id"
)

type geminiAPIKeyUpstreamRequestOptions struct {
	Model    string
	Action   string
	Stream   bool
	Body     []byte
	BodyMode geminiAPIKeyBodyMode
}

func (a *Account) GeminiUpstreamProfile() string {
	if a == nil {
		return GeminiUpstreamProfileGoogleAIStudio
	}
	raw := strings.TrimSpace(a.GetCredential(GeminiUpstreamProfileCredentialKey))
	if raw == "" {
		return GeminiUpstreamProfileGoogleAIStudio
	}
	normalized := strings.ToLower(strings.ReplaceAll(raw, "-", "_"))
	switch normalized {
	case "aistudio", "ai_studio", "google_ai_studio", GeminiUpstreamProfileGoogleAIStudio:
		return GeminiUpstreamProfileGoogleAIStudio
	case "qiniu", "qiniu_vertex", GeminiUpstreamProfileQiniuVertexBypass:
		return GeminiUpstreamProfileQiniuVertexBypass
	default:
		return normalized
	}
}

func (a *Account) IsQiniuVertexBypassGemini() bool {
	return a != nil && a.Platform == PlatformGemini && a.Type == AccountTypeAPIKey &&
		a.GeminiUpstreamProfile() == GeminiUpstreamProfileQiniuVertexBypass
}

func (a *Account) SupportsGeminiAIStudioGETEndpoints() bool {
	if a == nil || a.Platform != PlatformGemini {
		return false
	}
	switch a.Type {
	case AccountTypeAPIKey:
		return strings.TrimSpace(a.GetCredential("api_key")) != "" && !a.IsQiniuVertexBypassGemini()
	case AccountTypeOAuth:
		return true
	default:
		return false
	}
}

func buildGeminiAPIKeyUpstreamRequest(
	ctx context.Context,
	account *Account,
	normalizedBaseURL string,
	opts geminiAPIKeyUpstreamRequestOptions,
) (*http.Request, string, error) {
	if account == nil {
		return nil, "", errors.New("gemini account is required")
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return nil, "", errors.New("gemini api_key not configured")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(normalizedBaseURL), "/")
	if baseURL == "" {
		return nil, "", errors.New("gemini base_url not configured")
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		return nil, "", errors.New("gemini model is required")
	}
	action := strings.TrimSpace(opts.Action)
	switch action {
	case "generateContent", "streamGenerateContent", "countTokens":
	default:
		return nil, "", fmt.Errorf("unsupported gemini action: %s", action)
	}

	profile := account.GeminiUpstreamProfile()
	body := opts.Body
	var fullURL string
	switch profile {
	case GeminiUpstreamProfileGoogleAIStudio:
		fullURL = fmt.Sprintf("%s/v1beta/models/%s:%s", baseURL, model, action)
		if opts.BodyMode == geminiAPIKeyBodyNormalizeAIStudio {
			body = normalizeGeminiRequestForAIStudio(body)
		}
	case GeminiUpstreamProfileQiniuVertexBypass:
		switch action {
		case "generateContent", "streamGenerateContent":
			fullURL = fmt.Sprintf("%s/v1/models/%s:%s", baseURL, model, action)
		default:
			return nil, "", fmt.Errorf("%s does not support gemini action: %s", GeminiUpstreamProfileQiniuVertexBypass, action)
		}
	default:
		return nil, "", fmt.Errorf("unsupported %s: %s", GeminiUpstreamProfileCredentialKey, profile)
	}
	if opts.Stream {
		fullURL += "?alt=sse"
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	switch profile {
	case GeminiUpstreamProfileGoogleAIStudio:
		upstreamReq.Header.Set("x-goog-api-key", apiKey)
	case GeminiUpstreamProfileQiniuVertexBypass:
		upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
		upstreamReq.Header.Set("Accept", "application/json")
	}
	return upstreamReq, geminiAPIKeyRequestIDHeader, nil
}
