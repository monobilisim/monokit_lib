package lib

import "testing"

func TestCreateMarkdownTableStrings(t *testing.T) {
	// The exact byte layout matters: these tables go straight into Zulip and
	// Redmine bodies, and both need a real markdown separator row to render.
	got := CreateMarkdownTable(
		[]string{"Mountpoint", "Used"},
		[][]string{
			{"/", "91%"},
			{"/var", "87%"},
		},
	)

	want := "| Mountpoint | Used |\n" +
		"| --- | --- |\n" +
		"| / | 91% |\n" +
		"| /var | 87% |\n"

	if got != want {
		t.Errorf("table mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestCreateMarkdownTableFormatsNonStrings(t *testing.T) {
	// The function is generic over the cell type, so callers pass ints and floats
	// directly rather than pre-formatting every value.
	got := CreateMarkdownTable(
		[]string{"Pool", "Errors"},
		[][]int{{1, 0}, {2, 3}},
	)

	want := "| Pool | Errors |\n" +
		"| --- | --- |\n" +
		"| 1 | 0 |\n" +
		"| 2 | 3 |\n"

	if got != want {
		t.Errorf("table mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestCreateMarkdownTableNoRows(t *testing.T) {
	// An empty result set still has to produce a valid table: a header with no
	// separator row renders as literal text in Zulip.
	got := CreateMarkdownTable([]string{"Unit", "State"}, [][]string{})

	want := "| Unit | State |\n| --- | --- |\n"

	if got != want {
		t.Errorf("expected a header-only table, got:\n%s", got)
	}
}

func TestCreateMarkdownTableNoHeaders(t *testing.T) {
	got := CreateMarkdownTable([]string{}, [][]string{{"a"}})

	want := "|\n|\n| a |\n"

	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// TestCreateMarkdownTableRaggedRow documents that the function does not pad or
// truncate rows to the header width. A caller that builds rows of the wrong
// length produces a table markdown renderers will mangle, and the function will
// not stop it — worth knowing before blaming the renderer.
func TestCreateMarkdownTableRaggedRow(t *testing.T) {
	got := CreateMarkdownTable(
		[]string{"A", "B", "C"},
		[][]string{{"1"}},
	)

	want := "| A | B | C |\n" +
		"| --- | --- | --- |\n" +
		"| 1 |\n"

	if got != want {
		t.Errorf("expected the ragged row to be emitted as-is\ngot:\n%s\nwant:\n%s", got, want)
	}
}
