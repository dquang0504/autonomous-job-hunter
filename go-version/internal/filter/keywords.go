package filter

import (
	"go-version/internal/scraper"
)

func ShouldIncludeJob(job scraper.Job) bool {
	text := normalizeText(job.Title + " " + job.Description)
	//must contain golang/go
	if !keywordRegex.MatchString(text) {
		return false
	}

	//anti-title check: reject if title has toxic keywords AND does NOT contain go/golang
	titleText := normalizeText(job.Title)
	if antiTitleRegex.MatchString(titleText) && !keywordRegex.MatchString(titleText) {
		return false
	}

	//must not contain exclude keywords
	if excludeRegex.MatchString(text) {
		return false
	}

	//must not have >= 3 years exp
	if experienceRegex.MatchString(text) {
		return false
	}

	//must not be explicitly in a non-preferred location
	if HasExplicitNonPreferredLocation(job.Location) {
		return false
	}

	//must not be Hanoi ONLY
	fullText := text + " " + normalizeText(job.Location)
	if IsHanoiOnly(fullText) {
		return false
	}

	//must be recent (<= 60 days)
	if !IsRecentJob(job.PostedDate) {
		return false
	}

	return true
}
