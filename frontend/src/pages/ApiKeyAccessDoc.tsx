import React from 'react';
import { Button, Card, Space, Typography } from 'antd';
import { ArrowLeftOutlined, FileMarkdownOutlined } from '@ant-design/icons';
import { Link } from 'react-router-dom';
import { MarkdownRenderer, MARKDOWN_CSS } from '../utils/markdown';
import apiKeyAccessDoc from '../docs/api-key-access.md?raw';

const { Title, Text } = Typography;

const ApiKeyAccessDoc: React.FC = () => {
  const isMobile = window.innerWidth < 768;

  return (
    <div style={{ minHeight: '100vh', background: '#f5f7fb', padding: isMobile ? 12 : 32 }}>
      <style>{`
        ${MARKDOWN_CSS}
        .api-doc-page .markdown-body h1 { font-size: ${isMobile ? 24 : 32}px; margin: 0 0 16px; }
        .api-doc-page .markdown-body h2 { font-size: ${isMobile ? 18 : 22}px; margin-top: 28px; padding-top: 8px; border-top: 1px solid #f0f0f0; }
        .api-doc-page .markdown-body h3 { font-size: 18px; margin-top: 20px; }
        .api-doc-page .markdown-body p { margin: 8px 0; }
        .api-doc-page .markdown-body pre { background: #0f172a; color: #e2e8f0; padding: 14px; border-radius: 8px; }
        .api-doc-page .markdown-body code { color: #d4380d; background: #fff7e6; }
        .api-doc-page .markdown-body pre code { color: inherit; background: transparent; }
        .api-doc-page .markdown-body table { display: block; overflow-x: auto; white-space: nowrap; }
      `}</style>

      <div style={{ maxWidth: 1080, margin: '0 auto' }}>
        <Card style={{ marginBottom: 16, borderRadius: 12 }} bodyStyle={{ padding: isMobile ? 16 : 24 }}>
          <Space direction={isMobile ? 'vertical' : 'horizontal'} style={{ width: '100%', justifyContent: 'space-between' }}>
            <Space align="center">
              <FileMarkdownOutlined style={{ fontSize: 24, color: '#1677ff' }} />
              <div>
                <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}>API Key 接入文档</Title>
                <Text type="secondary">公开页面，无需登录和权限；内容由 Markdown 文档维护。</Text>
              </div>
            </Space>
            <Link to="/login">
              <Button icon={<ArrowLeftOutlined />}>返回平台</Button>
            </Link>
          </Space>
        </Card>

        <Card className="api-doc-page" style={{ borderRadius: 12 }} bodyStyle={{ padding: isMobile ? 16 : 32 }}>
          <MarkdownRenderer content={apiKeyAccessDoc} />
        </Card>
      </div>
    </div>
  );
};

export default ApiKeyAccessDoc;
