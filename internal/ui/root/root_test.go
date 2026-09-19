package root

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"lazynats/internal/natsclient"
	"lazynats/internal/ui/modal"
	"lazynats/internal/ui/theme"
)

// ---------------------------------------------------------------------
// Embedded NATS helper
// ---------------------------------------------------------------------

func startJetStreamServer(t *testing.T) string {
	t.Helper()
	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoLog:     true,
		NoSigs:    true,
	}
	srv, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	srv.Start()
	t.Cleanup(srv.Shutdown)
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("server not ready")
	}
	return srv.ClientURL()
}

func newTestClient(t *testing.T, url string) *natsclient.Client {
	t.Helper()
	c, err := natsclient.Connect(t.Context(), url)
	if err != nil {
		t.Fatalf("natsclient.New(%q): %v", url, err)
	}
	return c
}

func newTestModel(t *testing.T) Model {
	url := startJetStreamServer(t)
	client := newTestClient(t, url)
	return New(client, theme.Default())
}

// ---------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------

func TestNew(t *testing.T) {
	m := newTestModel(t)
	if m.tab != TabStreams {
		t.Errorf("tab = %v, want TabStreams", m.tab)
	}
	if m.modal != modalNone {
		t.Errorf("modal = %v, want modalNone", m.modal)
	}
	if m.banner != "" {
		t.Errorf("banner = %q, want empty", m.banner)
	}
}

func TestInit(t *testing.T) {
	m := newTestModel(t)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil")
	}
}

func TestWindowSize(t *testing.T) {
	m := newTestModel(t)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	rm := newM.(Model)
	if rm.width != 100 || rm.height != 40 {
		t.Errorf("size = (%d,%d), want (100,40)", rm.width, rm.height)
	}
}

func TestSwitchTabNext(t *testing.T) {
	m := newTestModel(t)
	m.banner = "some error"

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	rm := newM.(Model)
	if rm.tab != TabBuckets {
		t.Errorf("tab = %v, want TabBuckets", rm.tab)
	}
	if rm.banner != "" {
		t.Errorf("banner = %q, want empty after switch", rm.banner)
	}
}

func TestSwitchTabPrev(t *testing.T) {
	m := newTestModel(t)
	m.tab = TabBuckets

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	rm := newM.(Model)
	if rm.tab != TabStreams {
		t.Errorf("tab = %v, want TabStreams", rm.tab)
	}
}

func TestQuit(t *testing.T) {
	m := newTestModel(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	_ = cmd()
}

func TestRefresh(t *testing.T) {
	m := newTestModel(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected refresh command")
	}
}

func TestOpenCreateModal(t *testing.T) {
	m := newTestModel(t)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	rm := newM.(Model)
	if rm.modal != modalCreateStream {
		t.Errorf("modal = %v, want modalCreateStream", rm.modal)
	}
}

func TestOpenDeleteModalNothingSelected(t *testing.T) {
	m := newTestModel(t)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	rm := newM.(Model)
	if rm.modal != modalNone {
		t.Errorf("modal = %v, want modalNone (empty list)", rm.modal)
	}
}

func TestModalBlocksGlobalKeys(t *testing.T) {
	m := newTestModel(t)
	m.modal = modalCreateStream
	m.form = modal.NewForm(m.theme, "test", "hint", []modal.FieldSpec{{Label: "x"}})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		_ = cmd()
	}
}

func TestActionDoneMsgSuccess(t *testing.T) {
	m := newTestModel(t)
	newM, _ := m.Update(actionDoneMsg{tab: TabStreams, success: "created"})
	rm := newM.(Model)
	if rm.banner != "created" {
		t.Errorf("banner = %q, want %q", rm.banner, "created")
	}
	if rm.bannerDanger {
		t.Error("bannerDanger should be false on success")
	}
}

func TestActionDoneMsgError(t *testing.T) {
	m := newTestModel(t)
	newM, _ := m.Update(actionDoneMsg{tab: TabStreams, err: errString("fail")})
	rm := newM.(Model)
	if rm.banner != "fail" {
		t.Errorf("banner = %q, want %q", rm.banner, "fail")
	}
	if !rm.bannerDanger {
		t.Error("bannerDanger should be true on error")
	}
}

func TestViewRendersTabs(t *testing.T) {
	m := newTestModel(t)
	out := m.View()
	if !strings.Contains(out, "Streams") {
		t.Errorf("View() missing Streams tab, got:\n%s", out)
	}
	if !strings.Contains(out, "Buckets") {
		t.Errorf("View() missing Buckets tab, got:\n%s", out)
	}
}

