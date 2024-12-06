Output adopt_no_root.mp4

Set FontFamily "Cascadia Mono NF"

Set Shell "bash"
Set FontSize 20
Set Width 1200
Set Height 1200

Hide
Type '. ../test_utils.sh' Enter
Type 'new_temp_repo' Enter

Type 'git checkout -b stack-1' Enter
Type 'create_commit "my-file" "1a" "Commit 1a"' Enter
Type 'create_commit "my-file" "1b" "Commit 1b"' Enter

Type 'git checkout -b stack-2' Enter
Type 'create_commit "my-file" "2a" "Commit 2a"' Enter
Type 'create_commit "my-file" "2b" "Commit 2b"' Enter

Type 'git checkout -b stack-3' Enter
Type 'create_commit "my-file" "3a" "Commit 3a"' Enter
Type 'create_commit "my-file" "3b" "Commit 3b"' Enter

Type 'git switch stack-1' Enter

Type 'git checkout -b stack-4' Enter
Type 'create_commit "my-file" "4a" "Commit 4a"' Enter
Type 'create_commit "my-file" "4b" "Commit 4b"' Enter

Type 'git switch stack-1' Enter
Type 'clear' Enter
Show

Type 'git --no-pager log --graph --all' Enter
Sleep 3

Type 'av adopt' Enter
Sleep 3

Enter
Sleep 3
