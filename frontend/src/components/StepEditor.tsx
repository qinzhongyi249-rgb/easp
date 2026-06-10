import React, { useState, useEffect, useMemo } from 'react';
import { Button, Select, Input, Space, Card, Typography, Tooltip, Tag, Collapse, App } from 'antd';
import { PlusOutlined, DeleteOutlined, ArrowUpOutlined, ArrowDownOutlined, ToolOutlined, BranchesOutlined, CodeOutlined, ApiOutlined } from '@ant-design/icons';
import { mcpToolApi } from '../api/mcpTool';
import type { MCPTool } from '../api/mcpTool';
import type { SkillStep } from '../api/skill';

const { Text } = Typography;
const { TextArea } = Input;

interface StepEditorProps {
  value?: string; // JSON string of SkillStep[]
  onChange?: (value: string) => void;
  tenantId: string;
}

const STEP_TYPE_OPTIONS = [
  { value: 'mcp_tool', label: 'MCP工具', icon: <ToolOutlined />, color: 'blue' },
  { value: 'http_request', label: 'HTTP请求', icon: <ApiOutlined />, color: 'green' },
  { value: 'condition', label: '条件判断', icon: <BranchesOutlined />, color: 'orange' },
  { value: 'assign', label: '变量赋值', icon: <CodeOutlined />, color: 'purple' },
];

