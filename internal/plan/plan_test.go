package plan

import "testing"

func TestConversion(t *testing.T) {
	var bytes int64
	bytes = 536_870_912
	humanSize := BytesToHumanSize(bytes)
	t.Logf("bytes: %d", bytes)
	t.Logf("Human size:       %s", humanSize)

}
