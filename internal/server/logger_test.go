// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"log/slog"
	"os"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestNewLoggerHonorsConfiguredThreshold(t *testing.T) {
	for _, level := range []slog.Level{slog.LevelWarn, slog.LevelError} {
		t.Run(level.String(), func(t *testing.T) {
			path := t.TempDir() + "/server.log"
			logger, closeLog, err := newLogger(Config{LogFile: path, Level: level})
			assert.NilError(t, err)
			logger.Info("info message")
			logger.Warn("warn message")
			logger.Error("error message")
			closeLog()

			data, err := os.ReadFile(path)
			assert.NilError(t, err)
			logs := string(data)
			assert.Assert(t, !strings.Contains(logs, "info message"))
			if level == slog.LevelWarn {
				assert.Assert(t, strings.Contains(logs, "warn message"))
			}
			assert.Assert(t, strings.Contains(logs, "error message"))
		})
	}
}
