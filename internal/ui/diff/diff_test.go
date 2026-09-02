package diff

import (
	"strings"
	"testing"

	"github.com/idursun/jjui/internal/config"
	"github.com/idursun/jjui/internal/jj"
	"github.com/idursun/jjui/internal/jj/source"
	"github.com/idursun/jjui/internal/ui/common"
	"github.com/idursun/jjui/internal/ui/intents"
	"github.com/idursun/jjui/internal/ui/operations/target_picker"
	"github.com/idursun/jjui/internal/ui/render"
	"github.com/idursun/jjui/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_TrimsCarriageReturnsAndHandlesEmpty(t *testing.T) {
	model := New("line1\r\nline2\r\n")
	assert.Equal(t, "line1\nline2", test.Stripped(test.RenderImmediate(model, 20, 5)))

	emptyModel := New("")
	assert.Equal(t, "(empty)", test.Stripped(test.RenderImmediate(emptyModel, 10, 3)))
}

func TestScroll_ChangesVisibleContent(t *testing.T) {
	model := New("1\n2\n3\n4\n5")

	// Initially line 1 should be visible
	before := test.Stripped(test.RenderImmediate(model, 10, 2))
	assert.Contains(t, before, "1")

	// Scroll down 2 — line 1 should no longer be visible
	model.Update(intents.DiffScroll{Kind: intents.DiffScrollDown})
	model.Update(intents.DiffScroll{Kind: intents.DiffScrollDown})
	after := test.Stripped(test.RenderImmediate(model, 10, 2))
	assert.NotContains(t, after, "1")
	assert.Contains(t, after, "3")
}

func TestUpdate_ScrollMsgScrollsContent(t *testing.T) {
	model := New("a\nb\nc\nd\ne")

	// Line "a" should be visible before scroll
	before := test.Stripped(test.RenderImmediate(model, 10, 2))
	assert.Contains(t, before, "a")

	model.Update(ScrollMsg{Delta: 2})
	after := test.Stripped(test.RenderImmediate(model, 10, 2))
	assert.NotContains(t, after, "a")
	assert.Contains(t, after, "c")
}

func TestUpdate_DiffScrollIntentChangesContent(t *testing.T) {
	model := New("1\n2\n3\n4\n5")

	first := test.Stripped(test.RenderImmediate(model, 10, 2))
	assert.Contains(t, first, "1")

	model.Update(intents.DiffScroll{Kind: intents.DiffScrollDown})
	second := test.Stripped(test.RenderImmediate(model, 10, 2))
	assert.NotContains(t, second, "1")
	assert.Contains(t, second, "2")

	model.Update(intents.DiffScroll{Kind: intents.DiffScrollUp})
	third := test.Stripped(test.RenderImmediate(model, 10, 2))
	assert.Contains(t, third, "1")
}

func TestWrap_LongLinesWrapAtViewportWidth(t *testing.T) {
	// 20-character line rendered in a 10-wide viewport should produce 2 visual rows
	model := New("12345678901234567890")
	model.Update(intents.DiffToggleWrap{})

	rendered := test.Stripped(test.RenderImmediate(model, 10, 3))
	lines := strings.Split(rendered, "\n")
	// Both halves should be present
	assert.Equal(t, "1234567890", lines[0])
	assert.Equal(t, "1234567890", lines[1])
}

func TestWrap_ResizeRecomputes(t *testing.T) {
	// Line of 20 chars
	model := New("12345678901234567890")
	model.Update(intents.DiffToggleWrap{})

	// At width 10, line occupies 2 visual rows.
	rendered := test.Stripped(test.RenderImmediate(model, 10, 5))
	assert.Contains(t, rendered, "1234567890\n1234567890")

	// At width 5, line occupies 4 visual rows.
	rendered = test.Stripped(test.RenderImmediate(model, 5, 10))
	assert.Contains(t, rendered, "12345\n67890\n12345\n67890")
}

func TestNoWrap_HorizontalScroll(t *testing.T) {
	// 20-character line rendered in a 10-wide viewport
	model := New("abcdefghijklmnopqrst")

	// Without horizontal scroll, first 10 chars visible
	rendered := test.Stripped(test.RenderImmediate(model, 10, 1))
	assert.Equal(t, "abcdefghij", rendered)

	// Scroll right 5 columns
	for range 5 {
		model.Update(intents.DiffScrollHorizontal{Kind: intents.DiffScrollRight})
	}
	rendered = test.Stripped(test.RenderImmediate(model, 10, 1))
	assert.Equal(t, "fghijklmno", rendered)
}

