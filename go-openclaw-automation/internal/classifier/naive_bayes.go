package classifier

import (
	"encoding/json"
	"math"
	"os"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type Seeds struct {
	Positive []string `json:"positive"`
	Negative []string `json:"negative"`
}

type Model struct {
	DocCounts      map[string]int
	TokenTotals    map[string]int
	TokenCounts    map[string]map[string]int
	VocabularySize int
	TotalDocs      int
}

type ClassificationResult struct {
	Label      string
	IsHiring   bool
	Confidence float64
	Margin     float64
	Source     string
}

var (
	emailRegex    = regexp.MustCompile(`(?i)@[a-z0-9.-]+\.[a-z]{2,}`)
	applyRegex    = regexp.MustCompile(`(?i)\b(cv|resume|apply|inbox)\b`)
	salaryRegex   = regexp.MustCompile(`(?i)\b\d{1,3}\s?(tr|m|usd|vnd|vnđ)\b`)
	locationRegex = regexp.MustCompile(`(?i)\b(remote|hcm|ho chi minh|can tho|worldwide|global)\b`)
	goRoleRegex   = regexp.MustCompile(`(?i)\b(golang|go backend|go developer|go engineer)\b`)
	negativeRegex = regexp.MustCompile(`(?i)\b(open to work|my cv|hire me|my pick|tutorial|roadmap|showcase|side project)\b`)
	tokenRegex    = regexp.MustCompile(`[^a-z0-9@.+#\s-]`)
)

func normalizeText(text string) string {
	if text == "" {
		return ""
	}
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, text)
	return strings.ToLower(result)
}

func tokenize(text string) []string {
	normalized := normalizeText(text)
	cleaned := tokenRegex.ReplaceAllString(normalized, " ")
	tokens := strings.Fields(cleaned)
	
	var validTokens []string
	for _, token := range tokens {
		if len(token) >= 2 {
			validTokens = append(validTokens, token)
		}
	}
	return validTokens
}

func extractFeatures(text string) []string {
	tokens := tokenize(text)
	features := make([]string, len(tokens))
	copy(features, tokens)

	// bigrams
	for i := 0; i < len(tokens)-1; i++ {
		features = append(features, tokens[i]+"__"+tokens[i+1])
	}

	normalized := normalizeText(text)
	
	if emailRegex.MatchString(text) {
		features = append(features, "__has_email__")
	}
	if applyRegex.MatchString(normalized) {
		features = append(features, "__has_apply_signal__")
	}
	if salaryRegex.MatchString(normalized) {
		features = append(features, "__has_salary__")
	}
	if locationRegex.MatchString(normalized) {
		features = append(features, "__has_location__")
	}
	if goRoleRegex.MatchString(normalized) {
		features = append(features, "__has_go_role__")
	}
	if negativeRegex.MatchString(normalized) {
		features = append(features, "__negative_pattern__")
	}

	return features
}

func BuildModel(seedsPath string) (*Model, error) {
	data, err := os.ReadFile(seedsPath)
	if err != nil {
		return nil, err
	}

	var seeds Seeds
	if err := json.Unmarshal(data, &seeds); err != nil {
		return nil, err
	}

	classDocs := map[string][]string{
		"hiring":     seeds.Positive,
		"non_hiring": seeds.Negative,
	}

	docCounts := make(map[string]int)
	tokenTotals := make(map[string]int)
	tokenCounts := make(map[string]map[string]int)
	vocabulary := make(map[string]struct{})
	totalDocs := 0

	for label, docs := range classDocs {
		docCounts[label] = len(docs)
		totalDocs += len(docs)
		tokenTotals[label] = 0
		tokenCounts[label] = make(map[string]int)

		for _, doc := range docs {
			features := extractFeatures(doc)
			for _, feature := range features {
				vocabulary[feature] = struct{}{}
				tokenTotals[label]++
				tokenCounts[label][feature]++
			}
		}
	}

	return &Model{
		DocCounts:      docCounts,
		TokenTotals:    tokenTotals,
		TokenCounts:    tokenCounts,
		VocabularySize: len(vocabulary),
		TotalDocs:      totalDocs,
	}, nil
}

func (m *Model) scoreLabel(label string, features []string) float64 {
	prior := math.Log(float64(m.DocCounts[label]) / float64(m.TotalDocs))
	denom := float64(m.TokenTotals[label] + m.VocabularySize)

	score := prior
	for _, feature := range features {
		count := float64(m.TokenCounts[label][feature])
		score += math.Log((count + 1) / denom)
	}

	return score
}

func sigmoid(x float64) float64 {
	return 1 / (1 + math.Exp(-x))
}

func ClassifyWithSeedModel(model *Model, text string) ClassificationResult {
	features := extractFeatures(text)
	if len(features) == 0 {
		return ClassificationResult{
			Label:      "non_hiring",
			IsHiring:   false,
			Confidence: 0.5,
			Margin:     0,
			Source:     "seed",
		}
	}

	hiringScore := model.scoreLabel("hiring", features)
	nonHiringScore := model.scoreLabel("non_hiring", features)
	margin := hiringScore - nonHiringScore
	confidence := sigmoid(math.Abs(margin))
	isHiring := margin > 0

	label := "non_hiring"
	if isHiring {
		label = "hiring"
	}

	return ClassificationResult{
		Label:      label,
		IsHiring:   isHiring,
		Confidence: confidence,
		Margin:     margin,
		Source:     "seed",
	}
}
