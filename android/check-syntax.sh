#!/bin/bash

# Simple syntax checker for Kotlin files without full Android SDK
# This checks for obvious import and syntax issues

echo "Checking Kotlin syntax issues..."

# Check for unresolved imports
echo "=== Checking unresolved imports ==="
PROBLEM_IMPORTS=$(find /home/git/working/todoapp/android/src/main/java -name "*.kt" -exec grep -l "import.*data\.entities\|import.*data\.dao" {} \; 2>/dev/null)

if [ ! -z "$PROBLEM_IMPORTS" ]; then
    echo "ERROR: Found imports to non-existent packages (data.entities or data.dao):"
    echo "$PROBLEM_IMPORTS"
    exit 1
fi

# Check for missing @AndroidEntryPoint on Fragments
echo "=== Checking Fragment annotations ==="
FRAGMENTS_WITHOUT_ANNOTATION=$(find /home/git/working/todoapp/android/src/main/java -name "*Fragment.kt" -exec grep -L "@AndroidEntryPoint" {} \;)
if [ ! -z "$FRAGMENTS_WITHOUT_ANNOTATION" ]; then
    echo "ERROR: Fragments missing @AndroidEntryPoint:"
    echo "$FRAGMENTS_WITHOUT_ANNOTATION"
    exit 1
fi

# Check for basic syntax issues
echo "=== Checking for common syntax issues ==="
SYNTAX_ERRORS=0

# Check for unmatched braces
for file in $(find /home/git/working/todoapp/android/src/main/java -name "*.kt"); do
    OPEN_BRACES=$(grep -o "{" "$file" | wc -l)
    CLOSE_BRACES=$(grep -o "}" "$file" | wc -l)
    if [ $OPEN_BRACES -ne $CLOSE_BRACES ]; then
        echo "ERROR: Unmatched braces in $file"
        SYNTAX_ERRORS=1
    fi
done

# Check for missing imports in critical files
echo "=== Checking critical files ==="
CRITICAL_FILES=(
    "/home/git/working/todoapp/android/src/main/java/com/todoapp/data/local/AppDatabase.kt"
    "/home/git/working/todoapp/android/src/main/java/com/todoapp/ui/auth/LoginFragment.kt"
    "/home/git/working/todoapp/android/src/main/java/com/todoapp/TodoApp.kt"
)

for file in "${CRITICAL_FILES[@]}"; do
    if [ ! -f "$file" ]; then
        echo "ERROR: Critical file missing: $file"
        SYNTAX_ERRORS=1
    fi
done

if [ $SYNTAX_ERRORS -eq 0 ]; then
    echo "✅ All basic syntax checks passed!"
    echo "The code should compile with proper Android SDK setup."
else
    echo "❌ Found syntax errors that need to be fixed."
    exit 1
fi