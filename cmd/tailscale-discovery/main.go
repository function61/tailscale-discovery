package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"

	lambdahandler "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/function61/gokit/app/aws/lambdautils"
	"github.com/function61/gokit/app/cli"
	"github.com/function61/gokit/app/dynversion"
	. "github.com/function61/gokit/builtin"
	"github.com/function61/gokit/encoding/jsonfile"
	"github.com/function61/gokit/net/http/ezhttp"
	"github.com/function61/gokit/net/http/httputils"
	"github.com/function61/gokit/os/osutil"
	"github.com/function61/tailscale-discovery/pkg/tailscalediscoveryclient"
	"github.com/spf13/cobra"
)

func main() {
	if lambdautils.InLambda() {
		handler, err := newServerHandler()
		osutil.ExitIfError(err)
		lambdahandler.StartHandler(lambdautils.NewLambdaHttpHandlerAdapter(handler))
		return
	}

	app := &cobra.Command{
		Use:     os.Args[0],
		Short:   "Tailscale discovery",
		Version: dynversion.Version,
		Args:    cobra.NoArgs,
		Run:     cli.RunnerNoArgs(server),
	}

	app.AddCommand(renewAPIKeyEntrypoint())

	osutil.ExitIfError(app.Execute())
}

func server(ctx context.Context, _ *log.Logger) error {
	handler, err := newServerHandler()
	if err != nil {
		return err
	}

	srv := &http.Server{
		Handler:           handler,
		Addr:              ":80",
		ReadHeaderTimeout: httputils.DefaultReadHeaderTimeout,
	}

	return httputils.CancelableServer(ctx, srv, srv.ListenAndServe)
}

func newServerHandler() (http.Handler, error) {
	apiKey, err := osutil.GetenvRequired("TAILSCALE_API_KEY")
	if err != nil {
		return nil, err
	}

	tailnet := coalesce(os.Getenv("TAILSCALE_TAILNET"), "-") // '-' means the default Tailnet the API key has access to

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
		Tags      []string `json:"tags"`
	} `json:"devices"`
}

func getDevicesFromTailscale(ctx context.Context, tailnet string, apiKey string) ([]tailscalediscoveryclient.Device, error) {
	devicesOutput := tailscaleAPIDevicesOutput{}
	if _, err := ezhttp.Get(
		ctx,
		fmt.Sprintf("https://api.tailscale.com/api/v2/tailnet/%s/devices", tailnet),
		ezhttp.AuthBasic(apiKey, ""), // <- unusual way
		ezhttp.RespondsJSONAllowUnknownFields(&devicesOutput),
	); err != nil {
		return nil, fmt.Errorf("Tailscale API: %w", err)
	}

	devices := []tailscalediscoveryclient.Device{}
	for _, apiDevice := range devicesOutput.Devices {
		devices = append(devices, tailscalediscoveryclient.Device{
			IPv4:     apiDevice.Addresses[0], // first is always IPv4
			Hostname: apiDevice.Hostname,
			Tags: func() []string {
				if apiDevice.Tags != nil {
					return apiDevice.Tags
				} else {
					return []string{} // so it's not marshaled as "null"
				}
			}(),
		})
	}

	// stable return order
	sort.Slice(devices, func(i, j int) bool { return devices[i].Hostname < devices[j].Hostname })

	return devices, nil
}

func renewAPIKeyEntrypoint() *cobra.Command {
	return &cobra.Command{
		Use:   "renew-apikey",
		Short: "Updates to Lambda the new API key",
		Args:  cobra.NoArgs,
		Run: cli.RunnerNoArgs(func(ctx context.Context, _ *log.Logger) error {
			newApiKey, err := osutil.GetenvRequired("NEW_TAILSCALE_API_KEY")
			if err != nil {
				return err
			}

			awsSession, err := session.NewSession()
			if err != nil {
				return err
			}

			lambdaSvc := lambda.New(awsSession, aws.NewConfig().WithRegion("eu-central-1"))

			_, err = lambdaSvc.UpdateFunctionConfigurationWithContext(ctx, &lambda.UpdateFunctionConfigurationInput{
				FunctionName: Pointer("WebTailscaleDiscovery"),
				Environment: &lambda.Environment{
					Variables: map[string]*string{
						"TAILSCALE_API_KEY": &newApiKey,
					},
				},
			})
			return err
		}),
	}
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	} else {
		return b
	}
}
