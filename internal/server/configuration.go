// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"context"
	"encoding/json"
)

type serverSettings struct {
	ShellCheckPath       string
	ShellCheckArguments  []string
	FormatIndentSpaces   *uint
	WorkspaceScanEnabled bool
}

func defaultSettings() serverSettings {
	return serverSettings{ShellCheckPath: "shellcheck", WorkspaceScanEnabled: true}
}

func parseSettings(raw json.RawMessage, current serverSettings) serverSettings {
	if len(raw) == 0 || string(raw) == "null" {
		return current
	}
	settings := current
	applySettingsObject(raw, &settings)
	var nested struct {
		BashIde json.RawMessage `json:"bashIde"`
	}
	if err := json.Unmarshal(raw, &nested); err == nil && len(nested.BashIde) > 0 {
		applySettingsObject(nested.BashIde, &settings)
	}
	return settings
}

func applySettingsObject(raw json.RawMessage, settings *serverSettings) {
	var probe struct {
		ShellCheckPath       *string         `json:"shellcheckPath"`
		ShellCheckArguments  json.RawMessage `json:"shellcheckArguments"`
		FormatIndentSpaces   json.RawMessage `json:"formatIndentSpaces"`
		WorkspaceScanEnabled *bool           `json:"workspaceScanEnabled"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return
	}
	if probe.ShellCheckPath != nil {
		settings.ShellCheckPath = *probe.ShellCheckPath
	}
	if len(probe.ShellCheckArguments) > 0 {
		settings.ShellCheckArguments = parseStringList(probe.ShellCheckArguments, settings.ShellCheckArguments)
	}
	if len(probe.FormatIndentSpaces) > 0 {
		settings.FormatIndentSpaces = parseUintPointer(probe.FormatIndentSpaces, settings.FormatIndentSpaces)
	}
	if probe.WorkspaceScanEnabled != nil {
		settings.WorkspaceScanEnabled = *probe.WorkspaceScanEnabled
	}
}

func parseStringList(raw json.RawMessage, current []string) []string {
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		if one == "" {
			return nil
		}
		return []string{one}
	}
	return current
}

func parseUintPointer(raw json.RawMessage, current *uint) *uint {
	var n int
	if err := json.Unmarshal(raw, &n); err != nil || n < 0 {
		return current
	}
	u := uint(n)
	return &u
}

func (s *bashServer) applySettings(settings serverSettings) {
	s.settingsMu.Lock()
	s.settings = settings
	if settings.ShellCheckPath == "" {
		s.shellcheck = nil
	} else {
		s.shellcheck = newShellCheckRunner(settings.ShellCheckPath, settings.ShellCheckArguments, s.log).lint
	}
	s.settingsMu.Unlock()
}

func (s *bashServer) handleDidChangeConfiguration(ctx context.Context, raw json.RawMessage) {
	var p struct {
		Settings json.RawMessage `json:"settings"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		s.log.Warn("invalid configuration change", "error", err)
		return
	}
	s.settingsMu.RLock()
	current := s.settings
	s.settingsMu.RUnlock()
	s.applySettings(parseSettings(p.Settings, current))
	for _, d := range s.docs.All() {
		s.publishDiagnostics(ctx, d.URI, d.Version, s.documentDiagnostics(ctx, d))
	}
}
