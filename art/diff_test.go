package art

import "testing"

func Test_Diff(t *testing.T) {
	Diff(dsrc, dstt, map[string]bool{DiffSum: true, DiffTbl: true, DiffTrg: true}, nil)
}
