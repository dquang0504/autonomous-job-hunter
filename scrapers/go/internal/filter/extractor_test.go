package filter

import (
	"testing"
)

func TestExtractSalary(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "Range with currency symbols",
			text:     "The budget is $1000 - $2000 per month.",
			expected: "$1000 - $2000",
		},
		{
			name:     "Range with Vietnamese text",
			text:     "Mức lương khoảng 20 - 30 triệu VND.",
			expected: "20 - 30 triệu",
		},
		{
			name:     "Context base key: thoa thuan",
			text:     "Lương: thoả thuận cho ứng viên có năng lực.",
			expected: "thoả thuận cho ứng viên có năng...",
		},
		{
			name:     "Context base key: up to with currency",
			text:     "We offer Salary up to 3000 EUR for this role.",
			expected: "3000 EUR for this role",
		},
		{
			name:     "Single value fallback",
			text:     "Received a payment of 40 triệu yesterday.",
			expected: "ment of 40 triệu yesterday",
		},
		{
			name:     "No salary information",
			text:     "We are looking for a backend developer.",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSalary(tt.text)
			if got != tt.expected {
				t.Errorf("ExtractSalary(%q) = %q; want %q", tt.text, got, tt.expected)
			}
		})
	}
}
