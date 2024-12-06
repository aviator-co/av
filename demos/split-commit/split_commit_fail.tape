Output split_commit_fail.mp4

Set FontFamily "Cascadia Mono NF"

Set Shell "bash"
Set FontSize 20
Set Width 1200
Set Height 1200

Env EDITOR "nvim"
Env VISUAL "nvim"
Env GIT_EDITOR "nvim"

Hide

Type "STARTDIR=$(pwd)" Enter


Type '. ../test_utils.sh' Enter
Sleep 5
Type 'new_temp_repo' Enter
Sleep 5

# Copy api.go and create initial commit
Type 'cp $STARTDIR/api.go-demo api.go' Enter
Sleep 3
Type 'git add -A && git commit --amend --no-edit' Enter
Sleep 3

Type 'git checkout -b add_users_api' Enter
Sleep 3

# "modify" api.go, copy db.go and create second commit
Type 'rm -f api.go' Enter
Sleep 3
Type 'cp $STARTDIR/new-api.go-demo api.go' Enter
Sleep 3
Type 'cp $STARTDIR/db.go-demo db.go' Enter
Sleep 3
Type 'git add -A && git commit -m "Add users API"' Enter
Sleep 3

Type 'clear' Enter
Show

# show current HEAD state
Type 'git show --summary --stat' Enter
Sleep 3

Type 'clear' Enter

# Split the last commit
Type 'av split-commit' Enter
Sleep 3
Type 'q' Enter
Sleep 2

Type 'git switch --discard-changes add_users_api' Enter

Sleep 2
Screenshot split_fail_1.png
Sleep 2
