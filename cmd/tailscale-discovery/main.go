package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/function61/gokit/app/aws/lambdautils"
	"github.com/function61/gokit/encoding/jsonfile"
	"github.com/function61/gokit/net/http/ezhttp"
	"github.com/function61/gokit/net/http/httputils"
	"github.com/function61/gokit/os/osutil"
	"github.com/function61/tailscale-discovery/pkg/tailscalediscoveryclient"
)

func main() {
	if lambdautils.InLambda() {
		handler, err := newServerHandler()
		osutil.ExitIfError(err)
		lambda.StartHandler(lambdautils.NewLambdaHttpHandlerAdapter(handler))
		return
	}

	osutil.ExitIfError(server(
		osutil.CancelOnInterruptOrTerminate(nil)))
}

func server(ctx context.Context) error {
	handler, err := newServerHandler()
	if err != nil {
		return err
	}

	srv := &http.Server{
		Handler: handler,
		Addr:    ":80",
	}

	return httputils.CancelableServer(ctx, srv, srv.ListenAndServe)
}

func newServerHandler() (http.Handler, error) {
	apiKey, err := osutil.GetenvRequired("TAILSCALE_API_KEY")
	if err != nil {
		return nil, err
	}

	tailnet, err := osutil.GetenvRequired("TAILSCALE_TAILNET")
	if err != nil {
		return nil, err
	}

	routes := http.NewServeMux()
	routes.HandleFunc("/tailscale-discovery/api/devices", func(w http.ResponseWriter, r *http.Request) {
		devs, err := getDevicesFromTailscale(r.Context(), tailnet, apiKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_ = jsonfile.Marshal(w, devs)
	})

	return routes, nil
}

type tailscaleAPIDevicesOutput struct {
	Devices []struct {
		Addresses []string `json:"addresses"`
		Hostname  string   `json:"hostname"`
	} `json:"devices"`
}

func getDevicesFromTailscale(ctx context.Context, tailnet string, apiKey string) ([]tailscalediscoveryclient.Device, error) {
	devicesOutput := tailscaleAPIDevicesOutput{}
	if _, err := ezhttp.Get(
		ctx,
		fmt.Sprintf("https://api.tailscale.com/api/v2/tailnet/%s/devices", tailnet),
		ezhttp.AuthBasic(apiKey, ""), // <- unusual way
		ezhttp.RespondsJsonAllowUnknownFields(&devicesOutput),
	); err != nil {
		return nil, fmt.Errorf("Tailscale API: %w", err)
	}

	devices := []tailscalediscoveryclient.Device{}
	for _, apiDevice := range devicesOutput.Devices {
		devices = append(devices, tailscalediscoveryclient.Device{
			IPv4:     apiDevice.Addresses[0], // first is always IPv4
			Hostname: apiDevice.Hostname,
		})
	}

	return devices, nil
}
