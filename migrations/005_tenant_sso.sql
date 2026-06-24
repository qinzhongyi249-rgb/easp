-- 租户SSO配置表
CREATE TABLE IF NOT EXISTS tenant_sso_configs (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    enabled BOOLEAN DEFAULT FALSE,
    
    -- 业务系统登录接口配置
    login_url VARCHAR(500) NOT NULL COMMENT '业务系统登录接口地址',
    login_method VARCHAR(10) DEFAULT 'POST' COMMENT 'HTTP方法',
    login_headers TEXT COMMENT '自定义请求头JSON',
    login_body_template TEXT COMMENT '请求体模板JSON，支持 {{username}} {{password}} 占位符',
    
    -- 业务系统用户信息接口配置（可选）
    user_info_url VARCHAR(500) COMMENT '获取用户信息接口地址',
    user_info_method VARCHAR(10) DEFAULT 'GET',
    user_info_headers TEXT COMMENT '自定义请求头JSON',
    
    -- 响应映射配置
    response_mapping TEXT COMMENT '响应字段映射JSON',
    
    -- EASP回调配置
    callback_url VARCHAR(500) COMMENT 'EASP回调地址',
    
    -- 同步配置
    sync_user_on_login BOOLEAN DEFAULT TRUE COMMENT '登录时同步用户信息到业务系统',
    sync_url VARCHAR(500) COMMENT '同步用户信息的接口地址',
    sync_method VARCHAR(10) DEFAULT 'POST',
    sync_headers TEXT COMMENT '同步请求头JSON',

    -- 用户开通策略
    auto_create_user TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否允许SSO首次登录自动创建EASP用户',
    default_role_ids JSON NULL COMMENT 'SSO自动创建用户默认角色ID数组',
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    UNIQUE KEY uk_tenant_sso (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
