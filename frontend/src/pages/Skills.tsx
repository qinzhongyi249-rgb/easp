import React, { useMemo, useState, useEffect, useCallback } from 'react';
import { Table, Button, Modal, Form, Input, InputNumber, Space, Typography, App, Tag, Dropdown, Select, Tabs, Descriptions, Timeline, Card, Collapse, Popconfirm, Empty, Alert, Row, Col, Statistic } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, BulbOutlined, PlayCircleOutlined, MoreOutlined, HistoryOutlined, CheckCircleOutlined, CloseCircleOutlined, LoadingOutlined, ClockCircleOutlined, EyeOutlined, LockOutlined, SafetyOutlined, PartitionOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { Skill, SkillExecution, StepResult, JsonSchema } from '../api/skill';
import { skillApi, SKILL_CATEGORIES } from '../api/skill';
import StepEditor from '../components/StepEditor';

const { Title, Text, Paragraph } = Typography;
const { TextArea } = Input;
interface LayoutContext { currentTenant: string; }

const STATUS_META: Record<string, { label: string; color: string }> = {
  draft: { label: '草稿', color: 'blue' },
  testing: { label: '测试中', color: 'gold' },
  published: { label: '已发布', color: 'green' },
  disabled: { label: '已停用', color: 'default' },
  active: { label: '已发布(旧)', color: 'green' },
  archived: { label: '已停用(旧)', color: 'default' },
};

const EXECUTION_MODE_META: Record<string, { label: string; color: string }> = {
  dry_run: { label: '预演', color: 'purple' },
  sandbox: { label: '沙箱', color: 'blue' },
  production: { label: '正式', color: 'red' },
};

const normalizeStatus = (status?: string) => {
  if (status === 'active') return 'published';
  if (status === 'archived') return 'disabled';
  return status || 'draft';
};

const renderStatusTag = (status?: string) => {
  const meta = STATUS_META[status || ''] || { label: status || '草稿', color: 'default' };
  return <Tag color={meta.color}>{meta.label}</Tag>;
};

const renderExecutionModeTag = (mode?: string) => {
  const normalized = mode || 'sandbox';
  const meta = EXECUTION_MODE_META[normalized] || { label: normalized, color: 'default' };
  return <Tag color={meta.color}>{meta.label}</Tag>;
};

interface SkillFilters {
  keyword: string;
  status?: string;
  category?: string;
  hasInputSchema?: boolean;
  hasTriggers?: boolean;
  usageState?: 'used' | 'unused';
}

const normalizeText = (value?: string | null) => (value || '').toLowerCase();
const fmtTime = (value?: string | null) => value ? new Date(value).toLocaleString() : '-';
const isBuiltinSkill = (skill?: Skill | null) => (skill?.created_by || '').toLowerCase() === 'system';

const parseJsonArray = <T,>(value?: string | null, fallback: T[] = []): T[] => {
  if (!value) return fallback;
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed : fallback;
  } catch { return fallback; }
};

const parseSteps = (value?: string | null): Array<{ name?: string; type?: string; action?: string }> => parseJsonArray(value, []);
const prettyJson = (value?: string | null) => {
  if (!value) return '无';
  try { return JSON.stringify(JSON.parse(value), null, 2); }
  catch { return value; }
};

const skillRisk = (skill: Skill) => {
  const status = normalizeStatus(skill.status);
  if (status === 'disabled') return { label: '已停用', color: 'default', reason: '技能已停用，不应进入生产授权。' };
  if (status !== 'published') return { label: '未发布', color: 'orange', reason: '未发布技能只能沙箱/预演测试，正式执行需要发布。' };
  if (!skill.input_schema) return { label: '缺输入Schema', color: 'gold', reason: '缺少输入参数 Schema，助手追问和执行前校验不够稳定。' };
  if (parseSteps(skill.steps).length === 0) return { label: '无步骤', color: 'red', reason: '没有可执行步骤。' };
  return { label: '生产可用', color: 'green', reason: '已发布且具备基础执行定义。' };
};

// 解析 JSON Schema 生成表单字段
const parseInputSchema = (schemaStr?: string): { fields: Array<{ name: string; type: string; title: string; description?: string; required: boolean; defaultValue?: unknown }>; required: string[] } => {
  if (!schemaStr) return { fields: [], required: [] };
  try {
    const schema: JsonSchema = JSON.parse(schemaStr);
    const required = schema.required || [];
    const properties = schema.properties || {};
    const orderedNames = [
      ...required.filter(name => Object.prototype.hasOwnProperty.call(properties, name)),
      ...Object.keys(properties).filter(name => !required.includes(name)),
    ];
    const fields = orderedNames.map((name) => {
      const prop = properties[name];
      return {
        name,
        type: prop.type,
        title: prop.title || name,
        description: prop.description,
        required: required.includes(name),
        defaultValue: prop.default,
      };
    });
    return { fields, required };
  } catch { return { fields: [], required: [] }; }
};

