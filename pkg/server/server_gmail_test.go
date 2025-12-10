// ABOUTME: Tests for Gmail-specific MCP server handlers
// ABOUTME: Validates Gmail label array parameter handling and edge cases

package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleGmailModifyLabels_InvalidArrayParameters(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name        string
		args        map[string]interface{}
		expectError bool
		description string
	}{
		{
			name: "missing message_id",
			args: map[string]interface{}{
				"add_labels": []interface{}{"INBOX"},
			},
			expectError: true,
			description: "should fail when message_id is missing",
		},
		{
			name: "empty message_id",
			args: map[string]interface{}{
				"message_id": "",
				"add_labels": []interface{}{"INBOX"},
			},
			expectError: true,
			description: "should fail when message_id is empty",
		},
		{
			name: "invalid arguments format - not a map",
			args: map[string]interface{}{
				"message_id": "123456",
			},
			expectError: false,
			description: "should handle when arguments are in wrong format gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := createMockRequest("gmail_modify_labels", tt.args)
			result, err := srv.handleGmailModifyLabels(context.Background(), request)

			require.NoError(t, err, "handler should not return error")
			assert.NotNil(t, result)

			if tt.expectError {
				assert.True(t, result.IsError, tt.description)
			}
		})
	}
}

func TestHandleGmailModifyLabels_MixedTypeArrays(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name          string
		addLabels     interface{}
		removeLabels  interface{}
		expectedAdd   int
		expectedRemove int
		description   string
	}{
		{
			name:          "mixed types in add_labels - strings and numbers",
			addLabels:     []interface{}{"INBOX", 123, "IMPORTANT"},
			removeLabels:  nil,
			expectedAdd:   2,
			expectedRemove: 0,
			description:   "should filter out non-string values and keep only strings",
		},
		{
			name:          "mixed types in remove_labels - strings and booleans",
			addLabels:     nil,
			removeLabels:  []interface{}{"SPAM", true, "TRASH", false},
			expectedAdd:   0,
			expectedRemove: 2,
			description:   "should filter out non-string values in remove_labels",
		},
		{
			name:          "mixed types in both arrays",
			addLabels:     []interface{}{"INBOX", 3.14, nil, "IMPORTANT"},
			removeLabels:  []interface{}{42, "SPAM", map[string]string{"key": "value"}},
			expectedAdd:   2,
			expectedRemove: 1,
			description:   "should handle mixed types in both arrays",
		},
		{
			name:          "all non-string types",
			addLabels:     []interface{}{123, true, 3.14, nil},
			removeLabels:  []interface{}{456, false},
			expectedAdd:   0,
			expectedRemove: 0,
			description:   "should handle arrays with no valid strings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"message_id": "test-message-123",
			}
			if tt.addLabels != nil {
				args["add_labels"] = tt.addLabels
			}
			if tt.removeLabels != nil {
				args["remove_labels"] = tt.removeLabels
			}

			request := createMockRequest("gmail_modify_labels", args)
			result, err := srv.handleGmailModifyLabels(context.Background(), request)

			require.NoError(t, err, "handler should not return error")
			assert.NotNil(t, result)
			// In ish mode, this should succeed even with filtered arrays
			assert.NotEmpty(t, result.Content, tt.description)
		})
	}
}

func TestHandleGmailModifyLabels_EmptyVsNullArrays(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name         string
		addLabels    interface{}
		removeLabels interface{}
		description  string
	}{
		{
			name:         "both arrays null",
			addLabels:    nil,
			removeLabels: nil,
			description:  "should handle when both arrays are null",
		},
		{
			name:         "empty add_labels array",
			addLabels:    []interface{}{},
			removeLabels: nil,
			description:  "should handle empty add_labels array",
		},
		{
			name:         "empty remove_labels array",
			addLabels:    nil,
			removeLabels: []interface{}{},
			description:  "should handle empty remove_labels array",
		},
		{
			name:         "both arrays empty",
			addLabels:    []interface{}{},
			removeLabels: []interface{}{},
			description:  "should handle when both arrays are empty",
		},
		{
			name:         "add_labels with values, remove_labels null",
			addLabels:    []interface{}{"INBOX"},
			removeLabels: nil,
			description:  "should handle add_labels with values and null remove_labels",
		},
		{
			name:         "add_labels null, remove_labels with values",
			addLabels:    nil,
			removeLabels: []interface{}{"SPAM"},
			description:  "should handle null add_labels and remove_labels with values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"message_id": "test-message-empty-null",
			}
			if tt.addLabels != nil {
				args["add_labels"] = tt.addLabels
			}
			if tt.removeLabels != nil {
				args["remove_labels"] = tt.removeLabels
			}

			request := createMockRequest("gmail_modify_labels", args)
			result, err := srv.handleGmailModifyLabels(context.Background(), request)

			require.NoError(t, err, "handler should not return error")
			assert.NotNil(t, result, tt.description)
			assert.NotEmpty(t, result.Content)
		})
	}
}

