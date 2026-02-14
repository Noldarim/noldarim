// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package agents

import (
	"testing"
)

func TestParseSummary(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantReason string
		wantCount  int // number of changes expected
		wantNil    bool
		wantErr    bool
	}{
		{
			name: "valid summary",
			output: `Some agent output here...

Did some work.

---SUMMARY---
{"reason": "Fixed the bug in login", "changes": ["Updated auth logic", "Added validation"]}
---END SUMMARY---
`,
			wantReason: "Fixed the bug in login",
			wantCount:  2,
		},
		{
			name: "summary with single change",
			output: `---SUMMARY---
{"reason": "Quick fix", "changes": ["Fixed typo"]}
---END SUMMARY---`,
			wantReason: "Quick fix",
			wantCount:  1,
		},
		{
			name:    "no summary block",
			output:  "Just regular output without any summary",
			wantNil: true,
		},
		{
			name: "malformed JSON in summary",
			output: `---SUMMARY---
{not valid json}
---END SUMMARY---`,
			wantErr: true,
		},
		{
			name: "empty changes array",
			output: `---SUMMARY---
{"reason": "No changes made", "changes": []}
---END SUMMARY---`,
			wantReason: "No changes made",
			wantCount:  0,
		},
		{
			name: "summary with multiline content before",
			output: `Line 1
Line 2
Line 3

---SUMMARY---
{"reason": "Test", "changes": ["a", "b", "c"]}
---END SUMMARY---`,
			wantReason: "Test",
			wantCount:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, err := ParseStepSummary(tt.output)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseStepSummary() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseStepSummary() unexpected error: %v", err)
				return
			}

			if tt.wantNil {
				if summary != nil {
					t.Errorf("ParseStepSummary() expected nil, got %+v", summary)
				}
				return
			}

			if summary == nil {
				t.Errorf("ParseStepSummary() expected summary, got nil")
				return
			}

			if summary.Reason != tt.wantReason {
				t.Errorf("ParseStepSummary() reason = %q, want %q", summary.Reason, tt.wantReason)
			}

			if len(summary.Changes) != tt.wantCount {
				t.Errorf("ParseStepSummary() changes count = %d, want %d", len(summary.Changes), tt.wantCount)
			}
		})
	}
}

func TestGetAgentOutputWithoutSummary(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
	}{
		{
			name: "removes summary block",
			input: `Agent output here.

---SUMMARY---
{"reason": "test", "changes": ["a"]}
---END SUMMARY---
`,
			want: "Agent output here.",
		},
		{
			name:  "no summary to remove",
			input: "Just regular output",
			want:  "Just regular output",
		},
		{
			name: "removes summary keeping content",
			input: `Before summary
---SUMMARY---
{"reason": "x", "changes": []}
---END SUMMARY---
After summary`,
			want: "Before summary\n\nAfter summary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAgentOutputWithoutSummary(tt.input)
			if got != tt.want {
				t.Errorf("GetAgentOutputWithoutSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}