// 步骤结果状态图标
const isSuccessStatus = (status?: string) => status === 'success' || status === 'completed';
const isFailedStatus = (status?: string) => status === 'failed' || status === 'error';

const parseStepResults = (value: SkillExecution['step_results']): StepResult[] => {
  if (!value) return [];
  if (Array.isArray(value)) return value;
  if (typeof value === 'string') {
    try {
      const parsed = JSON.parse(value);
      return Array.isArray(parsed) ? parsed : [];
    } catch { return []; }
  }
  return [];
};

const parseOutputs = (value: SkillExecution['outputs']): Record<string, unknown> => {
  if (!value) return {};
  if (typeof value === 'string') {
    try {
      const parsed = JSON.parse(value);
      return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {};
    } catch { return {}; }
  }
  return typeof value === 'object' && !Array.isArray(value) ? value : {};
};

const calcDurationMs = (result: SkillExecution): number | undefined => {
  if (typeof result.duration_ms === 'number') return result.duration_ms;
  if (result.started_at && result.ended_at) {
    const ms = new Date(result.ended_at).getTime() - new Date(result.started_at).getTime();
    return Number.isFinite(ms) && ms >= 0 ? ms : undefined;
  }
  return undefined;
};

const StepStatusIcon = ({ status }: { status: string }) => {
  switch (status) {
    case 'success':
    case 'completed': return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
    case 'failed': return <CloseCircleOutlined style={{ color: '#ff4d4f' }} />;
    case 'running': return <LoadingOutlined style={{ color: '#1890ff' }} />;
    case 'skipped': return <ClockCircleOutlined style={{ color: '#999' }} />;
    default: return <ClockCircleOutlined style={{ color: '#d9d9d9' }} />;
  }
};

