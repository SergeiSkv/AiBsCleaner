package analyzer

// Common function and method names used across analyzers
const (
	// Go built-in functions
	funcMake    = "make"
	funcRecover = "recover"

	// Type names
	typeMutex   = "Mutex"
	typeRWMutex = "RWMutex"

	// Database methods
	methodBegin        = "Begin"
	methodBeginTx      = "BeginTx"
	methodCommit       = "Commit"
	methodRollback     = "Rollback"
	methodQuery        = "Query"
	methodQueryContext = "QueryContext"
	methodQueryRow     = "QueryRow"
	methodQueryRowCtx  = "QueryRowContext"
	methodExec         = "Exec"
	methodExecContext  = "ExecContext"
	methodSprintf      = "Sprintf"
	methodClose        = "Close"

	// HTTP methods
	methodGet       = "Get"
	methodPost      = "Post"
	methodHead      = "Head"
	methodPostForm  = "PostForm"
	methodClient    = "Client"
	methodTimeout   = "Timeout"
	methodDial      = "Dial"
	methodListen    = "Listen"
	methodListenTCP = "ListenTCP"
	methodListenUDP = "ListenUDP"

	// IO methods
	methodOpen     = "Open"
	methodOpenFile = "OpenFile"
	methodCreate   = "Create"
	methodNewFile  = "NewFile"

	// Concurrency helpers
	methodAdd = "Add"

	// Reflection/regex/serialization helpers
	methodCompile     = "Compile"
	methodMustCompile = "MustCompile"
	methodMarshal     = "Marshal"
	methodUnmarshal   = "Unmarshal"
	methodNow         = "Now"

	// Package names
	pkgFmt     = "fmt"
	pkgOS      = "os"
	pkgHTTP    = "http"
	pkgTime    = "time"
	pkgContext = "context"
	pkgSync    = "sync"
	pkgRegexp  = "regexp"
	pkgSQL     = "sql"
	pkgNet     = "net"
	pkgJSON    = "json"
	pkgReflect = "reflect"

	// Test constants
	testPackageMain = `package test
func main() {}`
)

// Numeric constants for performance thresholds
const (
	MaxNestedLoops    = 3
	MaxFunctionParams = 5
	MaxSliceCapacity  = 1000
	MaxMapCapacity    = 100
	MaxSearchDepth    = 10
)
