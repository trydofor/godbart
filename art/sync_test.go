package art

import "testing"

func Test_Sync(t *testing.T) {
	Sync(dsrc, dstt, SyncAll, nil)
}
