package filter

import (
	"regexp"
	"strings"
)

var (
	// Matches: 20 triệu, 20-30 triệu, 20tr, 20 - 30 tr, 1000$, 1000 - 1500 USD, up to 2000$, etc.
	vndRegex = regexp.MustCompile(`(?i)(?:lương\s*(?:net|gross)?\s*)?(?:từ\s*|up to\s*lên tới\s*|lên đến\s*|đến\s*|-|~)?\s*\d+(?:\.\d+)?\s*(?:-|~|đến|lên tới)\s*\d+(?:\.\d+)?\s*(?:triệu|tr|m|vnđ|vnd)`)
	vndRegex2 = regexp.MustCompile(`(?i)(?:lương\s*(?:net|gross)?\s*)?(?:từ\s*|up to\s*lên tới\s*|lên đến\s*|đến\s*|-|~)?\s*\d+(?:\.\d+)?\s*(?:triệu|tr|m|vnđ|vnd)`)
	
	usdRegex = regexp.MustCompile(`(?i)(?:lương\s*(?:net|gross)?\s*)?(?:từ\s*|up to\s*lên tới\s*|lên đến\s*|đến\s*|-|~)?\s*\d+(?:,\d{3})?(?:\.\d+)?\s*(?:-|~|đến|to)\s*\d+(?:,\d{3})?(?:\.\d+)?\s*(?:usd|\$)`)
	usdRegex2 = regexp.MustCompile(`(?i)(?:lương\s*(?:net|gross)?\s*)?(?:từ\s*|up to\s*|lên tới\s*|lên đến\s*|đến\s*|-|~)?\s*\$?\s*\d+(?:,\d{3})?(?:\.\d+)?\s*(?:usd|\$)?`)
)

// ExtractSalary attempts to find a salary string from unstructured text
func ExtractSalary(text string) string {
	// First try USD ranges
	if loc := usdRegex.FindString(text); loc != "" {
		return cleanExtracted(loc)
	}
	// Then try VND ranges
	if loc := vndRegex.FindString(text); loc != "" {
		return cleanExtracted(loc)
	}
	// Then try single USD values if they have clear context like "lương" or "$"
	if loc := regexp.MustCompile(`(?i)(?:lương\s*(?:net|gross)?\s*).{0,20}\$?\d+(?:,\d{3})?(?:\.\d+)?\s*(?:usd|\$)?`).FindString(text); loc != "" {
		return cleanExtracted(loc)
	}
	// Then try single VND values if they have clear context
	if loc := regexp.MustCompile(`(?i)(?:lương\s*(?:net|gross)?\s*).{0,20}\d+(?:\.\d+)?\s*(?:triệu|tr\b)`).FindString(text); loc != "" {
		return cleanExtracted(loc)
	}

	return ""
}

func cleanExtracted(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ") // remove extra spaces
	if len(text) > 40 {
		return text[:40] + "..."
	}
	return text
}
