package model

type KnowledgeContent struct {
	Document Document `json:"document"` // 知识文档记录
	Content  string   `json:"content"`  // Markdown 正文
}

type SearchMatch struct {
	DocumentID string `json:"documentId"` // 匹配的文档标识
	Title      string `json:"title"`      // 显示标题
	Snippet    string `json:"snippet"`    // 匹配的正文摘要
}
