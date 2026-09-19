package contentview

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"lazynats/internal/ui/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

// ---------------------------------------------------------------------
// PrettifyJSON
// ---------------------------------------------------------------------

func TestPrettifyJSON(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"empty", []byte{}, ""},
		{"plain text", []byte("hello"), "hello"},
		{"invalid json", []byte(`{a:1}`), `{a:1}`},
		{"object", []byte(`{"a":1}`), "{\n  \"a\": 1\n}"},
		{"array", []byte(`[1,2]`), "[\n  1,\n  2\n]"},
		{"nested", []byte(`{"x":{"y":"z"}}`), "{\n  \"x\": {\n    \"y\": \"z\"\n  }\n}"},
		{"no html escape", []byte(`{"url":"a&b"}`), "{\n  \"url\": \"a&b\"\n}"},
		{"string literal", []byte(`"hi"`), "\"hi\""},
		{"number literal", []byte(`42`), "42"},
		{"bool literal", []byte(`true`), "true"},
		{"null literal", []byte(`null`), "null"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(PrettifyJSON(c.in))
			if got != c.want {
				t.Errorf("PrettifyJSON(%q)\n got: %q\nwant: %q", c.in, got, c.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// isJSONLine
// ---------------------------------------------------------------------

func TestIsJSONLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"object open", `{"a":1}`, true},
		{"array open", `[1,2,3]`, true},
		{"string", `"hello"`, true},
		{"object close only", `}`, true},
		{"array close only", `]`, true},
		{"positive int", `42`, true},
		{"negative int", `-42`, true},
		{"float", `3.14`, true},
		{"true literal", `true`, true},
		{"false literal", `false`, true},
		{"null literal", `null`, true},
		{"plain word", `hello`, false},
		{"plain sentence", `just some log line`, false},
		{"word starting with t but not true", `testing`, false},
		{"word starting with f but not false", `foobar`, false},
		{"word starting with n but not null", `nothing`, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isJSONLine(c.in); got != c.want {
				t.Errorf("isJSONLine(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// highlightJSONLine
// ---------------------------------------------------------------------

func TestHighlightJSONLine_PreservesVisibleText(t *testing.T) {
	in := `  "name": "value", "count": 42, "ok": true, "bad": false, "missing": null`
	got := stripANSI(highlightJSONLine(in))
	if got != in {
		t.Errorf("visible text changed after highlighting.\n got: %q\nwant: %q", got, in)
	}
}

func TestHighlightJSONLine_KeyVsStringDistinction(t *testing.T) {
	in := `{"name": "value", "nested": {"k": "v"}}`
	got := stripANSI(highlightJSONLine(in))
	if got != in {
		t.Errorf("visible text changed after highlighting.\n got: %q\nwant: %q", got, in)
	}
}

func TestHighlightJSONLine_EscapedQuotesInString(t *testing.T) {
	in := `"he said \"hi\""`
	got := stripANSI(highlightJSONLine(in))
	if got != in {
		t.Errorf("escaped quotes not preserved.\n got: %q\nwant: %q", got, in)
	}
}

func TestHighlightJSONLine_NumbersWithExponent(t *testing.T) {
	in := `-1.5e-10`
	got := stripANSI(highlightJSONLine(in))
	if got != in {
		t.Errorf("exponent number not preserved.\n got: %q\nwant: %q", got, in)
	}
}

func TestHighlightJSONLine_UnterminatedString(t *testing.T) {
	in := `"unterminated`
	got := stripANSI(highlightJSONLine(in))
	if got != in {
		t.Errorf("visible text changed on unterminated string.\n got: %q\nwant: %q", got, in)
	}
}

// ---------------------------------------------------------------------
// highlightMixedContent
// ---------------------------------------------------------------------

func TestHighlightMixedContent_LeavesPlainLinesUntouched(t *testing.T) {
	in := "connected to nats\nsome plain log line"
	got := highlightMixedContent(in)
	if got != in {
		t.Errorf("plain (non-JSON) lines were modified.\n got: %q\nwant: %q", got, in)
	}
}

func TestHighlightMixedContent_TimestampLineIsTreatedAsJSONLike(t *testing.T) {
	in := "2024-01-01T00:00:00Z connected to nats"
	got := stripANSI(highlightMixedContent(in))
	if got != in {
		t.Errorf("visible text changed for timestamp-like line.\n got: %q\nwant: %q", got, in)
	}
}

func TestHighlightMixedContent_HighlightsOnlyJSONLines(t *testing.T) {
	in := "plain prefix line\n{\"a\": 1}\nanother plain line"
	got := highlightMixedContent(in)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "plain prefix line" {
		t.Errorf("first plain line changed: %q", lines[0])
	}
	if lines[2] != "another plain line" {
		t.Errorf("third plain line changed: %q", lines[2])
	}
	if stripANSI(lines[1]) != `{"a": 1}` {
		t.Errorf("json line visible text changed: %q", stripANSI(lines[1]))
	}
	if lines[1] == `{"a": 1}` {
		t.Errorf("expected JSON line to contain styling codes, got raw: %q", lines[1])
	}
}

func TestHighlightMixedContent_LeadingWhitespaceStillDetected(t *testing.T) {
	in := `    {"indented": true}`
	got := highlightMixedContent(in)
	if stripANSI(got) != in {
		t.Errorf("indented JSON line visible text changed.\n got: %q\nwant: %q", stripANSI(got), in)
	}
}

func TestHighlightMixedContent_EmptyInput(t *testing.T) {
	if got := highlightMixedContent(""); got != "" {
		t.Errorf("expected empty output for empty input, got %q", got)
	}
}

// ---------------------------------------------------------------------
// dividerLine
// ---------------------------------------------------------------------

func TestDividerLine(t *testing.T) {
	cases := []struct {
		width int
		want  int
	}{
		{10, 10},
		{1, 1},
		{0, 1},
		{-5, 1},
	}
	for _, c := range cases {
		got := dividerLine(c.width)
		if len(got) != c.want {
			t.Errorf("dividerLine(%d) length = %d, want %d", c.width, len(got), c.want)
		}
		for _, ch := range got {
			if ch != '-' {
				t.Errorf("dividerLine(%d) contains non-'-' rune: %q", c.width, got)
				break
			}
		}
	}
}

// ---------------------------------------------------------------------
// Model: SetContent / SetBytes / Body
// ---------------------------------------------------------------------

func newTestModel() Model {
	return New(theme.Theme{})
}

func TestModel_SetContent_SetsBodyAndResetsNotice(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 20)
	m.SetContent("subject.foo", "seq: 12", "hello world")

	if m.Body() != "hello world" {
		t.Errorf("Body() = %q, want %q", m.Body(), "hello world")
	}
	if m.notice != "" {
		t.Errorf("expected notice to be reset on SetContent, got %q", m.notice)
	}
	if m.title != "subject.foo" {
		t.Errorf("title = %q, want %q", m.title, "subject.foo")
	}
	if m.subtitle != "seq: 12" {
		t.Errorf("subtitle = %q, want %q", m.subtitle, "seq: 12")
	}
}

func TestModel_SetBytes_JSON(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 20)
	m.SetBytes("key", "rev: 7", []byte(`{"a":1}`))

	if m.title != "key" {
		t.Errorf("title = %q, want %q", m.title, "key")
	}
	if m.subtitle != "rev: 7" {
		t.Errorf("subtitle = %q, want %q", m.subtitle, "rev: 7")
	}
	wantBody := "{\n  \"a\": 1\n}"
	if m.Body() != wantBody {
		t.Errorf("Body() = %q, want %q", m.Body(), wantBody)
	}
}

func TestModel_SetBytes_NotJSON(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 20)
	raw := []byte("binary\x00data")
	m.SetBytes("raw", "", raw)

	if m.Body() != "binary\x00data" {
		t.Errorf("Body() = %q, want %q", m.Body(), "binary\x00data")
	}
}

func TestModel_SetContent_BodyUnaffectedByWrapping(t *testing.T) {
	m := newTestModel()
	m.SetSize(10, 20)
	long := strings.Repeat("word ", 20)
	m.SetContent("t", "", long)
	if m.Body() != long {
		t.Errorf("Body() was mutated by wrapping.\n got: %q\nwant: %q", m.Body(), long)
	}
}

func TestModel_SetContent_WithoutPriorSetSize(t *testing.T) {
	m := newTestModel()
	m.SetContent("t", "", "some body text")
	if m.Body() != "some body text" {
		t.Errorf("Body() = %q", m.Body())
	}
}

// ---------------------------------------------------------------------
// Model: SetSize
// ---------------------------------------------------------------------

func TestModel_SetSize_AdjustsViewportDimensions(t *testing.T) {
	m := newTestModel()
	m.SetSize(100, 23)
	if m.vp.Width != 100 {
		t.Errorf("vp.Width = %d, want 100", m.vp.Width)
	}
	if m.vp.Height != 21 {
		t.Errorf("vp.Height = %d, want 21", m.vp.Height)
	}
}

func TestModel_SetSize_ClampsNegativeHeightToZero(t *testing.T) {
	m := newTestModel()
	m.SetSize(50, 1)
	if m.vp.Height != 0 {
		t.Errorf("vp.Height = %d, want 0 (clamped)", m.vp.Height)
	}
}

func TestModel_SetSize_RewrapsExistingContent(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 20)
	m.SetContent("t", "", "hello world this is a body")

	m.SetSize(10, 20)
	if m.Body() != "hello world this is a body" {
		t.Errorf("Body() changed after resize: %q", m.Body())
	}
	if !strings.Contains(stripANSI(m.vp.View()), "hello") {
		t.Errorf("viewport content missing expected text after resize")
	}
}

// ---------------------------------------------------------------------
// Model: Update (copy-to-clipboard)
// ---------------------------------------------------------------------

func TestModel_Update_YKeyTriggersCopyCommand(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 20)
	m.SetContent("t", "", "copy me")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatal("expected a command to be returned for 'y' key, got nil")
	}
	msg := cmd()
	result, ok := msg.(CopyResultMsg)
	if !ok {
		t.Fatalf("expected CopyResultMsg, got %T", msg)
	}
	_ = result
}

