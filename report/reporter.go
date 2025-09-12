package report

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/SergeiSkv/AiBsCleaner/analyzer"
)

type Reporter interface {
	Print(issues []analyzer.Issue) error
}

type TextReporter struct{}

type JSONReporter struct{}

func NewReporter(format string) Reporter {
	switch format {
	case "json":
		return &JSONReporter{}
	default:
		return &TextReporter{}
	}
}

func (tr *TextReporter) Print(issues []analyzer.Issue) error {
	// Sort issues by severity and then by file/line
	sort.Slice(
		issues, func(i, j int) bool {
			if issues[i].Severity != issues[j].Severity {
				return severityWeight(issues[i].Severity) > severityWeight(issues[j].Severity)
			}
			if issues[i].File != issues[j].File {
				return issues[i].File < issues[j].File
			}
			return issues[i].Line < issues[j].Line
		},
	)

	fmt.Printf("\n🔍 Performance Analysis Results\n")
	fmt.Printf("================================\n\n")
	fmt.Printf("Found %d performance issues:\n\n", len(issues))

	// Group by severity
	bySeverity := make(map[analyzer.Severity][]analyzer.Issue, 10)
	for _, issue := range issues {
		bySeverity[issue.Severity] = append(bySeverity[issue.Severity], issue)
	}

	// Print by severity
	severities := []analyzer.Severity{
		analyzer.SeverityHigh,
		analyzer.SeverityMedium,
		analyzer.SeverityLow,
	}

	for _, severity := range severities {
		if severityIssues, ok := bySeverity[severity]; ok && len(severityIssues) > 0 {
			fmt.Printf("%s %s SEVERITY ISSUES (%d)\n", severityIcon(severity), severity, len(severityIssues))
			fmt.Println("----------------------------------------")

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			for _, issue := range severityIssues {
				fmt.Fprintf(w, "%s:%d:%d\t%s\n", issue.File, issue.Line, issue.Column, issue.Type)
				fmt.Fprintf(w, "\t└─ %s\n", issue.Message)
				if issue.Suggestion != "" {
					fmt.Fprintf(w, "\t   💡 %s\n", issue.Suggestion)
				}
				fmt.Fprintln(w)
			}
			w.Flush()
		}
	}

	// Summary
	fmt.Println("\n📊 Summary:")
	fmt.Printf("   High:   %d issues\n", len(bySeverity[analyzer.SeverityHigh]))
	fmt.Printf("   Medium: %d issues\n", len(bySeverity[analyzer.SeverityMedium]))
	fmt.Printf("   Low:    %d issues\n", len(bySeverity[analyzer.SeverityLow]))

	return nil
}

func (jr *JSONReporter) Print(issues []analyzer.Issue) error {
	output := struct {
		TotalIssues int              `json:"total_issues"`
		Issues      []analyzer.Issue `json:"issues"`
		Summary     map[string]int   `json:"summary"`
	}{
		TotalIssues: len(issues),
		Issues:      issues,
		Summary: map[string]int{
			"high":   countBySeverity(issues, analyzer.SeverityHigh),
			"medium": countBySeverity(issues, analyzer.SeverityMedium),
			"low":    countBySeverity(issues, analyzer.SeverityLow),
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func severityWeight(s analyzer.Severity) int {
	switch s {
	case analyzer.SeverityHigh:
		return 3
	case analyzer.SeverityMedium:
		return 2
	case analyzer.SeverityLow:
		return 1
	default:
		return 0
	}
}

func severityIcon(s analyzer.Severity) string {
	switch s {
	case analyzer.SeverityHigh:
		return "🔴"
	case analyzer.SeverityMedium:
		return "🟡"
	case analyzer.SeverityLow:
		return "🔵"
	default:
		return "⚪"
	}
}

func countBySeverity(issues []analyzer.Issue, severity analyzer.Severity) int {
	count := 0
	for _, issue := range issues {
		if issue.Severity == severity {
			count++
		}
	}
	return count
}
