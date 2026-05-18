package filter

import (
	"regexp"
	"strings"
)

var (
	// 1. Range-based extraction: matches explicit currency ranges anywhere
	// Example: $1000 - $2000, 20 - 30 triệu, 50k - 60k EUR, £40,000 - 50,000
	rangeRegex = regexp.MustCompile(`(?i)(?:[\$€£¥]\s*)?\d+(?:[,.]\d+)*(?:\s*(?:k|m|triệu|tr\b|vnd|usd|eur|sgd|gbp|aud|cad))?\s*(?:-|~|to|đến)\s*(?:[\$€£¥]\s*)?\d+(?:[,.]\d+)*\s*(?:k|m|triệu|tr\b|vnd|usd|eur|sgd|gbp|aud|cad)?`)

	// 2. Context-based extraction: captures what follows "Lương", "Salary", "Mức lương"
	// Example: "Lương: thoả thuận", "Salary up to 3000 EUR", "Mức lương hấp dẫn"
	// Stops capturing at newline, period, comma, or semicolon to avoid bleeding into next sentences.
	contextRegex = regexp.MustCompile(`(?i)(?:mức\s+lương|lương|salary|compensation|pay)\s*(?::|[-~]|from|up to|từ|lên tới|lên đến)?\s*([^\n\.,;]{4,40})`)

	// 3. Single value fallback (has clear currency symbol)
	// Example: $2000, 40 triệu, 5000 SGD
	singleRegex = regexp.MustCompile(`(?i)(?:[\$€£¥]\s*\d+(?:[,.]\d+)*(?:\s*(?:k|m))?)|(?:\b\d+(?:[,.]\d+)*\s*(?:triệu|tr\b|vnd|usd|eur|sgd|gbp|aud|cad))`)
)

// ExtractSalary attempts to find a salary string from unstructured text flexibly
func ExtractSalary(text string) string {
	// 1. Try explicit ranges first (most mathematically accurate)
	if loc := rangeRegex.FindString(text); loc != "" {
		return cleanExtracted(loc)
	}

	// 2. Try context-based (great for text like "Thoả thuận" or "Up to X EUR")
	if match := contextRegex.FindStringSubmatch(text); len(match) > 1 {
		return cleanExtracted(match[1])
	}

	// 3. Try single clear currency value anywhere
	if loc := singleRegex.FindString(text); loc != "" {
		return cleanExtracted(loc)
	}

	return ""
}

func cleanExtracted(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ") // clean extra whitespaces
	text = strings.TrimSpace(text)
	if len(text) > 40 {
		return text[:40] + "..."
	}
	return text
}
