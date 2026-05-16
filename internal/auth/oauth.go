package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/config"
)

type DeviceFlow struct {
	DeviceCode      string `json:"-"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type OAuthClient struct {
	cfg    config.Config
	client *http.Client
}

func NewOAuthClient(cfg config.Config, client *http.Client) OAuthClient {
	if client == nil {
		client = http.DefaultClient
	}
	return OAuthClient{cfg: cfg, client: client}
}

func (c OAuthClient) StartDeviceFlow(ctx context.Context) (DeviceFlow, error) {
	values := url.Values{}
	values.Set("client_id", c.cfg.GitHub.ClientID)
	values.Set("scope", c.cfg.GitHub.OAuthScope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.GitHub.DeviceCodeURL, strings.NewReader(values.Encode()))
	if err != nil {
		return DeviceFlow{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return DeviceFlow{}, err
	}
	defer resp.Body.Close()

	var payload struct {
		DeviceCode       string `json:"device_code"`
		UserCode         string `json:"user_code"`
		VerificationURI  string `json:"verification_uri"`
		ExpiresIn        int    `json:"expires_in"`
		Interval         int    `json:"interval"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return DeviceFlow{}, err
	}
	if payload.Error != "" {
		return DeviceFlow{}, fmt.Errorf("%s: %s", payload.Error, payload.ErrorDescription)
	}
	if payload.DeviceCode == "" || payload.UserCode == "" {
		return DeviceFlow{}, errors.New("github device flow response did not include a device code")
	}
	if payload.Interval <= 0 {
		payload.Interval = 5
	}
	if payload.VerificationURI == "" {
		payload.VerificationURI = "https://github.com/login/device"
	}
	return DeviceFlow{
		DeviceCode:      payload.DeviceCode,
		UserCode:        payload.UserCode,
		VerificationURI: payload.VerificationURI,
		ExpiresIn:       payload.ExpiresIn,
		Interval:        payload.Interval,
	}, nil
}

func (c OAuthClient) PollAccessToken(ctx context.Context, deviceCode string) (string, string, error) {
	values := url.Values{}
	values.Set("client_id", c.cfg.GitHub.ClientID)
	values.Set("device_code", deviceCode)
	values.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.GitHub.AccessTokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var payload struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Message     string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", err
	}
	if payload.AccessToken != "" {
		return payload.AccessToken, "", nil
	}
	if payload.Error != "" {
		return "", payload.Error, nil
	}
	return "", "", errors.New("github access token response did not include an access token")
}

func (c OAuthClient) WaitForAccessToken(ctx context.Context, flow DeviceFlow, onWait func(time.Duration)) (string, error) {
	deadline := time.Now().Add(time.Duration(flow.ExpiresIn) * time.Second)
	interval := time.Duration(flow.Interval) * time.Second
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
		token, oauthErr, err := c.PollAccessToken(ctx, flow.DeviceCode)
		if err != nil {
			return "", err
		}
		switch oauthErr {
		case "":
			return token, nil
		case "authorization_pending":
			if onWait != nil {
				onWait(time.Until(deadline))
			}
		case "slow_down":
			interval += 5 * time.Second
		case "expired_token":
			return "", errors.New("device code expired")
		case "access_denied":
			return "", errors.New("authorization denied")
		default:
			return "", fmt.Errorf("github oauth error: %s", oauthErr)
		}
	}
	return "", errors.New("authorization timed out")
}
