package api

import (
	"context"
	"net/http"
)

type User struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Plan          string `json:"plan"`
}

type DeviceTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

// ExchangeDeviceCode trades a one-time device-auth code for a JWT pair.
// The code is the credential — no prior auth is required.
func (c *Client) ExchangeDeviceCode(ctx context.Context, code string) (*DeviceTokens, error) {
	var out DeviceTokens
	if err := c.do(ctx, http.MethodPost, "/auth/device/token", nil, map[string]string{"code": code}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Me(ctx context.Context) (*User, error) {
	var out struct {
		User User `json:"user"`
	}
	if err := c.do(ctx, http.MethodGet, "/auth/me", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.User, nil
}
