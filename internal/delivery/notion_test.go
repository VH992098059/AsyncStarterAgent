package delivery

import (
	"testing"
)

// TestMarkdownToBlocks_Paragraphs tests paragraph block generation (FR-D01, T023).
func TestMarkdownToBlocks_Paragraphs(t *testing.T) {
	md := "# Title\n\nFirst paragraph.\n\nSecond paragraph.\n"
	blocks := markdownToBlocks(md)
	if len(blocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(blocks))
	}
	if blocks[0]["type"] != "heading_1" {
		t.Errorf("first block type: got %v, want heading_1", blocks[0]["type"])
	}
	if blocks[1]["type"] != "paragraph" {
		t.Errorf("second block type: got %v, want paragraph", blocks[1]["type"])
	}
}

// TestMarkdownToBlocks_EmptyLines tests that empty lines are skipped (FR-D01, T023).
func TestMarkdownToBlocks_EmptyLines(t *testing.T) {
	md := "\n\n# Title\n\n\n"
	blocks := markdownToBlocks(md)
	if len(blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(blocks))
	}
}

// TestMarkdownToBlocks_Headings tests heading level detection (FR-D01, T023).
func TestMarkdownToBlocks_Headings(t *testing.T) {
	md := "# H1\n## H2\n### H3\n"
	blocks := markdownToBlocks(md)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}
	if blocks[0]["type"] != "heading_1" {
		t.Errorf("block 0: got %v, want heading_1", blocks[0]["type"])
	}
	if blocks[1]["type"] != "heading_2" {
		t.Errorf("block 1: got %v, want heading_2", blocks[1]["type"])
	}
	if blocks[2]["type"] != "heading_3" {
		t.Errorf("block 2: got %v, want heading_3", blocks[2]["type"])
	}
}

// TestMarkdownToBlocks_ListItems tests bulleted list item detection (FR-D01, T023).
func TestMarkdownToBlocks_ListItems(t *testing.T) {
	md := "- item 1\n- item 2\n* item 3\n"
	blocks := markdownToBlocks(md)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}
	for i, b := range blocks {
		if b["type"] != "bulleted_list_item" {
			t.Errorf("block %d: got %v, want bulleted_list_item", i, b["type"])
		}
	}
}

// TestParagraphBlock tests the paragraphBlock helper (FR-D01, T023).
func TestParagraphBlock(t *testing.T) {
	b := paragraphBlock("hello")
	if b["object"] != "block" {
		t.Errorf("object: got %v, want block", b["object"])
	}
	if b["type"] != "paragraph" {
		t.Errorf("type: got %v, want paragraph", b["type"])
	}
}

// TestNewNotionAdapter tests adapter construction (FR-D01, T023).
func TestNewNotionAdapter(t *testing.T) {
	a := NewNotionAdapter(NotionConfig{APIKey: "test-key", ParentPageID: "page-123"})
	if a == nil {
		t.Fatal("nil adapter")
	}
	if a.cfg.APIKey != "test-key" {
		t.Errorf("APIKey: got %s, want test-key", a.cfg.APIKey)
	}
}
