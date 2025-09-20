package analyzer

import (
	"go/ast"
	"go/token"
)

// ChannelAnalyzer detects potential channel deadlocks and misuse
type ChannelAnalyzer struct {
	channels   map[string]*ChannelInfo
	goroutines []*GoroutineInfo
}

type ChannelInfo struct {
	Name       string
	Buffered   bool
	BufferSize int
	Sends      []token.Position
	Receives   []token.Position
	Closes     []token.Position
}

type GoroutineInfo struct {
	Position  token.Position
	Channels  []string
	HasSelect bool
}

func NewChannelAnalyzer() *ChannelAnalyzer {
	return &ChannelAnalyzer{
		channels:   make(map[string]*ChannelInfo),
		goroutines: []*GoroutineInfo{},
	}
}

func (ca *ChannelAnalyzer) Name() string {
	return "ChannelAnalyzer"
}

func (ca *ChannelAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Reset state
	ca.channels = make(map[string]*ChannelInfo)
	ca.goroutines = []*GoroutineInfo{}

	// First pass: collect channel declarations and operations
	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GenDecl:
			ca.analyzeChannelDecl(node, fset)
		case *ast.AssignStmt:
			ca.analyzeChannelAssign(node, fset)
		case *ast.SendStmt:
			ca.analyzeChannelSend(node, fset)
		case *ast.UnaryExpr:
			if node.Op == token.ARROW {
				ca.analyzeChannelReceive(node, fset)
			}
		case *ast.GoStmt:
			ca.analyzeGoroutine(node, fset)
		case *ast.CallExpr:
			ca.analyzeChannelClose(node, fset)
		}
		return true
	})

	// Analyze collected data for issues
	issues = append(issues, ca.detectDeadlocks(filename, fset)...)
	issues = append(issues, ca.detectUnbufferedInGoroutine(filename, fset)...)
	issues = append(issues, ca.detectMultipleClose(filename, fset)...)
	issues = append(issues, ca.detectSendOnClosed(filename, fset)...)

	return issues
}

func (ca *ChannelAnalyzer) analyzeChannelDecl(decl *ast.GenDecl, _ *token.FileSet) {
	for _, spec := range decl.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		ca.analyzeValueSpec(valueSpec)
	}
}

func (ca *ChannelAnalyzer) analyzeValueSpec(valueSpec *ast.ValueSpec) {
	for i, name := range valueSpec.Names {
		if i >= len(valueSpec.Values) {
			continue
		}

		call, ok := valueSpec.Values[i].(*ast.CallExpr)
		if !ok {
			continue
		}

		if info := ca.extractChannelInfo(call, name.Name); info != nil {
			ca.channels[name.Name] = info
		}
	}
}

func (ca *ChannelAnalyzer) extractChannelInfo(call *ast.CallExpr, name string) *ChannelInfo {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "make" || len(call.Args) == 0 {
		return nil
	}

	_, ok = call.Args[0].(*ast.ChanType)
	if !ok {
		return nil
	}

	info := &ChannelInfo{
		Name:     name,
		Buffered: len(call.Args) > 1,
	}

	if len(call.Args) > 1 {
		if lit, ok := call.Args[1].(*ast.BasicLit); ok {
			if lit.Kind == token.INT {
				info.BufferSize = 1 // Simplified
			}
		}
	}

	return info
}

func (ca *ChannelAnalyzer) analyzeChannelAssign(assign *ast.AssignStmt, _ *token.FileSet) {
	for i, rhs := range assign.Rhs {
		call, ok := rhs.(*ast.CallExpr)
		if !ok || i >= len(assign.Lhs) {
			continue
		}

		lhs, ok := assign.Lhs[i].(*ast.Ident)
		if !ok {
			continue
		}

		if info := ca.extractChannelInfo(call, lhs.Name); info != nil {
			ca.channels[lhs.Name] = info
		}
	}
}

func (ca *ChannelAnalyzer) analyzeChannelSend(send *ast.SendStmt, fset *token.FileSet) {
	if ident, ok := send.Chan.(*ast.Ident); ok {
		if info, exists := ca.channels[ident.Name]; exists {
			info.Sends = append(info.Sends, fset.Position(send.Pos()))
		}
	}
}

func (ca *ChannelAnalyzer) analyzeChannelReceive(recv *ast.UnaryExpr, fset *token.FileSet) {
	if ident, ok := recv.X.(*ast.Ident); ok {
		if info, exists := ca.channels[ident.Name]; exists {
			info.Receives = append(info.Receives, fset.Position(recv.Pos()))
		}
	}
}