const Skills: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [skills, setSkills] = useState<Skill[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Skill | null>(null);
  const [filters, setFilters] = useState<SkillFilters>({ keyword: '' });
  const [form] = Form.useForm();
  const isMobile = window.innerWidth < 768;

  // 执行相关状态
  const [execModalOpen, setExecModalOpen] = useState(false);
  const [executingSkill, setExecutingSkill] = useState<Skill | null>(null);
  const [execForm] = Form.useForm();
  const [executing, setExecuting] = useState(false);
  const [executionResult, setExecutionResult] = useState<SkillExecution | null>(null);

  // 执行历史
  const [historyModalOpen, setHistoryModalOpen] = useState(false);
  const [historySkill, setHistorySkill] = useState<Skill | null>(null);
  const [executions, setExecutions] = useState<SkillExecution[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [detailSkill, setDetailSkill] = useState<Skill | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try { const res = await skillApi.list(currentTenant); setSkills(res.data || []); }
    catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, [currentTenant]);

  // ========== 技能 CRUD ==========
  const openEditModal = useCallback((record?: Skill) => {
    if (record && isBuiltinSkill(record)) {
      message.warning('内置锁定 Skill 不可编辑');
      return;
    }
    if (record) {
      setEditing(record);
      // 解析 steps JSON 为可编辑字符串
      let stepsStr = record.steps || '[]';
      try { stepsStr = JSON.stringify(JSON.parse(stepsStr), null, 2); } catch { /* keep raw */ }
      let inputSchemaStr = record.input_schema || '';
      try { if (inputSchemaStr) inputSchemaStr = JSON.stringify(JSON.parse(inputSchemaStr), null, 2); } catch { /* keep raw */ }
      let outputSchemaStr = record.output_schema || '';
      try { if (outputSchemaStr) outputSchemaStr = JSON.stringify(JSON.parse(outputSchemaStr), null, 2); } catch { /* keep raw */ }
      let triggersStr = record.triggers || '';
      try { if (triggersStr) triggersStr = JSON.stringify(JSON.parse(triggersStr), null, 2); } catch { /* keep raw */ }
      let tagsStr = '';
      if (record.tags) {
        try { tagsStr = JSON.parse(record.tags).join(', '); } catch { tagsStr = record.tags; }
      }

      form.setFieldsValue({
        ...record,
        steps_text: stepsStr,
        input_schema_text: inputSchemaStr,
        output_schema_text: outputSchemaStr,
        triggers_text: triggersStr,
        tags_text: tagsStr,
      });
    } else {
      setEditing(null);
      form.resetFields();
      form.setFieldsValue({
        version: '1.0.0',
        status: 'draft',
        steps_text: '[\n  {\n    "name": "step1",\n    "type": "mcp_tool",\n    "action": "tool_name",\n    "params": {}\n  }\n]',
      });
    }
    setModalOpen(true);
  }, [form, message]);

  const onOk = async () => {
    const values = await form.validateFields();
    // 验证 JSON 字段
    let steps: string;
    try { steps = JSON.stringify(JSON.parse(values.steps_text)); } catch { message.error('执行步骤必须是合法JSON'); return; }

    let inputSchema = values.input_schema_text || null;
    if (inputSchema && inputSchema.trim()) {
      try { inputSchema = JSON.stringify(JSON.parse(inputSchema)); } catch { message.error('输入参数Schema必须是合法JSON'); return; }
    }

    let outputSchema = values.output_schema_text || null;
    if (outputSchema && outputSchema.trim()) {
      try { outputSchema = JSON.stringify(JSON.parse(outputSchema)); } catch { message.error('输出Schema必须是合法JSON'); return; }
    }

    let triggers = values.triggers_text || null;
    if (triggers && triggers.trim()) {
      try { triggers = JSON.stringify(JSON.parse(triggers)); } catch { message.error('触发条件必须是合法JSON'); return; }
    }

    let tags: string | undefined = undefined;
    if (values.tags_text && values.tags_text.trim()) {
      tags = JSON.stringify(values.tags_text.split(',').map((t: string) => t.trim()).filter(Boolean));
    }

    const payload: Partial<Skill> = {
      name: values.name,
      description: values.description,
      category: values.category,
      version: values.version,
      tags,
      triggers,
      input_schema: inputSchema,
      output_schema: outputSchema,
      steps,
      status: values.status,
    };

    try {
      if (editing) { await skillApi.update(currentTenant, editing.id, payload); message.success('更新成功'); }
      else { await skillApi.create(currentTenant, payload); message.success('创建成功'); }
      setModalOpen(false); form.resetFields(); setEditing(null); load();
    } catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '操作失败'); }
  };

  const onDelete = async (record: Skill) => {
    if (isBuiltinSkill(record)) {
      message.warning('内置锁定 Skill 不可删除');
      return;
    }
    try { await skillApi.delete(currentTenant, record.id); message.success('删除成功'); load(); }
    catch { message.error('删除失败'); }
  };

  // ========== 技能执行 ==========
  const openExecModal = (skill: Skill) => {
    setExecutingSkill(skill);
    setExecutionResult(null);
    execForm.resetFields();
    // 根据 input_schema 设置默认值
    const { fields } = parseInputSchema(skill.input_schema);
    const defaults: Record<string, unknown> = { execution_mode: 'sandbox' };
    fields.forEach(f => { if (f.defaultValue !== undefined) defaults[f.name] = f.defaultValue; });
    execForm.setFieldsValue(defaults);
    setExecModalOpen(true);
  };

  const onExecute = async () => {
    if (!executingSkill) return;
    const values = await execForm.validateFields();
    const { execution_mode = 'sandbox', ...inputs } = values;
    setExecuting(true);
    setExecutionResult(null);
    try {
      const res = await skillApi.execute(currentTenant, executingSkill.id, inputs, execution_mode);
      setExecutionResult(res.data);
      if (isFailedStatus(res.data?.status)) {
        message.error(res.data?.error || '执行失败');
      } else {
        message.success('执行完成');
      }
      load(); // 刷新使用次数
    } catch (err: unknown) {
      const e = err as { response?: { data?: SkillExecution & { error?: string } } };
      if (e.response?.data?.id) {
        setExecutionResult(e.response.data);
      }
      message.error(e.response?.data?.error || '执行失败');
    } finally { setExecuting(false); }
  };

  // ========== 执行历史 ==========
  const openHistory = async (skill: Skill) => {
    setHistorySkill(skill);
    setExecutions([]);
    setHistoryModalOpen(true);
    setHistoryLoading(true);
    try {
      const res = await skillApi.listExecutions(currentTenant, skill.id);
      const list = res.data?.executions;
      setExecutions(Array.isArray(list) ? list : []);
    } catch {
      setExecutions([]);
      message.error('加载历史失败');
    }
    finally { setHistoryLoading(false); }
  };

  // ========== 表格列定义 ==========
  const filteredSkills = useMemo(() => {
    const keyword = normalizeText(filters.keyword).trim();
    return skills.filter((skill) => {
      if (keyword) {
        const haystack = [skill.name, skill.description, skill.version, skill.tags]
          .map(normalizeText)
          .join(' ');
        if (!haystack.includes(keyword)) return false;
      }
      if (filters.status && skill.status !== filters.status) return false;
      if (filters.category && skill.category !== filters.category) return false;
      if (filters.hasInputSchema !== undefined) {
        const hasSchema = Boolean(skill.input_schema && skill.input_schema.trim());
        if (hasSchema !== filters.hasInputSchema) return false;
      }
      if (filters.hasTriggers !== undefined) {
        const hasTriggers = Boolean(skill.triggers && skill.triggers.trim());
        if (hasTriggers !== filters.hasTriggers) return false;
      }
      if (filters.usageState === 'used' && (skill.usage_count || 0) <= 0) return false;
      if (filters.usageState === 'unused' && (skill.usage_count || 0) > 0) return false;
      return true;
    });
  }, [filters, skills]);

  const hasActiveFilters = Boolean(
    filters.keyword || filters.status || filters.category || filters.hasInputSchema !== undefined ||
    filters.hasTriggers !== undefined || filters.usageState
  );

  const skillStats = useMemo(() => {
    const published = skills.filter(s => normalizeStatus(s.status) === 'published').length;
    const testing = skills.filter(s => normalizeStatus(s.status) === 'testing').length;
    const disabled = skills.filter(s => normalizeStatus(s.status) === 'disabled').length;
    const builtin = skills.filter(isBuiltinSkill).length;
    const withSchema = skills.filter(s => Boolean(s.input_schema)).length;
    const withTriggers = skills.filter(s => Boolean(s.triggers)).length;
    const productionReady = skills.filter(s => skillRisk(s).label === '生产可用').length;
    const mcpSteps = skills.filter(s => parseSteps(s.steps).some(step => step.type === 'mcp_tool')).length;
    return { published, testing, disabled, builtin, withSchema, withTriggers, productionReady, mcpSteps };
  }, [skills]);

  const openDetail = (skill: Skill) => {
    setDetailSkill(skill);
    setDetailOpen(true);
  };

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name', render: (v: string, r: Skill) => (
      <Space direction="vertical" size={2}>
        <Space wrap>
          <Text strong>{v}</Text>
          {r.category && <Tag>{SKILL_CATEGORIES.find(c => c.value === r.category)?.label || r.category}</Tag>}
          {isBuiltinSkill(r) && <Tag color="purple" icon={<LockOutlined />}>内置锁定</Tag>}
          <Tag color={skillRisk(r).color}>{skillRisk(r).label}</Tag>
        </Space>
        <Text type="secondary" style={{ fontSize: 12 }}>{r.description || '暂无描述'}</Text>
      </Space>
    )},
    ...(!isMobile ? [
      { title: '编排能力', key: 'steps', width: 150, render: (_: unknown, record: Skill) => {
        const steps = parseSteps(record.steps);
        const mcpCount = steps.filter(step => step.type === 'mcp_tool').length;
        return <Space direction="vertical" size={2}><Text>{steps.length} 步</Text><Text type="secondary">MCP {mcpCount} / 输入 {parseInputSchema(record.input_schema).fields.length}</Text></Space>;
      } },
      { title: '使用情况', key: 'usage', width: 120, render: (_: unknown, record: Skill) => <Space direction="vertical" size={2}><Text>{record.usage_count || 0} 次</Text><Text type="secondary">{fmtTime(record.last_used_at)}</Text></Space> },
    ] : []),
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => {
      return renderStatusTag(v);
    }},
    { title: '操作', key: 'action', width: isMobile ? 60 : 340, render: (_: unknown, record: Skill) => (
      isMobile ? (
        <Dropdown menu={{ items: [
          { key: 'detail', label: '详情', icon: <EyeOutlined />, onClick: () => openDetail(record) },
          { key: 'execute', label: '执行', icon: <PlayCircleOutlined />, onClick: () => openExecModal(record) },
          { key: 'history', label: '历史', icon: <HistoryOutlined />, onClick: () => openHistory(record) },
          { key: 'edit', label: isBuiltinSkill(record) ? '内置不可编辑' : '编辑', icon: <EditOutlined />, disabled: isBuiltinSkill(record), onClick: () => openEditModal(record) },
          { key: 'delete', label: isBuiltinSkill(record) ? '内置不可删除' : '删除', icon: <DeleteOutlined />, danger: true, disabled: isBuiltinSkill(record), onClick: () => onDelete(record) },
        ]}} trigger={['click']}>
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ) : (
        <Space>
          <Button size="small" icon={<EyeOutlined />} onClick={() => openDetail(record)}>详情</Button>
          <Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={() => openExecModal(record)}>执行</Button>
          <Button size="small" icon={<HistoryOutlined />} onClick={() => openHistory(record)}>历史</Button>
          <Button size="small" icon={<EditOutlined />} disabled={isBuiltinSkill(record)} onClick={() => openEditModal(record)}>编辑</Button>
          <Popconfirm title="确认删除?" onConfirm={() => onDelete(record)} disabled={isBuiltinSkill(record)}>
            <Button size="small" danger icon={<DeleteOutlined />} disabled={isBuiltinSkill(record)}>删除</Button>
          </Popconfirm>
        </Space>
      )
    )},
  ];

  // ========== 渲染输入表单字段 ==========
  const renderInputField = (field: { name: string; type: string; title: string; description?: string; required: boolean }) => {
    const rules = field.required ? [{ required: true, message: `请输入${field.title}` }] : [];

    switch (field.type) {
      case 'integer':
      case 'number':
        return (
          <Form.Item key={field.name} name={field.name} label={field.title} rules={rules} tooltip={field.description}>
            <InputNumber style={{ width: '100%' }} placeholder={field.description || `输入${field.title}`} />
          </Form.Item>
        );
      case 'boolean':
        return (
          <Form.Item key={field.name} name={field.name} label={field.title} rules={rules} tooltip={field.description}>
            <Select options={[{ value: 'true', label: '是' }, { value: 'false', label: '否' }]} />
          </Form.Item>
        );
      case 'array':
      case 'object':
        return (
          <Form.Item key={field.name} name={field.name} label={field.title} rules={rules} tooltip={field.description}>
            <TextArea rows={2} placeholder={`${field.description || field.title} (JSON格式)`} style={{ fontFamily: 'monospace' }} />
          </Form.Item>
        );
      default: // string
        return (
          <Form.Item key={field.name} name={field.name} label={field.title} rules={rules} tooltip={field.description}>
            <Input placeholder={field.description || `输入${field.title}`} />
          </Form.Item>
        );
    }
  };

  // ========== 渲染执行结果 ==========
  const renderExecutionResult = (result: SkillExecution) => {
    const stepResults = parseStepResults(result.step_results);
    const outputs = parseOutputs(result.outputs);
    const durationMs = calcDurationMs(result);

    return (
      <Card size="small" style={{ marginTop: 16 }}>
        <Descriptions column={2} size="small" bordered>
          <Descriptions.Item label="执行模式">
            {renderExecutionModeTag(result.execution_mode)}
          </Descriptions.Item>
          <Descriptions.Item label="状态">
            <Tag color={isSuccessStatus(result.status) ? 'green' : isFailedStatus(result.status) ? 'red' : 'blue'}>
              {result.status}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="耗时">{durationMs !== undefined ? `${durationMs}ms` : '-'}</Descriptions.Item>
        </Descriptions>

        {result.error && (
          <div style={{ marginTop: 12, padding: 8, background: '#fff2f0', borderRadius: 4, border: '1px solid #ffccc7' }}>
            <Text type="danger">{result.error}</Text>
          </div>
        )}

        {stepResults.length > 0 && (
          <>
            <Text strong style={{ display: 'block', marginTop: 16, marginBottom: 8 }}>执行步骤</Text>
            <Timeline items={stepResults.map((sr, idx) => ({
              dot: <StepStatusIcon status={sr.status} />,
              children: (
                <div key={idx}>
                  <Text strong>{sr.step_name}</Text>
                  <Tag style={{ marginLeft: 8 }} color={isSuccessStatus(sr.status) ? 'green' : isFailedStatus(sr.status) ? 'red' : 'default'}>
                    {sr.status}
                  </Tag>
                  {typeof sr.duration_ms === 'number' && <Text type="secondary" style={{ marginLeft: 8 }}>{sr.duration_ms}ms</Text>}
                  {sr.error && <div><Text type="danger">{sr.error}</Text></div>}
                  {sr.outputs && (
                    <Collapse size="small" style={{ marginTop: 4 }} items={[{
                      key: '1',
                      label: '输出详情',
                      children: <pre style={{ fontSize: 12, margin: 0, maxHeight: 200, overflow: 'auto' }}>{JSON.stringify(sr.outputs, null, 2)}</pre>
                    }]} />
                  )}
                </div>
              )
            }))} />
          </>
        )}

        {Object.keys(outputs).length > 0 && (
          <>
            <Text strong style={{ display: 'block', marginTop: 16, marginBottom: 8 }}>最终输出</Text>
            <pre style={{ fontSize: 12, background: '#f5f5f5', padding: 12, borderRadius: 4, maxHeight: 300, overflow: 'auto' }}>
              {JSON.stringify(outputs, null, 2)}
            </pre>
          </>
        )}
      </Card>
    );
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: isMobile ? 'flex-start' : 'center', marginBottom: 16, flexDirection: isMobile ? 'column' : 'row', gap: isMobile ? 12 : 0 }}>
        <div>
          <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><BulbOutlined /> Skill 治理工作台</Title>
          <Text type="secondary">统一治理 Skill 来源、生命周期、输入输出 Schema、执行编排、测试历史和生产授权风险。</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openEditModal()}>新建技能</Button>
      </div>

      <Space direction="vertical" size="middle" style={{ width: '100%', marginBottom: 16 }}>
        <Alert
          type="warning"
          showIcon
          message="内置/锁定资源保护"
          description="created_by=system 的内置 Skill 不可编辑、停用或删除；MCP Tool 和 Connector 标记 is_builtin/locked 后也不可删除、停用或被普通编辑破坏。前端只做 UX 提示，后端 API 和助手治理工具均会强制拦截。"
        />
        <Row gutter={[16, 16]}>
          <Col xs={12} md={6}><Card><Statistic title="Skill 总数" value={skills.length} prefix={<BulbOutlined />} /></Card></Col>
          <Col xs={12} md={6}><Card><Statistic title="生产可用" value={skillStats.productionReady} valueStyle={{ color: '#52c41a' }} prefix={<SafetyOutlined />} /></Card></Col>
          <Col xs={12} md={6}><Card><Statistic title="内置锁定" value={skillStats.builtin} valueStyle={{ color: '#722ed1' }} prefix={<LockOutlined />} /></Card></Col>
          <Col xs={12} md={6}><Card><Statistic title="测试/草稿" value={skills.length - skillStats.published - skillStats.disabled} prefix={<ClockCircleOutlined />} /></Card></Col>
        </Row>
        <Row gutter={[16, 16]}>
          <Col xs={24} md={8}>
            <Card size="small" title="生命周期">
              <Space wrap>
                <Tag color="green">已发布 {skillStats.published}</Tag>
                <Tag color="gold">测试中 {skillStats.testing}</Tag>
                <Tag>已停用 {skillStats.disabled}</Tag>
              </Space>
              <Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>只有已发布 Skill 才建议进入角色授权和正式执行。</Paragraph>
            </Card>
          </Col>
          <Col xs={24} md={8}>
            <Card size="small" title="Schema 与触发">
              <Space wrap>
                <Tag color="blue">有输入 Schema {skillStats.withSchema}</Tag>
                <Tag color="cyan">有触发条件 {skillStats.withTriggers}</Tag>
              </Space>
              <Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>输入 Schema 决定执行前参数校验和助手追问质量。</Paragraph>
            </Card>
          </Col>
          <Col xs={24} md={8}>
            <Card size="small" title="执行编排">
              <Space wrap>
                <Tag color="purple" icon={<PartitionOutlined />}>包含 MCP 步骤 {skillStats.mcpSteps}</Tag>
              </Space>
              <Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>Skill 执行仍受角色授权、MCP 工具授权和执行模式共同约束。</Paragraph>
            </Card>
          </Col>
        </Row>
      </Space>

      <div style={{ marginBottom: 16, padding: 12, background: '#fafafa', border: '1px solid #f0f0f0', borderRadius: 8 }}>
        <Space wrap size="middle" style={{ width: '100%' }}>
          <Input.Search
            allowClear
            placeholder="搜索名称/描述/版本/标签"
            style={{ width: isMobile ? '100%' : 260 }}
            value={filters.keyword}
            onChange={(e) => setFilters(prev => ({ ...prev, keyword: e.target.value }))}
          />
          <Select
            allowClear
            placeholder="生命周期"
            style={{ width: 130 }}
            value={filters.status}
            onChange={(value) => setFilters(prev => ({ ...prev, status: value }))}
            options={[
              { value: 'draft', label: '草稿' },
              { value: 'testing', label: '测试中' },
              { value: 'published', label: '已发布' },
              { value: 'disabled', label: '已停用' },
              { value: 'active', label: '已发布(旧)' },
              { value: 'archived', label: '已停用(旧)' },
            ]}
          />
          <Select
            allowClear
            placeholder="分类"
            style={{ width: 140 }}
            value={filters.category}
            onChange={(value) => setFilters(prev => ({ ...prev, category: value }))}
            options={SKILL_CATEGORIES}
          />
          <Select
            allowClear
            placeholder="输入参数"
            style={{ width: 130 }}
            value={filters.hasInputSchema}
            onChange={(value) => setFilters(prev => ({ ...prev, hasInputSchema: value }))}
            options={[
              { value: true, label: '有输入参数' },
              { value: false, label: '无输入参数' },
            ]}
          />
          <Select
            allowClear
            placeholder="触发条件"
            style={{ width: 130 }}
            value={filters.hasTriggers}
            onChange={(value) => setFilters(prev => ({ ...prev, hasTriggers: value }))}
            options={[
              { value: true, label: '有触发条件' },
              { value: false, label: '无触发条件' },
            ]}
          />
          <Select
            allowClear
            placeholder="使用情况"
            style={{ width: 120 }}
            value={filters.usageState}
            onChange={(value) => setFilters(prev => ({ ...prev, usageState: value }))}
            options={[
              { value: 'used', label: '已使用' },
              { value: 'unused', label: '未使用' },
            ]}
          />
          <Button disabled={!hasActiveFilters} onClick={() => setFilters({ keyword: '' })}>重置</Button>
          <Text type="secondary">共 {filteredSkills.length} / {skills.length} 个</Text>
        </Space>
      </div>

      <Table dataSource={filteredSkills} columns={columns} rowKey="id" loading={loading}
        size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 400 } : undefined} />

      <Modal title="Skill 治理详情" open={detailOpen} onCancel={() => setDetailOpen(false)} footer={null} width={isMobile ? '95%' : 820}>
        {detailSkill && (() => {
          const steps = parseSteps(detailSkill.steps);
          const risk = skillRisk(detailSkill);
          const inputFields = parseInputSchema(detailSkill.input_schema).fields;
          const tags = parseJsonArray<string>(detailSkill.tags, []);
          return (
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <Alert
                type={isBuiltinSkill(detailSkill) ? 'warning' : risk.color === 'green' ? 'success' : 'info'}
                showIcon
                message={isBuiltinSkill(detailSkill) ? '内置锁定 Skill' : risk.label}
                description={isBuiltinSkill(detailSkill) ? '该 Skill 由系统创建，前端禁用编辑/删除，后端 API 和助手治理工具也会强制拦截。' : risk.reason}
              />
              <Descriptions bordered size="small" column={isMobile ? 1 : 2}>
                <Descriptions.Item label="Skill ID">{detailSkill.id}</Descriptions.Item>
                <Descriptions.Item label="来源">{isBuiltinSkill(detailSkill) ? <Tag color="purple" icon={<LockOutlined />}>系统内置</Tag> : <Tag>租户自定义</Tag>}</Descriptions.Item>
                <Descriptions.Item label="名称">{detailSkill.name}</Descriptions.Item>
                <Descriptions.Item label="版本">{detailSkill.version}</Descriptions.Item>
                <Descriptions.Item label="生命周期">{renderStatusTag(detailSkill.status)}</Descriptions.Item>
                <Descriptions.Item label="风险/就绪"> <Tag color={risk.color}>{risk.label}</Tag></Descriptions.Item>
                <Descriptions.Item label="分类">{detailSkill.category || '-'}</Descriptions.Item>
                <Descriptions.Item label="标签">{tags.length ? tags.map(tag => <Tag key={tag}>{tag}</Tag>) : '-'}</Descriptions.Item>
                <Descriptions.Item label="使用次数">{detailSkill.usage_count || 0}</Descriptions.Item>
                <Descriptions.Item label="最近使用">{fmtTime(detailSkill.last_used_at)}</Descriptions.Item>
                <Descriptions.Item label="创建时间">{fmtTime(detailSkill.created_at)}</Descriptions.Item>
                <Descriptions.Item label="更新时间">{fmtTime(detailSkill.updated_at)}</Descriptions.Item>
                <Descriptions.Item label="描述" span={isMobile ? 1 : 2}>{detailSkill.description || '-'}</Descriptions.Item>
              </Descriptions>
              <Card size="small" title="执行编排">
                <Space wrap>
                  <Tag color="blue">步骤 {steps.length}</Tag>
                  <Tag color="purple">MCP 步骤 {steps.filter(step => step.type === 'mcp_tool').length}</Tag>
                  <Tag color="cyan">输入字段 {inputFields.length}</Tag>
                  <Tag color={detailSkill.triggers ? 'green' : 'default'}>{detailSkill.triggers ? '有触发条件' : '无触发条件'}</Tag>
                </Space>
                <Collapse size="small" style={{ marginTop: 12 }} items={[
                  { key: 'steps', label: '步骤 JSON', children: <pre style={{ maxHeight: 220, overflow: 'auto', margin: 0 }}>{JSON.stringify(steps, null, 2)}</pre> },
                  { key: 'input', label: '输入 Schema', children: <pre style={{ maxHeight: 180, overflow: 'auto', margin: 0 }}>{prettyJson(detailSkill.input_schema)}</pre> },
                ]} />
              </Card>
            </Space>
          );
        })()}
      </Modal>

      {/* ========== 编辑/新建技能弹窗 ========== */}
      <Modal title={editing ? '编辑技能' : '新建技能'} open={modalOpen} onOk={onOk} onCancel={() => setModalOpen(false)}
        width={isMobile ? '95%' : 700} destroyOnClose>
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message={editing ? '编辑租户自定义 Skill' : '创建租户自定义 Skill'}
          description="Skill 发布后才建议进入角色授权和正式执行；执行链仍由后端按角色、MCP 工具权限、执行模式和输入 Schema 校验。系统内置 Skill 不允许进入普通编辑。"
        />
        <Form form={form} layout="vertical" initialValues={{ version: '1.0.0', status: 'draft' }}>
          <Tabs items={[
            {
              key: 'basic',
              label: '基本信息',
              children: (
                <>
                  <Form.Item name="name" label="技能名称" rules={[{ required: true, message: '请输入名称' }]}>
                    <Input placeholder="如: 数据同步 / 报表生成 / 智能客服" />
                  </Form.Item>
                  <Form.Item name="description" label="技能描述">
                    <TextArea rows={2} placeholder="描述技能的功能和用途" />
                  </Form.Item>
                  <div style={{ display: 'flex', gap: 16 }}>
                    <Form.Item name="category" label="分类" style={{ flex: 1 }}>
                      <Select options={SKILL_CATEGORIES} allowClear placeholder="选择分类" />
                    </Form.Item>
                    <Form.Item name="version" label="版本" style={{ flex: 1 }}>
                      <Input placeholder="1.0.0" />
                    </Form.Item>
                    <Form.Item name="status" label="状态" style={{ flex: 1 }}>
                      <Select options={[
                        { value: 'draft', label: '草稿' },
                        { value: 'testing', label: '测试中' },
                        { value: 'published', label: '已发布' },
                        { value: 'disabled', label: '已停用' },
                      ]} />
                    </Form.Item>
                  </div>
                  <Form.Item name="tags_text" label="标签" tooltip="用逗号分隔多个标签">
                    <Input placeholder="如: 数据, 同步, 定时" />
                  </Form.Item>
                </>
              ),
            },
            {
              key: 'schema',
              label: '输入输出定义',
              children: (
                <>
                  <Form.Item name="input_schema_text" label="输入参数 Schema (JSON)" tooltip="定义技能执行时需要的输入参数">
                    <TextArea rows={6} placeholder={`{
  "type": "object",
  "properties": {
    "source": {
      "type": "string",
      "title": "数据源",
      "description": "数据来源地址"
    },
    "limit": {
      "type": "integer",
      "title": "数量限制",
      "default": 100
    }
  },
  "required": ["source"]
}`} style={{ fontFamily: 'monospace', fontSize: 13 }} />
                  </Form.Item>
                  <Form.Item name="output_schema_text" label="输出 Schema (JSON)" tooltip="定义技能执行后的输出格式">
                    <TextArea rows={4} placeholder={`{
  "type": "object",
  "properties": {
    "count": { "type": "integer", "title": "处理数量" },
    "status": { "type": "string", "title": "执行状态" }
  }
}`} style={{ fontFamily: 'monospace', fontSize: 13 }} />
                  </Form.Item>
                </>
              ),
            },
            {
              key: 'steps',
              label: '执行步骤',
              children: (
                <>
                  <Form.Item name="steps_text" label="执行步骤" rules={[{ required: true, message: '请添加至少一个步骤' }]}
                    tooltip="可视化编辑技能的执行步骤，支持选择MCP工具">
                    <StepEditor tenantId={currentTenant} />
                  </Form.Item>
                  <Form.Item name="triggers_text" label="触发条件 (JSON)" tooltip="自动触发规则">
                    <TextArea rows={3} placeholder='{"keywords": ["数据同步"], "schedule": "0 9 * * *"}' style={{ fontFamily: 'monospace', fontSize: 13 }} />
                  </Form.Item>
                </>
              ),
            },
          ]} />
        </Form>
      </Modal>

      {/* ========== 执行技能弹窗 ========== */}
      <Modal title={executingSkill ? `执行技能: ${executingSkill.name}` : '执行技能'} open={execModalOpen}
        onCancel={() => setExecModalOpen(false)} footer={null} width={isMobile ? '95%' : 600} destroyOnClose>
        {executingSkill && (
          <>
            {executingSkill.description && <Paragraph type="secondary" style={{ marginBottom: 16 }}>{executingSkill.description}</Paragraph>}

            {/* 输入表单 */}
            <Space style={{ marginBottom: 12 }}>
              <Text type="secondary">当前状态</Text>
              {renderStatusTag(executingSkill.status)}
            </Space>
            {normalizeStatus(executingSkill.status) !== 'published' && (
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message="未发布技能只能沙箱/预演测试；正式执行需要先发布。"
              />
            )}

            <Form form={execForm} layout="vertical">
              <Form.Item
                name="execution_mode"
                label="执行模式"
                tooltip="默认沙箱，不触发外部副作用；正式执行仅允许已发布技能。"
                rules={[{ required: true, message: '请选择执行模式' }]}
              >
                <Select options={[
                  { value: 'sandbox', label: '沙箱测试（默认，不触发外部副作用）' },
                  { value: 'dry_run', label: '预演（只校验流程，不触发外部副作用）' },
                  { value: 'production', label: '正式执行（仅已发布技能）', disabled: normalizeStatus(executingSkill.status) !== 'published' },
                ]} />
              </Form.Item>
              {(() => {
              const { fields } = parseInputSchema(executingSkill.input_schema);
              if (fields.length === 0) {
                return <Text type="secondary">该技能无需输入参数</Text>;
              }
              return fields.map(f => renderInputField(f));
              })()}
            </Form>

            <div style={{ marginTop: 16, display: 'flex', gap: 8 }}>
              <Button type="primary" icon={<PlayCircleOutlined />} loading={executing} onClick={onExecute}>
                {executing ? '执行中...' : '执行'}
              </Button>
              <Button onClick={() => setExecModalOpen(false)}>关闭</Button>
            </div>

            {/* 执行结果 */}
            {executionResult && renderExecutionResult(executionResult)}
          </>
        )}
      </Modal>

      {/* ========== 执行历史弹窗 ========== */}
      <Modal title={historySkill ? `${historySkill.name} - 执行历史` : '执行历史'} open={historyModalOpen}
        onCancel={() => setHistoryModalOpen(false)} footer={null} width={isMobile ? '95%' : 700} destroyOnClose>
        <Table dataSource={Array.isArray(executions) ? executions : []} loading={historyLoading} rowKey="id" size="small" pagination={{ pageSize: 10 }}
          locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无执行历史" /> }}
          columns={[
            { title: '模式', dataIndex: 'execution_mode', key: 'execution_mode', width: 90, render: (v: string) => renderExecutionModeTag(v) },
            { title: '状态', dataIndex: 'status', key: 'status', width: 80, render: (v: string) => (
              <Tag color={isSuccessStatus(v) ? 'green' : isFailedStatus(v) ? 'red' : 'blue'}>{v}</Tag>
            )},
            { title: '耗时', dataIndex: 'duration_ms', key: 'duration_ms', width: 80, render: (v: number, record: SkillExecution) => {
              const ms = typeof v === 'number' ? v : calcDurationMs(record);
              return ms !== undefined ? `${ms}ms` : '-';
            } },
            { title: '执行时间', dataIndex: 'started_at', key: 'started_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
            { title: '错误', dataIndex: 'error', key: 'error', ellipsis: true, render: (v: string) => v ? <Text type="danger">{v}</Text> : '-' },
          ]}
        />
      </Modal>
    </div>
  );
};

export default Skills;
