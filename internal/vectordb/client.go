package vectordb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/easp-platform/easp/internal/logger"
)

// Config 向量数据库配置
type Config struct {
	Endpoint string `json:"endpoint"` // 桥接服务地址
	Database string `json:"database"` // 数据库名
	Timeout  int    `json:"timeout"`  // 超时时间(秒)
}

// Client 向量数据库客户端 (通过Python桥接服务)
type Client struct {
	config     Config
	httpClient *http.Client
}

// NewClient 创建向量数据库客户端
func NewClient(config Config) *Client {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// Document 文档
type Document struct {
	ID     string                 `json:"id"`
	Vector []float32              `json:"vector,omitempty"` // 向量模式时使用
	Text   string                 `json:"text,omitempty"`   // 文本模式时使用，向量数据库自动 Embedding
	Fields map[string]interface{} `json:"fields,omitempty"`
}

// SearchResult 搜索结果
type SearchResult struct {
	ID     string                 `json:"id"`
	Score  float64                `json:"score"`
	Fields map[string]interface{} `json:"fields,omitempty"`
}

// HealthInfo 健康检查返回信息
type HealthInfo struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Embedding string `json:"embedding"`
	Dimension int    `json:"dimension"`
}

// apiRequest 发送API请求
func (c *Client) apiRequest(endpoint string, params interface{}) (json.RawMessage, error) {
	start := time.Now()
	bodyBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s%s", c.config.Endpoint, endpoint)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error("vectordb", "bridge request failed",
			logger.Field("endpoint", endpoint),
			logger.Field("duration_ms", time.Since(start).Milliseconds()),
			logger.Field("error", err.Error()),
		)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Code    int             `json:"code"`
		Message string          `json:"message,omitempty"`
		Error   string          `json:"error,omitempty"`
		Data    json.RawMessage `json:"data,omitempty"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		logger.Error("vectordb", "bridge response parse failed",
			logger.Field("endpoint", endpoint),
			logger.Field("status", resp.StatusCode),
			logger.Field("duration_ms", time.Since(start).Milliseconds()),
			logger.Field("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Error != "" {
		logger.Error("vectordb", "bridge api error",
			logger.Field("endpoint", endpoint),
			logger.Field("status", resp.StatusCode),
			logger.Field("duration_ms", time.Since(start).Milliseconds()),
			logger.Field("error", result.Error),
		)
		return nil, fmt.Errorf("API error: %s", result.Error)
	}

	logger.Info("vectordb", "bridge request completed",
		logger.Field("endpoint", endpoint),
		logger.Field("status", resp.StatusCode),
		logger.Field("duration_ms", time.Since(start).Milliseconds()),
	)
	return result.Data, nil
}

// ListDatabases 列出数据库
func (c *Client) ListDatabases() ([]string, error) {
	data, err := c.apiRequest("/api/database/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	var databases []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &databases); err != nil {
		return nil, err
	}

	names := make([]string, len(databases))
	for i, db := range databases {
		names[i] = db.Name
	}
	return names, nil
}

// CreateDatabase 创建数据库
func (c *Client) CreateDatabase(name string) error {
	_, err := c.apiRequest("/api/database/create", map[string]interface{}{"name": name})
	return err
}

// ListCollections 列出Collections
func (c *Client) ListCollections(database string) ([]string, error) {
	data, err := c.apiRequest("/api/collection/list", map[string]interface{}{"database": database})
	if err != nil {
		return nil, err
	}

	var collections []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &collections); err != nil {
		return nil, err
	}

	names := make([]string, len(collections))
	for i, coll := range collections {
		names[i] = coll.Name
	}
	return names, nil
}

// CreateCollection 创建Collection
func (c *Client) CreateCollection(database, name string, dimension int) error {
	_, err := c.apiRequest("/api/collection/create", map[string]interface{}{
		"database":      database,
		"collection":    name,
		"dimension":     dimension,
		"use_embedding": true,
	})
	return err
}

// Insert 插入文档 (向量模式 - 兼容旧接口)
func (c *Client) Insert(database, collection string, docs []Document) error {
	_, err := c.apiRequest("/api/document/insert", map[string]interface{}{
		"database":   database,
		"collection": collection,
		"documents":  docs,
	})
	return err
}

// InsertText 插入文档 (文本模式 - 推荐，向量数据库自动 Embedding)
func (c *Client) InsertText(database, collection, id, text string, fields map[string]interface{}) error {
	doc := map[string]interface{}{
		"id":      id,
		"content": text, // 向量数据库自动 Embedding 的字段
	}
	for k, v := range fields {
		doc[k] = v
	}
	_, err := c.apiRequest("/api/document/insert", map[string]interface{}{
		"database":   database,
		"collection": collection,
		"documents":  []map[string]interface{}{doc},
	})
	return err
}

// Search 向量搜索 (向量模式 - 兼容旧接口)
func (c *Client) Search(database, collection string, vector []float32, limit int) ([]SearchResult, error) {
	data, err := c.apiRequest("/api/document/search", map[string]interface{}{
		"database":   database,
		"collection": collection,
		"vector":     vector,
		"limit":      limit,
	})
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// SearchByText 文本搜索 (推荐 - 向量数据库自动 Embedding 后搜索)
func (c *Client) SearchByText(database, collection, queryText string, limit int, filters map[string]string) ([]SearchResult, error) {
	params := map[string]interface{}{
		"database":   database,
		"collection": collection,
		"query_text": queryText,
		"limit":      limit,
	}
	// 添加过滤条件
	for k, v := range filters {
		params[k] = v
	}

	data, err := c.apiRequest("/api/document/search", params)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// Delete 删除文档
func (c *Client) Delete(database, collection string, ids []string) error {
	_, err := c.apiRequest("/api/document/delete", map[string]interface{}{
		"database":   database,
		"collection": collection,
		"ids":        ids,
	})
	return err
}

// GetEmbedding 获取Embedding (兼容旧接口 - 推荐直接传文本给 insert/search)
func (c *Client) GetEmbedding(texts []string) ([][]float32, error) {
	data, err := c.apiRequest("/api/embedding", map[string]interface{}{
		"texts": texts,
	})
	if err != nil {
		return nil, err
	}

	var vectors [][]float32
	if err := json.Unmarshal(data, &vectors); err != nil {
		return nil, err
	}
	return vectors, nil
}

// GetDimension 获取向量维度 (使用 bge-large-zh-v1.5: 1024维)
func (c *Client) GetDimension() int {
	return 1024
}

// HealthCheck 健康检查
func (c *Client) HealthCheck() error {
	resp, err := c.httpClient.Get(c.config.Endpoint + "/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %d", resp.StatusCode)
	}
	return nil
}

// GetHealthInfo 厷取健康详情(含Embedding信息)
func (c *Client) GetHealthInfo() (*HealthInfo, error) {
	resp, err := c.httpClient.Get(c.config.Endpoint + "/health")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check failed: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var info HealthInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
