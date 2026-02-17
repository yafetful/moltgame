package twitter

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client handles Twitter OAuth 2.0 PKCE (user login) and App-Only Bearer Token (tweet search).
type Client struct {
	// OAuth 2.0 PKCE (user login)
	clientID     string
	clientSecret string
	callbackURL  string

	// App credentials (for Bearer Token)
	apiKey       string
	apiKeySecret string

	// Cached App-Only Bearer Token
	bearerToken string

	httpClient *http.Client
}

func NewClient(clientID, clientSecret, callbackURL, apiKey, apiKeySecret, _, _ string) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		callbackURL:  callbackURL,
		apiKey:       apiKey,
		apiKeySecret: apiKeySecret,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// ==================== OAuth 2.0 PKCE ====================

type AuthSession struct {
	State         string
	CodeVerifier  string
	CodeChallenge string
	AuthURL       string
}

// StartAuth generates the OAuth 2.0 authorization URL with PKCE.
func (c *Client) StartAuth() (*AuthSession, error) {
	state, err := randomString(32)
	if err != nil {
		return nil, err
	}
	verifier, err := randomString(64)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {c.clientID},
		"redirect_uri":          {c.callbackURL},
		"scope":                 {"users.read tweet.read tweet.write offline.access"},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}

	authURL := "https://x.com/i/oauth2/authorize?" + params.Encode()

	return &AuthSession{
		State:         state,
		CodeVerifier:  verifier,
		CodeChallenge: challenge,
		AuthURL:       authURL,
	}, nil
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// ExchangeCode exchanges the authorization code for tokens.
func (c *Client) ExchangeCode(code, codeVerifier string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.callbackURL},
		"code_verifier": {codeVerifier},
		"client_id":     {c.clientID},
	}

	req, err := http.NewRequest("POST", "https://api.x.com/2/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Confidential client: Basic Auth with client_id:client_secret
	if c.clientSecret != "" {
		req.SetBasicAuth(c.clientID, c.clientSecret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tok TokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	return &tok, nil
}

type TwitterUser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

// GetMe fetches the authenticated user's profile using an OAuth 2.0 user token.
func (c *Client) GetMe(accessToken string) (*TwitterUser, error) {
	req, err := http.NewRequest("GET", "https://api.x.com/2/users/me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get user request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get user failed (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data TwitterUser `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse user response: %w", err)
	}
	return &result.Data, nil
}

// ==================== Tweet Verification (App-Only Bearer Token) ====================

type Tweet struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// getAppBearerToken obtains an App-Only Bearer Token using Client Credentials Grant.
// Uses API Key (Consumer Key) and API Key Secret (Consumer Secret) via Basic Auth.
func (c *Client) getAppBearerToken() (string, error) {
	if c.bearerToken != "" {
		return c.bearerToken, nil
	}

	data := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequest("POST", "https://api.x.com/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.apiKey, c.apiKeySecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("bearer token request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("bearer token failed (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		TokenType   string `json:"token_type"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse bearer token: %w", err)
	}

	c.bearerToken = result.AccessToken
	return c.bearerToken, nil
}

// SearchRecentTweets searches recent tweets using App-Only Bearer Token.
func (c *Client) SearchRecentTweets(query string) ([]Tweet, error) {
	token, err := c.getAppBearerToken()
	if err != nil {
		return nil, fmt.Errorf("get bearer token: %w", err)
	}

	params := url.Values{
		"query":       {query},
		"max_results": {"10"},
	}
	fullURL := "https://api.x.com/2/tweets/search/recent?" + params.Encode()

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search tweets request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search tweets failed (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []Tweet `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}
	return result.Data, nil
}

// VerifyClaimTweet checks if a specific Twitter user posted a tweet containing the verification code.
func (c *Client) VerifyClaimTweet(twitterHandle, verificationCode string) (bool, error) {
	query := fmt.Sprintf("from:%s %s", twitterHandle, verificationCode)
	tweets, err := c.SearchRecentTweets(query)
	if err != nil {
		return false, err
	}
	for _, t := range tweets {
		if strings.Contains(t.Text, verificationCode) {
			return true, nil
		}
	}
	return false, nil
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:n], nil
}
