package actions

import "emperror.dev/errors"

var ErrRepoNotInitialized = errors.Sentinel("this repository is not initialized; please run `av init`")
