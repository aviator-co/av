package sequencer

import (
	"fmt"
	"os"
	"path/filepath"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

type RestackOp struct {
	Name plumbing.ReferenceName

	// New parent branch to sync to.
	NewParent plumbing.ReferenceName

	// Mark the new parent branch as trunk.
	NewParentIsTrunk bool

	// The new parent branch's hash. If not specified, the sequencer will use the new parent's
	// branch hash if the new parent is not trunk. Or if the new parent is trunk, the sequencer
	// will use the remote tracking branch's hash.
	NewParentHash plumbing.Hash
}

type branchSnapshot struct {
	// The branch name.
	Name plumbing.ReferenceName
	// The parent branch name.
	ParentBranch plumbing.ReferenceName
	// True if the parent branch is the trunk branch (refs/heads/master etc.).
	IsParentTrunk bool
	// Commit hash that the parent branch was previously at last time this was synced.
	// This is plumbing.ZeroHash if the parent branch is a trunk.
	PreviouslySyncedParentBranchHash plumbing.Hash
}

// Sequencer re-stacks the specified branches.
//
// This entire Sequencer object should be JSON serializable. The caller is expected to save this to
// file when the sequencer needs to be paused for more input.
type Sequencer struct {
	// The name of the remote (e.g. "origin").
	RemoteName string
	// All branch information initially when the sequencer started.
	OriginalBranchSnapshots map[plumbing.ReferenceName]*branchSnapshot
	// Ref that is currently being synced. Next time the sequencer runs, it will rebase this
	// ref.
	CurrentSyncRef plumbing.ReferenceName
	// If the rebase is stopped, these fields are set.
	SequenceInterruptedNewParentHash plumbing.Hash

	Operations []RestackOp
}

func NewSequencer(remoteName string, db meta.DB, ops []RestackOp) *Sequencer {
	var currentSyncRef plumbing.ReferenceName
	if len(ops) > 0 {
		currentSyncRef = ops[0].Name
	}
	return &Sequencer{
		RemoteName:              remoteName,
		OriginalBranchSnapshots: getBranchSnapshots(db),
		Operations:              ops,
		CurrentSyncRef:          currentSyncRef,
	}
}

func getBranchSnapshots(db meta.DB) map[plumbing.ReferenceName]*branchSnapshot {
	ret := map[plumbing.ReferenceName]*branchSnapshot{}
	for name, avbr := range db.ReadTx().AllBranches() {
		snapshot := &branchSnapshot{
			Name:         plumbing.ReferenceName("refs/heads/" + name),
			ParentBranch: plumbing.ReferenceName("refs/heads/" + avbr.Parent.Name),
		}
		ret[snapshot.Name] = snapshot
		if avbr.Parent.Trunk {
			snapshot.IsParentTrunk = true
		} else {
			snapshot.PreviouslySyncedParentBranchHash = plumbing.NewHash(avbr.Parent.Head)
		}
	}
	return ret
}

func (seq *Sequencer) Run(
	repo *git.Repo,
	db meta.DB,
	seqAbort, seqContinue, seqSkip, seqAutosquash  bool,
) (*git.RebaseResult, error) {
	if seqAbort || seqContinue || seqSkip || seqAutosquash {
		return seq.runFromInterruptedState(repo, db, seqAbort, seqContinue, seqSkip, seqAutosquash )
	}

	if seq.CurrentSyncRef == "" {
		return nil, nil
	}
	return seq.rebaseBranch(repo, db)
}

func (seq *Sequencer) runFromInterruptedState(
	repo *git.Repo,
	db meta.DB,
	seqAbort, seqContinue, seqSkip, seqAutosquash bool,
) (*git.RebaseResult, error) {
	if (seqAutosquash){
		// Abort the rebase if we need to
		if stat, _ := os.Stat(filepath.Join(repo.GitDir(), "REBASE_HEAD")); stat != nil {
			if _, err := repo.Rebase(git.RebaseOpts{Abort: true}); err != nil {
				return nil, errors.Errorf("failed to abort in-progress rebase: %v", err)
			}
		}
		seq.CurrentSyncRef = ""
		seq.SequenceInterruptedNewParentHash = plumbing.ZeroHash
		return nil, nil
	}

	if seq.CurrentSyncRef == "" {
		return nil, errors.New("no sync in progress")
	}
	if seq.SequenceInterruptedNewParentHash.IsZero() {
		panic("broken interruption state: no new parent hash")
	}
	if seqAbort {
		// Abort the rebase if we need to
		if stat, _ := os.Stat(filepath.Join(repo.GitDir(), "REBASE_HEAD")); stat != nil {
			if _, err := repo.Rebase(git.RebaseOpts{Abort: true}); err != nil {
				return nil, errors.Errorf("failed to abort in-progress rebase: %v", err)
			}
		}
		seq.CurrentSyncRef = ""
		seq.SequenceInterruptedNewParentHash = plumbing.ZeroHash
		return nil, nil
	}
	if seqContinue {
		if err := seq.checkNoUnstagedChanges(repo); err != nil {
			return nil, err
		}
		result, err := repo.RebaseParse(git.RebaseOpts{Continue: true})
		if err != nil {
			return nil, errors.Errorf("failed to continue in-progress rebase: %v", err)
		}
		if result.Status == git.RebaseConflict {
			return result, nil
		}
		if err := seq.postRebaseBranchUpdate(db, seq.SequenceInterruptedNewParentHash); err != nil {
			return nil, err
		}
		return result, nil
	}
	if seqSkip {
		result, err := repo.RebaseParse(git.RebaseOpts{Skip: true})
		if err != nil {
			return nil, errors.Errorf("failed to skip in-progress rebase: %v", err)
		}
		if result.Status == git.RebaseConflict {
			return result, nil
		}
		if err := seq.postRebaseBranchUpdate(db, seq.SequenceInterruptedNewParentHash); err != nil {
			return nil, err
		}
		return result, nil
	}
	panic("unreachable")
}

func (seq *Sequencer) rebaseBranch(repo *git.Repo, db meta.DB) (*git.RebaseResult, error) {
	op := seq.getCurrentOp()
	snapshot, ok := seq.OriginalBranchSnapshots[op.Name]
	if !ok {
		panic(fmt.Sprintf("branch %q not found in original branch infos", op.Name))
	}

	var previousParentHash plumbing.Hash
	if snapshot.IsParentTrunk {
		// Use the current remote tracking branch hash as the previous parent hash.
		var err error
		previousParentHash, err = seq.getRemoteTrackingBranchCommit(repo, snapshot.ParentBranch)
		if err != nil {
			return nil, err
		}
	} else {
		previousParentHash = snapshot.PreviouslySyncedParentBranchHash
	}

	var newParentHash plumbing.Hash
	if op.NewParentHash.IsZero() {
		if op.NewParentIsTrunk {
			var err error
			newParentHash, err = seq.getRemoteTrackingBranchCommit(repo, op.NewParent)
			if err != nil {
				return nil, err
			}
		} else {
			var err error
			newParentHash, err = seq.getBranchCommit(repo, op.NewParent)
			if err != nil {
				return nil, err
			}
		}
	} else {
		newParentHash = op.NewParentHash
	}

	// The commits from `rebaseFrom` to `snapshot.Name` should be rebased onto `rebaseOnto`.
	opts := git.RebaseOpts{
		Branch:   op.Name.Short(),
		Upstream: previousParentHash.String(),
		Onto:     newParentHash.String(),
	}
	result, err := repo.RebaseParse(opts)
	if err != nil {
		return nil, err
	}
	if result.Status == git.RebaseConflict {
		result.ErrorHeadline = fmt.Sprintf(
			"Failed to rebase %q onto %q (merge base is %q)\n",
			op.Name,
			op.NewParent,
			previousParentHash.String()[:7],
		) + result.ErrorHeadline
		seq.SequenceInterruptedNewParentHash = newParentHash
		return result, nil
	}
	if err := seq.postRebaseBranchUpdate(db, newParentHash); err != nil {
		return nil, err
	}
	return result, nil
}

func (seq *Sequencer) checkNoUnstagedChanges(repo *git.Repo) error {
	diff, err := repo.Diff(&git.DiffOpts{Quiet: true})
	if err != nil {
		return err
	}
	if !diff.Empty {
		return errors.New(
			"refusing to sync: there are unstaged changes in the working tree (use `git add` to stage changes)",
		)
	}
	return nil
}

func (seq *Sequencer) postRebaseBranchUpdate(db meta.DB, newParentHash plumbing.Hash) error {
	op := seq.getCurrentOp()
	newParentBranchState := meta.BranchState{
		Name:  op.NewParent.Short(),
		Trunk: op.NewParentIsTrunk,
	}
	if !op.NewParentIsTrunk {
		newParentBranchState.Head = newParentHash.String()
	}

	tx := db.WriteTx()
	br, _ := tx.Branch(op.Name.Short())
	br.Parent = newParentBranchState
	tx.SetBranch(br)
	if err := tx.Commit(); err != nil {
		return err
	}
	seq.SequenceInterruptedNewParentHash = plumbing.ZeroHash
	for i, op := range seq.Operations {
		if op.Name == seq.CurrentSyncRef {
			if i+1 < len(seq.Operations) {
				seq.CurrentSyncRef = seq.Operations[i+1].Name
			} else {
				seq.CurrentSyncRef = plumbing.ReferenceName("")
			}
			break
		}
	}
	return nil
}

func (seq *Sequencer) getCurrentOp() RestackOp {
	for _, op := range seq.Operations {
		if op.Name == seq.CurrentSyncRef {
			return op
		}
	}
	panic(fmt.Sprintf("op not found for ref %q", seq.CurrentSyncRef))
}

func (seq *Sequencer) getRemoteTrackingBranchCommit(
	repo *git.Repo,
	ref plumbing.ReferenceName,
) (plumbing.Hash, error) {
	remote, err := repo.GoGitRepo().Remote(seq.RemoteName)
	if err != nil {
		return plumbing.ZeroHash, errors.Errorf("failed to get remote %q: %v", seq.RemoteName, err)
	}
	rtb := mapToRemoteTrackingBranch(remote.Config(), ref)
	if rtb == nil {
		return plumbing.ZeroHash, errors.Errorf(
			"failed to get remote tracking branch in %q for %q",
			seq.RemoteName,
			ref,
		)
	}
	return seq.getBranchCommit(repo, *rtb)
}

func (seq *Sequencer) getBranchCommit(
	repo *git.Repo,
	ref plumbing.ReferenceName,
) (plumbing.Hash, error) {
	refObj, err := repo.GoGitRepo().Reference(ref, false)
	if err != nil {
		return plumbing.ZeroHash, errors.Errorf("failed to get branch %q: %v", ref, err)
	}
	if refObj.Type() != plumbing.HashReference {
		return plumbing.ZeroHash, errors.Errorf(
			"unexpected reference type for branch %q: %v",
			ref,
			refObj.Type(),
		)
	}
	return refObj.Hash(), nil
}

func mapToRemoteTrackingBranch(
	remoteConfig *config.RemoteConfig,
	refName plumbing.ReferenceName,
) *plumbing.ReferenceName {
	for _, fetch := range remoteConfig.Fetch {
		if fetch.Match(refName) {
			dst := fetch.Dst(refName)
			return &dst
		}
	}
	return nil
}
