// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
)

func TestPickPositionEncoding(t *testing.T) {
	cases := []struct {
		name string
		json string
		want PositionEncoding
	}{
		{"empty", `{}`, EncodingUTF16},
		{"utf-16 only", `{"capabilities":{"general":{"positionEncodings":["utf-16"]}}}`, EncodingUTF16},
		{"utf-8 offered", `{"capabilities":{"general":{"positionEncodings":["utf-8","utf-16"]}}}`, EncodingUTF8},
		{"unknown encoding", `{"capabilities":{"general":{"positionEncodings":["utf-32"]}}}`, EncodingUTF16},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pickPositionEncoding(json.RawMessage(c.json))
			assert.Equal(t, got, c.want)
		})
	}
}

func TestInitializeAdvertisesPickedEncoding(t *testing.T) {
	s, _ := newTestServer()
	res := s.initialize([]byte(`{"capabilities":{"general":{"positionEncodings":["utf-8"]}}}`))
	assert.Equal(t, res.Capabilities.PositionEncoding, EncodingUTF8)
	assert.Equal(t, s.encoding(), EncodingUTF8)
	assert.Equal(t, res.Capabilities.DefinitionProvider, true)
	assert.Assert(t, res.Capabilities.CompletionProvider != nil)
}
