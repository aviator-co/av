Output split_commit.mp4

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
Screenshot split_1.png
Sleep 3

Type 'clear' Enter

# Split the last commit
Type 'av split-commit' Enter
Sleep 3
Type 'n' Enter
Sleep 2
Type 'n' Enter
Sleep 2
Type 'y'
Sleep 2

# Screenshot showing state before we get to editor
Screenshot split_2.png
Sleep 3

Enter
Sleep 5

Type 'cc' Enter
Sleep 1
Type 'Add a method to get users'
Sleep 1

Escape
Sleep 2

# Screenshot showing state in vim
Screenshot split_3.png
Sleep 3

Type ':wq' Enter
Sleep 3

# Stage the next two commits
Type 'y' Enter
Sleep 2
Type 'y'
Sleep 2

# Screenshot showing rest of lines waiting to be committed
Screenshot split_4.png
Sleep 3

Enter

Sleep 3
# Screenshot showing the second editor window
Screenshot split_5.png
Sleep 2

Escape
Sleep 2
Type ':wq' Enter
Sleep 2

Type 'clear' Enter
Sleep 2

# Show the final state of the git repo
Type 'git log' Enter
Sleep 2
Screenshot split_6.png
Sleep 2
