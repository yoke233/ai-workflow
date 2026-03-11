//go:build !agentsdk

package web

import (
	"context"
	"errors"

	"github.com/yoke233/ai-workflow/internal/acpclient"
)

func isAgentSDKInprocLaunch(_ string) bool { return false }

func newAgentSDKInprocClient(
	_ context.Context,
	_ acpclient.LaunchConfig,
	_ any,
	_ acpclient.ClientCapabilities,
	_ ...acpclient.Option,
) (ChatACPClient, error) {
	return nil, errors.New("agentsdk-inproc not available: build without agentsdk tag")
}
