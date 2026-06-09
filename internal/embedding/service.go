package embedding

import (
	"github.com/easp-platform/easp/internal/vectordb"
)

// EmbeddingConfig Embedding配置
type EmbeddingConfig struct {
	Endpoint  string `json:"endpoint"`  // 桥接服务地址
	Dimension int    `json:"dimension"` // 向量维度
}

// EmbeddingService Embedding服务 (通过VectorDB桥接服务)
type EmbeddingService struct {
	client    *vectordb.Client
	dimension int
}

// NewEmbeddingService 创建Embedding服务
func NewEmbeddingService(config EmbeddingConfig) *EmbeddingService {
	return &EmbeddingService{
		client: vectordb.NewClient(vectordb.Config{
			Endpoint: config.Endpoint,
			Timeout:  30,
		}),
		dimension: config.Dimension,
	}
}

// GetEmbedding 获取单个文本的Embedding
func (s *EmbeddingService) GetEmbedding(text string) ([]float32, error) {
	vectors, err := s.GetEmbeddings([]string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, nil
	}
	return vectors[0], nil
}

// GetEmbeddings 批量获取Embedding
func (s *EmbeddingService) GetEmbeddings(texts []string) ([][]float32, error) {
	return s.client.GetEmbedding(texts)
}

// GetDimension 获取向量维度
func (s *EmbeddingService) GetDimension() int {
	if s.dimension > 0 {
		return s.dimension
	}
	return 1536
}
