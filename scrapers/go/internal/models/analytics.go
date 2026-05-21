package models

import "time"

// VercelStats represents the parsed analytics from Vercel
type VercelStats struct {
Visitors   string `json:"visitors"`
PageViews  string `json:"pageViews"`
BounceRate string `json:"bounceRate"`
TopPages   string `json:"topPages"`
Referrers  string `json:"referrers"`
Countries  string `json:"countries"`
Devices    string `json:"devices"`
OS         string `json:"os"`
}

// CloudflareStats represents the worker invocations stats
type CloudflareStats struct {
Hash          string                                `json:"hash"`
LastSentDate  string                                `json:"lastSentDate"`
Timestamp     string                                `json:"timestamp"`
TotalRequests int                                   `json:"totalRequests"`
Stats         map[string]CloudflareWorkerInvocation `json:"stats"`
}

type CloudflareWorkerInvocation struct {
Requests int `json:"requests"`
Errors   int `json:"errors"`
}

// ScraperIncident represents an error or block event
type ScraperIncident struct {
Platform      string    `json:"platform"`
IncidentType  string    `json:"incidentType"`
ErrorMessage  string    `json:"errorMessage"`
ScreenshotURL string    `json:"screenshotUrl"`
OccurredAt    time.Time `json:"occurredAt"`
}
