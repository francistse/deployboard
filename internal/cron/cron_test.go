package cron

import "testing"

func TestParseLines(t *testing.T) {
	text := `# comment
0 3 * * * /usr/local/bin/backup.sh
*/5 * * * * /opt/demo/refresh
`
	got := ParseLines(text, []string{"backup.sh", "missing"})
	if len(got) != 1 || got[0].Source != "cron" || !MatchCommand(got[0].Command, []string{"backup"}) {
		t.Fatalf("%+v", got)
	}
}

func TestMatchCommand(t *testing.T) {
	if !MatchCommand("/a/b/backup.sh --x", []string{"backup.sh"}) {
		t.Fatal("expected match")
	}
	if MatchCommand("echo hi", []string{"backup"}) {
		t.Fatal("no match")
	}
}
