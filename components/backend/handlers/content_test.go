package handlers

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestParseCommandOrder(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "test-command-order-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	commandsDir := filepath.Join(tmpDir, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	tests := []struct {
		name        string
		yamlContent string
		wantIds     []string
		wantErr     bool
		skipCreate  bool // Don't create the file at all
	}{
		{
			name: "valid order file",
			yamlContent: `commands:
  - command.one
  - command.two
  - command.three
`,
			wantIds: []string{"command.one", "command.two", "command.three"},
			wantErr: false,
		},
		{
			name: "empty commands list",
			yamlContent: `commands: []
`,
			wantIds: []string{},
			wantErr: false,
		},
		{
			name:        "no order file",
			skipCreate:  true,
			wantIds:     nil,
			wantErr:     false,
		},
		{
			name: "malformed yaml",
			yamlContent: `commands:
  - command.one
  - [invalid syntax
`,
			wantIds: nil,
			wantErr: true,
		},
		{
			name: "wrong structure",
			yamlContent: `wrongkey:
  - command.one
`,
			wantIds: []string{}, // Empty slice because "commands" key is missing
			wantErr: false,
		},
		{
			name: "single command",
			yamlContent: `commands:
  - single.command
`,
			wantIds: []string{"single.command"},
			wantErr: false,
		},
		{
			name: "commands with dots and underscores",
			yamlContent: `commands:
  - project.feature.do_something
  - project_v2.test.command
`,
			wantIds: []string{"project.feature.do_something", "project_v2.test.command"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orderFile := filepath.Join(commandsDir, "_order.yaml")

			// Clean up from previous test
			os.Remove(orderFile)

			if !tt.skipCreate {
				if err := os.WriteFile(orderFile, []byte(tt.yamlContent), 0644); err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
			}

			gotIds, err := parseCommandOrder(tmpDir)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseCommandOrder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(gotIds, tt.wantIds) {
				t.Errorf("parseCommandOrder() = %v, want %v", gotIds, tt.wantIds)
			}
		})
	}
}

func TestSortCommandsByOrder(t *testing.T) {
	// Helper to create command map entry
	makeCmd := func(id string) map[string]interface{} {
		return map[string]interface{}{
			"id":           id,
			"name":         id,
			"description":  "Test command",
			"slashCommand": "/" + id,
		}
	}

	tests := []struct {
		name       string
		commandMap map[string]map[string]interface{}
		orderedIds []string
		want       []string // IDs in expected order
	}{
		{
			name: "all commands ordered",
			commandMap: map[string]map[string]interface{}{
				"cmd.a": makeCmd("cmd.a"),
				"cmd.b": makeCmd("cmd.b"),
				"cmd.c": makeCmd("cmd.c"),
			},
			orderedIds: []string{"cmd.c", "cmd.a", "cmd.b"},
			want:       []string{"cmd.c", "cmd.a", "cmd.b"},
		},
		{
			name: "partial ordering with alphabetical fallback",
			commandMap: map[string]map[string]interface{}{
				"cmd.a": makeCmd("cmd.a"),
				"cmd.b": makeCmd("cmd.b"),
				"cmd.c": makeCmd("cmd.c"),
				"cmd.d": makeCmd("cmd.d"),
			},
			orderedIds: []string{"cmd.c", "cmd.a"},
			want:       []string{"cmd.c", "cmd.a", "cmd.b", "cmd.d"}, // c, a (ordered), then b, d (alphabetical)
		},
		{
			name: "ordered command not in map",
			commandMap: map[string]map[string]interface{}{
				"cmd.a": makeCmd("cmd.a"),
				"cmd.b": makeCmd("cmd.b"),
			},
			orderedIds: []string{"cmd.x", "cmd.a", "cmd.y"},
			want:       []string{"cmd.a", "cmd.b"}, // x and y skipped, b alphabetical
		},
		{
			name: "duplicate in order list",
			commandMap: map[string]map[string]interface{}{
				"cmd.a": makeCmd("cmd.a"),
				"cmd.b": makeCmd("cmd.b"),
			},
			orderedIds: []string{"cmd.a", "cmd.a", "cmd.b"},
			want:       []string{"cmd.a", "cmd.b"}, // Duplicate ignored
		},
		{
			name: "empty order list",
			commandMap: map[string]map[string]interface{}{
				"cmd.b": makeCmd("cmd.b"),
				"cmd.a": makeCmd("cmd.a"),
				"cmd.c": makeCmd("cmd.c"),
			},
			orderedIds: []string{},
			want:       []string{"cmd.a", "cmd.b", "cmd.c"}, // All alphabetical
		},
		{
			name:       "empty command map",
			commandMap: map[string]map[string]interface{}{},
			orderedIds: []string{"cmd.a", "cmd.b"},
			want:       []string{}, // No commands to return
		},
		{
			name: "single command",
			commandMap: map[string]map[string]interface{}{
				"cmd.only": makeCmd("cmd.only"),
			},
			orderedIds: []string{},
			want:       []string{"cmd.only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortCommandsByOrder(tt.commandMap, tt.orderedIds)

			// Extract IDs from result
			gotIds := make([]string, len(got))
			for i, cmd := range got {
				gotIds[i] = cmd["id"].(string)
			}

			if !reflect.DeepEqual(gotIds, tt.want) {
				t.Errorf("sortCommandsByOrder() = %v, want %v", gotIds, tt.want)
			}

			// Verify no duplicates
			seen := make(map[string]bool)
			for _, id := range gotIds {
				if seen[id] {
					t.Errorf("sortCommandsByOrder() returned duplicate: %s", id)
				}
				seen[id] = true
			}

			// Verify all commands from map are in result
			if len(got) != len(tt.commandMap) {
				t.Errorf("sortCommandsByOrder() returned %d commands, want %d", len(got), len(tt.commandMap))
			}
		})
	}
}

