package session

import (
	"context"
	"fmt"
	"testing"

	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"

	mocks "github.com/bpal410/ssm-helpers/testing"
)

func TestCheckSessionReadiness(t *testing.T) {
	ctx := context.Background()
	client := &mocks.MockSSMClient{
		ConnectionStatus: map[string]ssmtypes.ConnectionStatus{
			"i-notconnected": ssmtypes.ConnectionStatusNotConnected,
			"i-capitalised":  "Connected",
			"i-capitalised2": "NotConnected",
		},
		ConnectionErrors: map[string]error{
			"i-throttled": fmt.Errorf("operation error: %w", mocks.NewAPIError("ThrottlingException", "Rate exceeded")),
			"i-denied":    mocks.NewAPIError("AccessDeniedException", "denied"),
		},
	}

	tests := []struct {
		instanceID string
		ready      bool
		wantErr    bool
	}{
		{instanceID: "i-connected", ready: true},
		{instanceID: "i-notconnected", ready: false},
		{instanceID: "i-capitalised", ready: true},
		{instanceID: "i-capitalised2", ready: false},
		// Throttling is not reported as an error, even when wrapped
		{instanceID: "i-throttled", ready: false},
		{instanceID: "i-denied", ready: false, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.instanceID, func(t *testing.T) {
			ready, err := CheckSessionReadiness(ctx, client, tt.instanceID)
			assert.Equal(t, tt.ready, ready)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
