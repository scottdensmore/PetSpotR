package notification

import (
	"bytes"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"math"
	"strings"
	texttemplate "text/template"
)

// MatchAlertEmailData contains all parameters needed to render a match alert email notification.
type MatchAlertEmailData struct {
	MatchID           string  `json:"matchId"`
	PetID             string  `json:"petId"`
	PetName           string  `json:"petName"`
	PhotoURL          string  `json:"photoUrl"`
	Score             float64 `json:"score"`
	ConfidencePercent int     `json:"confidencePercent"`
	Explanation       string  `json:"explanation"`
	DetailsURL        string  `json:"detailsUrl,omitempty"`
}

// Validate checks that all required fields are present.
func (d *MatchAlertEmailData) Validate() error {
	if strings.TrimSpace(d.MatchID) == "" {
		return errors.New("template: matchId is required")
	}
	if strings.TrimSpace(d.PhotoURL) == "" {
		return errors.New("template: pet photo link is required")
	}
	if strings.TrimSpace(d.Explanation) == "" {
		return errors.New("template: score explanation is required")
	}
	return nil
}

// EmailContent contains the rendered email subject, HTML body, and plain text fallback.
type EmailContent struct {
	Subject  string
	HTMLBody string
	TextBody string
}

const matchAlertHTMLTemplate = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Match Found for {{.PetName}}</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 20px; background-color: #f9f9f9; }
.container { max-width: 600px; margin: auto; background: #fff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
.header { background-color: #4f46e5; color: white; padding: 24px; text-align: center; }
.content { padding: 24px; }
.photo { text-align: center; margin: 16px 0; }
.photo img { max-width: 100%; height: auto; border-radius: 8px; border: 1px solid #e5e7eb; }
.badge { display: inline-block; padding: 4px 12px; border-radius: 9999px; background: #ecfdf5; color: #065f46; font-weight: 600; }
.card { background: #f3f4f6; border-radius: 6px; padding: 16px; margin: 16px 0; }
.footer { text-align: center; font-size: 12px; color: #9ca3af; padding: 16px; }
.btn { display: inline-block; padding: 10px 20px; background-color: #4f46e5; color: #ffffff; text-decoration: none; border-radius: 6px; font-weight: 600; margin-top: 12px; }
</style>
</head>
<body>
<div class="container">
  <div class="header">
    <h1>🎉 Match Found for {{.PetName}}!</h1>
  </div>
  <div class="content">
    <p>Good news! We found a potential match for your pet with <strong>{{.ConfidencePercent}}%</strong> confidence score.</p>
    <div class="photo">
      <a href="{{.PhotoURL}}" target="_blank" rel="noopener noreferrer">
        <img src="{{.PhotoURL}}" alt="Photo of found pet" />
      </a>
      <p><a href="{{.PhotoURL}}" target="_blank" rel="noopener noreferrer">View Full Pet Photo</a></p>
    </div>
    <div class="card">
      <h3>Match Details</h3>
      <p><strong>Match ID:</strong> <code>{{.MatchID}}</code></p>
      <p><strong>Score Explanation:</strong> {{.Explanation}}</p>
    </div>
    {{if .DetailsURL}}
    <div style="text-align: center;">
      <a href="{{.DetailsURL}}" class="btn">View Match in PetSpotR</a>
    </div>
    {{end}}
  </div>
  <div class="footer">
    <p>Sent by PetSpotR &bull; Helping reunite pets with their families</p>
  </div>
</div>
</body>
</html>`

const matchAlertTextTemplate = `PetSpotR: Potential Match Found for {{.PetName}}!

We found a potential match with a confidence score of {{.ConfidencePercent}}%.

Match Details:
- Match ID: {{.MatchID}}
- Score Explanation: {{.Explanation}}
- Pet Photo Link: {{.PhotoURL}}
{{if .DetailsURL}}- View Match: {{.DetailsURL}}
{{end}}
Please review this candidate match as soon as possible.
-- 
PetSpotR Alerts
`

var (
	parsedHTMLTmpl = htmltemplate.Must(htmltemplate.New("matchAlertHTML").Parse(matchAlertHTMLTemplate))
	parsedTextTmpl = texttemplate.Must(texttemplate.New("matchAlertText").Parse(matchAlertTextTemplate))
)

// EmailTemplateRenderer provides template rendering for notification emails.
type EmailTemplateRenderer struct{}

// NewEmailTemplateRenderer constructs a new EmailTemplateRenderer.
func NewEmailTemplateRenderer() *EmailTemplateRenderer {
	return &EmailTemplateRenderer{}
}

// RenderMatchAlert renders both HTML and plain text email bodies for a match alert.
func (r *EmailTemplateRenderer) RenderMatchAlert(data MatchAlertEmailData) (EmailContent, error) {
	if err := data.Validate(); err != nil {
		return EmailContent{}, err
	}

	if data.PetName == "" {
		if data.PetID != "" {
			data.PetName = data.PetID
		} else {
			data.PetName = "Your Pet"
		}
	}

	if data.ConfidencePercent == 0 && data.Score > 0 {
		data.ConfidencePercent = int(math.Round(data.Score * 100))
	}

	var htmlBuf bytes.Buffer
	if err := parsedHTMLTmpl.Execute(&htmlBuf, data); err != nil {
		return EmailContent{}, fmt.Errorf("render html template: %w", err)
	}

	var textBuf bytes.Buffer
	if err := parsedTextTmpl.Execute(&textBuf, data); err != nil {
		return EmailContent{}, fmt.Errorf("render text template: %w", err)
	}

	subject := fmt.Sprintf("Match Found for Your Pet (%s)", data.PetName)

	return EmailContent{
		Subject:  subject,
		HTMLBody: htmlBuf.String(),
		TextBody: textBuf.String(),
	}, nil
}