func TestWrap_HorizontalScrollIsNoop(t *testing.T) {
	model := New("abcdefghijklmnopqrst")
	model.Update(intents.DiffToggleWrap{})
	before := test.Stripped(test.RenderImmediate(model, 10, 2))

	// Horizontal scroll should not change output in wrap mode.
	model.Update(intents.DiffScrollHorizontal{Kind: intents.DiffScrollRight})
	model.Update(intents.DiffScrollHorizontal{Kind: intents.DiffScrollRight})
	after := test.Stripped(test.RenderImmediate(model, 10, 2))
	assert.Equal(t, before, after)
}

func TestSetContent_ResetsScrollOffsets(t *testing.T) {
	model := New("abcdefghijklmnopqrstuvwxyz")

	model.Update(intents.DiffScrollHorizontal{Kind: intents.DiffScrollRight})
	model.Update(intents.DiffScrollHorizontal{Kind: intents.DiffScrollRight})
	model.Update(intents.DiffScrollHorizontal{Kind: intents.DiffScrollRight})
	model.Update(intents.DiffScrollHorizontal{Kind: intents.DiffScrollRight})
	model.Update(intents.DiffScrollHorizontal{Kind: intents.DiffScrollRight})
	model.Update(intents.DiffShow{Content: "abcdefghijklmnopqrstuvwxyz"})

	rendered := test.Stripped(test.RenderImmediate(model, 10, 1))
	assert.Equal(t, "abcdefghij", rendered)
}

func TestSetContent_PreservesWrapMode(t *testing.T) {
	model := New("12345678901234567890")
	model.Update(intents.DiffToggleWrap{})
	model.Update(intents.DiffShow{Content: "abcdefghij1234567890"})

	rendered := test.Stripped(test.RenderImmediate(model, 10, 3))
	assert.Contains(t, rendered, "abcdefghij\n1234567890")
}

func TestTabs_RenderIndentedInDefaultView(t *testing.T) {
	model := New("+\tfoo")

	rendered := test.RenderImmediate(model, 12, 1)
	assert.Equal(t, "+   foo", rendered)
}

func TestTabs_AffectHorizontalScrollWidth(t *testing.T) {
	model := New("\tabcdefghij")

	rendered := test.RenderImmediate(model, 8, 1)
	assert.Equal(t, "    abcd", rendered)

	for range 6 {
		model.Update(intents.DiffScrollHorizontal{Kind: intents.DiffScrollRight})
	}
	rendered = test.RenderImmediate(model, 8, 1)
	assert.Equal(t, "cdefghij", rendered)
}

func TestTabs_WrapUsingExpandedWidth(t *testing.T) {
	model := New("\t123456")
	model.Update(intents.DiffToggleWrap{})

	rendered := test.RenderImmediate(model, 5, 2)
	assert.Equal(t, "    1\n23456", rendered)
}

func TestTargetPickerUnavailableWithoutArgs(t *testing.T) {
	model := New("diff")

	cmd := model.Update(intents.DiffOpenTargetPicker{})
	require.NotNil(t, cmd)
	msg, ok := cmd().(intents.AddMessage)
	require.True(t, ok)
	assert.Contains(t, msg.Text, "unavailable")
}

func TestTargetPickerLoadsSummaryFiles(t *testing.T) {
	commandRunner := test.NewTestCommandRunner(t)
	defer commandRunner.Verify()
	args := jj.Diff("abc", jj.FileName{})
	commandRunner.Expect(append(jj.Diff("abc", jj.FileName{}), "--summary")).SetOutput([]byte("M a.go\nA b.go\nR {old => new}/path.txt\nM a.go"))

	model := NewWithContext(test.NewTestContext(commandRunner), "diff", args)
	cmd := model.Init()
	require.NotNil(t, cmd)
	summaryMsg, ok := cmd().(summaryLoadedMsg)
	require.True(t, ok)
	require.Nil(t, model.Update(summaryMsg))

	openCmd := model.Update(intents.DiffOpenTargetPicker{})
	require.NotNil(t, openCmd)
	openMsg, ok := openCmd().(common.OpenTargetPickerMsg)
	require.True(t, ok)
	require.Len(t, openMsg.Sources, 2)
	staticItems, err := openMsg.Sources[0].Fetch(nil)
	require.NoError(t, err)
	assert.Equal(t, []source.Item{{Name: allFilesTargetLabel}}, staticItems)
	items, err := openMsg.Sources[1].Fetch(nil)
	require.NoError(t, err)
	assert.Equal(t, []source.Item{
		{Name: "a.go", File: jj.NewFileName("a.go"), Kind: source.KindFile},
		{Name: "b.go", File: jj.NewFileName("b.go"), Kind: source.KindFile},
		{Name: "new/path.txt", File: jj.NewFileName("new/path.txt"), Kind: source.KindFile},
	}, items)
}

