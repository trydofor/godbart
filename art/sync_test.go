package art

import "testing"

func Test_Sync(t *testing.T) {
	Sync(dsrc, dstt, map[string]bool{SyncTbl: true, SyncTrg: true, SyncRow: true}, nil)
}
