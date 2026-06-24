import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { Table, Button, Modal, Form, Input, Typography, App, Tabs, Tag, Descriptions, Select, Switch, InputNumber, Space, Popconfirm, Dropdown, Card, Row, Col, Statistic, Alert, Progress, Divider, Tooltip, Empty, Drawer } from 'antd';
import { PlusOutlined, DatabaseOutlined, UserOutlined, ApartmentOutlined, BulbOutlined, BarChartOutlined, EditOutlined, DeleteOutlined, MoreOutlined, SafetyCertificateOutlined, SearchOutlined, AuditOutlined, HddOutlined, SyncOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { MemoryPool, MemoryEntry, UserMemory, Entity, SkillMemory, MemoryStats, MemorySettings, MemoryAuditLog, MemoryScoreBreakdown } from '../api/memory';
import { memoryApi } from '../api/memory';
import { useAuth } from '../contexts/AuthContext';

const { Title, Text, Paragraph } = Typography;
interface LayoutContext { currentTenant: string; }

const fmtTime = (v?: string | null) => (v ? new Date(v).toLocaleString() : '-');
const fmtScore = (v?: number) => Math.round((v || 0) * 100);

const statusColor: Record<string, string> = {
  active: 'green',
  archived: 'orange',
  deleted: 'red',
};

const actionColor: Record<string, string> = {
  save: 'green',
  sanitize: 'gold',
  deduplicate: 'blue',
  vector_index_failed: 'red',
  vector_search_fallback: 'orange',
  archive: 'purple',
};

const Memory: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { user } = useAuth();
  const { message } = App.useApp();
  const [pools, setPools] = useState<MemoryPool[]>([]);
  const [poolEntries, setPoolEntries] = useState<MemoryEntry[]>([]);
  const [entriesLoading, setEntriesLoading] = useState(false);
  const [selectedPoolId, setSelectedPoolId] = useState<string>('');
  const [userMemories, setUserMemories] = useState<UserMemory[]>([]);
  const [entities, setEntities] = useState<Entity[]>([]);
  const [skillMemories, setSkillMemories] = useState<SkillMemory[]>([]);
  const [stats, setStats] = useState<MemoryStats | null>(null);
  const [settings, setSettings] = useState<MemorySettings | null>(null);
  const [auditLogs, setAuditLogs] = useState<MemoryAuditLog[]>([]);
  const [recallResults, setRecallResults] = useState<UserMemory[]>([]);
  const [recallExplanations, setRecallExplanations] = useState<MemoryScoreBreakdown[]>([]);
  const [recallQuery, setRecallQuery] = useState('');
  const [recallUserId, setRecallUserId] = useState('');
  const [loading, setLoading] = useState(false);
  const [settingsSaving, setSettingsSaving] = useState(false);
  const [searchLoading, setSearchLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [selectedMemory, setSelectedMemory] = useState<UserMemory | null>(null);
  const [selectedAudit, setSelectedAudit] = useState<MemoryAuditLog | null>(null);
  const [editingPool, setEditingPool] = useState<MemoryPool | null>(null);
  const [activeTab, setActiveTab] = useState('governance');
  const [form] = Form.useForm();
  const isMobile = window.innerWidth < 768;

  const explanationMap = useMemo(() => {
    const map = new Map<string, MemoryScoreBreakdown>();
    recallExplanations.forEach((item) => map.set(item.memory_id, item));
    return map;
  }, [recallExplanations]);

  const archivedCount = useMemo(() => userMemories.filter((item) => item.status === 'archived').length, [userMemories]);
  const vectorIndexedCount = useMemo(() => userMemories.filter((item) => !!item.vector_indexed_at).length, [userMemories]);
  const activeMemoryCount = useMemo(() => userMemories.filter((item) => (item.status || 'active') === 'active').length, [userMemories]);
  const sourceStats = useMemo(() => userMemories.reduce<Record<string, number>>((acc, item) => {
    const key = item.source || 'unknown';
    acc[key] = (acc[key] || 0) + 1;
    return acc;
  }, {}), [userMemories]);
  const topMemorySource = useMemo(() => Object.entries(sourceStats).sort((a, b) => b[1] - a[1])[0], [sourceStats]);
  const vectorRate = userMemories.length ? Math.round((vectorIndexedCount / userMemories.length) * 100) : 0;

  const load = useCallback(async () => {
    if (!currentTenant) return;
    setLoading(true);
    try {
      const [poolsRes, userRes, entityRes, skillRes, statsRes, settingsRes, auditRes] = await Promise.all([
        memoryApi.listPools(currentTenant),
        memoryApi.listAllUserMemories(currentTenant, 100),
        memoryApi.listEntities(currentTenant),
        memoryApi.listSkillMemories(currentTenant),
        memoryApi.getStats(currentTenant),
        memoryApi.getSettings(currentTenant),
        memoryApi.listAuditLogs(currentTenant, 100),
      ]);
      setPools(poolsRes.data || []);
      setSelectedPoolId((prev) => prev || poolsRes.data?.[0]?.id || '');
      setUserMemories(userRes.data?.memories || []);
      setEntities(entityRes.data?.entities || []);
      setSkillMemories(skillRes.data?.memories || []);
      setStats(statsRes.data || null);
      setSettings(settingsRes.data || null);
      setAuditLogs(auditRes.data?.logs || []);
    } catch { message.error('加载失败'); }
    finally { setLoading(false); }
  }, [currentTenant, message]);

  useEffect(() => { load(); }, [load]);

  useEffect(() => {
    if (!selectedPoolId) {
      setPoolEntries([]);
      return;
    }
    setEntriesLoading(true);
    memoryApi.listEntries(selectedPoolId, 100)
      .then((res) => setPoolEntries(res.data || []))
      .catch(() => message.error('加载记忆池条目失败'))
      .finally(() => setEntriesLoading(false));
  }, [selectedPoolId, message]);

  useEffect(() => {
    if (!recallUserId && user?.id) setRecallUserId(user.id);
  }, [recallUserId, user?.id]);

  const openPoolModal = useCallback((record?: MemoryPool) => {
    if (record) {
      setEditingPool(record);
      form.setFieldsValue({
        ...record,
        trigger_rules: record.trigger_rules ? JSON.stringify(JSON.parse(record.trigger_rules), null, 2) : '',
      });
    } else {
      setEditingPool(null);
      form.resetFields();
      form.setFieldsValue({
        type: 'personal',
        purpose: 'conversation',
        priority: 5,
        max_tokens: 0,
        auto_activate: true,
        enabled: true,
      });
    }
    setModalOpen(true);
  }, [form]);

  const onPoolOk = async () => {
    const values = await form.validateFields();
    try {
      const triggerRules = values.trigger_rules || null;
      if (triggerRules && triggerRules.trim()) {
        try { JSON.parse(triggerRules); } catch { message.error('触发规则必须是合法JSON'); return; }
      }

      const payload: Partial<MemoryPool> = {
        ...values,
        trigger_rules: triggerRules,
      };

      if (editingPool) {
        await memoryApi.updatePool(editingPool.id, payload);
        message.success('更新成功');
      } else {
        await memoryApi.createPool(currentTenant, payload);
        message.success('创建成功');
      }
      setModalOpen(false); form.resetFields(); setEditingPool(null); load();
    } catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '操作失败'); }
  };

  const onDeletePool = async (id: string) => {
    try { await memoryApi.deletePool(id); message.success('删除成功'); load(); }
    catch { message.error('删除失败'); }
  };

  const updateSetting = async (patch: Partial<MemorySettings>) => {
    if (!currentTenant || !settings) return;
    const next = { ...settings, ...patch };
    setSettings(next);
    setSettingsSaving(true);
    try {
      const res = await memoryApi.updateSettings(currentTenant, patch);
      setSettings(res.data);
      message.success('配置已保存');
    } catch {
      setSettings(settings);
      message.error('配置保存失败');
    } finally {
      setSettingsSaving(false);
    }
  };

  const runRecallDebug = async () => {
    const uid = recallUserId.trim() || user?.id || '';
    if (!uid) { message.warning('请输入用户ID'); return; }
    if (!recallQuery.trim()) { message.warning('请输入检索问题'); return; }
    setSearchLoading(true);
    try {
      const res = await memoryApi.searchUserMemories(currentTenant, uid, recallQuery.trim(), 10);
      setRecallResults(res.data?.memories || []);
      setRecallExplanations(res.data?.explanations || []);
      message.success('召回完成');
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '召回调试失败');
    } finally {
      setSearchLoading(false);
    }
  };

  const poolColumns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    ...(!isMobile ? [
      { title: '类型', dataIndex: 'type', key: 'type', width: 80, render: (v: string) => {
        const colorMap: Record<string, string> = { personal: 'blue', team: 'green', system: 'purple' };
        return <Tag color={colorMap[v] || 'default'}>{v}</Tag>;
      }},
      { title: '用途', dataIndex: 'purpose', key: 'purpose', width: 100, render: (v: string) => {
        const colorMap: Record<string, string> = { conversation: 'cyan', skill: 'orange', knowledge: 'gold' };
        return <Tag color={colorMap[v] || 'default'}>{v}</Tag>;
      }},
      { title: '优先级', dataIndex: 'priority', key: 'priority', width: 70 },
      { title: 'Token限制', dataIndex: 'max_tokens', key: 'max_tokens', width: 100, render: (v: number) => v === 0 ? '不限' : v },
      { title: '自动激活', dataIndex: 'auto_activate', key: 'auto_activate', width: 90, render: (v: boolean) => v ? <Tag color="green">是</Tag> : <Tag>否</Tag> },
      { title: '记忆数', dataIndex: 'memory_count', key: 'memory_count', width: 80 },
      { title: '状态', dataIndex: 'enabled', key: 'enabled', width: 70, render: (v: boolean) => <Tag color={v ? 'green' : 'red'}>{v ? '启用' : '禁用'}</Tag> },
    ] : []),
    { title: '操作', key: 'action', width: isMobile ? 60 : 120, render: (_: unknown, record: MemoryPool) => (
      isMobile ? (
        <Dropdown menu={{ items: [
          { key: 'edit', label: '编辑', icon: <EditOutlined />, onClick: () => openPoolModal(record) },
          { key: 'delete', label: '删除', icon: <DeleteOutlined />, danger: true, onClick: () => onDeletePool(record.id) },
        ]}} trigger={['click']}>
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ) : (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => openPoolModal(record)}>编辑</Button>
          <Popconfirm title="确认删除?" onConfirm={() => onDeletePool(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      )
    )},
  ];

  const userMemoryColumns = [
    { title: '类型', dataIndex: 'type', key: 'type', width: 90, render: (v: string) => {
      const colorMap: Record<string, string> = { preference: 'blue', fact: 'green', feedback: 'orange' };
      return <Tag color={colorMap[v] || 'default'}>{v}</Tag>;
    }},
    { title: '内容', dataIndex: 'content', key: 'content', ellipsis: true, render: (v: string) => <Tooltip title={v}>{v}</Tooltip> },
    ...(!isMobile ? [
      { title: '用户ID', dataIndex: 'user_id', key: 'user_id', ellipsis: true, width: 130 },
      { title: '来源', dataIndex: 'source', key: 'source', width: 90, render: (v: string) => v ? <Tag>{v}</Tag> : '-' },
      { title: '状态', dataIndex: 'status', key: 'status', width: 90, render: (v: string) => <Tag color={statusColor[v] || 'default'}>{v || 'active'}</Tag> },
      { title: '访问/出现', key: 'access', width: 100, render: (_: unknown, record: UserMemory) => `${record.access_count || 0}/${record.last_seen_at ? 1 : 0}` },
      { title: '向量', dataIndex: 'vector_indexed_at', key: 'vector_indexed_at', width: 90, render: (v: string | null) => v ? <Tag color="green">已索引</Tag> : <Tag>未索引</Tag> },
    ] : []),
    { title: '更新时间', dataIndex: 'updated_at', key: 'updated_at', render: fmtTime },
    { title: '详情', key: 'detail', width: 80, render: (_: unknown, record: UserMemory) => <Button size="small" onClick={() => setSelectedMemory(record)}>查看</Button> },
  ];

  const poolEntryColumns = [
    { title: '类型', dataIndex: 'type', key: 'type', width: 100, render: (v: string) => <Tag>{v || 'fact'}</Tag> },
    { title: '内容', dataIndex: 'content', key: 'content', ellipsis: true, render: (v: string) => <Tooltip title={v}>{v}</Tooltip> },
    ...(!isMobile ? [
      { title: '敏感级别', dataIndex: 'sensitivity', key: 'sensitivity', width: 100, render: (v: string) => <Tag color={v === 'high' ? 'red' : v === 'medium' ? 'orange' : 'green'}>{v || 'low'}</Tag> },
      { title: '记忆池ID', dataIndex: 'pool_id', key: 'pool_id', ellipsis: true, width: 160 },
    ] : []),
    { title: '更新时间', dataIndex: 'updated_at', key: 'updated_at', render: fmtTime },
  ];

  const recallColumns = [
    { title: '内容', dataIndex: 'content', key: 'content', ellipsis: true, render: (v: string) => <Tooltip title={v}>{v}</Tooltip> },
    { title: '类型', dataIndex: 'type', key: 'type', width: 90, render: (v: string) => <Tag>{v}</Tag> },
    { title: '综合分', key: 'score', width: 120, render: (_: unknown, record: UserMemory) => {
      const item = explanationMap.get(record.id);
      return <Progress percent={fmtScore(item?.final_score)} size="small" />;
    }},
    ...(!isMobile ? [
      { title: '评分拆解', key: 'breakdown', width: 260, render: (_: unknown, record: UserMemory) => {
        const item = explanationMap.get(record.id);
        if (!item) return '-';
        return <Space size={4} wrap>
          <Tag color="blue">关键词 {fmtScore(item.keyword_score)}</Tag>
          <Tag color="purple">向量 {fmtScore(item.vector_score)}</Tag>
          <Tag color="cyan">新鲜 {fmtScore(item.recency_score)}</Tag>
          <Tag color="gold">频次 {fmtScore(item.frequency_score)}</Tag>
          <Tag>类型 {fmtScore(item.type_score)}</Tag>
        </Space>;
      }},
    ] : []),
    { title: '解释', key: 'explanation', ellipsis: true, render: (_: unknown, record: UserMemory) => explanationMap.get(record.id)?.explanation || '-' },
  ];

  const entityColumns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类型', dataIndex: 'type', key: 'type', width: 100, render: (v: string) => {
      const colorMap: Record<string, string> = { tenant: 'purple', user: 'blue', connector: 'cyan', tool: 'green', skill: 'orange' };
      return <Tag color={colorMap[v] || 'default'}>{v}</Tag>;
    }},
    ...(!isMobile ? [
      { title: '引用ID', dataIndex: 'ref_id', key: 'ref_id', ellipsis: true },
    ] : []),
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: fmtTime },
  ];

  const skillMemoryColumns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    ...(!isMobile ? [
      { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
      { title: '分类', dataIndex: 'category', key: 'category', width: 100, render: (v: string) => v ? <Tag>{v}</Tag> : '-' },
      { title: '使用次数', dataIndex: 'usage_count', key: 'usage_count', width: 80 },
    ] : []),
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: fmtTime },
  ];

  const auditColumns = [
    { title: '动作', dataIndex: 'action', key: 'action', width: 150, render: (v: string) => <Tag color={actionColor[v] || 'default'}>{v}</Tag> },
    { title: '来源', dataIndex: 'source', key: 'source', width: 100, render: (v: string) => <Tag>{v || '-'}</Tag> },
    ...(!isMobile ? [
      { title: '用户ID', dataIndex: 'user_id', key: 'user_id', ellipsis: true, width: 130 },
      { title: '原因', dataIndex: 'reason', key: 'reason', ellipsis: true, render: (v: string) => v || '-' },
      { title: '原始预览', dataIndex: 'original_preview', key: 'original_preview', ellipsis: true, render: (v: string) => v || '-' },
      { title: '处理后预览', dataIndex: 'sanitized_preview', key: 'sanitized_preview', ellipsis: true, render: (v: string) => v || '-' },
    ] : []),
    { title: '时间', dataIndex: 'created_at', key: 'created_at', render: fmtTime },
    { title: '详情', key: 'detail', width: 80, render: (_: unknown, record: MemoryAuditLog) => <Button size="small" onClick={() => setSelectedAudit(record)}>查看</Button> },
  ];

  const governanceView = (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Alert
        type="info"
        showIcon
        message="记忆治理工作台"
        description="这里统一查看记忆池、长期记忆、向量索引、召回解释和治理审计。前端只做配置和展示；敏感过滤、去重、混合检索、召回排序、容量归档仍由后端决策。"
      />
      <Row gutter={[16, 16]}>
        <Col xs={12} lg={6}><Card><Statistic title="用户记忆" value={stats?.total_user_memories || 0} prefix={<UserOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card><Statistic title="活跃记忆" value={activeMemoryCount} prefix={<SafetyCertificateOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card><Statistic title="向量索引率" value={vectorRate} suffix="%" prefix={<SyncOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card><Statistic title="记忆池" value={pools.length} prefix={<DatabaseOutlined />} /></Card></Col>
      </Row>
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card title="来源与状态" size="small">
            <Space wrap>
              <Tag color="blue">Top来源：{topMemorySource ? `${topMemorySource[0]} / ${topMemorySource[1]}` : '暂无'}</Tag>
              <Tag color="green">active：{activeMemoryCount}</Tag>
              <Tag color="orange">archived：{archivedCount}</Tag>
              <Tag color="purple">vector：{vectorIndexedCount}/{userMemories.length}</Tag>
            </Space>
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="治理链路" size="small">
            <Space wrap>
              <Tag color={settings?.auto_extract_enabled ? 'green' : 'red'}>自动提取：{settings?.auto_extract_enabled ? '开' : '关'}</Tag>
              <Tag color={settings?.sensitive_filter_enabled ? 'green' : 'red'}>敏感过滤：{settings?.sensitive_filter_enabled ? '开' : '关'}</Tag>
              <Tag color={settings?.recall_enabled ? 'green' : 'red'}>召回注入：{settings?.recall_enabled ? '开' : '关'}</Tag>
              <Tag color={settings?.hybrid_search_enabled ? 'green' : 'orange'}>检索：{settings?.hybrid_search_mode || '-'}</Tag>
            </Space>
          </Card>
        </Col>
      </Row>
      <Card title={<Space><SafetyCertificateOutlined />治理配置</Space>} loading={loading && !settings}>
        {settings ? (
          <Row gutter={[16, 16]}>
            <Col xs={24} md={12} lg={8}>
              <Space direction="vertical">
                <Text strong>自动提取</Text>
                <Switch checked={settings.auto_extract_enabled} loading={settingsSaving} onChange={(v) => updateSetting({ auto_extract_enabled: v })} />
                <Text type="secondary">控制对话是否自动沉淀为长期记忆。</Text>
              </Space>
            </Col>
            <Col xs={24} md={12} lg={8}>
              <Space direction="vertical">
                <Text strong>召回注入</Text>
                <Switch checked={settings.recall_enabled} loading={settingsSaving} onChange={(v) => updateSetting({ recall_enabled: v })} />
                <Text type="secondary">关闭后 AI 助手不会加载历史记忆。</Text>
              </Space>
            </Col>
            <Col xs={24} md={12} lg={8}>
              <Space direction="vertical">
                <Text strong>敏感过滤</Text>
                <Switch checked={settings.sensitive_filter_enabled} loading={settingsSaving} onChange={(v) => updateSetting({ sensitive_filter_enabled: v })} />
                <Text type="secondary">只作用于保存、展示、召回注入链路。</Text>
              </Space>
            </Col>
            <Col xs={24} md={12} lg={8}>
              <Space direction="vertical">
                <Text strong>审计日志</Text>
                <Switch checked={settings.audit_enabled} loading={settingsSaving} onChange={(v) => updateSetting({ audit_enabled: v })} />
                <Text type="secondary">记录脱敏、去重、归档和向量 fallback。</Text>
              </Space>
            </Col>
            <Col xs={24} md={12} lg={8}>
              <Space direction="vertical">
                <Text strong>混合检索</Text>
                <Switch checked={settings.hybrid_search_enabled} loading={settingsSaving} onChange={(v) => updateSetting({ hybrid_search_enabled: v })} />
                <Text type="secondary">融合关键词、向量、新鲜度、频次和类型评分。</Text>
              </Space>
            </Col>
            <Col xs={24} md={12} lg={8}>
              <Space direction="vertical" style={{ width: '100%' }}>
                <Text strong>检索模式</Text>
                <Select
                  value={settings.hybrid_search_mode}
                  style={{ width: '100%' }}
                  disabled={settingsSaving}
                  onChange={(v) => updateSetting({ hybrid_search_mode: v })}
                  options={[
                    { value: 'keyword', label: '仅关键词' },
                    { value: 'keyword+vector', label: '关键词 + 向量' },
                    { value: 'vector', label: '仅向量' },
                  ]}
                />
                <Text type="secondary">推荐生产使用“关键词 + 向量”。</Text>
              </Space>
            </Col>
          </Row>
        ) : <Empty description="暂无治理配置" />}
      </Card>
      <Card title={<Space><HddOutlined />容量与归档提示</Space>}>
        <Table dataSource={pools} columns={[
          { title: '记忆池', dataIndex: 'name', key: 'name' },
          { title: '用途', dataIndex: 'purpose', key: 'purpose', render: (v: string) => <Tag>{v}</Tag> },
          { title: '优先级', dataIndex: 'priority', key: 'priority', width: 90 },
          { title: '记忆数', dataIndex: 'memory_count', key: 'memory_count', width: 90 },
          { title: 'Token上限', dataIndex: 'max_tokens', key: 'max_tokens', width: 110, render: (v: number) => v === 0 ? '不限' : v },
          { title: '归档策略', key: 'archive_hint', render: (_: unknown, record: MemoryPool) => record.max_tokens === 0 ? '不限容量，不主动归档' : '超限时保留高价值记忆，低分记忆归档' },
        ]} rowKey="id" size={isMobile ? 'small' : 'middle'} pagination={false} scroll={isMobile ? { x: 620 } : undefined} />
      </Card>
    </Space>
  );

  const recallView = (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card title={<Space><SearchOutlined />召回调试与解释</Space>}>
        <Space direction={isMobile ? 'vertical' : 'horizontal'} style={{ width: '100%' }}>
          <Input value={recallUserId} onChange={(e) => setRecallUserId(e.target.value)} placeholder="用户ID，默认当前用户" style={{ width: isMobile ? '100%' : 260 }} />
          <Input.Search
            value={recallQuery}
            onChange={(e) => setRecallQuery(e.target.value)}
            onSearch={runRecallDebug}
            placeholder="输入问题，查看后端召回排序与解释"
            enterButton="调试召回"
            loading={searchLoading}
            style={{ width: isMobile ? '100%' : 520 }}
          />
        </Space>
        <Divider />
        <Table dataSource={recallResults} columns={recallColumns} rowKey="id" loading={searchLoading}
          size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 760 } : undefined} />
      </Card>
    </Space>
  );

  const tabItems = [
    {
      key: 'governance',
      label: <span><SafetyCertificateOutlined /> 治理</span>,
      children: governanceView,
    },
    {
      key: 'recall',
      label: <span><SearchOutlined /> 召回解释</span>,
      children: recallView,
    },
    {
      key: 'pools',
      label: <span><DatabaseOutlined /> 记忆池</span>,
      children: (
        <>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => openPoolModal()}>
              新建记忆池
            </Button>
          </div>
          <Table dataSource={pools} columns={poolColumns} rowKey="id" loading={loading}
            size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 640 } : undefined} />
        </>
      ),
    },
    {
      key: 'entries',
      label: <span><DatabaseOutlined /> 池条目 {poolEntries.length > 0 && <Tag>{poolEntries.length}</Tag>}</span>,
      children: (
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="这里展示 AI 助手记忆池工具写入的记忆条目"
            description="例如通过 AI 助手记住的“女儿喜欢玩水”会存入 memory_entries，并按记忆池归类展示。"
          />
          <Select
            value={selectedPoolId || undefined}
            onChange={setSelectedPoolId}
            placeholder="选择记忆池"
            style={{ width: isMobile ? '100%' : 360 }}
            options={pools.map((pool) => ({ label: `${pool.name}（${pool.memory_count || 0}）`, value: pool.id }))}
          />
          <Table dataSource={poolEntries} columns={poolEntryColumns} rowKey="id" loading={entriesLoading}
            size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 720 } : undefined} />
        </Space>
      ),
    },
    {
      key: 'user',
      label: <span><UserOutlined /> 用户记忆 {stats && <Tag>{stats.total_user_memories}</Tag>}</span>,
      children: (
        <Table dataSource={userMemories} columns={userMemoryColumns} rowKey="id" loading={loading}
          size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 820 } : undefined} />
      ),
    },
    {
      key: 'audit',
      label: <span><AuditOutlined /> 治理审计 {auditLogs.length > 0 && <Tag>{auditLogs.length}</Tag>}</span>,
      children: (
        <Table dataSource={auditLogs} columns={auditColumns} rowKey="id" loading={loading}
          size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 820 } : undefined} />
      ),
    },
    {
      key: 'entity',
      label: <span><ApartmentOutlined /> 实体 {stats && <Tag>{stats.total_entities}</Tag>}</span>,
      children: (
        <Table dataSource={entities} columns={entityColumns} rowKey="id" loading={loading}
          size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 350 } : undefined} />
      ),
    },
    {
      key: 'skill',
      label: <span><BulbOutlined /> 技能记忆 {stats && <Tag>{stats.total_skill_memories}</Tag>}</span>,
      children: (
        <Table dataSource={skillMemories} columns={skillMemoryColumns} rowKey="id" loading={loading}
          size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 350 } : undefined} />
      ),
    },
    {
      key: 'stats',
      label: <span><BarChartOutlined /> 统计</span>,
      children: stats ? (
        <Descriptions bordered column={isMobile ? 1 : 2} size={isMobile ? 'small' : 'middle'}>
          <Descriptions.Item label="用户记忆">{stats.total_user_memories}</Descriptions.Item>
          <Descriptions.Item label="会话记忆">{stats.total_session_memories}</Descriptions.Item>
          <Descriptions.Item label="实体">{stats.total_entities}</Descriptions.Item>
          <Descriptions.Item label="技能记忆">{stats.total_skill_memories}</Descriptions.Item>
          <Descriptions.Item label="按类型">{Object.entries(stats.by_type || {}).map(([k, v]) => <Tag key={k}>{k}: {v}</Tag>)}</Descriptions.Item>
        </Descriptions>
      ) : <Text type="secondary">暂无统计数据</Text>,
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><DatabaseOutlined /> 记忆治理</Title>
        <Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
          统一管理记忆池、长期记忆、来源状态、向量索引、召回解释和治理审计；核心提取、过滤、去重、召回策略由后端执行。
        </Paragraph>
      </div>

      <Tabs activeKey={activeTab} onChange={setActiveTab} items={tabItems} />

      <Drawer
        title="长期记忆详情"
        open={!!selectedMemory}
        onClose={() => setSelectedMemory(null)}
        width={isMobile ? '100%' : 620}
      >
        {selectedMemory && (
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <Alert
              type={(selectedMemory.status || 'active') === 'active' ? 'success' : 'warning'}
              showIcon
              message={`状态：${selectedMemory.status || 'active'} / 来源：${selectedMemory.source || 'unknown'}`}
              description="长期记忆详情用于排查为什么会被召回、是否已向量化、是否可能需要归档或治理。"
            />
            <Descriptions bordered column={1} size="small" title="内容与归因">
              <Descriptions.Item label="记忆ID">{selectedMemory.id}</Descriptions.Item>
              <Descriptions.Item label="用户ID">{selectedMemory.user_id}</Descriptions.Item>
              <Descriptions.Item label="记忆池ID">{selectedMemory.pool_id || '-'}</Descriptions.Item>
              <Descriptions.Item label="类型"><Tag>{selectedMemory.type}</Tag></Descriptions.Item>
              <Descriptions.Item label="来源"><Tag>{selectedMemory.source || 'unknown'}</Tag></Descriptions.Item>
              <Descriptions.Item label="内容"><Paragraph copyable style={{ margin: 0 }}>{selectedMemory.content}</Paragraph></Descriptions.Item>
              <Descriptions.Item label="Content Hash">{selectedMemory.content_hash || '-'}</Descriptions.Item>
            </Descriptions>
            <Descriptions bordered column={1} size="small" title="召回与向量状态">
              <Descriptions.Item label="状态"><Tag color={statusColor[selectedMemory.status || 'active'] || 'default'}>{selectedMemory.status || 'active'}</Tag></Descriptions.Item>
              <Descriptions.Item label="访问次数">{selectedMemory.access_count || 0}</Descriptions.Item>
              <Descriptions.Item label="最近访问">{fmtTime(selectedMemory.last_accessed_at)}</Descriptions.Item>
              <Descriptions.Item label="最近出现">{fmtTime(selectedMemory.last_seen_at)}</Descriptions.Item>
              <Descriptions.Item label="向量索引">{selectedMemory.vector_indexed_at ? <Tag color="green">已索引</Tag> : <Tag color="orange">未索引/关键词兜底</Tag>}</Descriptions.Item>
              <Descriptions.Item label="索引时间">{fmtTime(selectedMemory.vector_indexed_at)}</Descriptions.Item>
              <Descriptions.Item label="更新时间">{fmtTime(selectedMemory.updated_at)}</Descriptions.Item>
            </Descriptions>
          </Space>
        )}
      </Drawer>

      <Drawer
        title="记忆治理审计详情"
        open={!!selectedAudit}
        onClose={() => setSelectedAudit(null)}
        width={isMobile ? '100%' : 620}
      >
        {selectedAudit && (
          <Descriptions bordered column={1} size="small">
            <Descriptions.Item label="日志ID">{selectedAudit.id}</Descriptions.Item>
            <Descriptions.Item label="动作"><Tag color={actionColor[selectedAudit.action] || 'default'}>{selectedAudit.action}</Tag></Descriptions.Item>
            <Descriptions.Item label="来源"><Tag>{selectedAudit.source || '-'}</Tag></Descriptions.Item>
            <Descriptions.Item label="用户ID">{selectedAudit.user_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="记忆ID">{selectedAudit.memory_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="原因">{selectedAudit.reason || '-'}</Descriptions.Item>
            <Descriptions.Item label="原始预览"><Paragraph copyable style={{ margin: 0 }}>{selectedAudit.original_preview || '-'}</Paragraph></Descriptions.Item>
            <Descriptions.Item label="处理后预览"><Paragraph copyable style={{ margin: 0 }}>{selectedAudit.sanitized_preview || '-'}</Paragraph></Descriptions.Item>
            <Descriptions.Item label="时间">{fmtTime(selectedAudit.created_at)}</Descriptions.Item>
          </Descriptions>
        )}
      </Drawer>

      <Modal
        title={editingPool ? '编辑记忆池' : '新建记忆池'}
        open={modalOpen}
        onOk={onPoolOk}
        onCancel={() => setModalOpen(false)}
        width={isMobile ? '95%' : 600}
        destroyOnClose
      >
        <Form form={form} layout="vertical" initialValues={{
          type: 'personal',
          purpose: 'conversation',
          priority: 5,
          max_tokens: 0,
          auto_activate: true,
          enabled: true,
        }}>
          <Form.Item name="name" label="记忆池名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="如: 工作记忆 / 个人偏好 / 项目知识" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="记忆池的用途说明" />
          </Form.Item>

          <div style={{ display: 'flex', gap: 16 }}>
            <Form.Item name="type" label="共享类型" style={{ flex: 1 }}>
              <Select options={[
                { value: 'personal', label: '个人' },
                { value: 'team', label: '团队共享' },
                { value: 'system', label: '系统级' },
              ]} />
            </Form.Item>
            <Form.Item name="purpose" label="用途" style={{ flex: 1 }}>
              <Select options={[
                { value: 'conversation', label: '对话记忆' },
                { value: 'skill', label: '技能知识' },
                { value: 'knowledge', label: '知识库' },
              ]} />
            </Form.Item>
          </div>

          <div style={{ display: 'flex', gap: 16 }}>
            <Form.Item name="priority" label="优先级 (1-10)" style={{ flex: 1 }}>
              <InputNumber min={1} max={10} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="max_tokens" label="最大Token数" tooltip="0表示不限制" style={{ flex: 1 }}>
              <InputNumber min={0} step={100} style={{ width: '100%' }} />
            </Form.Item>
          </div>

          <div style={{ display: 'flex', gap: 16 }}>
            <Form.Item name="auto_activate" label="自动激活" valuePropName="checked" style={{ flex: 1 }}>
              <Switch />
            </Form.Item>
            <Form.Item name="enabled" label="启用状态" valuePropName="checked" style={{ flex: 1 }}>
              <Switch />
            </Form.Item>
          </div>

          <Form.Item name="trigger_rules" label="触发规则 (JSON)" tooltip='条件激活，如: {"keywords":["工作","会议"]}'>
            <Input.TextArea rows={3} placeholder='{"keywords":["工作"],"context":"对话中提到工作相关内容时激活"}' style={{ fontFamily: 'monospace', fontSize: 13 }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Memory;
