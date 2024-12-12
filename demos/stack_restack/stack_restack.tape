Output stack_restack.mp4

Set FontFamily "Cascadia Mono NF"

Set Shell "bash"
Set FontSize 20
Set Width 1200
Set Height 1200

Hide
Type '. ../test_utils.sh' Enter
Type 'new_temp_repo' Enter

Type 'av branch stack-1' Enter
Type 'create_commit "my-file" "1a" "Commit 1a"' Enter

Type 'av branch stack-2' Enter
Type 'create_commit "my-file" "2a" "Commit 2a"' Enter

Type 'av branch stack-3' Enter
Type 'create_commit "my-file" "3a" "Commit 3a"' Enter

Type 'git switch stack-1' Enter
Type 'clear' Enter
Show

Type 'av tree' Enter
Sleep 2

Type 'git commit --amend -m "Commit 1a (amended)"' Enter
Sleep 2

Type 'git --no-pager log --graph --all' Enter
Sleep 2

Type 'av restack' Enter
Sleep 3

Type 'git --no-pager log --graph --all' Enter
Sleep 3