func TestModel_Update_CopyResultSetsNoticeOnSuccess(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 20)
	m.SetContent("t", "", "body")

	m, _ = m.Update(CopyResultMsg{Err: nil})
	if m.notice != "copied to clipboard" {
		t.Errorf("notice = %q, want success notice", m.notice)
	}
}

func TestModel_Update_CopyResultSetsNoticeOnError(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 20)
	m.SetContent("t", "", "body")

	errMsg := errString("clipboard unavailable")
	m, _ = m.Update(CopyResultMsg{Err: errMsg})
	if !strings.HasPrefix(m.notice, "copy failed: ") {
		t.Errorf("notice = %q, want prefix %q", m.notice, "copy failed: ")
	}
	if !strings.Contains(m.notice, "clipboard unavailable") {
		t.Errorf("notice = %q, want it to contain the error text", m.notice)
	}
}

func TestModel_Update_OtherKeysDelegateToViewport(t *testing.T) {
	m := newTestModel()
	m.SetSize(20, 5)
	body := strings.Repeat("line\n", 50)
	m.SetContent("t", "", body)

	before := m.vp.YOffset
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.vp.YOffset == before {
		t.Errorf("expected viewport to scroll on KeyDown, YOffset stayed at %d", before)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// ---------------------------------------------------------------------
// Model: View
// ---------------------------------------------------------------------

func TestModel_View_ContainsTitleAndDivider(t *testing.T) {
	m := newTestModel()
	m.SetSize(40, 10)
	m.SetContent("bucket.mykey", "rev: 3", "the value")

	out := stripANSI(m.View())
	if !strings.Contains(out, "bucket.mykey") {
		t.Errorf("View() missing title, got: %q", out)
	}
	if !strings.Contains(out, "rev: 3") {
		t.Errorf("View() missing subtitle, got: %q", out)
	}
	if !strings.Contains(out, "the value") {
		t.Errorf("View() missing body content, got: %q", out)
	}
	if !strings.Contains(out, "----------") {
		t.Errorf("View() missing divider line, got: %q", out)
	}
}

func TestModel_View_ShowsNoticeAfterCopy(t *testing.T) {
	m := newTestModel()
	m.SetSize(40, 10)
	m.SetContent("t", "", "body")
	m, _ = m.Update(CopyResultMsg{Err: nil})

	out := stripANSI(m.View())
	if !strings.Contains(out, "copied to clipboard") {
		t.Errorf("View() missing copy notice, got: %q", out)
	}
}

func TestModel_View_NoSubtitleNoExtraSpacing(t *testing.T) {
	m := newTestModel()
	m.SetSize(40, 10)
	m.SetContent("only-title", "", "body")

	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) < 1 || strings.TrimRight(stripANSI(lines[0]), " ") != "only-title" {
		t.Errorf("expected first line to be the title, got lines: %v", lines)
	}
	if len(lines) < 2 || !strings.Contains(stripANSI(lines[1]), "---") {
		t.Errorf("expected second line to be the divider when subtitle/notice are empty, got: %q", lines)
	}
}
