package models

// SeverityLevel represents the severity of an issue
//
//go:generate go run github.com/dmarkham/enumer@latest -type=SeverityLevel -trimprefix=SeverityLevel
type SeverityLevel uint8

const (
	SeverityLevelLow    SeverityLevel = iota // 🟢 Low priority issues
	SeverityLevelMedium                      // 🟡 Medium priority issues
	SeverityLevelHigh                        // 🔴 High priority issues
)
