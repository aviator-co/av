# av 1 "" av-cli "Aviator CLI User Manual"

# NAME

av-commit-create - Record changes to the repository with commits.

# DESCRIPTION

Create a new commit containing the current contents of the index and the given log message describing the changes, then run **av stack sync** on all subsequent child branches with the new commit.

Previous to running **av-commit-create**, add changes to the index via **git-add** to incrementally "add" changes to the index.

# OPTIONS

-m <msg>
--message=<msg>
Use the given <msg> as the commit message.

-a
--all
Automatically stage modified/ deleted files, but new files you have not told Git about are not affected. (Same as git commit --all)
