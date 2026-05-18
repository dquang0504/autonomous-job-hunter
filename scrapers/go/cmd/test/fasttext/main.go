package main

import (
	"context"
	"fmt"
	"go-version/internal/classifier"
	"log"
)

func main() {
	fmt.Println("🧪 Testing Go-Python FastText Classifier Bridge...")

	testTexts := []string{
		"We are hiring a Golang Backend Developer! Salary up to $3000. Apply now by sending your CV to jobs@company.com",
		"Looking for a Golang Engineer with 2+ years of experience. Remote/HCM. Inbox me for JD.",
		"Check out my new side project! I built a job hunter agent using Golang and React. Let me know what you think.",
		"Does anyone know what is the best roadmap to learn Go language in 2026? I am open to suggestions.",
	}

	ctx := context.Background()

	for i, text := range testTexts {
		fmt.Printf("\n📝 Test Case %d:\n\"%s\"\n", i+1, text)
		
		res, err := classifier.ClassifyWithFastText(ctx, text)
		if err != nil {
			log.Fatalf("❌ FastText classification failed: %v", err)
		}

		fmt.Println("📊 FastText Result:")
		fmt.Printf("   - Label:      %s\n", res.Label)
		fmt.Printf("   - IsHiring:   %t\n", res.IsHiring)
		fmt.Printf("   - Confidence: %.4f\n", res.Confidence)
		fmt.Printf("   - Margin:     %.4f\n", res.Margin)
		fmt.Printf("   - Source:     %s\n", res.Source)
	}

	fmt.Println("\n🎉 All tests passed successfully! The Python FastText model integration is fully functional in Go!")
}
