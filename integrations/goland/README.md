# GoLand Integration for AiBsCleaner

## Method 1: External Tool (Easy)

1. **Open GoLand Settings**
   - File → Settings (Ctrl+Alt+S)
   - Tools → External Tools

2. **Add New Tool**
   - Click `+` to add new tool
   - Name: `AiBsCleaner`
   - Program: `aibscleaner`
   - Arguments: `-path $FilePath$`
   - Working directory: `$ProjectFileDir$`

3. **Add Keyboard Shortcut**
   - Settings → Keymap
   - Find "AiBsCleaner" under External Tools
   - Add shortcut (e.g., Ctrl+Shift+B)

## Method 2: File Watcher (Automatic)

1. **Install File Watchers Plugin**
   - Settings → Plugins
   - Search "File Watchers"
   - Install

2. **Configure Watcher**
   ```
   Name: AiBsCleaner
   File type: Go
   Scope: Project Files
   Program: aibscleaner
   Arguments: -path $FilePath$ -format json
   Output paths: $FileNameWithoutExtension$.aibsc.json
   ```

## Method 3: Run Configuration

1. **Create Run Configuration**
   - Run → Edit Configurations
   - Add → Shell Script
   
2. **Configure**
   ```bash
   Script text: aibscleaner -path . -format json
   Working directory: $ProjectFileDir$
   ```

## Method 4: Live Template for Quick Fix

1. **Settings → Editor → Live Templates**
2. **Add Template Group**: AiBsCleaner
3. **Add Templates**:

```go
// Template: fixstring
// Description: Fix string concatenation
var $VAR$ strings.Builder
$VAR$.Grow($SIZE$)
$END$

// Template: fixdefer  
// Description: Fix defer in loop
func $NAME$() {
    // Extracted from loop
    $END$
}
```

## Method 5: Inspection Profile

Create `.idea/inspectionProfiles/AiBsCleaner.xml`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<component name="InspectionProjectProfileManager">
  <profile version="1.0">
    <option name="myName" value="AiBsCleaner" />
    <inspection_tool class="GoUnusedVariable" enabled="true" level="WARNING" />
    <inspection_tool class="GoComplexity" enabled="true" level="WARNING">
      <option name="maxComplexity" value="10" />
    </inspection_tool>
  </profile>
</component>
```

## Method 6: Custom Script

Create `scripts/goland-integration.sh`:

```bash
#!/bin/bash
# GoLand integration script

OUTPUT=$(aibscleaner -path "$1" -format json)
if [ $? -ne 0 ]; then
    echo "$OUTPUT" | jq -r '.issues[] | "\(.file):\(.line):\(.column): \(.type): \(.message)"'
    exit 1
fi
```

Then use as External Tool with:
- Program: `bash`
- Arguments: `scripts/goland-integration.sh $FilePath$`