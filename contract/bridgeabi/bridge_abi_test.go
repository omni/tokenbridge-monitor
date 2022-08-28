package bridgeabi_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/omni/tokenbridge-monitor/contract/bridgeabi"
)

func TestEventSignatures(t *testing.T) {
	t.Parallel()

	require.NotZero(t, bridgeabi.ErcToNativeTransferEventSignature)
	require.NotZero(t, bridgeabi.ErcToNativeUserRequestForAffirmationEventSignature)
}