func TestViewRendersModal(t *testing.T) {
	m := newTestModel(t)
	m.modal = modalCreateStream
	m.form = modal.NewForm(m.theme, "test title", "hint", []modal.FieldSpec{{Label: "x"}})
	m.form.SetSize(80, 24)

	out := m.View()
	if !strings.Contains(out, "test title") {
		t.Errorf("View() missing modal title, got:\n%s", out)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// ---------------------------------------------------------------------
// handleFormSubmitted — Create Stream
// ---------------------------------------------------------------------

func TestHandleFormSubmitted_CreateStream_EmptyName(t *testing.T) {
	m := newTestModel(t)
	m.modal = modalCreateStream
	m.form = modal.NewForm(m.theme, "New stream", "hint", []modal.FieldSpec{
		{Label: "Name", Placeholder: "orders"},
		{Label: "Subjects", Placeholder: "orders.>"},
	})

	tm, cmd := m.handleFormSubmitted(modal.FormSubmittedMsg{Values: []string{"", "orders.>"}})
	rm := tm.(Model)
	if rm.modal != modalCreateStream {
		t.Error("modal should stay open on validation error")
	}
	if cmd != nil {
		t.Error("expected nil cmd on validation error")
	}
}

func TestHandleFormSubmitted_CreateStream_EmptySubjects(t *testing.T) {
	m := newTestModel(t)
	m.modal = modalCreateStream
	m.form = modal.NewForm(m.theme, "New stream", "hint", []modal.FieldSpec{
		{Label: "Name", Placeholder: "orders"},
		{Label: "Subjects", Placeholder: "orders.>"},
	})

	tm, cmd := m.handleFormSubmitted(modal.FormSubmittedMsg{Values: []string{"orders", "   "}})
	rm := tm.(Model)
	if rm.modal != modalCreateStream {
		t.Error("modal should stay open on validation error")
	}
	if cmd != nil {
		t.Error("expected nil cmd on validation error")
	}
}

func TestHandleFormSubmitted_CreateStream_Success(t *testing.T) {
	m := newTestModel(t)
	m.modal = modalCreateStream
	m.form = modal.NewForm(m.theme, "New stream", "hint", []modal.FieldSpec{
		{Label: "Name", Placeholder: "orders"},
		{Label: "Subjects", Placeholder: "orders.>"},
	})

	tm, cmd := m.handleFormSubmitted(modal.FormSubmittedMsg{Values: []string{"teststream", "test.>"}})
	rm := tm.(Model)
	if rm.modal != modalNone {
		t.Error("modal should be closed on success")
	}
	if rm.deleteTarget != "" {
		t.Error("deleteTarget should be empty")
	}
	if cmd == nil {
		t.Fatal("expected createStreamCmd")
	}
	msg := cmd()
	done, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("expected actionDoneMsg, got %T", msg)
	}
	if done.tab != TabStreams {
		t.Errorf("tab = %v, want TabStreams", done.tab)
	}
}

// ---------------------------------------------------------------------
// handleFormSubmitted — Create Bucket
// ---------------------------------------------------------------------

func TestHandleFormSubmitted_CreateBucket_EmptyName(t *testing.T) {
	m := newTestModel(t)
	m.modal = modalCreateBucket
	m.form = modal.NewForm(m.theme, "New bucket", "hint", []modal.FieldSpec{
		{Label: "Name", Placeholder: "config"},
	})

	tm, cmd := m.handleFormSubmitted(modal.FormSubmittedMsg{Values: []string{""}})
	rm := tm.(Model)
	if rm.modal != modalCreateBucket {
		t.Error("modal should stay open on validation error")
	}
	if cmd != nil {
		t.Error("expected nil cmd on validation error")
	}
}

func TestHandleFormSubmitted_CreateBucket_Success(t *testing.T) {
	m := newTestModel(t)
	m.modal = modalCreateBucket
	m.form = modal.NewForm(m.theme, "New bucket", "hint", []modal.FieldSpec{
		{Label: "Name", Placeholder: "config"},
	})

	tm, cmd := m.handleFormSubmitted(modal.FormSubmittedMsg{Values: []string{"testbucket"}})
	rm := tm.(Model)
	if rm.modal != modalNone {
		t.Error("modal should be closed on success")
	}
	if cmd == nil {
		t.Fatal("expected createBucketCmd")
	}
	msg := cmd()
	done, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("expected actionDoneMsg, got %T", msg)
	}
	if done.tab != TabBuckets {
		t.Errorf("tab = %v, want TabBuckets", done.tab)
	}
}

// ---------------------------------------------------------------------
// handleConfirmResult — Delete Stream
// ---------------------------------------------------------------------