func TestTargetPickerSelectedAllFilesLoadsOriginalDiff(t *testing.T) {
	commandRunner := test.NewTestCommandRunner(t)
	defer commandRunner.Verify()
	args := jj.Diff("abc", jj.FileName{})
	commandRunner.Expect(args).SetOutput([]byte("all files diff"))
	commandRunner.Expect(append(jj.Diff("abc", jj.FileName{}), "--summary")).SetOutput([]byte("M dir/a.go"))

	model := NewWithContext(test.NewTestContext(commandRunner), "file diff", args)
	initCmd := model.Init()
	require.NotNil(t, initCmd)
	loadedTargets, ok := initCmd().(summaryLoadedMsg)
	require.True(t, ok)
	require.Nil(t, model.Update(loadedTargets))

	cmd := model.Update(target_picker.TargetSelectedMsg{
		Target:  allFilesTargetLabel,
		Payload: targetPickerPayload{},
	})
	require.NotNil(t, cmd)
	loaded, ok := cmd().(fileLoadedMsg)
	require.True(t, ok)

	updateCmd := model.Update(loaded)
	require.Nil(t, updateCmd)
	assert.Equal(t, "all files diff", test.Stripped(test.RenderImmediate(model, 20, 3)))
	assert.Equal(t, []string(args), model.originalArgs)
	assert.True(t, model.currentFile.IsEmpty())
}

func TestTargetPickerSelectedFileLoadsDiffAndKeepsOriginalArgs(t *testing.T) {
	commandRunner := test.NewTestCommandRunner(t)
	defer commandRunner.Verify()
	args := jj.Diff("abc", jj.FileName{})
	commandRunner.Expect(append(jj.Diff("abc", jj.FileName{}), jj.NewFileName("dir/a b.go").Escaped())).SetOutput([]byte("file diff"))
	commandRunner.Expect(append(jj.Diff("abc", jj.FileName{}), "--summary")).SetOutput([]byte("M dir/a b.go"))

	model := NewWithContext(test.NewTestContext(commandRunner), "initial diff", args)
	initCmd := model.Init()
	require.NotNil(t, initCmd)
	loadedTargets, ok := initCmd().(summaryLoadedMsg)
	require.True(t, ok)
	require.Nil(t, model.Update(loadedTargets))

	cmd := model.Update(target_picker.TargetSelectedMsg{
		File:    jj.NewFileName("dir/a b.go"),
		Payload: targetPickerPayload{},
	})
	require.NotNil(t, cmd)
	loaded, ok := cmd().(fileLoadedMsg)
	require.True(t, ok)

	updateCmd := model.Update(loaded)
	require.Nil(t, updateCmd)
	assert.Equal(t, "file diff", test.Stripped(test.RenderImmediate(model, 20, 3)))
	assert.Equal(t, []string(args), model.originalArgs)
	assert.Equal(t, "dir/a b.go", model.currentFile.Path())

	summaryCmd := model.Update(intents.DiffOpenTargetPicker{})
	require.NotNil(t, summaryCmd)
	_, ok = summaryCmd().(common.OpenTargetPickerMsg)
	require.True(t, ok)
}

