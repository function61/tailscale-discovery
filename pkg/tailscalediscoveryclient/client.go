package tailscalediscoveryclient

import (
	"context"

	"github.com/function61/gokit/net/http/ezhttp"
)

const (
	Function61 = "https://function61"
	Localhost  = "http://localhost"
)

type Device struct {
	IPv4     string `json:"ip_v4"`
	Hostname string `json:"hostname"`
}

type Client struct {
	apiToken string
	baseURL  string
}

func NewClient(apiToken string, baseURL string) Client {
	return Client{
		apiToken: apiToken,
		baseURL:  baseURL,
	}
}

func (c Client) Devices(ctx context.Context) ([]Device, error) {
	devices := []Device{}
	_, err := ezhttp.Get(
		ctx,
		c.baseURL+"/tailscale-discovery/api/devices",
		ezhttp.AuthBearer(c.apiToken),
		ezhttp.RespondsJsonAllowUnknownFields(&devices))
	return devices, err
}
