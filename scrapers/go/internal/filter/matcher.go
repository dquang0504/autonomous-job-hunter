package filter

import (
	"go-version/internal/scraper"
	"go-version/internal/text"
	"regexp"
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

	// Anti-pattern for Titles
	antiTitleRegex = regexp.MustCompile(`(?i)\b(frontend|front-end|ui/ux|qa|qc|tester|mobile|ios|android|flutter|react native|ba|business analyst|data analyst|data scientist|designer|devops|sysadmin|system admin|security|network|php|wordpress|magento|shopify|sales|marketing|hr)\b`)

	// Social Hiring Signals
	hiringSignalRegex = regexp.MustCompile(`(?i)\b(we(?:'| a)?re hiring|now hiring|is hiring|#hiring|hiring for|job opening|open position|vacancy|vacancies|recruit(?:ing|er)?|apply now|send (?:your )?(?:cv|resume)|jd\b|join our team|headcount|tuy[eê]n|tuy[eê]n d[uụ]ng|c[oơ] h[oộ]i vi[eệ]c l[aà]m|vi[eệ]c l[aà]m|urgent hire|opening for|looking for)\b`)
	roleSignalRegex   = regexp.MustCompile(`(?i)\b(golang|go\s+developer|go\s+backend|go\s+engineer|backend engineer|backend developer|software engineer|software developer|developer|engineer|intern|fresher|junior|entry[\s-]?level|trainee)\b`)
	nonJobRegex       = regexp.MustCompile(`(?i)\b(my pick|my take|thoughts on|thought on|roadmap|tutorial|tip[s]?|learn(?:ing)?|study|review|comparison|showcase|side project|portfolio|demo|boilerplate|template|sample code|code snippet|cheat sheet|resource[s]?|bookmark[s]?|vs\b)\b`)
	candidateRegex    = regexp.MustCompile(`(?i)\b(open to work|looking for (?:a )?job|seeking (?:a )?(?:job|role|opportunit)|find(?:ing)? (?:a )?job|need a job|need work|my cv|my resume|hire me|available for work)\b`)
)

func IsSocialHiringPost(textVal string) bool {
	textVal = text.Normalize(textVal)
	if textVal == "" {
		return false
	}
	if candidateRegex.MatchString(textVal) || nonJobRegex.MatchString(textVal) {
		return false
	}
	return hiringSignalRegex.MatchString(textVal) && roleSignalRegex.MatchString(textVal)
}

func CalculateMatchScore(job scraper.Job) int {
	score := 0
	textVal := text.Normalize(job.Title + " " + job.Description + " " + job.Company)

	if keywordRegex.MatchString(textVal) {
		score += 3
	}
	if includeRegex.MatchString(textVal) {
		score += 3
	}

	location := text.Normalize(job.Location)
	if matchesPrimaryLocation(location) {
		score += 2
	} else if matchesSecondaryLocation(location) {
		score += 1
	}

	if techStackRegex.MatchString(textVal) {
		score += 1
	}

	if experienceRegex.MatchString(textVal) {
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

func matchesPrimaryLocation(location string) bool {
	return hcmRegex.MatchString(location) || canthoRegex.MatchString(location) || remoteRegex.MatchString(location) || globalRegex.MatchString(location)
}

func matchesSecondaryLocation(location string) bool {
	return false
}

func IsHanoiOnly(textVal string) bool {
	textVal = text.Normalize(textVal)
	isHanoi := hanoiRegex.MatchString(textVal)
	isHCM := hcmRegex.MatchString(textVal)
	isCanTho := canthoRegex.MatchString(textVal)
	isRemote := remoteRegex.MatchString(textVal)
	isGlobal := globalRegex.MatchString(textVal)
	return isHanoi && !isHCM && !isCanTho && !isRemote && !isGlobal
}

func HasPreferredLocation(textVal string) bool {
	textVal = text.Normalize(textVal)
	return hcmRegex.MatchString(textVal) || canthoRegex.MatchString(textVal) || remoteRegex.MatchString(textVal) || globalRegex.MatchString(textVal)
}

func IsUnknownLocation(textVal string) bool {
	textVal = text.Normalize(textVal)
	if textVal == "" {
		return true
	}
	return unknownLocRegex.MatchString(textVal)
}

func HasExplicitNonPreferredLocation(location string) bool {
	if IsUnknownLocation(location) {
		return false
	}
	return !HasPreferredLocation(location)
}