func TestFileNavigateLoadsAdjacentFilesAndWraps(t *testing.T) {
	commandRunner := test.NewTestCommandRunner(t)
	defer commandRunner.Verify()
	args := jj.Diff("abc", jj.FileName{})
	commandRunner.Expect(append(jj.Diff("abc", jj.FileName{}), jj.NewFileName("c.go").Escaped())).SetOutput([]byte("c diff"))
	commandRunner.Expect(append(jj.Diff("abc", jj.FileName{}), jj.NewFileName("a.go").Escaped())).SetOutput([]byte("a diff"))
	commandRunner.Expect(append(jj.Diff("abc", jj.FileName{}), jj.NewFileName("b.go").Escaped())).SetOutput([]byte("b diff"))
	commandRunner.Expect(append(jj.Diff("abc", jj.FileName{}), "--summary")).SetOutput([]byte("M a.go\nM b.go\nM c.go"))

	model := NewWithContext(test.NewTestContext(commandRunner), "initial diff", args)
	initCmd := model.Init()
	require.NotNil(t, initCmd)
	loadedTargets, ok := initCmd().(summaryLoadedMsg)
	require.True(t, ok)
	require.Nil(t, model.Update(loadedTargets))

	cmd := model.Update(intents.DiffFileNavigate{Delta: -1})
	require.NotNil(t, cmd)
	loaded, ok := cmd().(fileLoadedMsg)
	require.True(t, ok)
	require.Nil(t, model.Update(loaded))
	assert.Equal(t, "c diff", test.Stripped(test.RenderImmediate(model, 20, 3)))
	assert.Equal(t, "c.go", model.currentFile.Path())

	cmd = model.Update(intents.DiffFileNavigate{Delta: 1})
	require.NotNil(t, cmd)
	loaded, ok = cmd().(fileLoadedMsg)
	require.True(t, ok)
	require.Nil(t, model.Update(loaded))
	assert.Equal(t, "a diff", test.Stripped(test.RenderImmediate(model, 20, 3)))
	assert.Equal(t, "a.go", model.currentFile.Path())

	cmd = model.Update(intents.DiffFileNavigate{Delta: 1})
	require.NotNil(t, cmd)
	loaded, ok = cmd().(fileLoadedMsg)
	require.True(t, ok)
	require.Nil(t, model.Update(loaded))
	assert.Equal(t, "b diff", test.Stripped(test.RenderImmediate(model, 20, 3)))
	assert.Equal(t, "b.go", model.currentFile.Path())
}

func TestFileNavigateUnavailableWithoutArgs(t *testing.T) {
	model := New("diff")

	cmd := model.Update(intents.DiffFileNavigate{Delta: 1})
	require.NotNil(t, cmd)
	msg, ok := cmd().(intents.AddMessage)
	require.True(t, ok)
	assert.Contains(t, msg.Text, "unavailable")
}

func TestConfiguredWrap_OpensWrappedWithoutToggling(t *testing.T) {
	previous := config.Current
	t.Cleanup(func() { config.Current = previous })
	config.Current.Diff.Wrap = true

	model := New("12345678901234567890")

	rendered := test.Stripped(test.RenderImmediate(model, 10, 3))
	assert.Contains(t, rendered, "1234567890\n1234567890")
}

func TestConfiguredWrap_CanStillBeToggledOff(t *testing.T) {
	previous := config.Current
	t.Cleanup(func() { config.Current = previous })
	config.Current.Diff.Wrap = true

	model := New("abcdefghijklmnopqrst")
	model.Update(intents.DiffToggleWrap{})

	// Back to a single row that scrolls horizontally instead of folding.
	rendered := test.Stripped(test.RenderImmediate(model, 10, 2))
	assert.Equal(t, "abcdefghij", strings.Split(rendered, "\n")[0])
	assert.NotContains(t, rendered, "klmnopqrst")
}

// jj's built-in diff formatters (--git, --color-words) never wrap, whatever
// width they are told about, so the view itself has to guarantee that nothing
// extends past the right edge.
func TestConfiguredWrap_UnwrappableFormatterOutputNeverOverflows(t *testing.T) {
	previous := config.Current
	t.Cleanup(func() { config.Current = previous })
	config.Current.Diff.Wrap = true

	// Shaped like `jj diff --git` output, including SGR sequences, with a line
	// far wider than the viewport.
	content := strings.Join([]string{
		"\x1b[38;5;3mModified regular file src/main.go:\x1b[39m",
		"\x1b[38;5;1m   1\x1b[39m \x1b[38;5;1m-\x1b[39m" + strings.Repeat("old ", 80),
		"\x1b[38;5;2m   1\x1b[39m \x1b[38;5;2m+\x1b[39m" + strings.Repeat("new ", 80),
	}, "\n")

	const width = 40
	rendered := test.Stripped(test.RenderImmediate(New(content), width, 30))
	for i, line := range strings.Split(rendered, "\n") {
		assert.LessOrEqual(t, render.StringWidth(line), width, "row %d overflows the viewport", i)
	}
	// The tail of the long line has to be reachable by scrolling down.
	assert.Contains(t, rendered, "new")
}
