package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"golang.org/x/crypto/ssh"
)

type Config struct {
	ProviderEndpoint string
	TokenEndpoint    string
	ClientID         string
	HttpClient       *http.Client
}

type DeviceResponse struct {
	DeviceCode      string
	UserCode        string
	VerificationUri string
	ExpiresIn       int
	Interval        int
}

type TokenResponse struct {
	AccessToken string
	TokenType   string
	Scope       string
}

func NewConfig() *Config {
	return &Config{
		ProviderEndpoint: os.Getenv("OIDC_CODE_ENDPOINT"),
		TokenEndpoint:    os.Getenv("OIDC_TOKEN_ENDPOINT"),
		ClientID:         os.Getenv("OIDC_CLIENT_ID"),
		HttpClient:       http.DefaultClient,
	}
}

func (c *Config) getDeviceCode() (*DeviceResponse, error) {
	resp, err := c.HttpClient.PostForm(c.ProviderEndpoint, url.Values{
		"client_id": {c.ClientID},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	bodyString := string(bodyBytes)
	values, err := url.ParseQuery(bodyString)
	if err != nil {
		return nil, err
	}
	var deviceResponse DeviceResponse
	deviceResponse.DeviceCode = values.Get("device_code")
	deviceResponse.UserCode = values.Get("user_code")
	deviceResponse.VerificationUri = values.Get("verification_uri")
	_, err = fmt.Sscanf(values.Get("expires_in"), "%d", &deviceResponse.ExpiresIn)
	if err != nil {
		return nil, err
	}
	_, err = fmt.Sscanf(values.Get("interval"), "%d", &deviceResponse.Interval)
	if err != nil {
		return nil, err
	}

	return &deviceResponse, nil
}

func (c *Config) fetchToken(deviceCode string, interval int) (*TokenResponse, error) {
	var tokenResponse TokenResponse

	resp, err := c.HttpClient.PostForm(c.TokenEndpoint, url.Values{
		"client_id":   {c.ClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	bodyString := string(bodyBytes)
	values, err := url.ParseQuery(bodyString)
	if err != nil {
		return nil, err
	}
	if values.Get("error") == "authorization_pending" {
		return nil, errors.New("authorization pending")
	}
	tokenResponse.AccessToken = values.Get("access_token")
	tokenResponse.TokenType = values.Get("token_type")
	tokenResponse.Scope = values.Get("scope")
	if tokenResponse.AccessToken == "" {
		return nil, errors.New("no access token")
	}
	return &tokenResponse, nil
}

func (c *Config) getLogin(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return "", err
	}

	headers := http.Header{
		"Authorization": {"Bearer " + accessToken},
		"Accept":        {"application/vnd.github.v3+json"},
	}
	req.Header = headers
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", err
	}
	login := userInfo["login"].(string)
	return login, nil
}

func (c *Config) SshConfig() *ssh.ServerConfig {
	return &ssh.ServerConfig{
		KeyboardInteractiveCallback: func(s ssh.ConnMetadata, client ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			// Get device code from OIDC provider code endpoint
			deviceResponse, err := c.getDeviceCode()
			if err != nil {
				return nil, err
			}

			client(
				"Log in to GitHub",
				fmt.Sprintf("Go to %s and enter the code %s",
					deviceResponse.VerificationUri, deviceResponse.UserCode),
				[]string{"Press enter to continue..."},
				[]bool{true},
			)

			// Exchange device code for access token
			tokenResponse, err := c.fetchToken(deviceResponse.DeviceCode, deviceResponse.Interval)
			if err != nil {
				return nil, err
			}

			// Exchange access token for user login
			login, err := c.getLogin(tokenResponse.AccessToken)
			if err != nil {
				return nil, err
			}

			log.Printf("User %s logged in\n", login)
			return &ssh.Permissions{
				Extensions: map[string]string{
					"login": login,
				},
			}, nil
		},
	}
}
