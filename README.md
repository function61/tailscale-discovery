![Build status](https://github.com/function61/tailscale-discovery/workflows/Build/badge.svg)

Tailscale discovery

Runs on AWS Lambda as an readonly API that returns only hostnames and IP addresses for the devices.


Why?
----

Currently Tailscale's API token gives ultimate root access to your network, even allows configuring
subnet routers to your devices so it would allow an attacker gaining access to the API key to expose
any internal networks the Tailscale devices are connected to.

I just want to do device discovery with an readonly auth token that exposes a subset of device data.
This way if the token gets exposed it is not a big deal.


Usage
-----

- Create API key in Tailscale and set it as ENV var `TAILSCALE_API_KEY`
- Set your [tailnet ID](https://github.com/tailscale/tailscale/blob/main/api.md#tailnet) as `TAILSCALE_TAILNET`

Serve this Lambda function from Lambda. It is assumed that you have a reverse proxy in front of it
that implements your authorization (even though this is not very sensitive data).
