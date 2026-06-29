import React from 'react';
import { Button, Card, Row, Col, Space, Divider } from 'antd';
import { CheckCircle, Zap, Shield, Users, Code, BarChart } from 'lucide-react';
import './Home.css';

const Home: React.FC = () => {

  const features = [
    {
      title: 'API 一键转 MCP',
      description: '导入 OpenAPI 文档自动生成 MCP 工具，无需手动编码，企业现有系统秒级接入 AI 生态',
      icon: <Zap size={32} color="#1677ff" />
    },
    {
      title: '企业级权限管控',
      description: '多租户架构 + RBAC/ABAC 权限模型，细粒度控制工具访问，满足金融、政企安全合规要求',
      icon: <Shield size={32} color="#1677ff" />
    },
    {
      title: '开箱即用 SSO 集成',
      description: '支持企业微信、飞书、钉钉、OIDC 等多种身份源，一键完成用户单点登录接入',
      icon: <Users size={32} color="#1677ff" />
    },
    {
      title: '长效工作记忆池',
      description: '基于向量检索的智能记忆引擎，AI 记住用户偏好和业务上下文，对话更精准',
      icon: <BarChart size={32} color="#1677ff" />
    },
    {
      title: 'Skill 技能市场',
      description: '预置丰富行业技能，支持自定义技能开发，灵活扩展 AI 能力边界',
      icon: <Code size={32} color="#1677ff" />
    },
    {
      title: '全链路审计日志',
      description: '每一次工具调用、每一句对话完整记录，满足等保合规审计要求',
      icon: <CheckCircle size={32} color="#1677ff" />
    }
  ];

  const pricingPlans = [
    {
      name: '体验版',
      price: '0',
      description: '适合个人开发者和小型团队试用',
      features: [
        '1 个租户',
        '最多 10 个用户',
        '最多 20 个 MCP 工具',
        '社区版支持',
        '基础权限管控'
      ],
      buttonText: '免费体验',
      type: 'default'
    },
    {
      name: '企业版',
      price: '咨询',
      description: '适合中大型企业生产部署',
      features: [
        '无限租户',
        '无限用户',
        '无限 MCP 工具',
        '7×24 技术支持',
        '完整权限管控',
        'SSO 集成',
        '定制化开发',
        '专属部署方案'
      ],
      buttonText: '联系我们',
      type: 'primary',
      highlighted: true
    },
    {
      name: '私有部署',
      price: '定制',
      description: '完全私有化部署，数据自主可控',
      features: [
        '部署在企业内部机房',
        '数据完全隔离',
        '深度定制集成',
        '长期技术维保',
        '专属技术服务团队'
      ],
      buttonText: '咨询方案',
      type: 'default'
    }
  ];

  const services = [
    {
      title: '产品授权',
      description: '提供企业级软件授权，支持按年订阅和永久许可两种模式'
    },
    {
      title: '实施服务',
      description: '专业工程师上门/远程实施，协助企业完成系统部署和业务集成'
    },
    {
      title: '定制开发',
      description: '根据企业个性化需求，定制开发特定连接器、技能和功能模块'
    },
    {
      title: '技术培训',
      description: '为开发和运维团队提供专业技术培训，快速掌握产品使用和二次开发'
    },
    {
      title: '运维维保',
      description: '持续的版本升级和技术支持，保障系统稳定运行'
    },
    {
      title: '咨询规划',
      description: '帮助企业规划 AI 集成路线，设计合理架构方案'
    }
  ];

  return (
    <div className="home-page">
      {/* Hero 区域 */}
      <section className="hero-section">
        <div className="container">
          <Row gutter={[32, 32]} align="middle">
            <Col xs={24} md={14}>
              <div className="hero-content">
                <h1 className="hero-title">
                  北京金砥科技
                </h1>
                <h2 className="hero-subtitle">
                  EASP — 企业级 API 转 MCP 智能网关
                </h2>
                <p className="hero-description">
                  让企业现有系统一键接入 AI 大模型生态，释放 API 价值，赋能智能应用
                </p>
                <Space size="large" className="hero-buttons">
                  <Button
                    type="primary"
                    size="large"
                    onClick={() => window.location.href = '/login'}
                  >
                    立即登录体验
                  </Button>
                  <Button
                    size="large"
                    onClick={() => {
                      const contact = document.getElementById('contact');
                      contact?.scrollIntoView({ behavior: 'smooth' });
                    }}
                  >
                    联系我们
                  </Button>
                </Space>
              </div>
            </Col>
            <Col xs={24} md={10}>
              <div className="hero-image">
                <div className="hero-card-1">
                  <div className="badge">OpenAPI</div>
                  <div className="arrow">→</div>
                  <div className="badge">MCP</div>
                </div>
              </div>
            </Col>
          </Row>
        </div>
      </section>

      {/* 产品特色 */}
      <section className="features-section" id="features">
        <div className="container">
          <div className="section-header">
            <h2 className="section-title">核心特色</h2>
            <p className="section-description">专为企业 API 智能化接入设计的完整解决方案</p>
          </div>
          <Row gutter={[32, 32]}>
            {features.map((feature, index) => (
              <Col xs={24} sm={12} lg={8} key={index}>
                <Card className="feature-card" hoverable>
                  <div className="feature-icon">{feature.icon}</div>
                  <h3 className="feature-title">{feature.title}</h3>
                  <p className="feature-description">{feature.description}</p>
                </Card>
              </Col>
            ))}
          </Row>
        </div>
      </section>

      {/* 价格方案 */}
      <section className="pricing-section" id="pricing">
        <div className="container">
          <div className="section-header">
            <h2 className="section-title">价格方案</h2>
            <p className="section-description">灵活选择适合您企业的方案</p>
          </div>
          <Row gutter={[32, 32]} justify="center">
            {pricingPlans.map((plan, index) => (
              <Col xs={24} sm={20} md={8} key={index}>
                <Card
                  className={`pricing-card ${plan.highlighted ? 'highlighted' : ''}`}
                  hoverable
                >
                  <div className="pricing-header">
                    <h3 className="pricing-name">{plan.name}</h3>
                    <div className="pricing-price">
                      {plan.price === '0' ? (
                        <>¥<span className="price-number">0</span></>
                      ) : (
                        <span className="price-text">{plan.price}</span>
                      )}
                    </div>
                    <p className="pricing-description">{plan.description}</p>
                  </div>
                  <Divider />
                  <ul className="pricing-features">
                    {plan.features.map((feature, i) => (
                      <li key={i}>
                        <CheckCircle size={18} color="#52c41a" />
                        <span>{feature}</span>
                      </li>
                    ))}
                  </ul>
                  <div className="pricing-button">
                    <Button
                      type={plan.type === 'primary' ? 'primary' : 'default'}
                      size="large"
                      block
                      onClick={() => {
                        const contact = document.getElementById('contact');
                        contact?.scrollIntoView({ behavior: 'smooth' });
                      }}
                    >
                      {plan.buttonText}
                    </Button>
                  </div>
                </Card>
              </Col>
            ))}
          </Row>
        </div>
      </section>

      {/* 服务内容 */}
      <section className="services-section" id="services">
        <div className="container">
          <div className="section-header">
            <h2 className="section-title">我们的服务</h2>
            <p className="section-description">从授权到实施，全流程专业服务支持</p>
          </div>
          <Row gutter={[32, 32]}>
            {services.map((service, index) => (
              <Col xs={24} sm={12} lg={8} key={index}>
                <Card className="service-card" hoverable>
                  <h3 className="service-title">{service.title}</h3>
                  <p className="service-description">{service.description}</p>
                </Card>
              </Col>
            ))}
          </Row>
        </div>
      </section>

      {/* 关于我们 */}
      <section className="about-section" id="contact">
        <div className="container">
          <div className="section-header">
            <h2 className="section-title">联系我们</h2>
            <p className="section-description">北京金砥科技有限公司 — 专注企业 AI 集成基础设施</p>
          </div>
          <Row gutter={[32, 32]} justify="center">
            <Col xs={24} md={16}>
              <Card className="contact-card">
                <h3>商务咨询</h3>
                <p>欢迎企业客户咨询合作，我们将为您提供专业的解决方案和服务支持。</p>
                <p>邮箱：249312560@qq.com</p>
                <p>手机：15846554846</p>
                <p>地址：北京市</p>
              </Card>
            </Col>
          </Row>
        </div>
      </section>

      {/* Footer */}
      <footer className="footer">
        <div className="container">
          <div className="footer-content">
            <p>© 2026 北京金砥科技有限公司 版权所有</p>
            <p>
              <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer">
                京ICP备2026038568号-1
              </a>
            </p>
          </div>
        </div>
      </footer>
    </div>
  );
};

export default Home;
