package filter

import (
	"go-version/internal/scraper"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	keywordRegex    = regexp.MustCompile(`(?i)\b(golang|go\s?lang|go\s?dev|go\s?engineer|backend\s?go)\b`)
	excludeRegex    = regexp.MustCompile(`(?i)\b(senior|lead|manager|principal|staff|architect|(\d{2,}|[3-9])\s*(\+|plus)?\s*years?|2\+\s*years?)\b`)
	includeRegex    = regexp.MustCompile(`(?i)\b(fresher|intern|junior|entry[\s-]?level|graduate|trainee)\b`)
	techStackRegex  = regexp.MustCompile(`(?i)\b(docker|kubernetes|aws|gcp|microservices|rest\s*api|grpc|backend|back-end|fullstack|full-stack)\b`)
	experienceRegex = regexp.MustCompile(`(?i)\b([3-9]|\d{2,})\s*(\+|plus)?\s*(năm|nam|years?|yoe|yrs?)\b`)
	hanoiRegex      = regexp.MustCompile(`(?i)\b(hn|hanoi|ha noi|thu do|ha noi city)\b`)
	hcmRegex        = regexp.MustCompile(`(?i)\b(hcm|ho chi minh|saigon|tphcm|hochiminh|tp hcm)\b`)
	canthoRegex     = regexp.MustCompile(`(?i)\b(can tho|cantho)\b`)
	remoteRegex     = regexp.MustCompile(`(?i)\b(remote|tu xa|từ xa|work from home|wfh)\b`)
	globalRegex     = regexp.MustCompile(`(?i)\b(global|worldwide|world wide|anywhere|from anywhere|international)\b`)
	unknownLocRegex = regexp.MustCompile(`(?i)^\s*(unknown|n/a|na|not specified|unspecified|negotiable|multiple|various|tbd)\s*$`)

	// Social Hiring Signals
	hiringSignalRegex = regexp.MustCompile(`(?i)\b(we(?:'| a)?re hiring|now hiring|is hiring|#hiring|hiring for|job opening|open position|vacancy|vacancies|recruit(?:ing|er)?|apply now|send (?:your )?(?:cv|resume)|jd\b|join our team|headcount|tuy[eê]n|tuy[eê]n d[uụ]ng|c[oơ] h[oộ]i vi[eệ]c l[aà]m|vi[eệ]c l[aà]m|urgent hire|opening for|looking for)\b`)
	roleSignalRegex   = regexp.MustCompile(`(?i)\b(golang|go\s+developer|go\s+backend|go\s+engineer|backend engineer|backend developer|software engineer|software developer|developer|engineer|intern|fresher|junior|entry[\s-]?level|trainee)\b`)
	nonJobRegex       = regexp.MustCompile(`(?i)\b(my pick|my take|thoughts on|thought on|roadmap|tutorial|tip[s]?|learn(?:ing)?|study|review|comparison|showcase|side project|portfolio|demo|boilerplate|template|sample code|code snippet|cheat sheet|resource[s]?|bookmark[s]?|vs\b)\b`)
	candidateRegex    = regexp.MustCompile(`(?i)\b(open to work|looking for (?:a )?job|seeking (?:a )?(?:job|role|opportunit)|find(?:ing)? (?:a )?job|need a job|need work|my cv|my resume|hire me|available for work)\b`)
)

func IsSocialHiringPost(text string) bool {
	text = normalizeText(text)
	if text == "" {
		return false
	}
	if candidateRegex.MatchString(text) || nonJobRegex.MatchString(text) {
		return false
	}
	return hiringSignalRegex.MatchString(text) && roleSignalRegex.MatchString(text)
}

func CalculateMatchScore(job scraper.Job) int {
	score := 0
	text := normalizeText(job.Title + " " + job.Description + " " + job.Company)

	if keywordRegex.MatchString(text) {
		score += 3
	}
	if includeRegex.MatchString(text) {
		score += 3
	}

	location := normalizeText(job.Location)
	if matchesPrimaryLocation(location) {
		score += 2
	} else if matchesSecondaryLocation(location) {
		score += 1
	}

	if techStackRegex.MatchString(text) {
		score += 1
	}

	if experienceRegex.MatchString(text) {
		return 0
	}

	if score > 10 {
		return 10
	}
	if score < 0 {
		return 0
	}
	return score
}

func normalizeText(str string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, str)
	return strings.ToLower(result)
}

func matchesPrimaryLocation(location string) bool {
	return hcmRegex.MatchString(location) || canthoRegex.MatchString(location) || remoteRegex.MatchString(location) || globalRegex.MatchString(location)
}

func matchesSecondaryLocation(location string) bool {
	return false
}

func IsHanoiOnly(text string) bool {
	text = normalizeText(text)
	isHanoi := hanoiRegex.MatchString(text)
	isHCM := hcmRegex.MatchString(text)
	isCanTho := canthoRegex.MatchString(text)
	isRemote := remoteRegex.MatchString(text)
	isGlobal := globalRegex.MatchString(text)
	return isHanoi && !isHCM && !isCanTho && !isRemote && !isGlobal
}

func HasPreferredLocation(text string) bool {
	text = normalizeText(text)
	return hcmRegex.MatchString(text) || canthoRegex.MatchString(text) || remoteRegex.MatchString(text) || globalRegex.MatchString(text)
}

func IsUnknownLocation(text string) bool {
	text = normalizeText(text)
	if text == "" {
		return true
	}
	return unknownLocRegex.MatchString(text)
}

func HasExplicitNonPreferredLocation(location string) bool {
	if IsUnknownLocation(location) {
		return false
	}
	return !HasPreferredLocation(location)
}
