import React, { useState, useEffect, useCallback } from 'react';
import { Table, Button, Modal, Form, Input, InputNumber, Space, Typography, App, Tag, Dropdown, Select, Tabs, Descriptions, Timeline, Card, Collapse, Popconfirm, Empty } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, BulbOutlined, PlayCircleOutlined, MoreOutlined, HistoryOutlined, CheckCircleOutlined, CloseCircleOutlined, LoadingOutlined, ClockCircleOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { Skill, SkillExecution, StepResult, JsonSchema } from '../api/skill';
import { skillApi, SKILL_CATEGORIES } from '../api/skill';
import StepEditor from '../components/StepEditor';

const { Title, Text, Paragraph } = Typography;
const { TextArea } = Input;
interface LayoutContext { currentTenant: string; }

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
  }, [form]);

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

  const onDelete = async (id: string) => {
    try { await skillApi.delete(currentTenant, id); message.success('删除成功'); load(); }
    catch { message.error('删除失败'); }
  };

  // ========== 技能执行 ==========
  const openExecModal = (skill: Skill) => {
    setExecutingSkill(skill);
    setExecutionResult(null);
    execForm.resetFields();
    // 根据 input_schema 设置默认值
    const { fields } = parseInputSchema(skill.input_schema);
    const defaults: Record<string, unknown> = {};
    fields.forEach(f => { if (f.defaultValue !== undefined) defaults[f.name] = f.defaultValue; });
    execForm.setFieldsValue(defaults);
    setExecModalOpen(true);
  };

  const onExecute = async () => {
    if (!executingSkill) return;
    const values = await execForm.validateFields();
    setExecuting(true);
    setExecutionResult(null);
    try {
      const res = await skillApi.execute(currentTenant, executingSkill.id, values);
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
  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name', render: (v: string, r: Skill) => (
      <Space>
        <span>{v}</span>
        {r.category && <Tag>{SKILL_CATEGORIES.find(c => c.value === r.category)?.label || r.category}</Tag>}
      </Space>
    )},
    ...(!isMobile ? [
      { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
      { title: '版本', dataIndex: 'version', key: 'version', width: 70 },
      { title: '使用次数', dataIndex: 'usage_count', key: 'usage_count', width: 80 },
    ] : []),
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => {
      const colorMap: Record<string, string> = { active: 'green', draft: 'blue', archived: 'default' };
      return <Tag color={colorMap[v] || 'default'}>{v}</Tag>;
    }},
    { title: '操作', key: 'action', width: isMobile ? 60 : 280, render: (_: unknown, record: Skill) => (
      isMobile ? (
        <Dropdown menu={{ items: [
          { key: 'execute', label: '执行', icon: <PlayCircleOutlined />, onClick: () => openExecModal(record) },
          { key: 'history', label: '历史', icon: <HistoryOutlined />, onClick: () => openHistory(record) },
          { key: 'edit', label: '编辑', icon: <EditOutlined />, onClick: () => openEditModal(record) },
          { key: 'delete', label: '删除', icon: <DeleteOutlined />, danger: true, onClick: () => onDelete(record.id) },
        ]}} trigger={['click']}>
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ) : (
        <Space>
          <Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={() => openExecModal(record)}>执行</Button>
          <Button size="small" icon={<HistoryOutlined />} onClick={() => openHistory(record)}>历史</Button>
          <Button size="small" icon={<EditOutlined />} onClick={() => openEditModal(record)}>编辑</Button>
          <Popconfirm title="确认删除?" onConfirm={() => onDelete(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
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
        <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><BulbOutlined /> 技能管理</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openEditModal()}>新建技能</Button>
      </div>

      <Table dataSource={skills} columns={columns} rowKey="id" loading={loading}
        size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 400 } : undefined} />

      {/* ========== 编辑/新建技能弹窗 ========== */}
      <Modal title={editing ? '编辑技能' : '新建技能'} open={modalOpen} onOk={onOk} onCancel={() => setModalOpen(false)}
        width={isMobile ? '95%' : 700} destroyOnClose>
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
                        { value: 'active', label: '启用' },
                        { value: 'archived', label: '归档' },
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
            {(() => {
              const { fields } = parseInputSchema(executingSkill.input_schema);
              if (fields.length === 0) {
                return <Text type="secondary">该技能无需输入参数</Text>;
              }
              return (
                <Form form={execForm} layout="vertical">
                  {fields.map(f => renderInputField(f))}
                </Form>
              );
            })()}

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
