package debrief

import (
	"bytes"
	"os"
	"text/template"
	"time"

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// markdownTemplate is the template for formatting Output as markdown.
const markdownTemplate = `## Debrief Summary

**Generated:** {{.GeneratedAt.Format "2006-01-02 15:04"}}

### Summary

{{.Summary}}

{{if .KeyDecisions -}}
### Key Decisions

{{range .KeyDecisions -}}
- {{.}}
{{end}}
{{end -}}

{{if .Challenges -}}
### Challenges

{{range .Challenges -}}
- {{.}}
{{end}}
{{end -}}

{{if .LessonsLearned -}}
### Lessons Learned

{{range .LessonsLearned -}}
- {{.}}
{{end}}
{{end -}}

{{if .FollowUps -}}
### Follow-ups

{{range .FollowUps -}}
- [ ] {{.}}
{{end}}
{{end -}}
`

var tmpl = template.Must(template.New("debrief").Parse(markdownTemplate))

// FormatMarkdown formats the Output as markdown suitable for notes.
func (o *Output) FormatMarkdown() string {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, o); err != nil {
		// Fallback to simple format if template fails
		return o.formatSimple()
	}
	return buf.String()
}

// formatSimple provides a fallback markdown format.
func (o *Output) formatSimple() string {
	var buf bytes.Buffer
	buf.WriteString("## Debrief Summary\n\n")
	buf.WriteString("**Generated:** " + o.GeneratedAt.Format(time.RFC3339) + "\n\n")
	buf.WriteString("### Summary\n\n")
	buf.WriteString(o.Summary + "\n\n")

	if len(o.KeyDecisions) > 0 {
		buf.WriteString("### Key Decisions\n\n")
		for _, d := range o.KeyDecisions {
			buf.WriteString("- " + d + "\n")
		}
		buf.WriteString("\n")
	}

	if len(o.Challenges) > 0 {
		buf.WriteString("### Challenges\n\n")
		for _, c := range o.Challenges {
			buf.WriteString("- " + c + "\n")
		}
		buf.WriteString("\n")
	}

	if len(o.LessonsLearned) > 0 {
		buf.WriteString("### Lessons Learned\n\n")
		for _, l := range o.LessonsLearned {
			buf.WriteString("- " + l + "\n")
		}
		buf.WriteString("\n")
	}

	if len(o.FollowUps) > 0 {
		buf.WriteString("### Follow-ups\n\n")
		for _, f := range o.FollowUps {
			buf.WriteString("- [ ] " + f + "\n")
		}
		buf.WriteString("\n")
	}

	return buf.String()
}

// AppendToNotes appends the debrief output to an existing notes file.
// If the file doesn't exist, it creates it. The output is appended with
// a separator to distinguish it from existing content.
func AppendToNotes(notePath string, output *Output) error {
	markdown := output.FormatMarkdown()

	// Check if file exists
	_, err := os.Stat(notePath)
	fileExists := err == nil

	// Open file for appending (create if needed)
	f, err := os.OpenFile(notePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return rigerrors.Wrapf(err, "failed to open notes file: %s", notePath)
	}
	defer f.Close()

	// Add separator if appending to existing file
	if fileExists {
		if _, err := f.WriteString("\n---\n\n"); err != nil {
			return rigerrors.Wrap(err, "failed to write separator")
		}
	}

	// Write the markdown
	if _, err := f.WriteString(markdown); err != nil {
		return rigerrors.Wrap(err, "failed to write debrief output")
	}

	return nil
}

// WriteToFile writes the debrief output to a new file.
// It will fail if the file already exists to prevent accidental overwrites.
func WriteToFile(path string, output *Output) error {
	// Check if file exists
	if _, err := os.Stat(path); err == nil {
		return rigerrors.Newf("file already exists: %s", path)
	}

	markdown := output.FormatMarkdown()

	if err := os.WriteFile(path, []byte(markdown), 0o644); err != nil {
		return rigerrors.Wrapf(err, "failed to write file: %s", path)
	}

	return nil
}
