package tests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ensures a nodes resumes normal operation after rebooting
func TestReboot(t *testing.T) {
	e := setupInccounterTest(t, 3, []int{0, 1, 2})
	client := e.newInccounterClientWithFunds()

	_, err := client.PostRequest(incrementFuncName)
	require.NoError(t, err)
	e.counterEquals(1)

	// restart the nodes
	err = e.Clu.RestartNodes(0, 1, 2)
	require.NoError(t, err)

	// after rebooting, the chain should resume processing requests without issues
	_, err = client.PostRequest(incrementFuncName)
	require.NoError(t, err)
	e.counterEquals(2)
}
