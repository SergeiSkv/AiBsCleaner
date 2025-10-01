package analyzer

import "strings"

var issueAcronyms = map[string]string{
	"API":  "API",
	"AI":   "AI",
	"AWS":  "AWS",
	"CPU":  "CPU",
	"CGO":  "CGO",
	"DB":   "DB",
	"DNS":  "DNS",
	"GC":   "GC",
	"HTTP": "HTTP",
	"IO":   "IO",
	"IP":   "IP",
	"JSON": "JSON",
	"JWT":  "JWT",
	"MD5":  "MD5",
	"RSA":  "RSA",
	"SQL":  "SQL",
	"SSN":  "SSN",
	"URL":  "URL",
	"XML":  "XML",
}

func normalizeIssueName(name string) string {
	if name == "" {
		return ""
	}
	parts := strings.Split(name, "_")
	for i, part := range parts {
		upper := strings.ToUpper(part)
		if repl, ok := issueAcronyms[upper]; ok {
			parts[i] = repl
			continue
		}
		lower := strings.ToLower(part)
		if lower == "" {
			continue
		}
		parts[i] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(parts, "")
}
