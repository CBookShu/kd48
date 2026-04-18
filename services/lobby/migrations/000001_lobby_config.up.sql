-- +migrate Up
CREATE TABLE `lobby_config_revision` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
    `config_name` VARCHAR(64) NOT NULL COMMENT '配置名称',
    `revision` BIGINT NOT NULL COMMENT '版本号',
    `scope` VARCHAR(64) NOT NULL COMMENT '作用域',
    `title` VARCHAR(256) NULL COMMENT '标题',
    `tags` JSON NULL COMMENT '标签',
    `start_time` DATETIME(3) NULL COMMENT '生效开始时间',
    `end_time` DATETIME(3) NULL COMMENT '生效结束时间',
    `csv_text` MEDIUMTEXT NOT NULL COMMENT 'CSV格式配置内容',
    `json_payload` JSON NOT NULL COMMENT 'JSON格式配置内容',
    `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_config_revision` (`config_name`, `revision`),
    KEY `idx_config_latest` (`config_name`, `revision` DESC),
    KEY `idx_scope_config_rev` (`scope`, `config_name`, `revision` DESC),
    KEY `idx_scope_time` (`scope`, `start_time`, `end_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Lobby配置版本表';