func TestHandleConfirmResult_DeleteStream_Confirmed(t *testing.T) {
	m := newTestModel(t)
	m.modal = modalDeleteStream
	m.deleteTarget = "my-stream"

	tm, cmd := m.handleConfirmResult(modal.ConfirmResultMsg{Confirmed: true})
	rm := tm.(Model)
	if rm.modal != modalNone {
		t.Error("modal should be closed")
	}
	if rm.deleteTarget != "" {
		t.Error("deleteTarget should be reset")
	}
	if cmd == nil {
		t.Fatal("expected deleteStreamCmd")
	}
	msg := cmd()
	done, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("expected actionDoneMsg, got %T", msg)
	}
	if done.tab != TabStreams {
		t.Errorf("tab = %v, want TabStreams", done.tab)
	}
}

func TestHandleConfirmResult_DeleteStream_Cancelled(t *testing.T) {
	m := newTestModel(t)
	m.modal = modalDeleteStream
	m.deleteTarget = "my-stream"

	tm, cmd := m.handleConfirmResult(modal.ConfirmResultMsg{Confirmed: false})
	rm := tm.(Model)
	if rm.modal != modalNone {
		t.Error("modal should be closed")
	}
	if rm.deleteTarget != "" {
		t.Error("deleteTarget should be reset")
	}
	if cmd != nil {
		t.Error("expected nil cmd when cancelled")
	}
}

// ---------------------------------------------------------------------
// handleConfirmResult — Delete Bucket
// ---------------------------------------------------------------------

func TestHandleConfirmResult_DeleteBucket_Confirmed(t *testing.T) {
	m := newTestModel(t)
	m.modal = modalDeleteBucket
	m.deleteTarget = "my-bucket"

	tm, cmd := m.handleConfirmResult(modal.ConfirmResultMsg{Confirmed: true})
	rm := tm.(Model)
	if rm.modal != modalNone {
		t.Error("modal should be closed")
	}
	if cmd == nil {
		t.Fatal("expected deleteBucketCmd")
	}
	msg := cmd()
	done, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("expected actionDoneMsg, got %T", msg)
	}
	if done.tab != TabBuckets {
		t.Errorf("tab = %v, want TabBuckets", done.tab)
	}
}

// ---------------------------------------------------------------------
// openCreateModal
// ---------------------------------------------------------------------

func TestOpenCreateModal_Streams(t *testing.T) {
	m := newTestModel(t)
	newM, _ := m.openCreateModal()
	if newM.modal != modalCreateStream {
		t.Errorf("modal = %v, want modalCreateStream", newM.modal)
	}
}

func TestOpenCreateModal_Buckets(t *testing.T) {
	m := newTestModel(t)
	m.tab = TabBuckets
	newM, _ := m.openCreateModal()
	if newM.modal != modalCreateBucket {
		t.Errorf("modal = %v, want modalCreateBucket", newM.modal)
	}
}

// ---------------------------------------------------------------------
// openDeleteModal — integration
// ---------------------------------------------------------------------

func TestOpenDeleteModal_StreamSelected(t *testing.T) {
	url := startJetStreamServer(t)
	client := newTestClient(t, url)
	m := New(client, theme.Default())

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("nats connect: %v", err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("jetstream new: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err = js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "del-me",
		Subjects: []string{"test-del.>"},
	})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	cmd := m.streams.Init()
	msg := cmd()
	m.streams, _ = m.streams.Update(msg)

	m.tab = TabStreams
	newM, _ := m.openDeleteModal()
	if newM.modal != modalDeleteStream {
		t.Errorf("modal = %v, want modalDeleteStream", newM.modal)
	}
	if newM.deleteTarget != "del-me" {
		t.Errorf("deleteTarget = %q, want %q", newM.deleteTarget, "del-me")
	}
}

func TestOpenDeleteModal_BucketSelected(t *testing.T) {
	url := startJetStreamServer(t)
	client := newTestClient(t, url)
	m := New(client, theme.Default())

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("nats connect: %v", err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("jetstream new: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err = js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "del-bucket"})
	if err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	cmd := m.buckets.Init()
	msg := cmd()
	m.buckets, _ = m.buckets.Update(msg)

	m.tab = TabBuckets
	newM, _ := m.openDeleteModal()
	if newM.modal != modalDeleteBucket {
		t.Errorf("modal = %v, want modalDeleteBucket", newM.modal)
	}
	if newM.deleteTarget != "del-bucket" {
		t.Errorf("deleteTarget = %q, want %q", newM.deleteTarget, "del-bucket")
	}
}
