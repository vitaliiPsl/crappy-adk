package openaicompat

import (
	"encoding/base64"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func TestConvertContentPart_Text(t *testing.T) {
	part, ok := convertContentPart(kit.NewTextPart("hello"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	if part.OfText == nil {
		t.Fatal("expected text content part")
	}

	if part.OfText.Text != "hello" {
		t.Fatalf("text = %q, want %q", part.OfText.Text, "hello")
	}
}

func TestConvertContentPart_ImageData(t *testing.T) {
	part, ok := convertContentPart(kit.NewImageDataPart([]byte("png-bytes"), "image/png"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	if part.OfImageURL == nil {
		t.Fatal("expected image_url content part")
	}

	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("png-bytes"))
	if got := part.OfImageURL.ImageURL.URL; got != want {
		t.Fatalf("image_url = %q, want %q", got, want)
	}
}

func TestConvertContentPart_DocumentData(t *testing.T) {
	part, ok := convertContentPart(kit.NewDocumentDataPart([]byte("%PDF-1.7"), "application/pdf"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	if part.OfFile == nil {
		t.Fatal("expected file content part")
	}

	wantData := base64.StdEncoding.EncodeToString([]byte("%PDF-1.7"))
	if got := part.OfFile.File.FileData.Value; got != wantData {
		t.Fatalf("file_data = %q, want %q", got, wantData)
	}

	if got := part.OfFile.File.Filename.Value; got != "document.pdf" {
		t.Fatalf("filename = %q, want %q", got, "document.pdf")
	}
}

func TestConvertContentPart_DocumentURLUnsupported(t *testing.T) {
	_, ok := convertContentPart(kit.NewDocumentURLPart("https://example.com/files/spec.pdf"))
	if ok {
		t.Fatal("expected remote document URLs to be unsupported in chat-completions adapter")
	}
}

func TestConvertMessage_UserDropsUnsupportedRemoteDocumentURL(t *testing.T) {
	msgs := convertMessage(kit.NewUserMessage(
		kit.NewTextPart("see attached"),
		kit.NewDocumentURLPart("https://example.com/files/spec.pdf"),
		kit.NewImageURLPart("https://example.com/image.png"),
	))

	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}

	user := msgs[0].OfUser
	if user == nil {
		t.Fatal("expected user message")
	}

	if got := len(user.Content.OfArrayOfContentParts); got != 2 {
		t.Fatalf("len(content parts) = %d, want 2", got)
	}

	if user.Content.OfArrayOfContentParts[0].OfText == nil {
		t.Fatal("expected first content part to remain text")
	}

	if user.Content.OfArrayOfContentParts[1].OfImageURL == nil {
		t.Fatal("expected second content part to remain image")
	}
}

func TestDocumentFilename_URLWithQuery_UsesPathFilename(t *testing.T) {
	got := documentFilename(kit.ContentPart{
		Type: kit.ContentTypeDocument,
		URL:  "https://example.com/files/spec.pdf?download=1",
	})

	if got != "spec.pdf" {
		t.Fatalf("filename = %q, want %q", got, "spec.pdf")
	}
}

func TestConvertMessage_UserPreservesMultimodalContent(t *testing.T) {
	msgs := convertMessage(kit.NewUserMessage(
		kit.NewTextPart("see attached"),
		kit.NewImageURLPart("https://example.com/image.png"),
		kit.NewDocumentDataPart([]byte("%PDF-1.7"), "application/pdf"),
	))

	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}

	user := msgs[0].OfUser
	if user == nil {
		t.Fatal("expected user message")
	}

	if got := len(user.Content.OfArrayOfContentParts); got != 3 {
		t.Fatalf("len(content parts) = %d, want 3", got)
	}
}