func (ca *ChannelAnalyzer) analyzeChannelClose(call *ast.CallExpr, fset *token.FileSet) {
	if ident, ok := call.Fun.(*ast.Ident); ok { //nolint:nestif // AST analysis requires nested checks
		if ident.Name == "close" && len(call.Args) > 0 {
			if chanIdent, ok := call.Args[0].(*ast.Ident); ok {
				if info, exists := ca.channels[chanIdent.Name]; exists {
					info.Closes = append(info.Closes, fset.Position(call.Pos()))
				}
			}
		}
	}
}

func (ca *ChannelAnalyzer) analyzeGoroutine(goStmt *ast.GoStmt, fset *token.FileSet) {
	info := &GoroutineInfo{
		Position: fset.Position(goStmt.Pos()),
		Channels: []string{},
	}

	// Check for select statement
	ast.Inspect(goStmt.Call, func(n ast.Node) bool {
		switch node := n.(type) { //nolint:nestif // AST analysis requires nested checks
		case *ast.SelectStmt:
			info.HasSelect = true
		case *ast.SendStmt:
			if ident, ok := node.Chan.(*ast.Ident); ok {
				info.Channels = append(info.Channels, ident.Name)
			}
		case *ast.UnaryExpr:
			if node.Op == token.ARROW {
				if ident, ok := node.X.(*ast.Ident); ok {
					info.Channels = append(info.Channels, ident.Name)
				}
			}
		}
		return true
	})

	ca.goroutines = append(ca.goroutines, info)
}

func (ca *ChannelAnalyzer) detectDeadlocks(filename string, _ *token.FileSet) []Issue {
	var issues []Issue

	// Check for unbuffered channel operations outside goroutines
	for name, info := range ca.channels {
		if !info.Buffered {
			// If there are sends without corresponding receives in goroutines
			if len(info.Sends) > 0 && len(info.Receives) == 0 {
				for _, pos := range info.Sends {
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "CHANNEL_DEADLOCK",
						Severity:   SeverityHigh,
						Message:    "Potential deadlock: sending on unbuffered channel without receiver",
						Suggestion: "Use buffered channel or ensure receiver is ready",
					})
				}
			}
		}
		_ = name
	}

	return issues
}

func (ca *ChannelAnalyzer) detectUnbufferedInGoroutine(filename string, _ *token.FileSet) []Issue {
	var issues []Issue

	for _, goroutine := range ca.goroutines { //nolint:nestif // Channel analysis requires multiple levels
		if !goroutine.HasSelect {
			for _, chanName := range goroutine.Channels {
				if info, exists := ca.channels[chanName]; exists {
					if !info.Buffered {
						issues = append(issues, Issue{
							File:       filename,
							Line:       goroutine.Position.Line,
							Column:     goroutine.Position.Column,
							Position:   goroutine.Position,
							Type:       "UNBUFFERED_CHANNEL_IN_GOROUTINE",
							Severity:   SeverityMedium,
							Message:    "Unbuffered channel operation in goroutine without select",
							Suggestion: "Use select with default case or buffered channel to prevent blocking",
						})
						break
					}
				}
			}
		}
	}

	return issues
}

func (ca *ChannelAnalyzer) detectMultipleClose(filename string, _ *token.FileSet) []Issue {
	var issues []Issue

	for name, info := range ca.channels {
		if len(info.Closes) > 1 {
			for _, pos := range info.Closes[1:] {
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "CHANNEL_MULTIPLE_CLOSE",
					Severity:   SeverityHigh,
					Message:    "Channel '" + name + "' closed multiple times",
					Suggestion: "Ensure channel is closed only once, typically by the sender",
				})
			}
		}
	}

	return issues
}

func (ca *ChannelAnalyzer) detectSendOnClosed(filename string, _ *token.FileSet) []Issue {
	var issues []Issue

	for name, info := range ca.channels {
		if len(info.Closes) > 0 {
			closePos := info.Closes[0]
			for _, sendPos := range info.Sends {
				// Simplified: check if send appears after close in file
				if sendPos.Line > closePos.Line {
					issues = append(issues, Issue{
						File:       filename,
						Line:       sendPos.Line,
						Column:     sendPos.Column,
						Position:   sendPos,
						Type:       "SEND_ON_CLOSED_CHANNEL",
						Severity:   SeverityHigh,
						Message:    "Sending on potentially closed channel '" + name + "'",
						Suggestion: "Check channel state before sending or restructure code flow",
					})
				}
			}
		}
	}

	return issues
}