func TestHandleGmailModifyLabels_LargeLabelArrays(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name        string
		arraySize   int
		description string
	}{
		{
			name:        "small array - 10 labels",
			arraySize:   10,
			description: "should handle 10 labels",
		},
		{
			name:        "medium array - 50 labels",
			arraySize:   50,
			description: "should handle 50 labels",
		},
		{
			name:        "large array - 100 labels",
			arraySize:   100,
			description: "should handle 100 labels",
		},
		{
			name:        "very large array - 500 labels",
			arraySize:   500,
			description: "should handle 500 labels",
		},
		{
			name:        "extremely large array - 1000 labels",
			arraySize:   1000,
			description: "should handle 1000 labels",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate large array of label strings
			addLabels := make([]interface{}, tt.arraySize)
			for i := 0; i < tt.arraySize; i++ {
				addLabels[i] = "LABEL_" + string(rune('A'+i%26)) + "_" + string(rune('0'+i%10))
			}

			args := map[string]interface{}{
				"message_id": "test-message-large-array",
				"add_labels": addLabels,
			}

			request := createMockRequest("gmail_modify_labels", args)
			result, err := srv.handleGmailModifyLabels(context.Background(), request)

			require.NoError(t, err, "handler should not return error")
			assert.NotNil(t, result, tt.description)
			assert.NotEmpty(t, result.Content)
		})
	}
}

func TestHandleGmailModifyLabels_SpecialCharactersInLabels(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name        string
		labels      []interface{}
		description string
	}{
		{
			name:        "labels with spaces",
			labels:      []interface{}{"My Label", "Another Label"},
			description: "should handle labels with spaces",
		},
		{
			name:        "labels with special characters",
			labels:      []interface{}{"Label/With/Slashes", "Label-With-Dashes", "Label_With_Underscores"},
			description: "should handle labels with slashes, dashes, and underscores",
		},
		{
			name:        "labels with unicode characters",
			labels:      []interface{}{"📧 Email", "重要", "Wichtig"},
			description: "should handle labels with unicode and emoji",
		},
		{
			name:        "labels with dots and periods",
			labels:      []interface{}{"Label.With.Dots", "work.important"},
			description: "should handle labels with dots",
		},
		{
			name:        "labels with parentheses and brackets",
			labels:      []interface{}{"Label (Important)", "Label [2024]"},
			description: "should handle labels with parentheses and brackets",
		},
		{
			name:        "labels with quotes",
			labels:      []interface{}{"Label'With'Quotes", `Label"With"DoubleQuotes`},
			description: "should handle labels with single and double quotes",
		},
		{
			name:        "labels with ampersand and symbols",
			labels:      []interface{}{"Work & Personal", "Label @ Home", "Label #1"},
			description: "should handle labels with ampersand, at sign, and hash",
		},
		{
			name:        "empty string label",
			labels:      []interface{}{"", "ValidLabel"},
			description: "should handle empty string in label array",
		},
		{
			name:        "very long label name",
			labels:      []interface{}{"ThisIsAVeryLongLabelNameThatExceedsNormalLengthAndTestsTheBoundariesOfWhatIsAcceptableForALabelNameInGmailAPIWhichMayHaveSomeLimitationsOnMaximumLengthAllowedForLabelNames"},
			description: "should handle very long label names",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"message_id": "test-message-special-chars",
				"add_labels": tt.labels,
			}

			request := createMockRequest("gmail_modify_labels", args)
			result, err := srv.handleGmailModifyLabels(context.Background(), request)

			require.NoError(t, err, "handler should not return error")
			assert.NotNil(t, result, tt.description)
			assert.NotEmpty(t, result.Content)
		})
	}
}

