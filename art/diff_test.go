package art

import "testing"

func Test_Diff(t *testing.T) {
	Diff(dsrc, dstt, DiffAll, nil)
}
