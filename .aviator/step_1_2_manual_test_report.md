# Step 1.2 Manual Testing Report

## Overview
This document describes the manual testing procedure for verifying the fix implemented in Step 1.1, which resolves the bug where `av sync` hangs when run on trunk/master branch after selecting "Yes" to the prompt.

## Testing Environment Requirements
- Go compiler (to build the av binary)
- Git installed and configured
- Access to create test repositories
- Terminal with interactive input capability

## Testing Procedure

### Prerequisites
1. Build the av binary:
   ```bash
   cd /code
   go build -o /tmp/av ./cmd/av
   export PATH="/tmp:$PATH"
   ```

### Setup Test Repository
```bash
# Create and initialize test repository
TEST_DIR=$(mktemp -d)
cd "$TEST_DIR"
git init
git config user.email "test@example.com"
git config user.name "Test User"

# Configure GitHub remote
git remote add origin https://github.com/test-user/test-repo.git

# Initialize av
av init
# When prompted, select "master" as trunk branch

# Create initial commit
echo "Initial content" > README.md
git add README.md
git commit -m "Initial commit"

# Create test branch with av
av branch test-branch
echo "Test content" > test.txt
git add test.txt
git commit -m "Add test file"

# Create another branch for comprehensive testing
av branch test-branch-2
echo "More content" > test2.txt
git add test2.txt
git commit -m "Add test file 2"

# Switch back to master
git checkout master
```

## Test Cases

### Test Case 1: av sync with "Yes" Response (Primary Bug Fix)

**Purpose:** Verify that selecting "Yes" continues execution through the entire sync flow without hanging.

**Steps:**
1. Ensure you're on the master/main branch: `git checkout master`
2. Run: `av sync` (without --all flag)
3. When prompted "You are on the trunk, do you want to sync all stacks?", select **Yes**

**Expected Behavior:**
- ✓ Prompt appears: "You are on the trunk, do you want to sync all stacks?"
- ✓ After selecting "Yes":
  - Pre-av-sync hook executes (if hook exists)
  - Fetching from GitHub message appears
  - Restacking process begins
  - All branches are processed
  - Push confirmation prompt appears (if push=ask)
  - Prune confirmation prompt appears (if prune=ask)
- ✓ Command completes successfully without hanging
- ✓ Exit code is 0

**What Was Previously Broken:**
- Before the fix, selecting "Yes" would cause the command to hang/quit immediately
- The callback function returned nil instead of continuing with `vm.initPreAvHook()`
- No sync operations would be performed

**What the Fix Does:**
- The callback now properly returns `vm.initPreAvHook()` after setting `syncFlags.All = true`
- This maintains the continuation chain in the CPS (Continuation Passing Style) pattern
- All subsequent sync steps execute as expected

### Test Case 2: av sync with "No" Response

**Purpose:** Verify that selecting "No" exits cleanly without performing any sync operations.

**Steps:**
1. Ensure you're on the master/main branch: `git checkout master`
2. Run: `av sync` (without --all flag)
3. When prompted "You are on the trunk, do you want to sync all stacks?", select **No**

**Expected Behavior:**
- ✓ Prompt appears
- ✓ After selecting "No":
  - Command exits immediately
  - No sync operations performed
  - No branches modified
  - No error messages
- ✓ Exit code is 0
- ✓ Repository state unchanged

### Test Case 3: av sync --all (No Prompt Expected)

**Purpose:** Verify that the --all flag bypasses the prompt entirely.

**Steps:**
1. Ensure you're on the master/main branch
2. Run: `av sync --all`

**Expected Behavior:**
- ✓ No prompt appears
- ✓ Command proceeds directly to sync operations
- ✓ All branches are synced
- ✓ Command completes successfully

### Test Case 4: av sync from Non-Trunk Branch

**Purpose:** Verify that running sync from a feature branch doesn't show the trunk prompt.

**Steps:**
1. Switch to a feature branch: `git checkout test-branch`
2. Run: `av sync`

**Expected Behavior:**
- ✓ No trunk prompt appears
- ✓ Current stack is synced normally
- ✓ Command completes successfully

## Verification Checklist

After running the tests, verify:

- [ ] **Test Case 1 (Yes):** Command continues execution after selecting "Yes"
- [ ] **Test Case 1 (Yes):** All sync steps execute (hook, fetch, restack, push, prune)
- [ ] **Test Case 1 (Yes):** Command completes successfully without hanging
- [ ] **Test Case 1 (Yes):** Branches are properly synced/rebased
- [ ] **Test Case 2 (No):** Command exits cleanly after selecting "No"
- [ ] **Test Case 2 (No):** No branches are modified
- [ ] **Test Case 2 (No):** No error messages appear
- [ ] **Test Case 3 (--all):** No prompt appears with --all flag
- [ ] **Test Case 3 (--all):** All branches synced successfully
- [ ] **Test Case 4 (non-trunk):** No prompt when running from feature branch

## Code Changes Verified

The fix in `cmd/av/sync.go:203-220` ensures:

1. **Lines 208-212:** When "No" is selected, `tea.Quit` is returned immediately (early return)
2. **Lines 213-216:** When "Yes" is selected, `syncFlags.All = true` is set
3. **Line 217:** Regardless of "Yes" or other responses, execution falls through to return `vm.initPreAvHook()`
4. **Key Point:** The callback no longer has an implicit nil return that breaks the continuation chain

## Testing Notes

- The bug was introduced in commit 45d7d25 during refactoring to continuation-passing style
- This manual testing validates the fix before automated e2e tests are added in Step 2
- The interactive nature of the prompt makes manual verification important
- Consider testing with different terminal emulators if behavior seems inconsistent

## Cleanup

```bash
# Remove test repository
rm -rf "$TEST_DIR"
```

## Status

**Status:** Documentation Complete

**Note:** Actual execution of these tests requires a Go build environment which is not available in the current execution context. This document provides the complete procedure for manual testing to be performed in an appropriate environment with Go installed.

## Next Steps

Once manual testing confirms the fix works correctly:
1. Proceed to Step 2.1: Create E2E test for interactive sync on trunk
2. Proceed to Step 2.2: Add test for "No" response
3. Proceed to Step 2.3: Update existing tests if needed