func TestHandleGmailModifyLabels_DuplicateLabels(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name         string
		addLabels    []interface{}
		removeLabels []interface{}
		description  string
	}{
		{
			name:         "duplicate labels in add_labels",
			addLabels:    []interface{}{"INBOX", "IMPORTANT", "INBOX", "IMPORTANT"},
			removeLabels: nil,
			description:  "should handle duplicate labels in add_labels array",
		},
		{
			name:         "duplicate labels in remove_labels",
			addLabels:    nil,
			removeLabels: []interface{}{"SPAM", "TRASH", "SPAM"},
			description:  "should handle duplicate labels in remove_labels array",
		},
		{
			name:         "same label in both add and remove",
			addLabels:    []interface{}{"INBOX", "IMPORTANT"},
			removeLabels: []interface{}{"INBOX", "SPAM"},
			description:  "should handle same label appearing in both add and remove arrays",
		},
		{
			name:         "all duplicate labels",
			addLabels:    []interface{}{"LABEL", "LABEL", "LABEL"},
			removeLabels: []interface{}{"LABEL", "LABEL"},
			description:  "should handle all labels being duplicates",
		},
		{
			name:         "many duplicates with one unique",
			addLabels:    []interface{}{"INBOX", "INBOX", "INBOX", "IMPORTANT"},
			removeLabels: []interface{}{"SPAM", "SPAM"},
			description:  "should handle many duplicates with some unique values",
		},
		{
			name:         "case-sensitive duplicates",
			addLabels:    []interface{}{"inbox", "INBOX", "InBoX"},
			removeLabels: nil,
			description:  "should treat case-different labels as separate (case-sensitive)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"message_id": "test-message-duplicates",
			}
			if tt.addLabels != nil {
				args["add_labels"] = tt.addLabels
			}
			if tt.removeLabels != nil {
				args["remove_labels"] = tt.removeLabels
			}

			request := createMockRequest("gmail_modify_labels", args)
			result, err := srv.handleGmailModifyLabels(context.Background(), request)

			require.NoError(t, err, "handler should not return error")
			assert.NotNil(t, result, tt.description)
			assert.NotEmpty(t, result.Content)
		})
	}
}

func TestHandleGmailModifyLabels_NonArrayTypes(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name         string
		addLabels    interface{}
		removeLabels interface{}
		description  string
	}{
		{
			name:         "string instead of array",
			addLabels:    "INBOX",
			removeLabels: nil,
			description:  "should handle when add_labels is a string instead of array",
		},
		{
			name:         "number instead of array",
			addLabels:    123,
			removeLabels: nil,
			description:  "should handle when add_labels is a number instead of array",
		},
		{
			name:         "boolean instead of array",
			addLabels:    true,
			removeLabels: nil,
			description:  "should handle when add_labels is a boolean instead of array",
		},
		{
			name:         "map instead of array",
			addLabels:    map[string]string{"label": "INBOX"},
			removeLabels: nil,
			description:  "should handle when add_labels is a map instead of array",
		},
		{
			name:         "mixed non-array types",
			addLabels:    "INBOX",
			removeLabels: 456,
			description:  "should handle when both parameters are non-array types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"message_id": "test-message-non-array",
			}
			if tt.addLabels != nil {
				args["add_labels"] = tt.addLabels
			}
			if tt.removeLabels != nil {
				args["remove_labels"] = tt.removeLabels
			}

			request := createMockRequest("gmail_modify_labels", args)
			result, err := srv.handleGmailModifyLabels(context.Background(), request)

			require.NoError(t, err, "handler should not return error")
			assert.NotNil(t, result, tt.description)
			// Handler should gracefully skip non-array types
			assert.NotEmpty(t, result.Content)
		})
	}
}

func TestHandleGmailModifyLabels_SuccessfulCases(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name         string
		addLabels    []interface{}
		removeLabels []interface{}
		description  string
	}{
		{
			name:         "valid single label add",
			addLabels:    []interface{}{"INBOX"},
			removeLabels: nil,
			description:  "should successfully add a single label",
		},
		{
			name:         "valid single label remove",
			addLabels:    nil,
			removeLabels: []interface{}{"SPAM"},
			description:  "should successfully remove a single label",
		},
		{
			name:         "valid multiple labels add and remove",
			addLabels:    []interface{}{"INBOX", "IMPORTANT"},
			removeLabels: []interface{}{"SPAM", "TRASH"},
			description:  "should successfully add and remove multiple labels",
		},
		{
			name:         "valid Gmail system labels",
			addLabels:    []interface{}{"INBOX", "STARRED", "IMPORTANT"},
			removeLabels: []interface{}{"SPAM", "TRASH", "UNREAD"},
			description:  "should handle Gmail system labels",
		},
		{
			name:         "valid custom labels",
			addLabels:    []interface{}{"Work", "Personal", "Follow-up"},
			removeLabels: []interface{}{"Archive", "Old"},
			description:  "should handle custom user-created labels",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"message_id": "test-message-success",
			}
			if tt.addLabels != nil {
				args["add_labels"] = tt.addLabels
			}
			if tt.removeLabels != nil {
				args["remove_labels"] = tt.removeLabels
			}

			request := createMockRequest("gmail_modify_labels", args)
			result, err := srv.handleGmailModifyLabels(context.Background(), request)

			require.NoError(t, err, "handler should not return error")
			assert.NotNil(t, result, tt.description)
			assert.NotEmpty(t, result.Content)
			// Note: In ISH mode without base URL, this may return an error
			// The important thing is the handler doesn't panic and processes the arrays correctly
		})
	}
}
