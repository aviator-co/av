package git

type DiffIndex struct {
	// The tree(ish) to compare against
	Tree string
}

const (
	DiffStatusAddition     = "A"
	DiffStatusCopy         = "C"
	DiffStatusDeletion     = "D"
	DiffStatusModification = "M"
	DiffStatusRename       = "R"
	DiffStatusChangeType   = "T"
	DiffStatusUnmerged     = "U"
)

type DiffIndexItem struct {
	SrcMode string
	DstMode string
	// The hash of the file as it appears in the reference tree.
	SrcHash string
	// The dash of the file in the index.
	DstHash string
	// The status of the diff
	Status string
}

func (r *Repo) DiffIndex(di *DiffIndex) error {
	return nil
}