func TestSortCommandsByOrder_AlphabeticalRemaining(t *testing.T) {
	// Test that unordered commands are sorted alphabetically
	makeCmd := func(id string) map[string]interface{} {
		return map[string]interface{}{
			"id":   id,
			"name": id,
		}
	}

	commandMap := map[string]map[string]interface{}{
		"zebra":   makeCmd("zebra"),
		"alpha":   makeCmd("alpha"),
		"charlie": makeCmd("charlie"),
		"bravo":   makeCmd("bravo"),
	}

	orderedIds := []string{"bravo"} // Only bravo is ordered

	result := sortCommandsByOrder(commandMap, orderedIds)

	// Extract IDs
	gotIds := make([]string, len(result))
	for i, cmd := range result {
		gotIds[i] = cmd["id"].(string)
	}

	// Expected: bravo first (ordered), then alpha, charlie, zebra (alphabetical)
	want := []string{"bravo", "alpha", "charlie", "zebra"}

	if !reflect.DeepEqual(gotIds, want) {
		t.Errorf("sortCommandsByOrder() alphabetical ordering failed: got %v, want %v", gotIds, want)
	}

	// Verify the remaining commands after "bravo" are sorted
	remaining := gotIds[1:] // Skip first ordered command
	sortedRemaining := make([]string, len(remaining))
	copy(sortedRemaining, remaining)
	sort.Strings(sortedRemaining)

	if !reflect.DeepEqual(remaining, sortedRemaining) {
		t.Errorf("Remaining commands not alphabetically sorted: got %v, want %v", remaining, sortedRemaining)
	}
}

func TestCommandOrderIntegration(t *testing.T) {
	// Integration test: Create a workflow structure and verify end-to-end behavior
	tmpDir, err := os.MkdirTemp("", "test-workflow-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	commandsDir := filepath.Join(tmpDir, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	// Create command files
	commands := []string{
		"project.feature.implement",
		"project.feature.test",
		"project.feature.document",
		"project.release.build",
		"project.release.deploy",
	}

	for _, cmd := range commands {
		content := `---
displayName: ` + cmd + `
description: Test command
---
# Test Command
`
		filePath := filepath.Join(commandsDir, cmd+".md")
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write command file: %v", err)
		}
	}

	// Create order file
	orderContent := `commands:
  - project.release.deploy
  - project.release.build
  - project.feature.implement
`
	orderFile := filepath.Join(commandsDir, "_order.yaml")
	if err := os.WriteFile(orderFile, []byte(orderContent), 0644); err != nil {
		t.Fatalf("Failed to write order file: %v", err)
	}

	// Parse order
	orderedIds, err := parseCommandOrder(tmpDir)
	if err != nil {
		t.Fatalf("parseCommandOrder() failed: %v", err)
	}

	// Build command map (simulate what ContentWorkflowMetadata does)
	commandMap := make(map[string]map[string]interface{})
	files, _ := os.ReadDir(commandsDir)
	for _, file := range files {
		if file.Name() == "_order.yaml" {
			continue
		}
		if filepath.Ext(file.Name()) == ".md" {
			cmdId := file.Name()[:len(file.Name())-3] // Remove .md
			commandMap[cmdId] = map[string]interface{}{
				"id":   cmdId,
				"name": cmdId,
			}
		}
	}

	// Sort commands
	result := sortCommandsByOrder(commandMap, orderedIds)

	// Extract IDs
	gotIds := make([]string, len(result))
	for i, cmd := range result {
		gotIds[i] = cmd["id"].(string)
	}

	// Expected order:
	// 1. Ordered: deploy, build, implement
	// 2. Alphabetical remaining: document, test
	want := []string{
		"project.release.deploy",
		"project.release.build",
		"project.feature.implement",
		"project.feature.document", // Alphabetical
		"project.feature.test",     // Alphabetical
	}

	if !reflect.DeepEqual(gotIds, want) {
		t.Errorf("Integration test failed:\ngot  %v\nwant %v", gotIds, want)
	}
}
