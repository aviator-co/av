# # DANGER DANGER DANGER

# # THIS IS INTENTIONALLY ALL COMMENTED OUT

# # to run this demo, uncomment all of the lines below
# # change the cd line to point to an appropriate directory
# # that is initialized with github, and is not one you care about
# # make sure that you delete all branches
# # if you need to rerun this make sure you delete the branches it creates
# # and that you delete the remote branches it creates as well
# # you will likely need to swap references from main to master
# # if your test repo uses master as the default branch

# # DANGER DANGER DANGER

# Output sync.mp4
# Output demo.gif

# Set FontFamily "Cascadia Mono NF"

# Set Shell "bash"
# Set FontSize 20
# Set Width 1200
# Set Height 1200

# Hide
# Type '. ../test_utils.sh' Enter
# Sleep 2

# Type 'cd ~/dev/og-testing' Enter
# Sleep 2

# Type 'git checkout main' Enter
# Sleep 2

# Type 'alias av="~/go/bin/av"' Enter
# Sleep 2

# Type 'av branch stack-1' Enter
# Sleep 2
# Type 'create_commit "my-file1" "1" "PR 1 Commit 1"' Enter
# Sleep 2
# Type 'av pr -t PR1 -b sample' Enter
# Sleep 3

# Type 'av branch stack-2' Enter
# Sleep 2
# Type 'create_commit "my-file2" "2" "PR 2 Commit 1"' Enter
# Sleep 2
# Type 'av pr -t PR2 -b sample' Enter
# Sleep 3

# Type 'av branch test-1' Enter
# Sleep 2
# Type 'create_commit "my-file3" "3" "Commit 3"' Enter
# Sleep 2

# Type 'av branch test-2' Enter
# Sleep 2
# Type 'create_commit "my-file4" "4" "Commit 4"' Enter
# Sleep 2

# Type 'create_commit "my-file5" "5" "Commit 5"' Enter
# Sleep 2
# Type 'av branch test-3' Enter
# Sleep 2

# Type 'av prev 2' Enter
# Sleep 2
# Type 'create_commit "my-file6" "6" "Commit 6"' Enter
# Sleep 2
# Type 'av branch test-4' Enter
# Sleep 2

# Type 'create_commit "my-file7" "7" "Commit 7"' Enter
# Sleep 2
# Type 'av branch test-5' Enter
# Sleep 2

# Type 'git checkout stack-1' Enter
# Sleep 2
# Type 'create_commit "my-file8" "8" "Updating PR 1"' Enter
# Sleep 2

# Type 'git checkout stack-2' Enter
# Sleep 2
# Type 'create_commit "my-file9" "9" "Updating PR 2"' Enter
# Sleep 2

# Type 'clear' Enter
# Show

# Type 'av sync --rebase-to-trunk' Enter
# Sleep 10

# Enter
# Sleep 20
