package openai

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

	if part.OfInputText == nil {
		t.Fatal("expected input_text payload")
	}

	if part.OfInputText.Text != "hello" {
		t.Fatalf("text = %q, want %q", part.OfInputText.Text, "hello")
	}
}

func TestConvertContentPart_ImageData(t *testing.T) {
	part, ok := convertContentPart(kit.NewImageDataPart([]byte("png-bytes"), "image/png"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	if part.OfInputImage == nil {
		t.Fatal("expected input_image payload")
	}

	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("png-bytes"))
	if got := part.OfInputImage.ImageURL.Value; got != want {
		t.Fatalf("image_url = %q, want %q", got, want)
	}
}

func TestConvertContentPart_DocumentData(t *testing.T) {
	part, ok := convertContentPart(kit.NewDocumentDataPart([]byte("%PDF-1.7"), "application/pdf"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	if part.OfInputFile == nil {
		t.Fatal("expected input_file payload")
	}

	wantData := base64.StdEncoding.EncodeToString([]byte("%PDF-1.7"))
	if got := part.OfInputFile.FileData.Value; got != wantData {
		t.Fatalf("file_data = %q, want %q", got, wantData)
	}

	if got := part.OfInputFile.Filename.Value; got != "document.pdf" {
		t.Fatalf("filename = %q, want %q", got, "document.pdf")
	}
}

func TestConvertContentPart_DocumentURL(t *testing.T) {
	part, ok := convertContentPart(kit.NewDocumentURLPart("https://example.com/files/spec.pdf"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	if part.OfInputFile == nil {
		t.Fatal("expected input_file payload")
	}

	if got := part.OfInputFile.FileURL.Value; got != "https://example.com/files/spec.pdf" {
		t.Fatalf("file_url = %q", got)
	}

	if got := part.OfInputFile.Filename.Value; got != "spec.pdf" {
		t.Fatalf("filename = %q, want %q", got, "spec.pdf")
	}
}

func TestConvertContentPart_DocumentURLWithQuery_UsesPathFilename(t *testing.T) {
	part, ok := convertContentPart(kit.NewDocumentURLPart("https://example.com/files/spec.pdf?download=1"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	if part.OfInputFile == nil {
		t.Fatal("expected input_file payload")
	}

	if got := part.OfInputFile.Filename.Value; got != "spec.pdf" {
		t.Fatalf("filename = %q, want %q", got, "spec.pdf")
	}
}
