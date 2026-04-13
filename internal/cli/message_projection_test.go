package cli

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/protocol"
	"github.com/m0n0x41d/haft/internal/session"
	_ "modernc.org/sqlite"
)

func TestMsgInfoFromMessagePreservesPromptTextAndAttachments(t *testing.T) {
	message := agent.Message{
		ID:   "msg-1",
		Role: agent.RoleUser,
		Parts: []agent.Part{
			agent.TextPart{Text: "line one\n[not an attachment]\nline three"},
			agent.ImagePart{Filename: "clipboard.png", MIMEType: "image/png", Data: []byte("png")},
			agent.TextPart{Text: persistedFileAttachmentText("spec.md", "spec body")},
		},
	}

	got := msgInfoFromMessage(message)
	want := protocol.MsgInfo{
		ID:   "msg-1",
		Role: "user",
		Text: "line one\n[not an attachment]\nline three",
		Attachments: []protocol.MessageAttachment{
			{Name: "clipboard.png", IsImage: true},
			{Name: "spec.md", IsImage: false},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("msgInfoFromMessage() = %#v, want %#v", got, want)
	}
}

func TestMsgsToMsgInfosPreservesAttachmentsAfterSQLiteReload(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "session.db")
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	store, err := session.NewSQLiteStore(sqlDB)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	sess := &agent.Session{
		ID:        "sess-1",
		Title:     "resume attachments",
		Model:     "test-model",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("Create session: %v", err)
	}

	message := &agent.Message{
		ID:        "msg-1",
		SessionID: sess.ID,
		Role:      agent.RoleUser,
		Parts: []agent.Part{
			agent.TextPart{Text: "line one\n[not an attachment]\nline three"},
			agent.ImagePart{Filename: "clipboard.png", MIMEType: "image/png", Data: []byte("png")},
			agent.TextPart{Text: persistedFileAttachmentText("spec.md", "spec body")},
		},
		CreatedAt: now,
	}
	if err := store.Save(ctx, message); err != nil {
		t.Fatalf("Save message: %v", err)
	}

	storedMessages, err := store.ListBySession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("ListBySession: %v", err)
	}

	got := msgsToMsgInfos(storedMessages)
	want := []protocol.MsgInfo{
		{
			ID:   "msg-1",
			Role: "user",
			Text: "line one\n[not an attachment]\nline three",
			Attachments: []protocol.MessageAttachment{
				{Name: "clipboard.png", IsImage: true},
				{Name: "spec.md", IsImage: false},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("msgsToMsgInfos() = %#v, want %#v", got, want)
	}
}
