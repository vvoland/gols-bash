// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"gotest.tools/v3/assert"
)

func TestRunStopsOnExitAfterShutdownWithOpenConnection(t *testing.T) {
	client, result := startLifecycleServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := client.Call(ctx, protocol.MethodInitialize, map[string]any{}, nil)
	assert.NilError(t, err)
	_, err = client.Call(ctx, protocol.MethodShutdown, nil, nil)
	assert.NilError(t, err)
	assert.NilError(t, client.Notify(ctx, protocol.MethodExit, nil))

	select {
	case err := <-result:
		assert.NilError(t, err)
	case <-ctx.Done():
		t.Fatal("Run did not stop after exit")
	}
}

func TestRunReportsExitWithoutShutdown(t *testing.T) {
	client, result := startLifecycleServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	assert.NilError(t, client.Notify(ctx, protocol.MethodExit, nil))
	select {
	case err := <-result:
		assert.Assert(t, errors.Is(err, ErrExitWithoutShutdown))
	case <-ctx.Done():
		t.Fatal("Run did not stop after exit")
	}
}

func TestRunRejectsRequestsAfterShutdown(t *testing.T) {
	client, result := startLifecycleServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := client.Call(ctx, protocol.MethodShutdown, nil, nil)
	assert.NilError(t, err)
	_, err = client.Call(ctx, protocol.MethodWorkspaceSymbol, protocol.WorkspaceSymbolParams{}, nil)
	assert.Assert(t, err != nil, "requests after shutdown must be rejected")
	rpcErr, ok := err.(*jsonrpc2.Error)
	assert.Assert(t, ok)
	assert.Equal(t, rpcErr.Code, jsonrpc2.InvalidRequest)
	assert.NilError(t, client.Notify(ctx, protocol.MethodExit, nil))

	select {
	case err := <-result:
		assert.NilError(t, err)
	case <-ctx.Done():
		t.Fatal("Run did not stop after exit")
	}
}

func startLifecycleServer(t *testing.T) (jsonrpc2.Conn, <-chan error) {
	t.Helper()
	serverSide, clientSide := net.Pipe()
	result := make(chan error, 1)
	go func() {
		result <- Run(context.Background(), Config{In: serverSide, Out: serverSide})
	}()

	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	client.Go(context.Background(), func(ctx context.Context, reply jsonrpc2.Replier, _ jsonrpc2.Request) error {
		return reply(ctx, nil, nil)
	})
	t.Cleanup(func() { _ = client.Close() })
	return client, result
}
