import React, { useState, useEffect } from 'react';
import { Alert, App, Button, Card, Collapse, Descriptions, Form, Input, Select, Space, Steps, Switch, Tag, Typography } from 'antd';
import { ApiOutlined, CheckCircleOutlined, KeyOutlined, LinkOutlined, SaveOutlined, SettingOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import client from '../api/client';

const { Title, Text, Paragraph } = Typography;
const { TextArea } = Input;

interface LayoutContext { currentTenant: string; }
interface RoleOption { id: string; name: string; }
interface ProviderField {
  name: string;
  label: string;
  component: 'input' | 'textarea' | 'select';
  placeholder?: string;
  required?: boolean;
  advanced?: boolean;
  help?: string;
}
interface ProviderTemplate {
  key: string;
  name: string;
  description: string;
  badge: string;
  recommended?: boolean;
  values: Record<string, unknown>;
  fields: ProviderField[];
  docs: string[];
}

const SSOConfigPage: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(false);
  const [roles, setRoles] = useState<RoleOption[]>([]);
  const [templates, setTemplates] = useState<ProviderTemplate[]>([]);
  const [selectedTemplateKey, setSelectedTemplateKey] = useState('');
  const [form] = Form.useForm();
  const isMobile = window.innerWidth < 768;

  const selectedTemplate = templates.find(item => item.key === selectedTemplateKey) || templates[0];
  const basicFields = selectedTemplate?.fields.filter(field => !field.advanced) || [];
  const advancedFields = selectedTemplate?.fields.filter(field => field.advanced) || [];

  const normalizeConfig = (raw: Record<string, unknown>) => {
    const data = { ...raw };
    if (typeof data.default_role_ids === 'string') {
      try { data.default_role_ids = JSON.parse(data.default_role_ids as string); } catch { data.default_role_ids = []; }
    }
    return data;
  };

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try {
      const [templateRes, configRes, rolesRes] = await Promise.all([
        client.get(`/tenants/${currentTenant}/sso/templates`),
        client.get(`/tenants/${currentTenant}/sso/config`),
        client.get(`/tenants/${currentTenant}/roles`),
      ]);
      const serverTemplates = templateRes.data?.templates || [];
      setTemplates(serverTemplates);
      setSelectedTemplateKey((prev) => prev || serverTemplates[0]?.key || '');
      form.setFieldsValue(normalizeConfig(configRes.data || {}));
      setRoles(rolesRes.data?.tenant_roles || []);
    } catch (err) {
      message.error('加载身份源配置失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const applyTemplate = (tpl: ProviderTemplate) => {
    setSelectedTemplateKey(tpl.key);
    form.setFieldsValue({
      ...tpl.values,
      enabled: form.getFieldValue('enabled') ?? false,
      auto_create_user: form.getFieldValue('auto_create_user') ?? false,
      default_role_ids: form.getFieldValue('default_role_ids') || [],
    });
    message.success(`已选择${tpl.name}模板`);
  };

  const onSave = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await client.put(`/tenants/${currentTenant}/sso/config`, values);
      message.success('保存成功');
      load();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const copyLink = async () => {
    const link = `${window.location.origin}/sso/${currentTenant}`;
    try {
      await navigator.clipboard.writeText(link);
      message.success('链接已复制');
    } catch {
      message.warning('复制失败，请手动复制登录链接');
    }
  };

  const renderField = (field: ProviderField) => {
    const rules = field.required ? [{ required: true, message: `请输入${field.label}` }] : undefined;
    const commonProps = { placeholder: field.placeholder };
    return <Form.Item key={field.name} name={field.name} label={field.label} rules={rules} tooltip={field.help}>
      {field.component === 'textarea' ? <TextArea rows={field.advanced ? 3 : 2} {...commonProps} /> :
        field.component === 'select' ? <Select options={[{ value: 'GET', label: 'GET' }, { value: 'POST', label: 'POST' }, { value: 'PUT', label: 'PUT' }]} /> :
          <Input {...commonProps} />}
    </Form.Item>;
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: isMobile ? 'flex-start' : 'center', marginBottom: 16, flexDirection: isMobile ? 'column' : 'row', gap: 12 }}>
        <div>
          <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><KeyOutlined /> 身份源配置</Title>
          <Text type="secondary">用于员工登录 EASP 控制台；业务系统嵌入 AI 助手请走“应用接入”。</Text>
        </div>
        <Space wrap>
          <Button icon={<ApiOutlined />} href="/docs/sso.html" target="_blank">查看配置说明</Button>
          <Button icon={<LinkOutlined />} onClick={copyLink}>复制登录链接</Button>
          <Button type="primary" icon={<SaveOutlined />} onClick={onSave} loading={saving}>保存</Button>
        </Space>
      </div>

      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message="身份源配置是辅助能力：让员工用企业微信/飞书/钉钉/OIDC 登录 EASP 控制台。"
          description="业务系统内嵌 AI 助手不依赖这里配置，请使用“用户管理 → 应用接入”的 app_id/app_secret 换短期 token。"
        />

        <Card>
          <Steps
            size="small"
            current={selectedTemplateKey ? 1 : 0}
            direction={isMobile ? 'vertical' : 'horizontal'}
            items={[
              { title: '选择身份源', description: '企业微信/飞书/钉钉/OIDC' },
              { title: '填写最少字段', description: '登录地址和用户信息地址' },
              { title: '高级字段可选', description: '请求头、模板、字段映射' },
              { title: '复制登录链接', description: `/sso/${currentTenant}` },
            ]}
          />
        </Card>

        <Card title="选择身份源模板" loading={loading}>
          <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(4, minmax(0, 1fr))', gap: 12 }}>
            {templates.map(tpl => (
              <Card
                key={tpl.key}
                size="small"
                hoverable
                onClick={() => applyTemplate(tpl)}
                style={{ height: '100%', borderColor: selectedTemplateKey === tpl.key ? '#1677ff' : undefined, background: selectedTemplateKey === tpl.key ? '#f0f7ff' : undefined }}
              >
                <Space direction="vertical" size="small" style={{ width: '100%' }}>
                  <Space wrap><Text strong>{tpl.name}</Text><Tag color={tpl.recommended ? 'green' : 'blue'}>{tpl.badge}</Tag></Space>
                  <Text type="secondary">{tpl.description}</Text>
                  {selectedTemplateKey === tpl.key && <Tag icon={<CheckCircleOutlined />} color="processing">当前模板</Tag>}
                  <Button size="small" type={selectedTemplateKey === tpl.key ? 'primary' : 'default'} onClick={(event) => { event.stopPropagation(); applyTemplate(tpl); }}>
                    {selectedTemplateKey === tpl.key ? '已选择' : '使用此模板'}
                  </Button>
                </Space>
              </Card>
            ))}
          </div>
        </Card>

        <Form form={form} layout="vertical" size={isMobile ? 'middle' : 'large'}>
          <Card title="基础设置" extra={<Text type="secondary">先保证登录链路可用</Text>}>
            <Form.Item name="enabled" label="启用身份源登录" valuePropName="checked">
              <Switch checkedChildren="启用" unCheckedChildren="停用" />
            </Form.Item>
            <Form.Item name="auto_create_user" label="未开通用户自动创建" valuePropName="checked" tooltip="关闭后，第三方登录成功但 EASP 没有该用户时，将拒绝签发 Token 并提示联系管理员。">
              <Switch checkedChildren="自动创建" unCheckedChildren="需预开通" />
            </Form.Item>
            <Form.Item name="default_role_ids" label="自动创建默认角色" tooltip="仅自动创建用户时生效；只允许选择当前租户角色，不包含系统级角色。未选择时回退为普通用户。">
              <Select mode="multiple" allowClear placeholder="默认回退为普通用户" options={roles.map(role => ({ value: role.id, label: role.name }))} />
            </Form.Item>
          </Card>

          <Card title="模板字段" style={{ marginTop: 16 }}>
            <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(2, minmax(0, 1fr))', gap: 16 }}>
              {basicFields.map(renderField)}
            </div>
            {selectedTemplate && <Alert type="success" showIcon message={`${selectedTemplate.name} 配置步骤`} description={<ol style={{ marginBottom: 0, paddingLeft: 18 }}>{selectedTemplate.docs.map((item, index) => <li key={index}>{item}</li>)}</ol>} />}
          </Card>

          <Collapse
            style={{ marginTop: 16 }}
            items={[{
              key: 'advanced',
              label: <Space><SettingOutlined />高级 OAuth/接口字段</Space>,
              children: <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(2, minmax(0, 1fr))', gap: 16 }}>{advancedFields.map(renderField)}</div>,
            }]}
          />
        </Form>

        <Card title="登录链接">
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label="控制台身份源登录"><Text copyable>{`${window.location.origin}/sso/${currentTenant}`}</Text></Descriptions.Item>
          </Descriptions>
          <Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0 }}>该链接用于员工登录 EASP 控制台；业务系统前端嵌入助手请使用应用接入生成的短期 token。</Paragraph>
        </Card>
      </Space>
    </div>
  );
};

export default SSOConfigPage;
