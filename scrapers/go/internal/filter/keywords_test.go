package filter

import (
	"go-version/internal/scraper"
	"testing"
)

func TestShouldIncludeJob(t *testing.T) {
	tests := []struct {
		name     string
		job      scraper.Job
		expected bool
	}{
		{
			name: "Valid Junior Golang job",
			job: scraper.Job{
				Title:       "Junior Golang Developer",
				Description: "Build microservices with Go, Docker, and REST APIs.",
				Location:    "Ho Chi Minh",
				PostedDate:  "Recent",
			},
			expected: true,
		},
		{
			name: "Invalid Senior Golang job",
			job: scraper.Job{
				Title:       "Senior Golang Developer",
				Description: "Lead the backend architecture, 5+ years experience required.",
				Location:    "Remote",
				PostedDate:  "Recent",
			},
			expected: false,
		},
		{
			name: "Irrelevant Python job",
			job: scraper.Job{
				Title:       "Junior Python Developer",
				Description: "Develop django backend and analytics tools.",
				Location:    "Ho Chi Minh",
				PostedDate:  "Recent",
			},
			expected: false,
		},
		{
			name: "Hanoi only job rejected",
			job: scraper.Job{
				Title:       "Junior Go Developer",
				Description: "Working in beautiful Hanoi city office.",
				Location:    "Hanoi",
				PostedDate:  "Recent",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldIncludeJob(tt.job)
			if got != tt.expected {
				t.Errorf("ShouldIncludeJob() for %q = %v; want %v", tt.name, got, tt.expected)
			}
		})
	}
}
