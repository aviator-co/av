Output tidy_orphaned.mp4

Set FontFamily "Cascadia Mono NF"

Set Shell "bash"
Set FontSize 20
Set Width 1200
Set Height 1200

Hide
Type '. ../test_utils.sh' Enter
Type 'new_temp_repo' Enter

Type 'av branch stack-1' Enter
Type 'av branch stack-2' Enter
Type 'git switch main' Enter
Type 'git branch -D stack-1' Enter
Type 'clear' Enter
Show

Type 'av tidy' Enter
Sleep 3
