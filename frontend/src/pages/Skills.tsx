import React, { useState, useEffect } from 'react';
import {
  Table, Button, Modal, Form, Input, Space, Typography, Popconfirm, Tag, App,
  Drawer, Descriptions, Spin, Empty, Collapse
} from 'antd';
import {
  PlusOutlined, EditOutlined, DeleteOutlined, PlayCircleOutlined,
  CheckCircleOutlined, CloseCircleOutlined,
  ClockCircleOutlined
} from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { Skill } from '../api/skill';
import { skillApi } from '../api/skill';

const { Title, Text, Paragraph } = Typography;
const { TextArea } = Input;
interface LayoutContext { currentTenant: string; }

interface StepResult {
  step_name: string;
  status: string;
  outputs: Record<string, unknown>;
  error?: string;
  duration_ms: number;
}

interface ExecutionResult {
  id: string;
  skill_id: string;
  status: string;
  inputs: Record<string, unknown>;
  outputs: Record<string, unknown>;
  step_results: StepResult[];
  started_at: string;
  ended_at?: string;
  error?: string;
}

const Skills: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [data, setData] = useState<Skill[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Skill | null>(null);
  const [form] = Form.useForm();
  const [currentForm] = Form.useForm();

  // 执行相关状态
  const [execDrawerOpen, setExecDrawerOpen] = useState(false);
  const [executingSkill, setExecutingSkill] = useState<Skill | null>(null);
  const [executing, setExecuting] = useState(false);
  const [execResult, setExecResult] = useState<ExecutionResult | null>(null);

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try { setData((await skillApi.list(currentTenant)).data || []); }
    catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const onOk = async () => {
    const values = await form.validateFields();
    try {
      if (editing) { await skillApi.update(currentTenant, editing.id, values); message.success('更新成功'); }
      else { await skillApi.create(currentTenant, values); message.success('创建成功'); }
      setModalOpen(false); form.resetFields(); setEditing(null); load();
    } catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '操作失败'); }
  };

  // 打开执行面板
  const onOpenExecute = (skill: Skill) => {
    setExecutingSkill(skill);
    setExecResult(null);
    setExecuting(false);
    currentForm.resetFields();
    setExecDrawerOpen(true);
  };

  // 执行技能
  const onExecute = async () => {
    if (!executingSkill) return;
    setExecuting(true);
    setExecResult(null);
    try {
      const inputs = currentForm.getFieldsValue();
      const resp = await skillApi.execute(currentTenant, executingSkill.id, inputs);
      setExecResult(resp.data as ExecutionResult);
      if ((resp.data as ExecutionResult).status === 'completed') {
        message.success('执行成功');
      } else {
        message.error('执行失败: ' + ((resp.data as ExecutionResult).error || '未知错误'));
      }
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '执行失败');
    } finally {
      setExecuting(false);
    }
  };

  // 解析步骤定义，提取需要的输入参数
  const parseSkillInputs = (skill: Skill): { name: string; label: string; required: boolean }[] => {
    try {
      const steps = JSON.parse(skill.steps || '[]');
      const inputs: { name: string; label: string; required: boolean }[] = [];
      for (const step of steps) {
        if (step.params) {
          for (const [, val] of Object.entries(step.params)) {
            if (typeof val === 'string' && val.startsWith('{{') && val.endsWith('}}')) {
              const varName = val.slice(2, -2);
              if (!inputs.find(i => i.name === varName)) {
                inputs.push({ name: varName, label: varName, required: true });
              }
            }
          }
        }
      }
      return inputs;
    } catch { return []; }
  };

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name', render: (v: string) => <Text strong>{v}</Text> },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: '步骤数', key: 'steps', width: 80, render: (_: unknown, r: Skill) => {
      try { return <Tag>{JSON.parse(r.steps || '[]').length} 步</Tag>; }
      catch { return '-'; }
    }},
    { title: '状态', dataIndex: 'status', key: 'status', width: 80, render: (v: string) => <Tag color={v === 'active' ? 'green' : 'default'}>{v || 'active'}</Tag> },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 160, render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
    { title: '操作', key: 'action', width: 200, render: (_: unknown, record: Skill) => (
      <Space>
        <Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={() => onOpenExecute(record)}>测试执行</Button>
        <Button size="small" icon={<EditOutlined />} onClick={() => { setEditing(record); form.setFieldsValue(record); setModalOpen(true); }}>编辑</Button>
        <Popconfirm title="确认删除?" onConfirm={() => { skillApi.delete(currentTenant, record.id).then(load); }}>
          <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
        </Popconfirm>
      </Space>
    )},
  ];

  // 渲染执行结果
  const renderExecResult = () => {
    if (!execResult) return <Empty description="点击「执行」查看结果" />;

    const statusIcon = execResult.status === 'completed'
      ? <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 20 }} />
      : <CloseCircleOutlined style={{ color: '#ff4d4f', fontSize: 20 }} />;

    const totalMs = execResult.ended_at && execResult.started_at
      ? new Date(execResult.ended_at).getTime() - new Date(execResult.started_at).getTime()
      : 0;

    return (
      <div>
        {/* 执行概览 */}
        <Descriptions size="small" column={2} style={{ marginBottom: 16 }}>
          <Descriptions.Item label="状态">
            <Space>{statusIcon} <Tag color={execResult.status === 'completed' ? 'green' : 'red'}>{execResult.status}</Tag></Space>
          </Descriptions.Item>
          <Descriptions.Item label="总耗时"><Tag>{totalMs}ms</Tag></Descriptions.Item>
          <Descriptions.Item label="执行ID"><Text copyable={{ text: execResult.id }} style={{ fontSize: 11 }}>{execResult.id.slice(0, 12)}...</Text></Descriptions.Item>
          <Descriptions.Item label="时间">{new Date(execResult.started_at).toLocaleString()}</Descriptions.Item>
        </Descriptions>

        {/* 错误信息 */}
        {execResult.error && (
          <div style={{ background: '#fff2f0', border: '1px solid #ffccc7', borderRadius: 6, padding: 12, marginBottom: 16 }}>
            <Text type="danger">{execResult.error}</Text>
          </div>
        )}

        {/* 步骤执行详情 */}
        {execResult.step_results && execResult.step_results.length > 0 && (
          <Collapse
            defaultActiveKey={execResult.step_results.map((_, i) => String(i))}
            items={execResult.step_results.map((step, i) => ({
              key: String(i) as string,
              label: (
                <Space>
                  {step.status === 'completed'
                    ? <CheckCircleOutlined style={{ color: '#52c41a' }} />
                    : step.status === 'skipped'
                    ? <ClockCircleOutlined style={{ color: '#999' }} />
                    : <CloseCircleOutlined style={{ color: '#ff4d4f' }} />}
                  <Text strong>{step.step_name}</Text>
                  <Tag style={{ marginLeft: 4 }}>{step.duration_ms}ms</Tag>
                </Space>
              ),
              children: (
                <div>
                  {step.error && <Text type="danger" style={{ display: 'block', marginBottom: 8 }}>错误: {step.error}</Text>}
                  <pre style={{
                    background: '#f5f5f5', padding: 12, borderRadius: 6,
                    fontSize: 12, maxHeight: 300, overflow: 'auto', margin: 0,
                  }}>
                    {JSON.stringify(step.outputs, null, 2)}
                  </pre>
                </div>
              ),
            }))}
          />
        )}

        {/* 完整输出 */}
        <Collapse style={{ marginTop: 16 }}>
          <Collapse.Panel header="完整输出 (JSON)" key="outputs">
            <pre style={{
              background: '#f5f5f5', padding: 12, borderRadius: 6,
              fontSize: 12, maxHeight: 400, overflow: 'auto',
            }}>
              {JSON.stringify(execResult.outputs, null, 2)}
            </pre>
          </Collapse.Panel>
        </Collapse>
      </div>
    );
  };

  const inputFields = executingSkill ? parseSkillInputs(executingSkill) : [];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={3}>技能管理</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); setModalOpen(true); }}>新建技能</Button>
      </div>
      <Table dataSource={data} columns={columns} rowKey="id" loading={loading} />

      {/* 编辑弹窗 */}
      <Modal title={editing ? '编辑技能' : '新建技能'} open={modalOpen} onOk={onOk} onCancel={() => setModalOpen(false)} width={640}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="description" label="描述"><Input /></Form.Item>
          <Form.Item name="steps" label="步骤定义 (JSON)"><TextArea rows={6} placeholder='[{"type":"mcp_tool","action":"list_users","name":"list_users"}]' /></Form.Item>
          <Form.Item name="triggers" label="触发器 (JSON)"><TextArea rows={3} placeholder='["manual","schedule"]' /></Form.Item>
        </Form>
      </Modal>

      {/* 执行抽屉 */}
      <Drawer
        title={
          <Space>
            <PlayCircleOutlined />
            <span>测试执行: {executingSkill?.name}</span>
          </Space>
        }
        open={execDrawerOpen}
        onClose={() => setExecDrawerOpen(false)}
        width={640}
        extra={
          <Button type="primary" icon={<PlayCircleOutlined />} onClick={onExecute} loading={executing}>
            执行
          </Button>
        }
      >
        {/* 技能信息 */}
        {executingSkill && (
          <div style={{ marginBottom: 16 }}>
            <Paragraph type="secondary">{executingSkill.description}</Paragraph>
          </div>
        )}

        {/* 输入参数 */}
        {inputFields.length > 0 && (
          <div style={{ marginBottom: 16 }}>
            <Title level={5}>输入参数</Title>
            <Form form={currentForm} layout="vertical">
              {inputFields.map(f => (
                <Form.Item key={f.name} name={f.name} label={f.label} rules={f.required ? [{ required: true }] : []}>
                  <Input placeholder={`请输入 ${f.label}`} />
                </Form.Item>
              ))}
            </Form>
          </div>
        )}

        {/* 执行状态 */}
        {executing && (
          <div style={{ textAlign: 'center', padding: '40px 0' }}>
            <Spin size="large" />
            <div style={{ marginTop: 16 }}><Text type="secondary">正在执行...</Text></div>
          </div>
        )}

        {/* 执行结果 */}
        {renderExecResult()}
      </Drawer>
    </div>
  );
};

export default Skills;
