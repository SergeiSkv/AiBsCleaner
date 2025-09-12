package analyzer

// Common function and method names used across analyzers
const (
	// Go built-in functions
	funcMake    = "make"
	funcAppend  = "append"
	funcPrintln = "println"
	funcNew     = "new"
	funcLock    = "Lock"

	// Sync methods
	methodLock           = "Lock"
	methodUnlock         = "Unlock"
	methodRLock          = "RLock"
	methodRUnlock        = "RUnlock"
	methodClose          = "Close"
	methodPut            = "Put"
	methodEncodeToString = "EncodeToString"

	// Type names
	typeMutex     = "Mutex"
	typeRWMutex   = "RWMutex"
	typeError     = "error"
	typeInterface = "interface"
	typeSlice     = "slice"

	// Database methods
	methodBegin   = "Begin"
	methodBeginTx = "BeginTx"

	methodSprintf = "Sprintf"

	// HTTP methods
	methodGet       = "Get"
	methodPost      = "Post"
	methodClient    = "Client"
	methodTransport = "Transport"
	methodTimeout   = "Timeout"
	methodDial      = "Dial"

	// IO methods
	methodRead     = "Read"
	methodWrite    = "Write"
	methodReadAll  = "ReadAll"
	methodReadFile = "ReadFile"
	methodOpen     = "Open"
	methodCreate   = "Create"

	// Package names
	pkgFmt     = "fmt"
	pkgRand    = "rand"
	pkgOS      = "os"
	pkgHTTP    = "http"
	pkgIO      = "io"
	pkgIOutil  = "ioutil"
	pkgTime    = "time"
	pkgContext = "context"
	pkgSync    = "sync"
	pkgRegexp  = "regexp"
	pkgBase64  = "base64"
	pkgBytes   = "bytes"
	pkgSQL     = "sql"
	pkgNet     = "net"

	// Method names for serialization
	methodDecodeString = "DecodeString"

	// Test constants
	testPackageMain = `package test
func main() {}`

	// String constants
	emptyString = `""`
	nilString   = "nil"

	// Crypto methods
	methodSum = "Sum"

	// External API severity strings (for parsing OSV/CVE data)
	osvSeverityCritical = "CRITICAL"
	osvSeverityHigh     = "HIGH"
	osvSeverityMedium   = "MEDIUM"
	osvSeverityLow      = "LOW"

	// License constants
	licenseApache2 = "Apache-2.0"
	licenseBSD3    = "BSD-3-Clause"
	licenseUnknown = "Unknown"
)
