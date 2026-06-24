import React, { useEffect, useMemo, useState } from 'react';
import { Card, Row, Col, Statistic, Typography, Space, Select, DatePicker, Button, Table, Tag, Progress, Empty, Spin, Alert, Drawer, Descriptions } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { BarChartOutlined, ThunderboltOutlined, ToolOutlined, BulbOutlined, RobotOutlined, ReloadOutlined, EyeOutlined, ClockCircleOutlined, DatabaseOutlined, CheckCircleOutlined, WarningOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';
import { usageApi } from '../api/usage';
import type { UsageAnalyticsResponse, UsageDetail, UsageGroupStats, ToolUsageStats } from '../api/usage';

const { Title, Text } = Typography;
const { RangePicker } = DatePicker;

interface LayoutContext {
  currentTenant: string;
}

const sourceLabels: Record<string, string> = {
  ai_assistant: 'AI助手',
  embed: '嵌入式助手',
  mcp_api: 'MCP API',
  skill: 'Skill调用',
  manual: '手动执行',
  unknown: '未知',
};

const resourceLabels: Record<string, string> = {
  mcp_tool: 'MCP工具',
  skill: 'Skill',
  builtin_tool: '内置工具',
  assistant: 'AI助手',
  embed: '嵌入式助手',
};

const fmt = (n?: number) => (n || 0).toLocaleString();
const fmtMs = (n?: number) => (n && n > 0 ? `${n}ms` : '-');

const MiniBars: React.FC<{ data: UsageGroupStats[]; valueKey?: 'total_tokens' | 'calls' }> = ({ data, valueKey = 'total_tokens' }) => {
  const max = Math.max(...data.map((i) => i[valueKey] || 0), 1);
  if (!data.length) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无数据" />;
  return (
    <Space direction="vertical" style={{ width: '100%' }} size={10}>
      {data.slice(0, 8).map((item) => (
        <div key={item.name}>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
            <Text ellipsis style={{ maxWidth: 220 }}>{sourceLabels[item.name] || item.name}</Text>
            <Text strong>{fmt(item[valueKey])}</Text>
          </div>
          <Progress percent={Math.round(((item[valueKey] || 0) / max) * 100)} showInfo={false} size="small" />
        </div>
      ))}
    </Space>
  );
};

const ToolBars: React.FC<{ data: ToolUsageStats[] }> = ({ data }) => {
  const max = Math.max(...data.map((i) => i.calls || 0), 1);
  if (!data.length) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无工具调用" />;
  return (
    <Space direction="vertical" style={{ width: '100%' }} size={10}>
      {data.slice(0, 8).map((item, idx) => (
        <div key={`${item.resource_type}-${item.resource_id}-${item.resource_name}-${idx}`}>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
            <Space size={6}>
              <Tag color={item.resource_type === 'skill' ? 'purple' : item.resource_type === 'mcp_tool' ? 'blue' : 'default'}>
                {resourceLabels[item.resource_type] || item.resource_type}
              </Tag>
              <Text ellipsis style={{ maxWidth: 180 }}>{item.resource_name || item.resource_id || '-'}</Text>
            </Space>
            <Text strong>{fmt(item.calls)}次</Text>
          </div>
          <Progress percent={Math.round(((item.calls || 0) / max) * 100)} showInfo={false} size="small" />
        </div>
      ))}
    </Space>
  );
};

const UsageAnalytics: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const isMobile = window.innerWidth < 768;
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<UsageAnalyticsResponse | null>(null);
  const [granularity, setGranularity] = useState<'day' | 'month' | 'year'>('day');
  const [source, setSource] = useState<string | undefined>();
  const [resourceType, setResourceType] = useState<string | undefined>();
  const [range, setRange] = useState<[Dayjs, Dayjs]>([dayjs().subtract(29, 'day'), dayjs()]);
  const [page, setPage] = useState(1);
  const [selectedDetail, setSelectedDetail] = useState<UsageDetail | null>(null);

  const load = async (nextPage = page) => {
    if (!currentTenant) return;
    setLoading(true);
    try {
      const res = await usageApi.analytics(currentTenant, {
        start_date: range[0].format('YYYY-MM-DD'),
        end_date: range[1].format('YYYY-MM-DD'),
        granularity,
        source,
        resource_type: resourceType,
        page: nextPage,
        page_size: 20,
      });
      setData(res.data);
      setPage(nextPage);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setData(null);
    setPage(1);
    load(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentTenant]);

  const trendMax = useMemo(() => Math.max(...(data?.trend || []).map((i) => i.total_tokens || 0), 1), [data]);

  const detailColumns: ColumnsType<UsageDetail> = [
    {
      title: '时间',
      dataIndex: 'created_at',
      width: 170,
      render: (v) => dayjs(v).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: '类型',
      dataIndex: 'kind',
      width: 90,
      render: (v) => <Tag color={v === 'model' ? 'geekblue' : 'green'}>{v === 'model' ? '模型' : '工具'}</Tag>,
    },
    {
      title: '来源',
      dataIndex: 'source',
      width: 110,
      render: (v) => sourceLabels[v] || v || '-',
    },
    {
      title: '模型/资源',
      render: (_, r) => r.kind === 'model'
        ? <Text>{r.provider}/{r.model}</Text>
        : <Space size={4}><Tag>{resourceLabels[r.resource_type] || r.resource_type}</Tag><Text>{r.resource_name || r.resource_id}</Text></Space>,
    },
    {
      title: 'Input',
      dataIndex: 'input_tokens',
      width: 100,
      align: 'right',
      render: fmt,
    },
    {
      title: 'Output',
      dataIndex: 'output_tokens',
      width: 100,
      align: 'right',
      render: fmt,
    },
    {
      title: '缓存命中',
      dataIndex: 'cached_tokens',
      width: 100,
      align: 'right',
      render: fmt,
    },
    {
      title: '总Tokens/次数',
      dataIndex: 'total_tokens',
      width: 130,
      align: 'right',
      render: (v, r) => (r.kind === 'model' ? fmt(v) : '1次'),
    },
    {
      title: '耗时',
      dataIndex: 'latency_ms',
      width: 90,
      align: 'right',
      render: fmtMs,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (v) => <Tag color={v === 'success' ? 'green' : 'red'}>{v || 'success'}</Tag>,
    },
    {
      title: '详情',
      width: 90,
      fixed: 'right',
      render: (_, r) => <Button size="small" icon={<EyeOutlined />} onClick={() => setSelectedDetail(r)}>查看</Button>,
    },
  ];

  const summary = data?.summary;
  const cacheRate = summary?.input_tokens ? Math.round(((summary.cached_tokens || 0) / summary.input_tokens) * 100) : 0;
  const totalCalls = (summary?.model_calls || 0) + (summary?.tool_calls || 0);
  const failedToolCalls = (data?.by_tool || []).reduce((sum, item) => sum + (item.failed_calls || 0), 0);
  const successRate = summary?.tool_calls ? Math.round((((summary.tool_calls || 0) - failedToolCalls) / Math.max(1, summary.tool_calls || 0)) * 100) : 100;
  const topSource = data?.by_source?.[0];
  const topModel = data?.by_model?.[0];

  const detailKindLabel = (r?: UsageDetail | null) => r?.kind === 'model' ? '模型调用' : '工具/Skill调用';

  return (
    <div>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }} wrap>
        <div>
          <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><BarChartOutlined /> 成本与调用可观测</Title>
          <Text type="secondary">统一查看模型 Token、缓存命中、工具/Skill 调用、来源归因、耗时和失败情况。</Text>
        </div>
        <Space wrap>
          <RangePicker
            value={range}
            onChange={(v) => v?.[0] && v?.[1] && setRange([v[0], v[1]])}
            allowClear={false}
          />
          <Select value={granularity} onChange={setGranularity} style={{ width: 100 }} options={[
            { value: 'day', label: '按日' },
            { value: 'month', label: '按月' },
            { value: 'year', label: '按年' },
          ]} />
          <Select allowClear placeholder="来源" value={source} onChange={setSource} style={{ width: 130 }} options={[
            { value: 'ai_assistant', label: 'AI助手' },
            { value: 'embed', label: '嵌入式助手' },
            { value: 'skill', label: 'Skill调用' },
            { value: 'mcp_api', label: 'MCP API' },
          ]} />
          <Select allowClear placeholder="资源类型" value={resourceType} onChange={setResourceType} style={{ width: 130 }} options={[
            { value: 'mcp_tool', label: 'MCP工具' },
            { value: 'skill', label: 'Skill' },
            { value: 'builtin_tool', label: '内置工具' },
          ]} />
          <Button type="primary" icon={<ReloadOutlined />} onClick={() => load(1)} loading={loading}>查询</Button>
        </Space>
      </Space>

      <Spin spinning={loading}>
        <Space direction="vertical" size="middle" style={{ width: '100%', marginBottom: 16 }}>
          <Alert
            type="info"
            showIcon
            message="用量分析用于回答：谁在调用、从哪里调用、消耗多少 Token、工具是否失败、耗时是否异常。"
            description="来源归因与审计日志保持一致：AI助手 / 嵌入式助手 / Skill调用 / MCP API / 手动执行。历史 unknown 已在查询层只读归一到 AI助手。"
          />
          <Row gutter={[16, 16]}>
            <Col xs={12} lg={6}><Card><Statistic title="总调用" value={totalCalls} prefix={<CheckCircleOutlined />} /></Card></Col>
            <Col xs={12} lg={6}><Card><Statistic title="缓存命中率" value={cacheRate} suffix="%" prefix={<DatabaseOutlined />} /></Card></Col>
            <Col xs={12} lg={6}><Card><Statistic title="工具成功率" value={successRate} suffix="%" valueStyle={{ color: successRate < 95 ? '#cf1322' : undefined }} prefix={successRate < 95 ? <WarningOutlined /> : <CheckCircleOutlined />} /></Card></Col>
            <Col xs={12} lg={6}><Card><Statistic title="主来源" value={topSource ? (sourceLabels[topSource.name] || topSource.name) : '-'} /></Card></Col>
          </Row>
          <Card size="small" title="当前查询摘要" extra={<Text type="secondary">基于筛选条件实时变化</Text>}>
            <Space wrap>
              <Tag color="blue">Top 来源：{topSource ? `${sourceLabels[topSource.name] || topSource.name} / ${fmt(topSource.total_tokens)} tokens` : '-'}</Tag>
              <Tag color="purple">Top 模型：{topModel ? `${topModel.name} / ${fmt(topModel.total_tokens)} tokens` : '-'}</Tag>
              <Tag icon={<ClockCircleOutlined />} color={(summary?.avg_latency_ms || 0) > 3000 ? 'red' : 'green'}>平均耗时：{summary?.avg_latency_ms || 0} ms</Tag>
              <Tag color={failedToolCalls > 0 ? 'red' : 'green'}>失败工具调用：{fmt(failedToolCalls)}</Tag>
            </Space>
          </Card>
        </Space>

        <Row gutter={[16, 16]}>
          <Col xs={12} lg={6}><Card><Statistic title="总 Tokens" value={summary?.total_tokens || 0} prefix={<ThunderboltOutlined />} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="输入 Tokens" value={summary?.input_tokens || 0} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="输出 Tokens" value={summary?.output_tokens || 0} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="缓存命中 Tokens" value={summary?.cached_tokens || 0} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="模型调用" value={summary?.model_calls || 0} prefix={<RobotOutlined />} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="工具调用" value={summary?.tool_calls || 0} prefix={<ToolOutlined />} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="Skill调用" value={summary?.skill_calls || 0} prefix={<BulbOutlined />} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="平均耗时" value={summary?.avg_latency_ms || 0} suffix="ms" /></Card></Col>
        </Row>

        <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
          <Col xs={24} lg={12}>
            <Card title={<Space><BarChartOutlined />Token趋势</Space>}>
              {(data?.trend || []).length ? (
                <Space direction="vertical" style={{ width: '100%' }} size={10}>
                  {(data?.trend || []).map((item) => (
                    <div key={item.period}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                        <Text>{item.period}</Text>
                        <Text strong>{fmt(item.total_tokens)} tokens / 输入 {fmt(item.input_tokens)} / 输出 {fmt(item.output_tokens)} / 缓存 {fmt(item.cached_tokens)} / {fmt(item.calls)}次</Text>
                      </div>
                      <Progress percent={Math.round(((item.total_tokens || 0) / trendMax) * 100)} showInfo={false} strokeColor="#1677ff" />
                    </div>
                  ))}
                </Space>
              ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无趋势数据" />}
            </Card>
          </Col>
          <Col xs={24} lg={12}>
            <Card title="按功能来源消耗">
              <MiniBars data={data?.by_source || []} />
            </Card>
          </Col>
          <Col xs={24} lg={12}>
            <Card title="按模型消耗 Top">
              <MiniBars data={data?.by_model || []} />
            </Card>
          </Col>
          <Col xs={24} lg={12}>
            <Card title="工具/Skill 调用 Top">
              <ToolBars data={data?.by_tool || []} />
            </Card>
          </Col>
        </Row>

        <Card title="明细记录" style={{ marginTop: 16 }}>
          <Table
            rowKey={(r) => `${r.kind}-${r.id}`}
            columns={detailColumns}
            dataSource={data?.details || []}
            scroll={{ x: 1000 }}
            size={isMobile ? 'small' : 'middle'}
            pagination={{
              current: page,
              pageSize: data?.page_size || 20,
              total: data?.total || 0,
              onChange: (p) => load(p),
              showSizeChanger: false,
            }}
          />
        </Card>
      </Spin>

      <Drawer
        title={selectedDetail ? `${detailKindLabel(selectedDetail)}详情` : '调用详情'}
        open={!!selectedDetail}
        onClose={() => setSelectedDetail(null)}
        width={isMobile ? '100%' : 720}
      >
        {selectedDetail && (
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <Alert
              type={selectedDetail.status === 'success' || selectedDetail.kind === 'model' ? 'success' : 'error'}
              showIcon
              message={selectedDetail.kind === 'model' ? '模型 Token 计量记录' : '工具 / Skill 调用计量记录'}
              description="用量明细可与审计日志通过来源、用户、Request ID、资源类型/资源ID进行交叉定位。"
            />
            <Card size="small" title="来源与身份">
              <Descriptions column={1} bordered size="small">
                <Descriptions.Item label="来源">{sourceLabels[selectedDetail.source] || selectedDetail.source || '-'}</Descriptions.Item>
                <Descriptions.Item label="来源名称">{selectedDetail.source_name || sourceLabels[selectedDetail.source] || '-'}</Descriptions.Item>
                <Descriptions.Item label="EASP 用户">{selectedDetail.user_id || '-'}</Descriptions.Item>
                <Descriptions.Item label="Request ID">{selectedDetail.request_id || '-'}</Descriptions.Item>
                <Descriptions.Item label="时间">{selectedDetail.created_at ? dayjs(selectedDetail.created_at).format('YYYY-MM-DD HH:mm:ss') : '-'}</Descriptions.Item>
              </Descriptions>
            </Card>
            <Card size="small" title="资源与消耗">
              <Descriptions column={1} bordered size="small">
                <Descriptions.Item label="类型"><Tag color={selectedDetail.kind === 'model' ? 'geekblue' : 'green'}>{detailKindLabel(selectedDetail)}</Tag></Descriptions.Item>
                <Descriptions.Item label="模型 / Provider">{selectedDetail.kind === 'model' ? `${selectedDetail.provider}/${selectedDetail.model}` : '-'}</Descriptions.Item>
                <Descriptions.Item label="资源类型">{resourceLabels[selectedDetail.resource_type] || selectedDetail.resource_type || '-'}</Descriptions.Item>
                <Descriptions.Item label="资源ID">{selectedDetail.resource_id || '-'}</Descriptions.Item>
                <Descriptions.Item label="资源名称">{selectedDetail.resource_name || '-'}</Descriptions.Item>
                <Descriptions.Item label="Input Tokens">{fmt(selectedDetail.input_tokens)}</Descriptions.Item>
                <Descriptions.Item label="Output Tokens">{fmt(selectedDetail.output_tokens)}</Descriptions.Item>
                <Descriptions.Item label="缓存命中 Tokens">{fmt(selectedDetail.cached_tokens)}</Descriptions.Item>
                <Descriptions.Item label="总 Tokens / 次数">{selectedDetail.kind === 'model' ? fmt(selectedDetail.total_tokens) : '1次'}</Descriptions.Item>
                <Descriptions.Item label="耗时">{fmtMs(selectedDetail.latency_ms)}</Descriptions.Item>
                <Descriptions.Item label="状态"><Tag color={selectedDetail.status === 'success' ? 'green' : 'red'}>{selectedDetail.status || 'success'}</Tag></Descriptions.Item>
                <Descriptions.Item label="错误信息">{selectedDetail.error_message || '-'}</Descriptions.Item>
              </Descriptions>
            </Card>
          </Space>
        )}
      </Drawer>
    </div>
  );
};

export default UsageAnalytics;
