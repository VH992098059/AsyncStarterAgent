// F011: 飞书云文档交付适配器（FR-D03, 决策 #7）
// 通过 lark SDK 以 user_access_token 身份创建云文档并写入 markdown 内容，
// 同时支持向原始飞书任务回写草稿链接评论。
package delivery

import (
	"context"
	"fmt"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
	larktask "github.com/larksuite/oapi-sdk-go/v3/service/task/v2"
)

// 飞书 docx block_type 枚举（SDK 未导出常量，按官方 API 文档定义）
// https://open.feishu.cn/document/server-docs/docs/docs/docx-v1/document-block/list
const (
	feishuBlockTypeText     = 2 // 文本（段落）
	feishuBlockTypeHeading1 = 3
	feishuBlockTypeHeading2 = 4
	feishuBlockTypeHeading3 = 5
)

// FeishuAdapter 通过飞书 lark.Client 创建云文档并回写任务评论（决策 #7: 以用户身份调用）。
// cli 是共享的基础 client（不持有用户身份），userToken 在每次请求时通过
// larkcore.WithUserAccessToken 注入，因此 adapter 本身可安全复用。
type FeishuAdapter struct {
	cli       *lark.Client
	userToken string
}

// NewFeishuAdapter 构造飞书交付适配器。
func NewFeishuAdapter(cli *lark.Client, userToken string) *FeishuAdapter {
	return &FeishuAdapter{cli: cli, userToken: userToken}
}

// CreateDoc 创建飞书云文档并写入 markdown 转换后的 block 内容。
// 返回新文档的 URL（https://feishu.cn/docx/{document_id}）。
//
// 已知限制：markdownToFeishuBlocks 仅处理标题（#/##/###）与段落，
// 列表/代码块/引用等暂降级为段落。若 block 写入失败但文档已创建，
// 返回已创建文档的 URL 并附带错误（保证草稿链接可用）。
func (a *FeishuAdapter) CreateDoc(ctx context.Context, title, markdown string) (string, error) {
	if a.cli == nil {
		return "", fmt.Errorf("feishu adapter: nil lark client")
	}

	// 1. 创建空文档
	createReq := larkdocx.NewCreateDocumentReqBuilder().
		Body(larkdocx.NewCreateDocumentReqBodyBuilder().
			Title(title).
			Build()).
		Build()
	createResp, err := a.cli.Docx.Document.Create(ctx, createReq, larkcore.WithUserAccessToken(a.userToken))
	if err != nil {
		return "", fmt.Errorf("feishu create doc: %w", err)
	}
	if !createResp.Success() {
		return "", fmt.Errorf("feishu create doc: code=%d msg=%s", createResp.Code, createResp.Msg)
	}
	if createResp.Data == nil || createResp.Data.Document == nil || createResp.Data.Document.DocumentId == nil {
		return "", fmt.Errorf("feishu create doc: empty document id in response")
	}
	docID := *createResp.Data.Document.DocumentId
	// 已知限制（M5）：硬编码 feishu.cn 域名，国际版（larksuite.com）用户需另行处理，
	// 暂不区分租户区域。V1.5 再按 app_id 区域动态选择域名。
	docURL := "https://feishu.cn/docx/" + docID

	// 2. 写入 markdown 转换的 blocks（文档根 block id = document id）
	blocks := markdownToFeishuBlocks(markdown)
	if len(blocks) == 0 {
		return docURL, nil
	}
	blockReq := larkdocx.NewCreateDocumentBlockChildrenReqBuilder().
		DocumentId(docID).
		BlockId(docID).
		Body(larkdocx.NewCreateDocumentBlockChildrenReqBodyBuilder().
			Children(blocks).
			Build()).
		Build()
	blockResp, err := a.cli.Docx.DocumentBlockChildren.Create(ctx, blockReq, larkcore.WithUserAccessToken(a.userToken))
	if err != nil {
		// 文档已创建，仅 block 写入失败：返回 URL 但上报错误
		return docURL, fmt.Errorf("feishu write blocks: %w (doc created at %s)", err, docURL)
	}
	if !blockResp.Success() {
		return docURL, fmt.Errorf("feishu write blocks: code=%d msg=%s (doc created at %s)", blockResp.Code, blockResp.Msg, docURL)
	}
	return docURL, nil
}

// CreateTaskComment 向飞书任务（task/v2）添加评论，用于回写草稿链接（FR-D04）。
// taskGUID 为任务的全局唯一标识。
func (a *FeishuAdapter) CreateTaskComment(ctx context.Context, taskGUID, content string) error {
	if taskGUID == "" {
		return fmt.Errorf("feishu create task comment: empty task guid")
	}
	if a.cli == nil {
		return fmt.Errorf("feishu adapter: nil lark client")
	}
	// task/v2 comment 的任务关联通过 InputComment.ResourceType/ResourceId 表达
	// （API 路径为 /open-apis/task/v2/comments，task_guid 不在 path 中）
	req := larktask.NewCreateCommentReqBuilder().
		InputComment(larktask.NewInputCommentBuilder().
			Content(content).
			ResourceType("task").
			ResourceId(taskGUID).
			Build()).
		Build()
	resp, err := a.cli.Task.V2.Comment.Create(ctx, req, larkcore.WithUserAccessToken(a.userToken))
	if err != nil {
		return fmt.Errorf("feishu create task comment: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("feishu create task comment: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// markdownToFeishuBlocks 将 markdown 转换为飞书 docx Block 列表。
// 简化实现：按行处理，#/##/### 转为对应级别标题块，其余转为段落块，空行跳过。
// 列表/代码块/引用等暂降级为段落。
func markdownToFeishuBlocks(md string) []*larkdocx.Block {
	lines := strings.Split(md, "\n")
	blocks := make([]*larkdocx.Block, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "### "):
			blocks = append(blocks, larkdocx.NewBlockBuilder().
				BlockType(feishuBlockTypeHeading3).
				Heading3(feishuText(trimmed[4:])).
				Build())
		case strings.HasPrefix(trimmed, "## "):
			blocks = append(blocks, larkdocx.NewBlockBuilder().
				BlockType(feishuBlockTypeHeading2).
				Heading2(feishuText(trimmed[3:])).
				Build())
		case strings.HasPrefix(trimmed, "# "):
			blocks = append(blocks, larkdocx.NewBlockBuilder().
				BlockType(feishuBlockTypeHeading1).
				Heading1(feishuText(trimmed[2:])).
				Build())
		default:
			blocks = append(blocks, larkdocx.NewBlockBuilder().
				BlockType(feishuBlockTypeText).
				Text(feishuText(trimmed)).
				Build())
		}
	}
	return blocks
}

// feishuText 构造只含单个 TextRun（纯文本）的 *Text，供标题/段落块复用。
func feishuText(content string) *larkdocx.Text {
	return larkdocx.NewTextBuilder().
		Elements([]*larkdocx.TextElement{
			larkdocx.NewTextElementBuilder().
				TextRun(larkdocx.NewTextRunBuilder().Content(content).Build()).
				Build(),
		}).
		Build()
}
