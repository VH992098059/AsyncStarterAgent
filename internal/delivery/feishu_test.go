package delivery

import (
	"context"
	"testing"

	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
)

func TestMarkdownToFeishuBlocks_Heading(t *testing.T) {
	md := "# Title\n\n## Section\n\nContent"
	blocks := markdownToFeishuBlocks(md)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}
	// 校验 block 类型：heading1=3, heading2=4, text=2
	wantTypes := []int{feishuBlockTypeHeading1, feishuBlockTypeHeading2, feishuBlockTypeText}
	for i, want := range wantTypes {
		got := *blocks[i].BlockType
		if got != want {
			t.Errorf("block %d: want block_type %d, got %d", i, want, got)
		}
	}
	// M4: 校验标题文本已正确提取（# / ## 前缀被剥离）
	if got := feishuBlockText(blocks[0].Heading1); got != "Title" {
		t.Errorf("heading1 text: want %q, got %q", "Title", got)
	}
	if got := feishuBlockText(blocks[1].Heading2); got != "Section" {
		t.Errorf("heading2 text: want %q, got %q", "Section", got)
	}
}

func TestMarkdownToFeishuBlocks_EmptyLines(t *testing.T) {
	md := "\n\n# Title\n\n\n"
	blocks := markdownToFeishuBlocks(md)
	if len(blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(blocks))
	}
}

func TestMarkdownToFeishuBlocks_Paragraph(t *testing.T) {
	md := "First paragraph.\nSecond paragraph."
	blocks := markdownToFeishuBlocks(md)
	if len(blocks) != 2 {
		t.Errorf("expected 2 paragraphs, got %d", len(blocks))
	}
	for i, b := range blocks {
		if *b.BlockType != feishuBlockTypeText {
			t.Errorf("block %d: want text block_type %d, got %d", i, feishuBlockTypeText, *b.BlockType)
		}
	}
}

func TestMarkdownToFeishuBlocks_Heading3(t *testing.T) {
	md := "### Subsection"
	blocks := markdownToFeishuBlocks(md)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if *blocks[0].BlockType != feishuBlockTypeHeading3 {
		t.Errorf("want heading3 block_type %d, got %d", feishuBlockTypeHeading3, *blocks[0].BlockType)
	}
	if blocks[0].Heading3 == nil || len(blocks[0].Heading3.Elements) != 1 {
		t.Error("expected heading3 block with one element")
	}
}

func TestMarkdownToFeishuBlocks_Empty(t *testing.T) {
	blocks := markdownToFeishuBlocks("")
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks for empty input, got %d", len(blocks))
	}
}

func TestNewFeishuAdapter(t *testing.T) {
	a := NewFeishuAdapter(nil, "fake-token")
	if a == nil {
		t.Fatal("nil adapter")
	}
	if a.userToken != "fake-token" {
		t.Errorf("token: %s", a.userToken)
	}
}

func TestFeishuAdapter_CreateTaskComment_EmptyGUID(t *testing.T) {
	a := NewFeishuAdapter(nil, "fake-token")
	err := a.CreateTaskComment(nil, "", "content")
	if err == nil {
		t.Fatal("expected error for empty task guid")
	}
}

func TestFeishuAdapter_CreateDoc_NilClient(t *testing.T) {
	a := NewFeishuAdapter(nil, "tok")
	_, err := a.CreateDoc(context.Background(), "title", "# heading")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

// feishuBlockText 安全提取标题/段落块的纯文本内容（首个 TextRun 的 Content），
// 任一中间指针为 nil 时返回空串，避免测试因 SDK 指针链断裂而 panic。
func feishuBlockText(txt *larkdocx.Text) string {
	if txt == nil || len(txt.Elements) == 0 {
		return ""
	}
	el := txt.Elements[0]
	if el == nil || el.TextRun == nil || el.TextRun.Content == nil {
		return ""
	}
	return *el.TextRun.Content
}