const StepEditor: React.FC<StepEditorProps> = ({ value, onChange, tenantId }) => {
  const { message } = App.useApp();
  const [steps, setSteps] = useState<SkillStep[]>([]);
  const [mcpTools, setMcpTools] = useState<MCPTool[]>([]);
  const [loadingTools, setLoadingTools] = useState(false);
  const [showJson, setShowJson] = useState(false);
  const [jsonText, setJsonText] = useState('');

  // 解析外部 value
  useEffect(() => {
    if (value) {
      try {
        const parsed = JSON.parse(value);
        if (Array.isArray(parsed)) {
          setSteps(parsed);
          setJsonText(JSON.stringify(parsed, null, 2));
        }
      } catch { /* ignore */ }
    } else {
      setSteps([]);
      setJsonText('[]');
    }
  }, [value]);

  // 加载 MCP 工具列表
  useEffect(() => {
    if (!tenantId) return;
    setLoadingTools(true);
    mcpToolApi.list(tenantId)
      .then(res => {
        const tools = Array.isArray(res.data) ? res.data : [];
        setMcpTools(tools.filter(t => t.enabled));
      })
      .catch(() => {})
      .finally(() => setLoadingTools(false));
  }, [tenantId]);

  // 工具名到工具信息的映射
  const toolMap = useMemo(() => {
    const map: Record<string, MCPTool> = {};
    mcpTools.forEach(t => { map[t.name] = t; });
    return map;
  }, [mcpTools]);

  // 同步到外部
  const emitChange = (newSteps: SkillStep[]) => {
    setSteps(newSteps);
    const json = JSON.stringify(newSteps, null, 2);
    setJsonText(json);
    onChange?.(json);
  };

  // 添加步骤
  const addStep = () => {
    const newStep: SkillStep = {
      name: `步骤${steps.length + 1}`,
      type: 'mcp_tool',
      action: '',
      params: {},
      output_var: '',
    };
    emitChange([...steps, newStep]);
  };

  // 删除步骤
  const removeStep = (index: number) => {
    const newSteps = steps.filter((_, i) => i !== index);
    emitChange(newSteps);
  };

  // 上移步骤
  const moveStep = (index: number, direction: -1 | 1) => {
    const newIndex = index + direction;
    if (newIndex < 0 || newIndex >= steps.length) return;
    const newSteps = [...steps];
    [newSteps[index], newSteps[newIndex]] = [newSteps[newIndex], newSteps[index]];
    emitChange(newSteps);
  };

  // 更新步骤字段
  const updateStep = (index: number, field: keyof SkillStep, value: unknown) => {
    const newSteps = steps.map((step, i) => {
      if (i !== index) return step;
      const updated = { ...step, [field]: value };
      // 切换类型时重置 action
      if (field === 'type' && value !== 'mcp_tool') {
        updated.action = '';
      }
      return updated;
    });
    emitChange(newSteps);
  };

  // 从 JSON 编辑器同步
  const onJsonChange = (text: string) => {
    setJsonText(text);
    try {
      const parsed = JSON.parse(text);
      if (Array.isArray(parsed)) {
        setSteps(parsed);
        onChange?.(text);
      }
    } catch { /* JSON 不合法，不更新 */ }
  };

  // 获取工具的参数 Schema 字段
  const getToolParamHints = (toolName: string): string[] => {
    const tool = toolMap[toolName];
    if (!tool?.input_schema) return [];
    try {
      const schema = JSON.parse(tool.input_schema);
      return Object.keys(schema.properties || {});
    } catch { return []; }
  };

  // 渲染单个步骤
  const renderStep = (step: SkillStep, index: number) => {
    const typeOption = STEP_TYPE_OPTIONS.find(t => t.value === step.type);
    const paramHints = step.type === 'mcp_tool' && step.action ? getToolParamHints(step.action) : [];

    return (
      <Card
        key={index}
        size="small"
        style={{ marginBottom: 8, border: `1px solid ${typeOption?.color === 'blue' ? '#91caff' : typeOption?.color === 'green' ? '#b7eb8f' : typeOption?.color === 'orange' ? '#ffd591' : '#d3adf7'}` }}
        title={
          <Space size="small">
            <Tag color={typeOption?.color}>{typeOption?.icon} {typeOption?.label}</Tag>
            <Text strong style={{ fontSize: 13 }}>#{index + 1}</Text>
          </Space>
        }
        extra={
          <Space size="small">
            <Tooltip title="上移">
              <Button type="text" size="small" icon={<ArrowUpOutlined />} disabled={index === 0} onClick={() => moveStep(index, -1)} />
            </Tooltip>
            <Tooltip title="下移">
              <Button type="text" size="small" icon={<ArrowDownOutlined />} disabled={index === steps.length - 1} onClick={() => moveStep(index, 1)} />
            </Tooltip>
            <Tooltip title="删除">
              <Button type="text" size="small" danger icon={<DeleteOutlined />} onClick={() => removeStep(index)} />
            </Tooltip>
          </Space>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }} size="small">
          {/* 第一行：名称 + 类型 */}
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Input
              placeholder="步骤名称"
              value={step.name}
              onChange={e => updateStep(index, 'name', e.target.value)}
              style={{ flex: 1, minWidth: 120 }}
            />
            <Select
              value={step.type}
              onChange={v => updateStep(index, 'type', v)}
              style={{ width: 130 }}
              options={STEP_TYPE_OPTIONS.map(t => ({
                value: t.value,
                label: <Space size="small">{t.icon}{t.label}</Space>,
              }))}
            />
            <Input
              placeholder="输出变量名 (如 result)"
              value={step.output_var}
              onChange={e => updateStep(index, 'output_var', e.target.value)}
              style={{ width: 150 }}
            />
          </div>

          {/* MCP工具选择 */}
          {step.type === 'mcp_tool' && (
            <Select
              showSearch
              placeholder="选择 MCP 工具"
              value={step.action || undefined}
              onChange={v => updateStep(index, 'action', v)}
              loading={loadingTools}
              style={{ width: '100%' }}
              optionFilterProp="label"
              options={mcpTools.map(t => ({
                value: t.name,
                label: `${t.name}${t.description ? ' - ' + t.description : ''}`,
              }))}
            />
          )}

          {/* HTTP请求 URL */}
          {step.type === 'http_request' && (
            <Input
              placeholder="请求URL (如 https://api.example.com/data)"
              value={step.action}
              onChange={e => updateStep(index, 'action', e.target.value)}
            />
          )}

          {/* 条件判断 */}
          {step.type === 'condition' && (
            <Input
              placeholder="条件表达式 (如: places exists / result.status == 'ok')"
              value={step.condition}
              onChange={e => updateStep(index, 'condition', e.target.value)}
            />
          )}

          {/* 参数编辑 */}
          {step.type !== 'condition' && (
            <div>
              <Text type="secondary" style={{ fontSize: 12, marginBottom: 4, display: 'block' }}>
                参数 (JSON)
                {paramHints.length > 0 && (
                  <span style={{ marginLeft: 8 }}>
                    可用字段: {paramHints.map(h => (
                      <Tag key={h} style={{ fontSize: 11, cursor: 'pointer' }} onClick={() => {
                        const current = step.params || {};
                        emitChange(steps.map((s, i) => i === index ? { ...s, params: { ...current, [h]: `\${inputs.${h}}` } } : s));
                      }}>{h}</Tag>
                    ))}
                  </span>
                )}
              </Text>
              <TextArea
                rows={2}
                placeholder={step.type === 'mcp_tool' ? '{"key": "${inputs.value}"}' : step.type === 'assign' ? '{"key": "value"}' : '{"url": "..."}'}
                value={typeof step.params === 'string' ? step.params : JSON.stringify(step.params || {}, null, 2)}
                onChange={e => {
                  try {
                    const parsed = JSON.parse(e.target.value);
                    updateStep(index, 'params', parsed);
                  } catch {
                    updateStep(index, 'params', e.target.value);
                  }
                }}
                style={{ fontFamily: 'monospace', fontSize: 12 }}
              />
            </div>
          )}

          {/* 条件分支跳转 */}
          {step.type === 'condition' && (
            <div style={{ display: 'flex', gap: 8 }}>
              <Input
                placeholder="条件成立 → 步骤名"
                value={step.next_on_ok}
                onChange={e => updateStep(index, 'next_on_ok', e.target.value)}
                style={{ flex: 1 }}
              />
              <Input
                placeholder="条件不成立 → 步骤名"
                value={step.next_on_fail}
                onChange={e => updateStep(index, 'next_on_fail', e.target.value)}
                style={{ flex: 1 }}
              />
            </div>
          )}

          {/* 工具描述提示 */}
          {step.type === 'mcp_tool' && step.action && toolMap[step.action]?.description && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              💡 {toolMap[step.action].description}
            </Text>
          )}
        </Space>
      </Card>
    );
  };

  return (
    <div>
      {/* 步骤列表 */}
      {steps.map((step, index) => renderStep(step, index))}

      {/* 添加按钮 */}
      <Button
        type="dashed"
        block
        icon={<PlusOutlined />}
        onClick={addStep}
        style={{ marginTop: 8 }}
      >
        添加步骤
      </Button>

      {/* JSON 预览/编辑切换 */}
      <div style={{ marginTop: 12, textAlign: 'right' }}>
        <Button type="link" size="small" onClick={() => setShowJson(!showJson)}>
          {showJson ? '收起JSON' : '查看JSON'}
        </Button>
      </div>

      {showJson && (
        <TextArea
          rows={6}
          value={jsonText}
          onChange={e => onJsonChange(e.target.value)}
          style={{ fontFamily: 'monospace', fontSize: 12 }}
        />
      )}
    </div>
  );
};

export default StepEditor;
